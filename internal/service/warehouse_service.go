package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"gandm/internal/models"
	"gandm/internal/repository"
	"gandm/internal/storage"
)

const ToolSubmitFillReport = "submit_fill_report"
const fillReportURLTTL = 15 * time.Minute

var allowedPhotoContentTypes = map[string]bool{"image/jpeg": true, "image/png": true}

type WarehouseService struct {
	db      *pgxpool.Pool
	storage *storage.S3Client
}

func NewWarehouseService(db *pgxpool.Pool, s3 *storage.S3Client) *WarehouseService {
	return &WarehouseService{db: db, storage: s3}
}

type FillReportView struct {
	models.WarehouseFillReport
	PhotoViewURL *string `json:"photo_view_url,omitempty"`
}

func (s *WarehouseService) withPhotoURL(ctx context.Context, fr models.WarehouseFillReport) FillReportView {
	view := FillReportView{WarehouseFillReport: fr}
	if fr.PhotoURL != nil {
		if url, err := s.storage.PresignedGetURL(ctx, *fr.PhotoURL, fillReportURLTTL); err == nil {
			view.PhotoViewURL = &url
		}
	}
	return view
}

type CreateFillReportInput struct {
	ExpectedFillPercent float64
	ActualFillPercent   float64
	ReportDate          time.Time
	Photo               *multipart.FileHeader
}

func (s *WarehouseService) requireEligible(ctx context.Context, userID uuid.UUID) error {
	user, err := repository.NewUserRepository(s.db).GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !isEligibleStatus(user.Status) {
		return ErrAccountNotEligible
	}
	return nil
}

func (s *WarehouseService) CreateFillReport(ctx context.Context, userID uuid.UUID, in CreateFillReportInput) (*FillReportView, error) {
	user, err := repository.NewUserRepository(s.db).GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Status != models.UserStatusActive {
		return nil, ErrAccountNotEligible
	}
	hasTool, err := repository.NewToolRepository(s.db).UserHasTool(ctx, userID, ToolSubmitFillReport)
	if err != nil {
		return nil, err
	}
	if !hasTool {
		return nil, ErrForbiddenTool
	}
	if in.ExpectedFillPercent < 0 || in.ExpectedFillPercent > 100 {
		return nil, fmt.Errorf("%w: expected_fill_percent must be between 0 and 100", ErrInvalidInput)
	}
	if in.ActualFillPercent < 0 || in.ActualFillPercent > 100 {
		return nil, fmt.Errorf("%w: actual_fill_percent must be between 0 and 100", ErrInvalidInput)
	}
	if in.ReportDate.IsZero() {
		return nil, fmt.Errorf("%w: report_date is required (YYYY-MM-DD)", ErrInvalidInput)
	}
	report := &models.WarehouseFillReport{ID: uuid.New(), UserID: userID, ExpectedFillPercent: in.ExpectedFillPercent, ActualFillPercent: in.ActualFillPercent, ReportDate: in.ReportDate, CreatedAt: time.Now()}
	if in.Photo != nil {
		key, err := s.uploadPhoto(ctx, userID, in.Photo)
		if err != nil {
			return nil, err
		}
		report.PhotoURL = &key
	}
	if err := repository.NewFillReportRepository(s.db).Create(ctx, report); err != nil {
		if report.PhotoURL != nil {
			return nil, cleanupUploadedObject(ctx, s.storage, *report.PhotoURL, err)
		}
		return nil, err
	}
	view := s.withPhotoURL(ctx, *report)
	return &view, nil
}

func (s *WarehouseService) uploadPhoto(ctx context.Context, userID uuid.UUID, header *multipart.FileHeader) (string, error) {
	if header.Size > maxDocumentSize {
		return "", ErrFileTooLarge
	}
	file, err := header.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	sniff := make([]byte, 512)
	n, err := file.Read(sniff)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	contentType := http.DetectContentType(sniff[:n])
	if !allowedPhotoContentTypes[contentType] {
		return "", ErrUnsupportedFile
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	key := fmt.Sprintf("fill-reports/%s/%s_%s", userID, uuid.New(), sanitizeFilename(header.Filename))
	if err := s.storage.Upload(ctx, key, file, header.Size, contentType); err != nil {
		return "", err
	}
	return key, nil
}

func (s *WarehouseService) ListMyFillReports(ctx context.Context, userID uuid.UUID) ([]FillReportView, error) {
	if err := s.requireEligible(ctx, userID); err != nil {
		return nil, err
	}
	reports, err := repository.NewFillReportRepository(s.db).ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	views := make([]FillReportView, 0, len(reports))
	for _, fr := range reports {
		views = append(views, s.withPhotoURL(ctx, fr))
	}
	return views, nil
}

func (s *WarehouseService) GetLatestFillReport(ctx context.Context, userID uuid.UUID) (*FillReportView, error) {
	report, err := repository.NewFillReportRepository(s.db).LatestByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	view := s.withPhotoURL(ctx, *report)
	return &view, nil
}

func (s *AdminService) ListUserFillReports(ctx context.Context, userID uuid.UUID) ([]FillReportView, error) {
	reports, err := repository.NewFillReportRepository(s.db).ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	views := make([]FillReportView, 0, len(reports))
	for _, fr := range reports {
		view := FillReportView{WarehouseFillReport: fr}
		if fr.PhotoURL != nil {
			if url, err := s.storage.PresignedGetURL(ctx, *fr.PhotoURL, fillReportURLTTL); err == nil {
				view.PhotoViewURL = &url
			}
		}
		views = append(views, view)
	}
	return views, nil
}
