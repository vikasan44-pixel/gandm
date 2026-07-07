package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

var ErrRouteExists = repository.ErrRouteExists

// requireActiveUser gates the self-service routes endpoints: the brief says
// "Доступно active-участнику" — stricter than the pending/active rule used
// for cargo submission and document upload. Admins can still manage routes
// for any user through the admin endpoints.
func (s *CargoService) requireActiveUser(ctx context.Context, userID uuid.UUID) error {
	userRepo := repository.NewUserRepository(s.db)
	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.Status != models.UserStatusActive {
		return ErrAccountNotEligible
	}
	return nil
}

func validateRoutePoints(origin, destination models.GeoPoint) (models.GeoPoint, models.GeoPoint, error) {
	origin, err := validateGeoPoint("origin", origin)
	if err != nil {
		return origin, destination, err
	}
	destination, err = validateGeoPoint("destination", destination)
	if err != nil {
		return origin, destination, err
	}
	return origin, destination, nil
}

func (s *CargoService) ListMyRoutes(ctx context.Context, userID uuid.UUID) ([]models.ParticipantRoute, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	routeRepo := repository.NewParticipantRouteRepository(s.db)
	return routeRepo.ListByUserID(ctx, userID)
}

func (s *CargoService) AddMyRoute(ctx context.Context, userID uuid.UUID, origin, destination models.GeoPoint) (*models.ParticipantRoute, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	origin, destination, err := validateRoutePoints(origin, destination)
	if err != nil {
		return nil, err
	}

	route := &models.ParticipantRoute{
		ID:          uuid.New(),
		UserID:      userID,
		Origin:      origin,
		Destination: destination,
		CreatedAt:   time.Now(),
	}

	routeRepo := repository.NewParticipantRouteRepository(s.db)
	if err := routeRepo.Create(ctx, route); err != nil {
		return nil, err
	}
	return route, nil
}

// DeleteMyRoute deletes only the caller's own route. A route belonging to
// someone else is reported as not-found rather than forbidden, so the
// endpoint doesn't confirm that a guessed route id exists.
func (s *CargoService) DeleteMyRoute(ctx context.Context, userID, routeID uuid.UUID) error {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return err
	}

	routeRepo := repository.NewParticipantRouteRepository(s.db)
	route, err := routeRepo.GetByID(ctx, routeID)
	if err != nil {
		return err
	}
	if route.UserID != userID {
		return repository.ErrNotFound
	}
	return routeRepo.Delete(ctx, routeID)
}
