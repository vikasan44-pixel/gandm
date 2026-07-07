package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

const maxMessageBodyLen = 4000

// requireChatParticipant is the chat access gate: only rows in
// chat_participants grant access. A chat the caller isn't part of is
// reported as not-found — same don't-confirm-existence policy as routes.
func (s *CargoService) requireChatParticipant(ctx context.Context, chatID, userID uuid.UUID) error {
	chatRepo := repository.NewChatRepository(s.db)
	isParticipant, err := chatRepo.IsParticipant(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !isParticipant {
		return repository.ErrNotFound
	}
	return nil
}

func (s *CargoService) ListMyChats(ctx context.Context, userID uuid.UUID) ([]repository.ChatView, error) {
	chatRepo := repository.NewChatRepository(s.db)
	return chatRepo.ListByUserID(ctx, userID)
}

// ListChatMessages returns the chat's messages, optionally only those after
// the given cursor. The cursor accepts either a message id (uuid) or an
// RFC3339 timestamp, per the brief.
func (s *CargoService) ListChatMessages(ctx context.Context, userID, chatID uuid.UUID, after string) ([]models.Message, error) {
	if err := s.requireChatParticipant(ctx, chatID, userID); err != nil {
		return nil, err
	}

	var afterTime *time.Time
	if after != "" {
		if msgID, err := uuid.Parse(after); err == nil {
			msgRepo := repository.NewMessageRepository(s.db)
			msg, err := msgRepo.GetByID(ctx, msgID)
			if err != nil {
				return nil, fmt.Errorf("%w: unknown after message id", ErrInvalidInput)
			}
			if msg.ChatID != chatID {
				return nil, fmt.Errorf("%w: after message belongs to another chat", ErrInvalidInput)
			}
			afterTime = &msg.CreatedAt
		} else if ts, err := time.Parse(time.RFC3339Nano, after); err == nil {
			afterTime = &ts
		} else {
			return nil, fmt.Errorf("%w: after must be a message id or RFC3339 timestamp", ErrInvalidInput)
		}
	}

	msgRepo := repository.NewMessageRepository(s.db)
	return msgRepo.ListByChatID(ctx, chatID, afterTime)
}

func (s *CargoService) SendChatMessage(ctx context.Context, userID, chatID uuid.UUID, body, attachmentURL string) (*models.Message, error) {
	if _, err := s.requireEligibleUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireChatParticipant(ctx, chatID, userID); err != nil {
		return nil, err
	}

	body = strings.TrimSpace(body)
	attachmentURL = strings.TrimSpace(attachmentURL)
	if body == "" && attachmentURL == "" {
		return nil, fmt.Errorf("%w: message body or attachment_url is required", ErrInvalidInput)
	}
	if len(body) > maxMessageBodyLen {
		return nil, fmt.Errorf("%w: message body exceeds %d characters", ErrInvalidInput, maxMessageBodyLen)
	}

	msg := &models.Message{
		ID:        uuid.New(),
		ChatID:    chatID,
		SenderID:  userID,
		Body:      body,
		CreatedAt: time.Now(),
	}
	if attachmentURL != "" {
		msg.AttachmentURL = &attachmentURL
	}

	msgRepo := repository.NewMessageRepository(s.db)
	if err := msgRepo.Create(ctx, msg); err != nil {
		return nil, err
	}
	return msg, nil
}
