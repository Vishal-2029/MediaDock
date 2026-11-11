package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/repository"
)

// UpdateSettingsRequest represents user preference updates.
type UpdateSettingsRequest struct {
	Theme         string `json:"theme" validate:"omitempty,oneof=light dark system"`
	DefaultFormat string `json:"default_format" validate:"omitempty,oneof=mp4 mov mkv webm mp3 aac m4a wav"`
}

// Service manages user profile and settings operations.
type Service struct {
	repo repository.UserRepository
}

// NewService constructs a users Service.
func NewService(repo repository.UserRepository) *Service {
	return &Service{repo: repo}
}

// GetSettings retrieves settings for userID, creating defaults if missing.
func (s *Service) GetSettings(ctx context.Context, userID uuid.UUID) (*repository.UserSettings, error) {
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			settings = &repository.UserSettings{
				ID:            uuid.New(),
				UserID:        userID,
				Theme:         "system",
				DefaultFormat: "mp4",
			}
			if saveErr := s.repo.UpdateSettings(ctx, settings); saveErr != nil {
				return nil, saveErr
			}
			return settings, nil
		}
		return nil, err
	}
	return settings, nil
}

// UpdateSettings updates validated user preferences.
func (s *Service) UpdateSettings(ctx context.Context, userID uuid.UUID, req UpdateSettingsRequest) (*repository.UserSettings, error) {
	settings, err := s.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.Theme != "" {
		settings.Theme = strings.ToLower(req.Theme)
	}
	if req.DefaultFormat != "" {
		settings.DefaultFormat = strings.ToLower(req.DefaultFormat)
	}
	settings.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateSettings(ctx, settings); err != nil {
		return nil, err
	}
	return settings, nil
}

