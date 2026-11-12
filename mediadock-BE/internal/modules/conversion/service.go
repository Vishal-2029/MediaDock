package conversion

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
	"github.com/mediadock/backend/internal/modules/processing"
	"github.com/mediadock/backend/internal/modules/storage"
	"github.com/mediadock/backend/internal/queue"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/utils"
)

var (
	ErrConversionNotReady = errors.New("conversion job is not completed yet")
	ErrConversionExpired  = errors.New("conversion job has expired")
	ErrThumbnailNotFound  = errors.New("conversion thumbnail is not available")
)

// ConversionFileResult wraps an open output/thumbnail stream or S3 signed URL.
type ConversionFileResult struct {
	Reader    io.ReadCloser
	Size      int64
	FileName  string
	SignedURL string
}

// CreateConversionRequest captures options for a user-uploaded media conversion.
type CreateConversionRequest struct {
	TargetFormat     string `form:"target_format" validate:"required,oneof=mp4 mov mkv webm mp3 aac m4a wav flac ogg"`
	TargetResolution string `form:"target_resolution" validate:"omitempty,oneof=original 1080p 720p 480p 360p"`
	Compress         bool   `form:"compress"`
	ExtractAudio     bool   `form:"extract_audio"`
}

// StatusResponse represents GET /api/v1/conversions/:id/status.
type StatusResponse struct {
	ID           uuid.UUID            `json:"id"`
	Status       repository.JobStatus `json:"status"`
	Progress     int                  `json:"progress"`
	ErrorMessage string               `json:"error_message,omitempty"`
	ExpiresAt    *time.Time           `json:"expires_at,omitempty"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

// Service manages safe FFmpeg conversion jobs.
type Service struct {
	jobs      repository.ConversionJobRepository
	processor processing.Processor
	store     storage.Storage
	queue     *queue.WorkerQueue
	cfg       *config.Config
	log       *slog.Logger
}

// NewService constructs a conversion Service.
func NewService(
	jobs repository.ConversionJobRepository,
	processor processing.Processor,
	store storage.Storage,
	workerQueue *queue.WorkerQueue,
	cfg *config.Config,
	log *slog.Logger,
) *Service {
	return &Service{
		jobs:      jobs,
		processor: processor,
		store:     store,
		queue:     workerQueue,
		cfg:       cfg,
		log:       log,
	}
}

// CreateFromFile validates an uploaded file, persists a ConversionJob, and enqueues it.
func (s *Service) CreateFromFile(
	ctx context.Context,
	userID uuid.UUID,
	originalFilename string,
	srcReader io.Reader,
	req CreateConversionRequest,
) (*repository.ConversionJob, error) {
	safeName := utils.SanitizeFilename(originalFilename)
	ext := strings.ToLower(filepath.Ext(safeName))
	if !utils.AllowedMediaExtensions[ext] {
		return nil, fmt.Errorf("%w: extension %q is not allowed", utils.ErrUnsupportedFileType, ext)
	}

	targetFmt := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(req.TargetFormat), "."))
	if req.ExtractAudio && !processing.SupportedAudioFormats[targetFmt] {
		return nil, fmt.Errorf("%w: extract_audio requires an audio target format", processing.ErrUnsupportedTargetFormat)
	}
	if !processing.SupportedVideoFormats[targetFmt] && !processing.SupportedAudioFormats[targetFmt] {
		return nil, fmt.Errorf("%w: %q", processing.ErrUnsupportedTargetFormat, targetFmt)
	}

	tempFile, err := utils.CreateSafeTempFile(s.store.TempDir(), "conv-in-"+userID.String(), ext)
	if err != nil {
		return nil, err
	}
	tempPath := tempFile.Name()

	written, copyErr := io.Copy(tempFile, io.LimitReader(srcReader, s.cfg.MaxFileSize+1))
	_ = tempFile.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return nil, copyErr
	}
	if written > s.cfg.MaxFileSize {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("%w: file exceeds limit of %d bytes", utils.ErrFileTooLarge, s.cfg.MaxFileSize)
	}

	contentType, fileSize, valErr := utils.ValidateMediaFile(tempPath, s.cfg.MaxFileSize)
	if valErr != nil {
		_ = os.Remove(tempPath)
		return nil, valErr
	}

	jobID := uuid.New()
	inputKey := filepath.ToSlash(filepath.Join("conversions", userID.String(), jobID.String()+"-input"+ext))
	if _, err := s.store.SaveFile(ctx, inputKey, tempPath, contentType); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	_ = os.Remove(tempPath)

	job := &repository.ConversionJob{
		ID:               jobID,
		UserID:           userID,
		SourceFilename:   safeName,
		SourceFormat:     strings.TrimPrefix(ext, "."),
		TargetFormat:     targetFmt,
		TargetResolution: strings.ToLower(strings.TrimSpace(req.TargetResolution)),
		Compress:         req.Compress,
		ExtractAudio:     req.ExtractAudio,
		Status:           repository.JobStatusPending,
		Progress:         0,
		InputPath:        inputKey,
		FileSize:         fileSize,
	}

	if err := s.jobs.Create(ctx, job); err != nil {
		_ = s.store.Delete(ctx, inputKey)
		return nil, err
	}

	if err := s.queue.Enqueue(ctx, queue.JobTypeConversion, job.ID, ""); err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "failed to enqueue conversion job", 0, 0, nil)
		return nil, err
	}

	return job, nil
}

// GetJob returns full conversion job details owned by userID.
func (s *Service) GetJob(ctx context.Context, id, userID uuid.UUID) (*repository.ConversionJob, error) {
	return s.jobs.FindByID(ctx, id, userID)
}

// GetStatus returns lightweight conversion job status.
func (s *Service) GetStatus(ctx context.Context, id, userID uuid.UUID) (*StatusResponse, error) {
	job, err := s.jobs.FindByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	return &StatusResponse{
		ID:           job.ID,
		Status:       job.Status,
		Progress:     job.Progress,
		ErrorMessage: job.ErrorMessage,
		ExpiresAt:    job.ExpiresAt,
		UpdatedAt:    job.UpdatedAt,
	}, nil
}

// ProcessJob executes a queued conversion job using FFmpeg.
func (s *Service) ProcessJob(ctx context.Context, payload queue.JobPayload) error {
	job, err := s.jobs.FindByIDSystem(ctx, payload.ID)
	if err != nil {
		return err
	}

	_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusProcessing, 15, "", "", 0, 0, nil)

	srcStream, _, err := s.store.Open(ctx, job.InputPath)
	if err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "source file missing", 0, 0, nil)
		return err
	}

	inTemp, err := utils.CreateSafeTempFile(s.store.TempDir(), "conv-work-in-"+job.ID.String(), "."+job.SourceFormat)
	if err != nil {
		_ = srcStream.Close()
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "temp workspace error", 0, 0, nil)
		return err
	}
	inPath := inTemp.Name()
	_, copyErr := io.Copy(inTemp, srcStream)
	_ = inTemp.Close()
	_ = srcStream.Close()
	defer os.Remove(inPath)
	if copyErr != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "failed to read source file", 0, 0, nil)
		return copyErr
	}

	outTemp, err := utils.CreateSafeTempFile(s.store.TempDir(), "conv-work-out-"+job.ID.String(), "."+job.TargetFormat)
	if err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "temp output error", 0, 0, nil)
		return err
	}
	outPath := outTemp.Name()
	_ = outTemp.Close()
	defer os.Remove(outPath)

	_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusProcessing, 45, "", "", 0, 0, nil)

	err = s.processor.ProcessMedia(ctx, processing.ProcessOptions{
		InputPath:        inPath,
		OutputPath:       outPath,
		TargetFormat:     job.TargetFormat,
		TargetResolution: job.TargetResolution,
		Compress:         job.Compress,
		ExtractAudio:     job.ExtractAudio,
	})
	if err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", err.Error(), 0, 0, nil)
		return err
	}

	info, err := os.Stat(outPath)
	if err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "converted output missing", 0, 0, nil)
		return err
	}

	duration := 0
	if probe, probeErr := s.processor.InspectMetadata(ctx, outPath); probeErr == nil && probe != nil {
		duration = probe.DurationSeconds
	}

	outKey := filepath.ToSlash(filepath.Join("conversions", job.UserID.String(), job.ID.String()+"-output."+job.TargetFormat))
	if _, err := s.store.SaveFile(ctx, outKey, outPath, ""); err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusFailed, 0, "", "failed to store output", 0, 0, nil)
		return err
	}

	// Best-effort thumbnail extraction for video outputs.
	if !job.ExtractAudio && !processing.SupportedAudioFormats[strings.ToLower(job.TargetFormat)] {
		if thumbTemp, thumbErr := utils.CreateSafeTempFile(s.store.TempDir(), "conv-thumb-"+job.ID.String(), ".jpg"); thumbErr == nil {
			thumbLocalPath := thumbTemp.Name()
			_ = thumbTemp.Close()
			if genErr := s.processor.GenerateThumbnail(ctx, outPath, thumbLocalPath); genErr == nil {
				thumbKey := filepath.ToSlash(filepath.Join("conversions", job.UserID.String(), job.ID.String()+"-thumb.jpg"))
				if _, saveErr := s.store.SaveFile(ctx, thumbKey, thumbLocalPath, "image/jpeg"); saveErr == nil {
					_ = s.jobs.UpdateThumbnailPath(ctx, job.ID, thumbKey)
				}
			}
			_ = os.Remove(thumbLocalPath)
		}
	}

	// Clean up raw input copy once conversion succeeded.
	_ = s.store.Delete(ctx, job.InputPath)

	expiresAt := time.Now().UTC().Add(s.cfg.JobRetentionTTL)
	return s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusCompleted, 100, outKey, "", info.Size(), duration, &expiresAt)
}

// OpenOutput opens the converted output artifact for download or generates an S3 signed URL.
func (s *Service) OpenOutput(ctx context.Context, id, userID uuid.UUID) (*ConversionFileResult, error) {
	job, err := s.jobs.FindByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if job.Status == repository.JobStatusExpired || (job.ExpiresAt != nil && time.Now().UTC().After(*job.ExpiresAt)) {
		_ = s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusExpired, job.Progress, "", "conversion expired", job.FileSize, job.Duration, job.ExpiresAt)
		if job.OutputPath != "" {
			_ = s.store.Delete(ctx, job.OutputPath)
		}
		if job.ThumbnailPath != "" {
			_ = s.store.Delete(ctx, job.ThumbnailPath)
		}
		return nil, ErrConversionExpired
	}
	if job.Status != repository.JobStatusCompleted || job.OutputPath == "" {
		return nil, ErrConversionNotReady
	}

	base := strings.TrimSuffix(job.SourceFilename, filepath.Ext(job.SourceFilename))
	fileName := fmt.Sprintf("%s.%s", base, job.TargetFormat)

	if s.cfg.StorageType == "s3" {
		signed, err := s.store.SignedURL(ctx, job.OutputPath, 15*time.Minute)
		if err != nil {
			return nil, err
		}
		return &ConversionFileResult{
			FileName:  fileName,
			SignedURL: signed,
		}, nil
	}

	reader, size, err := s.store.Open(ctx, job.OutputPath)
	if err != nil {
		return nil, err
	}
	return &ConversionFileResult{
		Reader:   reader,
		Size:     size,
		FileName: fileName,
	}, nil
}

// OpenThumbnail opens the generated JPEG thumbnail for a completed video conversion.
func (s *Service) OpenThumbnail(ctx context.Context, id, userID uuid.UUID) (*ConversionFileResult, error) {
	job, err := s.jobs.FindByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if job.Status == repository.JobStatusExpired || (job.ExpiresAt != nil && time.Now().UTC().After(*job.ExpiresAt)) {
		return nil, ErrConversionExpired
	}
	if job.ThumbnailPath == "" {
		return nil, ErrThumbnailNotFound
	}

	base := strings.TrimSuffix(job.SourceFilename, filepath.Ext(job.SourceFilename))
	fileName := fmt.Sprintf("%s-thumb.jpg", base)

	if s.cfg.StorageType == "s3" {
		signed, err := s.store.SignedURL(ctx, job.ThumbnailPath, 15*time.Minute)
		if err != nil {
			return nil, err
		}
		return &ConversionFileResult{
			FileName:  fileName,
			SignedURL: signed,
		}, nil
	}

	reader, size, err := s.store.Open(ctx, job.ThumbnailPath)
	if err != nil {
		return nil, err
	}
	return &ConversionFileResult{
		Reader:   reader,
		Size:     size,
		FileName: fileName,
	}, nil
}

// CleanupExpired expires past-due conversion jobs and deletes their artifacts.
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
		if job.ThumbnailPath != "" {
			_ = s.store.Delete(ctx, job.ThumbnailPath)
		}
		if job.InputPath != "" {
			_ = s.store.Delete(ctx, job.InputPath)
		}
		if err := s.jobs.UpdateStatus(ctx, job.ID, repository.JobStatusExpired, job.Progress, "", "expired after retention window", job.FileSize, job.Duration, job.ExpiresAt); err == nil {
			cleaned++
		}
	}
	if cleaned > 0 && s.log != nil {
		s.log.Info("cleaned expired conversion jobs", "count", cleaned)
	}
	return cleaned, nil
}

