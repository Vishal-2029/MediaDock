package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("record not found")

// UserRepository defines persistence operations for users and their settings.
type UserRepository interface {
	CreateWithSettings(ctx context.Context, user *User, settings *UserSettings) error
	EnsureGuestUser(ctx context.Context, guestID uuid.UUID) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateName(ctx context.Context, id uuid.UUID, name string) (*User, error)
	GetSettings(ctx context.Context, userID uuid.UUID) (*UserSettings, error)
	UpdateSettings(ctx context.Context, settings *UserSettings) error
}

// MediaRepository defines persistence operations for media sources and items.
type MediaRepository interface {
	CreateSourceAndItem(ctx context.Context, source *MediaSource, item *MediaItem) error
	FindItemByID(ctx context.Context, id, userID uuid.UUID) (*MediaItem, error)
	ListItemsByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]MediaItem, int64, error)
	UpdateItemDuration(ctx context.Context, id uuid.UUID, duration int) error
	DeleteItem(ctx context.Context, id, userID uuid.UUID) error
}

// DownloadJobRepository defines persistence operations for download jobs.
type DownloadJobRepository interface {
	Create(ctx context.Context, job *DownloadJob) error
	FindByID(ctx context.Context, id, userID uuid.UUID) (*DownloadJob, error)
	FindByIDSystem(ctx context.Context, id uuid.UUID) (*DownloadJob, error)
	ListByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]DownloadJob, int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status JobStatus, progress int, outputPath, errMsg string, expiresAt *time.Time) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
	FindExpired(ctx context.Context, now time.Time) ([]DownloadJob, error)
}

// ConversionJobRepository defines persistence operations for conversion jobs.
type ConversionJobRepository interface {
	Create(ctx context.Context, job *ConversionJob) error
	FindByID(ctx context.Context, id, userID uuid.UUID) (*ConversionJob, error)
	FindByIDSystem(ctx context.Context, id uuid.UUID) (*ConversionJob, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status JobStatus, progress int, outputPath, errMsg string, fileSize int64, duration int, expiresAt *time.Time) error
	UpdateThumbnailPath(ctx context.Context, id uuid.UUID, thumbnailPath string) error
	FindExpired(ctx context.Context, now time.Time) ([]ConversionJob, error)
}

// HistoryRepository defines persistence operations for download history records.
type HistoryRepository interface {
	Create(ctx context.Context, record *DownloadHistory) error
	ListByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]DownloadHistory, int64, error)
	FindByID(ctx context.Context, id, userID uuid.UUID) (*DownloadHistory, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// GormUserRepository implements UserRepository using GORM.
type GormUserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) CreateWithSettings(ctx context.Context, user *User, settings *UserSettings) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		settings.UserID = user.ID
		return tx.Create(settings).Error
	})
}

func (r *GormUserRepository) EnsureGuestUser(ctx context.Context, guestID uuid.UUID) error {
	if _, err := r.FindByID(ctx, guestID); err == nil {
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}

	guest := &User{
		ID:           guestID,
		Name:         "MediaDock Guest",
		Email:        "guest-" + guestID.String() + "@mediadock.local",
		PasswordHash: "guest-workspace-no-login",
	}
	settings := &UserSettings{
		ID:            uuid.New(),
		Theme:         "dark",
		DefaultFormat: "mp4",
	}
	err := r.CreateWithSettings(ctx, guest, settings)
	if err != nil {
		// Handle concurrent creation races gracefully by re-checking presence.
		if _, findErr := r.FindByID(ctx, guestID); findErr == nil {
			return nil
		}
		return err
	}
	return nil
}

func (r *GormUserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &user, err
}

func (r *GormUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &user, err
}

func (r *GormUserRepository) UpdateName(ctx context.Context, id uuid.UUID, name string) (*User, error) {
	res := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":       name,
		"updated_at": time.Now().UTC(),
	})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *GormUserRepository) GetSettings(ctx context.Context, userID uuid.UUID) (*UserSettings, error) {
	var settings UserSettings
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &settings, err
}

func (r *GormUserRepository) UpdateSettings(ctx context.Context, settings *UserSettings) error {
	return r.db.WithContext(ctx).Save(settings).Error
}

// GormMediaRepository implements MediaRepository using GORM.
type GormMediaRepository struct {
	db *gorm.DB
}

func NewMediaRepository(db *gorm.DB) *GormMediaRepository {
	return &GormMediaRepository{db: db}
}

func (r *GormMediaRepository) CreateSourceAndItem(ctx context.Context, source *MediaSource, item *MediaItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(source).Error; err != nil {
			return err
		}
		item.SourceID = source.ID
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		if !item.Downloadable {
			return tx.Model(item).Update("downloadable", false).Error
		}
		return nil
	})
}

func (r *GormMediaRepository) FindItemByID(ctx context.Context, id, userID uuid.UUID) (*MediaItem, error) {
	var item MediaItem
	err := r.db.WithContext(ctx).Preload("Source").Where("id = ? AND user_id = ?", id, userID).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &item, err
}

func (r *GormMediaRepository) ListItemsByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]MediaItem, int64, error) {
	var items []MediaItem
	var total int64

	q := r.db.WithContext(ctx).Model(&MediaItem{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := r.db.WithContext(ctx).Preload("Source").Where("user_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *GormMediaRepository) DeleteItem(ctx context.Context, id, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&MediaItem{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormMediaRepository) UpdateItemDuration(ctx context.Context, id uuid.UUID, duration int) error {
	return r.db.WithContext(ctx).Model(&MediaItem{}).Where("id = ?", id).Updates(map[string]interface{}{
		"duration":   duration,
		"updated_at": time.Now().UTC(),
	}).Error
}

// GormDownloadJobRepository implements DownloadJobRepository using GORM.
type GormDownloadJobRepository struct {
	db *gorm.DB
}

func NewDownloadJobRepository(db *gorm.DB) *GormDownloadJobRepository {
	return &GormDownloadJobRepository{db: db}
}

func (r *GormDownloadJobRepository) Create(ctx context.Context, job *DownloadJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *GormDownloadJobRepository) FindByID(ctx context.Context, id, userID uuid.UUID) (*DownloadJob, error) {
	var job DownloadJob
	err := r.db.WithContext(ctx).
		Preload("Media").
		Preload("Media.Source").
		Where("id = ? AND user_id = ?", id, userID).
		First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &job, err
}

func (r *GormDownloadJobRepository) FindByIDSystem(ctx context.Context, id uuid.UUID) (*DownloadJob, error) {
	var job DownloadJob
	err := r.db.WithContext(ctx).
		Preload("Media").
		Preload("Media.Source").
		Where("id = ?", id).
		First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &job, err
}

func (r *GormDownloadJobRepository) ListByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]DownloadJob, int64, error) {
	var jobs []DownloadJob
	var total int64

	q := r.db.WithContext(ctx).Model(&DownloadJob{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := r.db.WithContext(ctx).Preload("Media").Preload("Media.Source").Where("user_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&jobs).Error; err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}

func (r *GormDownloadJobRepository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status JobStatus,
	progress int,
	outputPath, errMsg string,
	expiresAt *time.Time,
) error {
	updates := map[string]interface{}{
		"status":        status,
		"progress":      progress,
		"error_message": errMsg,
		"updated_at":    time.Now().UTC(),
	}
	if outputPath != "" {
		updates["output_path"] = outputPath
	}
	if expiresAt != nil {
		updates["expires_at"] = expiresAt
	}
	return r.db.WithContext(ctx).Model(&DownloadJob{}).Where("id = ?", id).Updates(updates).Error
}

func (r *GormDownloadJobRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&DownloadJob{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormDownloadJobRepository) FindExpired(ctx context.Context, now time.Time) ([]DownloadJob, error) {
	var jobs []DownloadJob
	err := r.db.WithContext(ctx).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", JobStatusCompleted, now).
		Find(&jobs).Error
	return jobs, err
}

// GormConversionJobRepository implements ConversionJobRepository using GORM.
type GormConversionJobRepository struct {
	db *gorm.DB
}

func NewConversionJobRepository(db *gorm.DB) *GormConversionJobRepository {
	return &GormConversionJobRepository{db: db}
}

func (r *GormConversionJobRepository) Create(ctx context.Context, job *ConversionJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *GormConversionJobRepository) FindByID(ctx context.Context, id, userID uuid.UUID) (*ConversionJob, error) {
	var job ConversionJob
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &job, err
}

func (r *GormConversionJobRepository) FindByIDSystem(ctx context.Context, id uuid.UUID) (*ConversionJob, error) {
	var job ConversionJob
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &job, err
}

func (r *GormConversionJobRepository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status JobStatus,
	progress int,
	outputPath, errMsg string,
	fileSize int64,
	duration int,
	expiresAt *time.Time,
) error {
	updates := map[string]interface{}{
		"status":        status,
		"progress":      progress,
		"error_message": errMsg,
		"updated_at":    time.Now().UTC(),
	}
	if outputPath != "" {
		updates["output_path"] = outputPath
	}
	if fileSize > 0 {
		updates["file_size"] = fileSize
	}
	if duration > 0 {
		updates["duration"] = duration
	}
	if expiresAt != nil {
		updates["expires_at"] = expiresAt
	}
	return r.db.WithContext(ctx).Model(&ConversionJob{}).Where("id = ?", id).Updates(updates).Error
}

func (r *GormConversionJobRepository) UpdateThumbnailPath(ctx context.Context, id uuid.UUID, thumbnailPath string) error {
	return r.db.WithContext(ctx).Model(&ConversionJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"thumbnail_path": thumbnailPath,
		"updated_at":     time.Now().UTC(),
	}).Error
}

func (r *GormConversionJobRepository) FindExpired(ctx context.Context, now time.Time) ([]ConversionJob, error) {
	var jobs []ConversionJob
	err := r.db.WithContext(ctx).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", JobStatusCompleted, now).
		Find(&jobs).Error
	return jobs, err
}

// GormHistoryRepository implements HistoryRepository using GORM.
type GormHistoryRepository struct {
	db *gorm.DB
}

func NewHistoryRepository(db *gorm.DB) *GormHistoryRepository {
	return &GormHistoryRepository{db: db}
}

func (r *GormHistoryRepository) Create(ctx context.Context, record *DownloadHistory) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *GormHistoryRepository) ListByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]DownloadHistory, int64, error) {
	var records []DownloadHistory
	var total int64

	q := r.db.WithContext(ctx).Model(&DownloadHistory{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("downloaded_at DESC").Offset(offset).Limit(limit).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r *GormHistoryRepository) FindByID(ctx context.Context, id, userID uuid.UUID) (*DownloadHistory, error) {
	var record DownloadHistory
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &record, err
}

func (r *GormHistoryRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&DownloadHistory{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormHistoryRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("downloaded_at < ?", cutoff).Delete(&DownloadHistory{})
	return res.RowsAffected, res.Error
}

