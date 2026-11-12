package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JobStatus represents lifecycle states for download and conversion jobs.
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusExpired    JobStatus = "expired"
)

// User represents a registered MediaDock user.
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name         string    `gorm:"size:120;not null" json:"name"`
	Email        string    `gorm:"size:255;uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"not null" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// MediaSource tracks an inspected source URL and its authorization verification metadata.
type MediaSource struct {
	ID                    uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	Provider              string     `gorm:"size:64;index;not null" json:"provider"`
	SourceURL             string     `gorm:"type:text;not null" json:"source_url"`
	AuthorizationMethod   string     `gorm:"size:64;not null" json:"authorization_method"`
	AuthorizationVerified *time.Time `gorm:"column:authorization_verified_at" json:"authorization_verified_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

func (m *MediaSource) BeforeCreate(_ *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// MediaItem holds inspected metadata for an authorized or previewable media resource.
type MediaItem struct {
	ID                    uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID                uuid.UUID    `gorm:"type:uuid;index;not null" json:"user_id"`
	SourceID              uuid.UUID    `gorm:"type:uuid;index;not null" json:"source_id"`
	Source                *MediaSource `gorm:"foreignKey:SourceID" json:"source,omitempty"`
	Title                 string       `gorm:"size:500;not null" json:"title"`
	MediaType             string       `gorm:"size:16;not null;default:video" json:"media_type"`
	ThumbnailURL          string       `gorm:"type:text" json:"thumbnail_url"`
	Duration              int          `gorm:"not null;default:0" json:"duration"`
	OriginalFormat        string       `gorm:"size:32;not null" json:"original_format"`
	FileSize              int64        `gorm:"not null;default:0" json:"file_size"`
	Downloadable          bool         `gorm:"not null" json:"downloadable"`
	DownloadBlockedReason string       `gorm:"type:text" json:"download_blocked_reason,omitempty"`
	CreatedAt             time.Time    `json:"created_at"`
}

func (m *MediaItem) BeforeCreate(_ *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// DownloadJob represents an asynchronous authorized media retrieval and optional processing job.
type DownloadJob struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	MediaID          uuid.UUID  `gorm:"type:uuid;index;not null" json:"media_id"`
	Media            *MediaItem `gorm:"foreignKey:MediaID" json:"media,omitempty"`
	Status           JobStatus  `gorm:"size:32;index;not null" json:"status"`
	Progress         int        `gorm:"not null;default:0" json:"progress"`
	RequestedFormat  string     `gorm:"size:32;not null" json:"requested_format"`
	RequestedQuality string     `gorm:"size:32;not null;default:original" json:"requested_quality"`
	OutputPath       string     `gorm:"type:text" json:"-"`
	ErrorMessage     string     `gorm:"type:text" json:"error_message,omitempty"`
	ExpiresAt        *time.Time `gorm:"index" json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (d *DownloadJob) BeforeCreate(_ *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

// DownloadHistory logs completed user downloads for auditing and history management.
type DownloadHistory struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	JobID        uuid.UUID `gorm:"type:uuid;index;not null" json:"job_id"`
	FileName     string    `gorm:"size:255;not null" json:"file_name"`
	DownloadedAt time.Time `gorm:"index;not null" json:"downloaded_at"`
}

func (DownloadHistory) TableName() string {
	return "download_history"
}

func (h *DownloadHistory) BeforeCreate(_ *gorm.DB) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	if h.DownloadedAt.IsZero() {
		h.DownloadedAt = time.Now().UTC()
	}
	return nil
}

// UserSettings stores user preferences.
type UserSettings struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID        uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	Theme         string    `gorm:"size:32;not null;default:system" json:"theme"`
	DefaultFormat string    `gorm:"size:32;not null;default:mp4" json:"default_format"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (s *UserSettings) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// ConversionJob represents a standalone FFmpeg conversion job for user-uploaded media.
type ConversionJob struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	SourceFilename   string     `gorm:"size:255;not null" json:"source_filename"`
	SourceFormat     string     `gorm:"size:32;not null" json:"source_format"`
	TargetFormat     string     `gorm:"size:32;not null" json:"target_format"`
	TargetResolution string     `gorm:"size:32" json:"target_resolution,omitempty"`
	Compress         bool       `gorm:"not null;default:false" json:"compress"`
	ExtractAudio     bool       `gorm:"not null;default:false" json:"extract_audio"`
	Status           JobStatus  `gorm:"size:32;index;not null" json:"status"`
	Progress         int        `gorm:"not null;default:0" json:"progress"`
	InputPath        string     `gorm:"type:text;not null" json:"-"`
	OutputPath       string     `gorm:"type:text" json:"-"`
	ThumbnailPath    string     `gorm:"type:text" json:"-"`
	FileSize         int64      `gorm:"not null;default:0" json:"file_size"`
	Duration         int        `gorm:"not null;default:0" json:"duration"`
	ErrorMessage     string     `gorm:"type:text" json:"error_message,omitempty"`
	ExpiresAt        *time.Time `gorm:"index" json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (c *ConversionJob) BeforeCreate(_ *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

