package history

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/repository"
)

// Service manages download history records and retention cleanup.
type Service struct {
	repo          repository.HistoryRepository
	retentionDays int
	log           *slog.Logger
}

// NewService constructs a history Service.
func NewService(repo repository.HistoryRepository, retentionDays int, log *slog.Logger) *Service {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	return &Service{
		repo:          repo,
		retentionDays: retentionDays,
		log:           log,
	}
}

// Record creates a new download history entry.
func (s *Service) Record(ctx context.Context, userID, jobID uuid.UUID, fileName string) error {
	rec := &repository.DownloadHistory{
		ID:           uuid.New(),
		UserID:       userID,
		JobID:        jobID,
		FileName:     fileName,
		DownloadedAt: time.Now().UTC(),
	}
	return s.repo.Create(ctx, rec)
}

// List returns paginated download history for userID.
func (s *Service) List(ctx context.Context, userID uuid.UUID, offset, limit int) ([]repository.DownloadHistory, int64, error) {
	return s.repo.ListByUser(ctx, userID, offset, limit)
}

// GetByID returns a single download history record owned by userID.
func (s *Service) GetByID(ctx context.Context, id, userID uuid.UUID) (*repository.DownloadHistory, error) {
	return s.repo.FindByID(ctx, id, userID)
}

// Delete removes a download history record owned by userID.
func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.Delete(ctx, id, userID)
}

// ApplyRetention deletes history records older than the configured retention period.
func (s *Service) ApplyRetention(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -s.retentionDays)
	deleted, err := s.repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	if deleted > 0 && s.log != nil {
		s.log.Info("applied download history retention policy", "deleted_records", deleted, "cutoff", cutoff)
	}
	return deleted, nil
}

