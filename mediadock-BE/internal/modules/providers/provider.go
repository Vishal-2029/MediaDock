package providers

import (
	"context"
	"errors"
	"net/url"
	"time"
)

// PlatformID uniquely identifies a media source integration.
type PlatformID string

const (
	PlatformInstagram PlatformID = "instagram"
	PlatformYouTube   PlatformID = "youtube"
	PlatformFacebook  PlatformID = "facebook"
	PlatformGeneric   PlatformID = "generic"
)

var (
	ErrUnsupportedURL         = errors.New("URL is not supported by any registered provider")
	ErrAuthorizationRequired  = errors.New("authorized platform credentials or ownership verification required")
	ErrUnauthorizedMedia      = errors.New("user is not authorized to access or download this media")
	ErrMediaUnavailable       = errors.New("authorized source media is unavailable or expired")
	ErrPlatformNotImplemented = errors.New("platform integration is not yet enabled")
)

// ProviderError represents a structured, compliant, user-safe provider error.
type ProviderError struct {
	Provider    PlatformID `json:"provider"`
	Code        string     `json:"code"`
	Message     string     `json:"message"`
	Alternative string     `json:"alternative,omitempty"`
	HTTPStatus  int        `json:"-"`
	Cause       error      `json:"-"`
}

func (e *ProviderError) Error() string {
	return e.Message
}

func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// AuthCheckRequest supplies authorization context (e.g., per-request OAuth access token or ownership attestation).
// Platform tokens are never persisted as passwords or session cookies.
type AuthCheckRequest struct {
	URL                 string
	OAuthAccessToken    string
	AuthorizationMethod string
	OwnerAttested       bool
}

// AuthorizationResult reports the outcome of authorization verification.
type AuthorizationResult struct {
	Authorized          bool      `json:"authorized"`
	AuthorizationMethod string    `json:"authorization_method"`
	AccountIdentifier   string    `json:"account_identifier,omitempty"`
	MediaIdentifier     string    `json:"media_identifier,omitempty"`
	VerifiedAt          time.Time `json:"verified_at"`
	Reason              string    `json:"reason,omitempty"`
	Alternative         string    `json:"alternative,omitempty"`
}

// MetadataRequest specifies the inspected URL and per-request authorization token.
type MetadataRequest struct {
	URL              string
	OAuthAccessToken string
	OwnerAttested    bool
}

// MediaMetadata describes inspected media returned by a provider.
type MediaMetadata struct {
	Provider       PlatformID `json:"provider"`
	SourceURL      string     `json:"source_url"`
	MediaID        string     `json:"media_id,omitempty"`
	MediaType      string     `json:"media_type"` // video, audio, image
	Title          string     `json:"title"`
	ThumbnailURL   string     `json:"thumbnail_url,omitempty"`
	Duration       int        `json:"duration"`
	OriginalFormat string     `json:"original_format"`
	FileSize       int64      `json:"file_size"`
	DownloadURL    string     `json:"-"` // Internal authorized download URL; never exposed directly as scraped raw secret
	OwnerUsername  string     `json:"owner_username,omitempty"`
}

// AvailabilityResult indicates whether authorized source media can be retrieved.
type AvailabilityResult struct {
	Available   bool   `json:"available"`
	Reason      string `json:"reason,omitempty"`
	Alternative string `json:"alternative,omitempty"`
}

// FormatOption describes an authorized download or conversion format/quality choice.
type FormatOption struct {
	Format      string `json:"format"`
	Quality     string `json:"quality"`
	Container   string `json:"container"`
	Description string `json:"description"`
	AudioOnly   bool   `json:"audio_only,omitempty"`
}

// RetrievalRequest instructs a provider to stream/save authorized source media to DestPath.
type RetrievalRequest struct {
	SourceURL        string
	OAuthAccessToken string
	OwnerAttested    bool
	DestPath         string
	MaxFileSize      int64
}

// RetrievedMedia describes the downloaded authorized source artifact on local temp disk.
type RetrievedMedia struct {
	FilePath    string
	Format      string
	ContentType string
	FileSize    int64
}

// Provider defines the common contract every platform integration must implement.
type Provider interface {
	ID() PlatformID
	Name() string
	CanHandle(rawURL string) bool
	ValidateURL(rawURL string) (*url.URL, error)
	VerifyAuthorization(ctx context.Context, req AuthCheckRequest) (*AuthorizationResult, error)
	FetchMetadata(ctx context.Context, req MetadataRequest) (*MediaMetadata, error)
	CheckAvailability(ctx context.Context, req MetadataRequest) (*AvailabilityResult, error)
	ListFormats(ctx context.Context, meta *MediaMetadata) ([]FormatOption, error)
	RetrieveMedia(ctx context.Context, req RetrievalRequest) (*RetrievedMedia, error)
	MapError(err error) *ProviderError
}

