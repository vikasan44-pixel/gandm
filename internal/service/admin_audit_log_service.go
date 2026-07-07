package service

import (
	"context"

	"gandm/internal/repository"
)

func (s *AdminService) ListAuditLog(ctx context.Context, limit, offset int) ([]repository.AuditLogEntry, error) {
	auditRepo := repository.NewAuditLogRepository(s.db)
	return auditRepo.List(ctx, limit, offset)
}
