package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

func (s *AdminService) ListUserRoutes(ctx context.Context, userID uuid.UUID) ([]models.ParticipantRoute, error) {
	userRepo := repository.NewUserRepository(s.db)
	if _, err := userRepo.GetByID(ctx, userID); err != nil {
		return nil, err
	}
	routeRepo := repository.NewParticipantRouteRepository(s.db)
	return routeRepo.ListByUserID(ctx, userID)
}

func (s *AdminService) AddUserRoute(ctx context.Context, adminID, userID uuid.UUID, origin, destination models.GeoPoint) (*models.ParticipantRoute, error) {
	origin, destination, err := validateRoutePoints(origin, destination)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	userRepo := repository.NewUserRepository(tx)
	if _, err := userRepo.GetByID(ctx, userID); err != nil {
		return nil, err
	}

	route := &models.ParticipantRoute{
		ID:          uuid.New(),
		UserID:      userID,
		Origin:      origin,
		Destination: destination,
		CreatedAt:   time.Now(),
	}

	routeRepo := repository.NewParticipantRouteRepository(tx)
	if err := routeRepo.Create(ctx, route); err != nil {
		return nil, err
	}

	details := map[string]any{
		"route_id":          route.ID,
		"origin_label":      origin.Label,
		"destination_label": destination.Label,
		"origin":            []float64{origin.Lat, origin.Lng},
		"destination":       []float64{destination.Lat, destination.Lng},
	}
	if err := writeAuditLog(ctx, tx, adminID, "user_route_added", &userID, details); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return route, nil
}

// DeleteUserRoute deletes a route only if it belongs to the user named in
// the URL — a routeId under the wrong user is not-found, keeping the nested
// route consistent with what it claims to address.
func (s *AdminService) DeleteUserRoute(ctx context.Context, adminID, userID, routeID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	routeRepo := repository.NewParticipantRouteRepository(tx)
	route, err := routeRepo.GetByID(ctx, routeID)
	if err != nil {
		return err
	}
	if route.UserID != userID {
		return repository.ErrNotFound
	}
	if err := routeRepo.Delete(ctx, routeID); err != nil {
		return err
	}

	details := map[string]any{
		"route_id":          routeID,
		"origin_label":      route.Origin.Label,
		"destination_label": route.Destination.Label,
	}
	if err := writeAuditLog(ctx, tx, adminID, "user_route_deleted", &userID, details); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
