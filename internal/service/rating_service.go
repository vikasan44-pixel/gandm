package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

var (
	ErrAlreadyRated = repository.ErrAlreadyRated
	// ErrNoDealBetween: rating is allowed only between counterparties of a
	// COMPLETED deal (matched single cargo or matched consolidated).
	ErrNoDealBetween = errors.New("no completed deal between these users")
)

const maxRatingCommentLen = 1000

type CreateRatingInput struct {
	RatedUserID uuid.UUID
	Score       int
	Comment     string
	DealID      *uuid.UUID
}

// CreateRating: both sides of a completed deal may rate each other, once
// per deal. If deal_id is omitted, the service resolves some completed deal
// between the pair itself — the UNIQUE(rated, rater, deal) constraint would
// be meaningless with NULL deal ids.
func (s *CargoService) CreateRating(ctx context.Context, raterID uuid.UUID, in CreateRatingInput) (*models.Rating, error) {
	if in.Score < 1 || in.Score > 5 {
		return nil, fmt.Errorf("%w: score must be between 1 and 5", ErrInvalidInput)
	}
	in.Comment = strings.TrimSpace(in.Comment)
	if len(in.Comment) > maxRatingCommentLen {
		return nil, fmt.Errorf("%w: comment exceeds %d characters", ErrInvalidInput, maxRatingCommentLen)
	}
	if in.RatedUserID == raterID {
		return nil, fmt.Errorf("%w: cannot rate yourself", ErrInvalidInput)
	}

	if _, err := s.requireEligibleUser(ctx, raterID); err != nil {
		return nil, err
	}

	ratingRepo := repository.NewRatingRepository(s.db)

	var dealID uuid.UUID
	if in.DealID != nil {
		ok, err := ratingRepo.IsDealBetween(ctx, *in.DealID, raterID, in.RatedUserID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrNoDealBetween
		}
		dealID = *in.DealID
	} else {
		found, err := ratingRepo.FindDealBetween(ctx, raterID, in.RatedUserID)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNoDealBetween
		}
		if err != nil {
			return nil, err
		}
		dealID = found
	}

	rating := &models.Rating{
		ID:          uuid.New(),
		DealID:      &dealID,
		RatedUserID: in.RatedUserID,
		RaterUserID: raterID,
		Score:       in.Score,
		CreatedAt:   time.Now(),
	}
	if in.Comment != "" {
		rating.Comment = &in.Comment
	}

	if err := ratingRepo.Create(ctx, rating); err != nil {
		return nil, err
	}
	return rating, nil
}

// GetUserRating is the public (any authenticated user) rating breakdown:
// composite score (ТЗ §8) plus the raw components, so participants see
// what the number is made of. The old average/count fields keep their JSON
// names for compatibility.
func (s *CargoService) GetUserRating(ctx context.Context, userID uuid.UUID) (RatingBreakdown, error) {
	userRepo := repository.NewUserRepository(s.db)
	if _, err := userRepo.GetByID(ctx, userID); err != nil {
		return RatingBreakdown{}, err
	}
	components, err := repository.NewRatingRepository(s.db).ComponentsForUsers(ctx, []uuid.UUID{userID})
	if err != nil {
		return RatingBreakdown{}, err
	}
	return breakdownFromComponents(components[userID]), nil
}

func (s *CargoService) ListMyReceivedRatings(ctx context.Context, userID uuid.UUID) ([]models.Rating, error) {
	if _, err := s.requireEligibleUser(ctx, userID); err != nil {
		return nil, err
	}
	ratingRepo := repository.NewRatingRepository(s.db)
	return ratingRepo.ListReceived(ctx, userID)
}
