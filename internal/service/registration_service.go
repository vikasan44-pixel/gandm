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

var allowedParticipantTypes = map[models.ParticipantType]bool{
	models.ParticipantClient:     true,
	models.ParticipantWarehouse:  true,
	models.ParticipantCarrier:    true,
	models.ParticipantDriver:     true,
	models.ParticipantBroker:     true,
	models.ParticipantCustomsRep: true,
}

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
	Email           string
	Phone           string
	CompanyName     string
	ParticipantType models.ParticipantType
	Password        string
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
		ID:              uuid.New(),
		Email:           in.Email,
		Phone:           in.Phone,
		CompanyName:     in.CompanyName,
		ParticipantType: in.ParticipantType,
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

	if err := userRepo.Create(ctx, user); err != nil {
		return nil, auth.TokenPair{}, err
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
	if !allowedParticipantTypes[in.ParticipantType] {
		return fmt.Errorf("%w: unknown participant_type", ErrInvalidInput)
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

// Refresh exchanges a valid refresh token for a fresh token pair. The
// account must still exist; status is deliberately not checked, mirroring
// Login — actual actions are gated by status/tool checks in the services.
func (s *RegistrationService) Refresh(ctx context.Context, refreshToken string) (auth.TokenPair, error) {
	userID, err := s.tokens.ParseRefreshToken(refreshToken, auth.SubjectUser)
	if err != nil {
		return auth.TokenPair{}, ErrInvalidCredentials
	}
	userRepo := repository.NewUserRepository(s.db)
	if _, err := userRepo.GetByID(ctx, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return auth.TokenPair{}, ErrInvalidCredentials
		}
		return auth.TokenPair{}, err
	}
	return s.tokens.IssueTokenPair(userID, auth.SubjectUser)
}

func (s *RegistrationService) GetMe(ctx context.Context, userID uuid.UUID) (*models.User, *models.VerificationRequest, error) {
	userRepo := repository.NewUserRepository(s.db)
	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	verRepo := repository.NewVerificationRepository(s.db)
	verification, err := verRepo.GetLatestByUserID(ctx, userID)
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
