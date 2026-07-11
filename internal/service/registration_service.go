package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"gandm/internal/auth"
	"gandm/internal/models"
	"gandm/internal/repository"
	"gandm/internal/storage"
)

var (
	ErrEmailTaken         = repository.ErrEmailTaken
	ErrUserNotFound       = repository.ErrNotFound
	ErrAccountNotEligible = errors.New("account status does not allow this action")
	ErrInvalidInput       = errors.New("invalid input")
	ErrUnsupportedFile    = errors.New("unsupported file type")
	ErrFileTooLarge       = errors.New("file too large")
)

var allowedUserStatuses = map[models.UserStatus]bool{
	models.UserStatusPending:  true,
	models.UserStatusActive:   true,
	models.UserStatusBlocked:  true,
	models.UserStatusRejected: true,
}

var allowedDocumentTypes = map[models.DocumentType]bool{
	models.DocumentIDCard:             true,
	models.DocumentFoundingDocs:       true,
	models.DocumentBusinessLicense:    true,
	models.DocumentEmploymentContract: true,
	models.DocumentVehicleDoc:         true,
}

var allowedDocumentContentTypes = map[string]bool{
	"application/pdf": true,
	"image/jpeg":      true,
	"image/png":       true,
}

const maxDocumentSize = 15 << 20 // 15 MB

type RegisterInput struct {
	Email       string
	Phone       string
	CompanyName string
	Password    string
	// ToolIDs — инструменты, которые участник выбрал себе при регистрации
	// (вместо роли). Разрешены только участнические (self-selectable). Роль
	// как понятие убрана — доступ определяют инструменты.
	ToolIDs []uuid.UUID
}

type RegistrationService struct {
	db      *pgxpool.Pool
	tokens  *auth.Manager
	storage *storage.S3Client
}

func NewRegistrationService(db *pgxpool.Pool, tokens *auth.Manager, storage *storage.S3Client) *RegistrationService {
	return &RegistrationService{db: db, tokens: tokens, storage: storage}
}

// Register creates the user account (status=pending) and its verification
// request in a single transaction, then issues a token pair so the caller can
// immediately upload documents to their own account. The token grants no
// tools/permissions by itself.
func (s *RegistrationService) Register(ctx context.Context, in RegisterInput) (*models.User, auth.TokenPair, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Phone = strings.TrimSpace(in.Phone)
	in.CompanyName = strings.TrimSpace(in.CompanyName)

	if err := validateRegisterInput(in); err != nil {
		return nil, auth.TokenPair{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, auth.TokenPair{}, err
	}

	now := time.Now()
	user := &models.User{
		ID:          uuid.New(),
		Email:       in.Email,
		Phone:       in.Phone,
		CompanyName: in.CompanyName,
		// participant_type — legacy-колонка (роли больше нет); заполняем
		// нейтральным значением, дальше доступ определяют инструменты.
		ParticipantType: models.ParticipantClient,
		PasswordHash:    string(passwordHash),
		Status:          models.UserStatusPending,
		HasSubscription: false,
		Language:        "ru",
		CreatedAt:       now,
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, auth.TokenPair{}, err
	}
	defer tx.Rollback(ctx)

	userRepo := repository.NewUserRepository(tx)
	verRepo := repository.NewVerificationRepository(tx)
	toolRepo := repository.NewToolRepository(tx)

	if err := userRepo.Create(ctx, user); err != nil {
		return nil, auth.TokenPair{}, err
	}

	// Выбранные при регистрации инструменты: только участнические.
	if len(in.ToolIDs) > 0 {
		allowed, err := toolRepo.SelfSelectableIDSet(ctx)
		if err != nil {
			return nil, auth.TokenPair{}, err
		}
		for _, id := range in.ToolIDs {
			if !allowed[id] {
				return nil, auth.TokenPair{}, fmt.Errorf("%w: tool is not self-selectable", ErrInvalidInput)
			}
		}
		if err := toolRepo.ReplaceUserTools(ctx, user.ID, in.ToolIDs); err != nil {
			return nil, auth.TokenPair{}, err
		}
	}

	verification := &models.VerificationRequest{
		ID:        uuid.New(),
		UserID:    user.ID,
		Status:    models.VerificationPending,
		CreatedAt: now,
	}
	if err := verRepo.Create(ctx, verification); err != nil {
		return nil, auth.TokenPair{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, auth.TokenPair{}, err
	}

	tokens, err := s.tokens.IssueTokenPair(user.ID, auth.SubjectUser)
	if err != nil {
		return nil, auth.TokenPair{}, err
	}

	return user, tokens, nil
}

func validateRegisterInput(in RegisterInput) error {
	if in.Email == "" || !strings.Contains(in.Email, "@") {
		return fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if in.Phone == "" {
		return fmt.Errorf("%w: phone is required", ErrInvalidInput)
	}
	if in.CompanyName == "" {
		return fmt.Errorf("%w: company_name is required", ErrInvalidInput)
	}
	if len(in.Password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	return nil
}

// UploadDocument stores one document for the given user. The document type is
// validated against the fixed enum, and the file content is sniffed (not
// trusted from the client-supplied Content-Type header) against a whitelist.
func (s *RegistrationService) UploadDocument(ctx context.Context, userID uuid.UUID, docType models.DocumentType, fileHeader *multipart.FileHeader) (*models.Document, error) {
	userRepo := repository.NewUserRepository(s.db)
	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Status != models.UserStatusPending && user.Status != models.UserStatusActive {
		return nil, ErrAccountNotEligible
	}

	if !allowedDocumentTypes[docType] {
		return nil, fmt.Errorf("%w: unknown document type", ErrInvalidInput)
	}
	if fileHeader.Size > maxDocumentSize {
		return nil, ErrFileTooLarge
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sniff := make([]byte, 512)
	n, err := file.Read(sniff)
	if err != nil && err != io.EOF {
		return nil, err
	}
	contentType := http.DetectContentType(sniff[:n])
	if !allowedDocumentContentTypes[contentType] {
		return nil, ErrUnsupportedFile
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("documents/%s/%s/%s_%s", userID, docType, uuid.New(), sanitizeFilename(fileHeader.Filename))

	if err := s.storage.Upload(ctx, key, file, fileHeader.Size, contentType); err != nil {
		return nil, err
	}

	doc := &models.Document{
		ID:           uuid.New(),
		UserID:       userID,
		Type:         docType,
		FileURL:      key,
		OriginalName: fileHeader.Filename,
		UploadedAt:   time.Now(),
	}

	docRepo := repository.NewDocumentRepository(s.db)
	if err := docRepo.Create(ctx, doc); err != nil {
		return nil, err
	}

	return doc, nil
}

// Login authenticates a participant with the same JWT machinery as admin
// login, issuing SubjectUser tokens. Account status is deliberately NOT
// checked here: blocked/rejected users may still log in to see their status
// and reject reason via /me — every actual action is already gated by
// status/tool checks in the services.
func (s *RegistrationService) Login(ctx context.Context, email, password string) (*models.User, auth.TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	userRepo := repository.NewUserRepository(s.db)
	user, err := userRepo.GetByEmail(ctx, email)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, auth.TokenPair{}, ErrInvalidCredentials
	}
	if err != nil {
		return nil, auth.TokenPair{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, auth.TokenPair{}, ErrInvalidCredentials
	}

	tokens, err := s.tokens.IssueTokenPair(user.ID, auth.SubjectUser)
	if err != nil {
		return nil, auth.TokenPair{}, err
	}
	return user, tokens, nil
}

// Refresh exchanges a valid refresh token for a fresh token pair.
// Заблокированный/отклонённый аккаунт refresh НЕ получает: тихая вечная
// сессия по refresh-токену недопустима. Login для blocked остаётся (явное
// действие, чтобы посмотреть статус через /me), но бизнес-данные и там
// закрыты статус-проверками сервисов.
func (s *RegistrationService) Refresh(ctx context.Context, refreshToken string) (auth.TokenPair, error) {
	userID, err := s.tokens.ParseRefreshToken(refreshToken, auth.SubjectUser)
	if err != nil {
		return auth.TokenPair{}, ErrInvalidCredentials
	}
	userRepo := repository.NewUserRepository(s.db)
	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return auth.TokenPair{}, ErrInvalidCredentials
		}
		return auth.TokenPair{}, err
	}
	if !isEligibleStatus(user.Status) {
		return auth.TokenPair{}, ErrInvalidCredentials
	}
	return s.tokens.IssueTokenPair(userID, auth.SubjectUser)
}

func (s *RegistrationService) GetMe(ctx context.Context, userID uuid.UUID) (*models.User, *models.VerificationRequest, error) {
	userRepo := repository.NewUserRepository(s.db)
	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	// У сотрудников компании (ТЗ §13.1) верификационной заявки нет —
	// компания уже проверена; /me отдаёт verification = null.
	verRepo := repository.NewVerificationRepository(s.db)
	verification, err := verRepo.GetLatestByUserID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return user, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	return user, verification, nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	if name == "." || name == "/" || name == "" {
		return "file"
	}
	return name
}
