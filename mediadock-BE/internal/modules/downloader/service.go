package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/config"
	"github.com/mediadock/backend/internal/modules/history"
	"github.com/mediadock/backend/internal/modules/processing"
	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/modules/storage"
	"github.com/mediadock/backend/internal/queue"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/utils"
)

var (
	ErrJobNotReady   = errors.New("download job is not completed yet")
	ErrJobExpired    = errors.New("download job has expired")
	ErrInvalidFormat = errors.New("requested format or quality is not supported")
	ErrPreviewOnly   = errors.New("this media item is preview-only and cannot be downloaded without authorization")
)

// CreateDownloadRequest represents POST /api/v1/downloads.
type CreateDownloadRequest struct {
	MediaID          uuid.UUID `json:"media_id" validate:"required"`
	RequestedFormat  string    `json:"requested_format" validate:"required,oneof=mp4 mov mkv webm mp3 aac m4a wav flac ogg jpg jpeg png webp"`
	RequestedQuality string    `json:"requested_quality" validate:"omitempty,oneof=original 1080p 720p 480p 360p audio"`
	AccessToken      string    `json:"access_token,omitempty"`
}

// JobStatusResponse returns lightweight status and progress for polling.
type JobStatusResponse struct {
	ID           uuid.UUID            `json:"id"`
	Status       repository.JobStatus `json:"status"`
	Progress     int                  `json:"progress"`
	ErrorMessage string               `json:"error_message,omitempty"`
	ExpiresAt    *time.Time           `json:"expires_at,omitempty"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

// DownloadFileResult wraps an open file stream or signed URL ready for delivery.
type DownloadFileResult struct {
	Reader    io.ReadCloser
	Size      int64
	FileName  string
	SignedURL string
}

// Service orchestrates authorized download jobs without containing any platform scraping logic.
type Service struct {
	jobs      repository.DownloadJobRepository
	mediaRepo repository.MediaRepository
	registry  *providers.Registry
	processor processing.Processor
	store     storage.Storage
	history   *history.Service
	queue     *queue.WorkerQueue
	cfg       *config.Config
	log       *slog.Logger
}

// NewService constructs a downloader Service.
func NewService(
	jobs repository.DownloadJobRepository,
	mediaRepo repository.MediaRepository,
	registry *providers.Registry,
	processor processing.Processor,
	store storage.Storage,
	historySvc *history.Service,
	workerQueue *queue.WorkerQueue,
	cfg *config.Config,
	log *slog.Logger,
) *Service {
	return &Service{
		jobs:      jobs,
		mediaRepo: mediaRepo,
		registry:  registry,
		processor: processor,
		store:     store,
		history:   historySvc,
		queue:     workerQueue,
		cfg:       cfg,
		log:       log,
	}
}

// CreateJob validates ownership and creates an asynchronous download job.
func (s *Service) CreateJob(ctx context.Context, userID uuid.UUID, req CreateDownloadRequest) (*repository.DownloadJob, error) {
	item, err := s.mediaRepo.FindItemByID(ctx, req.MediaID, userID)
	if err != nil {
		return nil, err
	}
	if !item.Downloadable {
		return nil, ErrPreviewOnly
	}

	quality := strings.ToLower(strings.TrimSpace(req.RequestedQuality))
	if quality == "" {
		quality = "original"
	}
	format := strings.ToLower(strings.TrimSpace(req.RequestedFormat))

	job := &repository.DownloadJob{
		ID:               uuid.New(),
		UserID:           userID,
		MediaID:          item.ID,
		Status:           repository.JobStatusPending,
		Progress:         0,
		RequestedFormat:  format,
		RequestedQuality: quality,
	}

	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, err
	}

	if err := s.queue.Enqueue(ctx, queue.JobTypeDownload, job.ID, strings.TrimSpace(req.AccessToken)); err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "failed to enqueue download job", nil)
		return nil, fmt.Errorf("enqueue download job: %w", err)
	}

	return job, nil
}

// GetJob returns full download job details owned by userID.
func (s *Service) GetJob(ctx context.Context, id, userID uuid.UUID) (*repository.DownloadJob, error) {
	return s.jobs.FindByID(ctx, id, userID)
}

// ListJobs returns paginated download jobs owned by userID.
func (s *Service) ListJobs(ctx context.Context, userID uuid.UUID, offset, limit int) ([]repository.DownloadJob, int64, error) {
	return s.jobs.ListByUser(ctx, userID, offset, limit)
}

// GetStatus returns lightweight progress and status for a download job.
func (s *Service) GetStatus(ctx context.Context, id, userID uuid.UUID) (*JobStatusResponse, error) {
	job, err := s.jobs.FindByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	return &JobStatusResponse{
		ID:           job.ID,
		Status:       job.Status,
		Progress:     job.Progress,
		ErrorMessage: job.ErrorMessage,
		ExpiresAt:    job.ExpiresAt,
		UpdatedAt:    job.UpdatedAt,
	}, nil
}

// ProcessJob executes the queued download and optional FFmpeg processing pipeline.
func (s *Service) ProcessJob(ctx context.Context, payload queue.JobPayload) error {
	job, err := s.jobs.FindByIDSystem(ctx, payload.ID)
	if err != nil {
		return err
	}
	if job.Media == nil || job.Media.Source == nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "missing source metadata", nil)
		return errors.New("missing media source metadata")
	}

	_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusProcessing, 10, "", "", nil)

	prov, err := s.registry.GetByID(providers.PlatformID(job.Media.Source.Provider))
	if err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "provider unavailable", nil)
		return err
	}

	origExt := strings.TrimPrefix(strings.ToLower(job.Media.OriginalFormat), ".")
	if origExt == "" {
		origExt = "mp4"
	}

	rawTempFile, err := utils.CreateSafeTempFile(s.store.TempDir(), "dl-raw-"+job.ID.String(), origExt)
	if err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "failed to prepare temporary file", nil)
		return err
	}
	rawTempPath := rawTempFile.Name()
	_ = rawTempFile.Close()
	defer os.Remove(rawTempPath)

	retrieved, err := prov.RetrieveMedia(ctx, providers.RetrievalRequest{
		SourceURL:        job.Media.Source.SourceURL,
		OAuthAccessToken: payload.Token,
		OwnerAttested:    job.Media.Source.AuthorizationMethod == "owner_attestation",
		DestPath:         rawTempPath,
		MaxFileSize:      s.cfg.MaxFileSize,
	})
	if err != nil {
		provErr := prov.MapError(err)
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", provErr.Message, nil)
		return err
	}

	_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusProcessing, 55, "", "", nil)

	finalLocalPath := retrieved.FilePath
	targetExt := strings.ToLower(strings.TrimPrefix(job.RequestedFormat, "."))
	needsProcessing := targetExt != strings.ToLower(retrieved.Format) ||
		(job.RequestedQuality != "" && job.RequestedQuality != "original")

	if needsProcessing && s.processor != nil {
		procTempFile, err := utils.CreateSafeTempFile(s.store.TempDir(), "dl-proc-"+job.ID.String(), targetExt)
		if err != nil {
			_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "failed to prepare output temp file", nil)
			return err
		}
		procPath := procTempFile.Name()
		_ = procTempFile.Close()
		defer os.Remove(procPath)

		err = s.processor.ProcessMedia(ctx, processing.ProcessOptions{
			InputPath:        rawTempPath,
			OutputPath:       procPath,
			TargetFormat:     targetExt,
			TargetResolution: job.RequestedQuality,
			ExtractAudio:     processing.SupportedAudioFormats[targetExt],
		})
		if err != nil {
			_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "media processing failed: "+err.Error(), nil)
			return err
		}
		finalLocalPath = procPath
	}

	_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusProcessing, 85, "", "", nil)

	if s.processor != nil && s.mediaRepo != nil && (job.Media == nil || job.Media.Duration == 0) {
		if probe, probeErr := s.processor.InspectMetadata(ctx, finalLocalPath); probeErr == nil && probe != nil && probe.DurationSeconds > 0 {
			_ = s.mediaRepo.UpdateItemDuration(ctx, job.MediaID, probe.DurationSeconds)
		}
	}

	storageKey := filepath.ToSlash(filepath.Join("downloads", job.UserID.String(), job.ID.String()+"."+targetExt))
	savedKey, err := s.store.SaveFile(ctx, storageKey, finalLocalPath, retrieved.ContentType)
	if err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "failed to persist processed media", nil)
		return err
	}

	expiresAt := time.Now().UTC().Add(s.cfg.JobRetentionTTL)
	if err := s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusCompleted, 100, savedKey, "", &expiresAt); err != nil {
		return err
	}

	return nil
}

// OpenFile verifies download authorization, checks expiration, records history, and opens the artifact.
func (s *Service) OpenFile(ctx context.Context, id, userID uuid.UUID) (*DownloadFileResult, error) {
	job, err := s.jobs.FindByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if job.Status == repository.JobStatusExpired || (job.ExpiresAt != nil && time.Now().UTC().After(*job.ExpiresAt)) {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusExpired, job.Progress, "", "download expired", job.ExpiresAt)
		if job.OutputPath != "" {
			_ = s.store.Delete(ctx, job.OutputPath)
		}
		return nil, ErrJobExpired
	}
	if job.Status != repository.JobStatusCompleted || job.OutputPath == "" {
		return nil, ErrJobNotReady
	}

	baseTitle := "mediadock-" + job.ID.String()
	if job.Media != nil && job.Media.Title != "" {
		baseTitle = utils.SanitizeFilename(job.Media.Title)
	}
	fileName := fmt.Sprintf("%s.%s", strings.TrimSuffix(baseTitle, filepath.Ext(baseTitle)), job.RequestedFormat)

	// Record download history entry.
	if s.history != nil {
		_ = s.history.Record(ctx, userID, job.ID, fileName)
	}

	if s.cfg.StorageType == "s3" {
		signed, err := s.store.SignedURL(ctx, job.OutputPath, 15*time.Minute)
		if err != nil {
			return nil, err
		}
		return &DownloadFileResult{
			FileName:  fileName,
			SignedURL: signed,
		}, nil
	}

	reader, size, err := s.store.Open(ctx, job.OutputPath)
	if err != nil {
		return nil, err
	}

	return &DownloadFileResult{
		Reader:   reader,
		Size:     size,
		FileName: fileName,
	}, nil
}

// DeleteJob removes the stored output file and deletes the download job record.
func (s *Service) DeleteJob(ctx context.Context, id, userID uuid.UUID) error {
	job, err := s.jobs.FindByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if job.OutputPath != "" {
		_ = s.store.Delete(ctx, job.OutputPath)
	}
	return s.jobs.Delete(ctx, id, userID)
}

// CleanupExpired expires past-due download jobs and removes their stored artifacts.
func (s *Service) CleanupExpired(ctx context.Context) (int, error) {
	expired, err := s.jobs.FindExpired(ctx, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	cleaned := 0
	for _, job := range expired {
		if job.OutputPath != "" {
			_ = s.store.Delete(ctx, job.OutputPath)
		}
		if err := s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusExpired, job.Progress, "", "expired after retention window", job.ExpiresAt); err == nil {
			cleaned++
		}
	}
	if cleaned > 0 && s.log != nil {
		s.log.Info("cleaned expired download jobs", "count", cleaned)
	}
	return cleaned, nil
}

