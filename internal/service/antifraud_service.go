package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"gandm/internal/models"
	"gandm/internal/repository"
	"gandm/internal/storage"
)

var ErrNoDealWithPartner = errors.New("favorites are limited to counterparties of completed deals")

// AntifraudService — «Избранное + Документы» (ТЗ §6.2) и детект
// подозрительных паттернов (§6.1). Отдельный сервис, потому что документам
// сделок нужен S3, которого у CargoService нет намеренно.
type AntifraudService struct {
	db      *pgxpool.Pool
	storage *storage.S3Client
}

func NewAntifraudService(db *pgxpool.Pool, storage *storage.S3Client) *AntifraudService {
	return &AntifraudService{db: db, storage: storage}
}

// --- избранное ---

// requireEligible: заблокированный/отклонённый аккаунт не читает и не
// пишет бизнес-данные (та же политика, что requireEligibleUser в
// CargoService).
func (s *AntifraudService) requireEligible(ctx context.Context, userID uuid.UUID) error {
	user, err := repository.NewUserRepository(s.db).GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !isEligibleStatus(user.Status) {
		return ErrAccountNotEligible
	}
	return nil
}

// AddFavorite: в избранное можно добавить только контрагента по
// завершённой сделке — иначе избранное превратилось бы в канал деанона
// анонимных предложений.
func (s *AntifraudService) AddFavorite(ctx context.Context, clientID, participantID uuid.UUID) error {
	if err := s.requireEligible(ctx, clientID); err != nil {
		return err
	}
	if clientID == participantID {
		return fmt.Errorf("%w: cannot favorite yourself", ErrInvalidInput)
	}
	ratingRepo := repository.NewRatingRepository(s.db)
	if _, err := ratingRepo.FindDealBetween(ctx, clientID, participantID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNoDealWithPartner
		}
		return err
	}
	return repository.NewAntifraudRepository(s.db).AddFavorite(ctx, clientID, participantID)
}

func (s *AntifraudService) RemoveFavorite(ctx context.Context, clientID, participantID uuid.UUID) error {
	if err := s.requireEligible(ctx, clientID); err != nil {
		return err
	}
	return repository.NewAntifraudRepository(s.db).RemoveFavorite(ctx, clientID, participantID)
}

func (s *AntifraudService) ListFavorites(ctx context.Context, clientID uuid.UUID) ([]repository.FavoriteEntry, error) {
	if err := s.requireEligible(ctx, clientID); err != nil {
		return nil, err
	}
	return repository.NewAntifraudRepository(s.db).ListFavorites(ctx, clientID)
}

// --- документы сделок ---

const maxDealDocumentSize = 15 << 20 // 15 MB, как у документов регистрации

var allowedDealDocContentTypes = map[string]bool{
	"application/pdf": true,
	"image/jpeg":      true,
	"image/png":       true,
}

// DealDocumentView — документ с временной ссылкой на просмотр.
type DealDocumentView struct {
	repository.DealDocument
	ViewURL string `json:"view_url"`
}

const dealDocumentURLTTL = 15 * time.Minute

// requireDealParty: uploader must be a counterparty of the completed deal.
func (s *AntifraudService) requireDealParty(ctx context.Context, dealID, userID uuid.UUID) error {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM cargo_requests cr
			JOIN contact_reveals rev ON rev.cargo_request_id = cr.id
			WHERE cr.id = $1 AND cr.status = 'matched' AND (cr.client_id = $2 OR rev.participant_id = $2)
			UNION
			SELECT 1 FROM consolidated_requests cons
			JOIN consolidated_selections sel ON sel.consolidated_request_id = cons.id
			JOIN offers o ON o.id = sel.offer_id
			WHERE cons.id = $1 AND cons.status = 'matched' AND (sel.client_id = $2 OR o.participant_id = $2)
		)
	`
	var ok bool
	if err := s.db.QueryRow(ctx, q, dealID, userID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return repository.ErrNotFound
	}
	return nil
}

// UploadDealDocument stores the confirming contract for a deal (ТЗ §6.2).
// Content type is sniffed, never trusted from the client.
func (s *AntifraudService) UploadDealDocument(ctx context.Context, userID, dealID uuid.UUID, fileHeader *multipart.FileHeader) (*DealDocumentView, error) {
	if err := s.requireEligible(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireDealParty(ctx, dealID, userID); err != nil {
		return nil, err
	}
	if fileHeader.Size > maxDealDocumentSize {
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
	if !allowedDealDocContentTypes[contentType] {
		return nil, ErrUnsupportedFile
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("deal-docs/%s/%s_%s", dealID, uuid.New(), sanitizeFilename(fileHeader.Filename))
	if err := s.storage.Upload(ctx, key, file, fileHeader.Size, contentType); err != nil {
		return nil, err
	}

	doc := &repository.DealDocument{
		ID:           uuid.New(),
		DealID:       dealID,
		UploaderID:   userID,
		FileURL:      key,
		OriginalName: fileHeader.Filename,
		UploadedAt:   time.Now(),
	}
	if err := repository.NewAntifraudRepository(s.db).CreateDealDocument(ctx, doc); err != nil {
		return nil, cleanupUploadedObject(ctx, s.storage, key, err)
	}

	view := &DealDocumentView{DealDocument: *doc}
	if url, err := s.storage.PresignedGetURL(ctx, key, dealDocumentURLTTL); err == nil {
		view.ViewURL = url
	}
	return view, nil
}

func (s *AntifraudService) ListDealDocuments(ctx context.Context, userID, dealID uuid.UUID) ([]DealDocumentView, error) {
	if err := s.requireEligible(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireDealParty(ctx, dealID, userID); err != nil {
		return nil, err
	}
	docs, err := repository.NewAntifraudRepository(s.db).ListDealDocuments(ctx, dealID)
	if err != nil {
		return nil, err
	}
	views := make([]DealDocumentView, 0, len(docs))
	for _, d := range docs {
		view := DealDocumentView{DealDocument: d}
		if url, err := s.storage.PresignedGetURL(ctx, d.FileURL, dealDocumentURLTTL); err == nil {
			view.ViewURL = url
		}
		views = append(views, view)
	}
	return views, nil
}

// --- подозрительные паттерны (админ) ---

const suspiciousMinDeals = 2

func (s *AntifraudService) ListSuspiciousPairs(ctx context.Context) ([]repository.SuspiciousPair, error) {
	return repository.NewAntifraudRepository(s.db).ListSuspiciousPairs(ctx, suspiciousMinDeals)
}

// NotifyRepeatDeal — «при повторной работе с тем же партнёром система
// запрашивает договор» (ТЗ §6.2): после закрытия сделки, если у пары это
// уже не первая сделка, обе стороны получают просьбу загрузить документ.
// Ошибки не роняют сделку — уведомление вторично.
func NotifyRepeatDeal(ctx context.Context, q repository.Querier, dealID, clientID, participantID uuid.UUID) {
	antifraudRepo := repository.NewAntifraudRepository(q)
	count, err := antifraudRepo.CountDealsBetween(ctx, clientID, participantID)
	if err != nil || count < 2 {
		return
	}

	payload, err := json.Marshal(map[string]any{"deal_id": dealID})
	if err != nil {
		return
	}
	notifRepo := repository.NewNotificationRepository(q)
	now := time.Now()
	for _, uid := range []uuid.UUID{clientID, participantID} {
		// Best-effort notification, but log failures instead of dropping them
		// silently so a broken notification pipeline is visible.
		if err := notifRepo.Create(ctx, &models.Notification{
			ID:        uuid.New(),
			UserID:    uid,
			Type:      "repeat_deal_document_requested",
			Payload:   payload,
			IsRead:    false,
			CreatedAt: now,
		}); err != nil {
			log.Printf("repeat-deal notification for %s: %v", uid, err)
		}
	}
}
