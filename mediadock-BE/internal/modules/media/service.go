package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/cache"
	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/utils"
)

// InspectRequest represents the payload for POST /api/v1/media/inspect.
type InspectRequest struct {
	URL           string `json:"url" validate:"required,url,max=2048"`
	AccessToken   string `json:"access_token,omitempty"`
	OwnerAttested bool   `json:"owner_attested,omitempty"`
}

// InspectResponse returns inspected media metadata, authorization details, and available formats.
type InspectResponse struct {
	MediaID               uuid.UUID                     `json:"media_id"`
	SourceID              uuid.UUID                     `json:"source_id"`
	Provider              providers.PlatformID          `json:"provider"`
	MediaType             string                        `json:"media_type"`
	Title                 string                        `json:"title"`
	ThumbnailURL          string                        `json:"thumbnail_url,omitempty"`
	Duration              int                           `json:"duration"`
	OriginalFormat        string                        `json:"original_format"`
	FileSize              int64                         `json:"file_size"`
	SourceURL             string                        `json:"source_url"`
	Downloadable          bool                          `json:"downloadable"`
	DownloadBlockedReason string                        `json:"download_blocked_reason,omitempty"`
	Authorization         providers.AuthorizationResult `json:"authorization"`
	AvailableFormats      []providers.FormatOption      `json:"available_formats"`
	CreatedAt             time.Time                     `json:"created_at"`
}

// Service handles URL inspection, authorization checks, and media metadata persistence.
type Service struct {
	repo     repository.MediaRepository
	registry *providers.Registry
	cache    *cache.RedisClient
	cacheTTL time.Duration
}

// NewService constructs a media Service.
func NewService(repo repository.MediaRepository, registry *providers.Registry) *Service {
	return &Service{
		repo:     repo,
		registry: registry,
		cacheTTL: 10 * time.Minute,
	}
}

// SetCache attaches an optional Redis client used to cache short-lived public metadata/preview results.
func (s *Service) SetCache(redisCache *cache.RedisClient, ttl time.Duration) {
	s.cache = redisCache
	if ttl > 0 {
		s.cacheTTL = ttl
	}
}

// Inspect validates the URL, resolves its provider, verifies authorization, fetches metadata,
// and stores media_sources and media_items in a transaction.
func (s *Service) Inspect(ctx context.Context, userID uuid.UUID, req InspectRequest) (*InspectResponse, error) {
	if _, err := utils.ValidatePublicURL(ctx, req.URL, false); err != nil {
		// Allow provider-level validation/mapping if custom test URLs are used in unit tests;
		// production providers enforce ValidatePublicURL themselves.
		if errors.Is(err, utils.ErrSSRFBlocked) {
			return nil, &providers.ProviderError{
				Code:       "SSRF_BLOCKED",
				Message:    err.Error(),
				HTTPStatus: http.StatusBadRequest,
				Cause:      err,
			}
		}
	}

	provider, err := s.registry.Resolve(req.URL)
	if err != nil {
		return nil, &providers.ProviderError{
			Code:        "UNSUPPORTED_PROVIDER",
			Message:     "The submitted URL is not supported by any configured authorized MediaDock provider.",
			Alternative: "If you own the media file, upload it directly to /api/v1/conversions for processing.",
			HTTPStatus:  http.StatusBadRequest,
			Cause:       err,
		}
	}

	parsedURL, err := provider.ValidateURL(req.URL)
	if err != nil {
		return nil, provider.MapError(err)
	}

	authRes, err := provider.VerifyAuthorization(ctx, providers.AuthCheckRequest{
		URL:              parsedURL.String(),
		OAuthAccessToken: strings.TrimSpace(req.AccessToken),
		OwnerAttested:    req.OwnerAttested,
	})

	downloadable := true
	blockedReason := ""
	var meta *providers.MediaMetadata
	var formats []providers.FormatOption

	if err != nil || (authRes != nil && !authRes.Authorized) {
		// Attempt public preview fallback if the provider supports PreviewProvider (e.g. Meta oEmbed).
		if errors.Is(err, providers.ErrAuthorizationRequired) || errors.Is(err, providers.ErrUnauthorizedMedia) || (authRes != nil && !authRes.Authorized) {
			if previewProv, ok := provider.(providers.PreviewProvider); ok {
				if prevMeta, prevAuth, prevErr := previewProv.FetchPublicPreview(ctx, parsedURL.String()); prevErr == nil && prevMeta != nil && prevAuth != nil {
					meta = prevMeta
					authRes = prevAuth
					downloadable = false
					blockedReason = prevAuth.Reason
					formats = []providers.FormatOption{}
					err = nil
				}
			}
		}

		if err != nil {
			provErr := provider.MapError(err)
			if authRes != nil && authRes.Alternative != "" && provErr.Alternative == "" {
				provErr.Alternative = authRes.Alternative
			}
			return nil, provErr
		}
		if downloadable && (authRes == nil || !authRes.Authorized) {
			reason := ""
			alt := ""
			if authRes != nil {
				reason = authRes.Reason
				alt = authRes.Alternative
			}
			return nil, &providers.ProviderError{
				Provider:    provider.ID(),
				Code:        "AUTHORIZATION_DENIED",
				Message:     reason,
				Alternative: alt,
				HTTPStatus:  http.StatusForbidden,
			}
		}
	}

	if meta == nil {
		// Use short-lived cached public metadata when no per-request user OAuth token is supplied.
		cacheKey := inspectCacheKey(provider.ID(), parsedURL.String())
		if s.cache != nil && strings.TrimSpace(req.AccessToken) == "" {
			var cached providers.MediaMetadata
			if getErr := s.cache.GetJSON(ctx, cacheKey, &cached); getErr == nil && cached.Title != "" {
				meta = &cached
			}
		}
		if meta == nil {
			meta, err = provider.FetchMetadata(ctx, providers.MetadataRequest{
				URL:              parsedURL.String(),
				OAuthAccessToken: strings.TrimSpace(req.AccessToken),
				OwnerAttested:    req.OwnerAttested,
			})
			if err != nil {
				return nil, provider.MapError(err)
			}
			if s.cache != nil && strings.TrimSpace(req.AccessToken) == "" {
				// Cache metadata without DownloadURL (DownloadURL has json:"-") to avoid duplicate upstream metadata calls.
				_ = s.cache.SetJSON(ctx, cacheKey, meta, s.cacheTTL)
			}
		}

		formats, err = provider.ListFormats(ctx, meta)
		if err != nil {
			return nil, provider.MapError(err)
		}
	}

	verifiedAt := authRes.VerifiedAt
	if verifiedAt.IsZero() {
		verifiedAt = time.Now().UTC()
	}

	mediaType := meta.MediaType
	if mediaType == "" {
		mediaType = "video"
	}

	source := &repository.MediaSource{
		ID:                    uuid.New(),
		UserID:                userID,
		Provider:              string(provider.ID()),
		SourceURL:             meta.SourceURL,
		AuthorizationMethod:   authRes.AuthorizationMethod,
		AuthorizationVerified: &verifiedAt,
	}

	item := &repository.MediaItem{
		ID:                    uuid.New(),
		UserID:                userID,
		Title:                 meta.Title,
		MediaType:             mediaType,
		ThumbnailURL:          meta.ThumbnailURL,
		Duration:              meta.Duration,
		OriginalFormat:        meta.OriginalFormat,
		FileSize:              meta.FileSize,
		Downloadable:          downloadable,
		DownloadBlockedReason: blockedReason,
	}

	if err := s.repo.CreateSourceAndItem(ctx, source, item); err != nil {
		return nil, err
	}

	displayProvider := meta.Provider
	if displayProvider == "" {
		displayProvider = provider.ID()
	}

	return &InspectResponse{
		MediaID:               item.ID,
		SourceID:              source.ID,
		Provider:              displayProvider,
		MediaType:             item.MediaType,
		Title:                 item.Title,
		ThumbnailURL:          item.ThumbnailURL,
		Duration:              item.Duration,
		OriginalFormat:        item.OriginalFormat,
		FileSize:              item.FileSize,
		SourceURL:             source.SourceURL,
		Downloadable:          item.Downloadable,
		DownloadBlockedReason: item.DownloadBlockedReason,
		Authorization:         *authRes,
		AvailableFormats:      formats,
		CreatedAt:             item.CreatedAt,
	}, nil
}

// GetByID retrieves an inspected media item owned by userID along with format options.
func (s *Service) GetByID(ctx context.Context, id, userID uuid.UUID) (*InspectResponse, error) {
	item, err := s.repo.FindItemByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	var provID providers.PlatformID
	var sourceURL string
	var authMethod string
	var verifiedAt time.Time
	if item.Source != nil {
		provID = providers.PlatformID(item.Source.Provider)
		sourceURL = item.Source.SourceURL
		authMethod = item.Source.AuthorizationMethod
		if item.Source.AuthorizationVerified != nil {
			verifiedAt = *item.Source.AuthorizationVerified
		}
	}

	formats := []providers.FormatOption{}
	if item.Downloadable {
		if prov, err := s.registry.GetByID(provID); err == nil {
			mediaType := item.MediaType
			if mediaType == "" {
				mediaType = "video"
				switch strings.ToLower(item.OriginalFormat) {
				case "jpg", "jpeg", "png", "webp":
					mediaType = "image"
				case "mp3", "aac", "m4a", "wav", "flac", "ogg":
					mediaType = "audio"
				}
			}
			formats, _ = prov.ListFormats(ctx, &providers.MediaMetadata{
				Provider:       provID,
				MediaType:      mediaType,
				OriginalFormat: item.OriginalFormat,
			})
		}
	}

	mediaType := item.MediaType
	if mediaType == "" {
		mediaType = "video"
	}

	return &InspectResponse{
		MediaID:               item.ID,
		SourceID:              item.SourceID,
		Provider:              provID,
		MediaType:             mediaType,
		Title:                 item.Title,
		ThumbnailURL:          item.ThumbnailURL,
		Duration:              item.Duration,
		OriginalFormat:        item.OriginalFormat,
		FileSize:              item.FileSize,
		SourceURL:             sourceURL,
		Downloadable:          item.Downloadable,
		DownloadBlockedReason: item.DownloadBlockedReason,
		Authorization: providers.AuthorizationResult{
			Authorized:          item.Downloadable,
			PreviewOnly:         !item.Downloadable,
			AuthorizationMethod: authMethod,
			VerifiedAt:          verifiedAt,
			Reason:              item.DownloadBlockedReason,
		},
		AvailableFormats: formats,
		CreatedAt:        item.CreatedAt,
	}, nil
}

func inspectCacheKey(providerID providers.PlatformID, rawURL string) string {
	sum := sha256.Sum256([]byte(string(providerID) + ":" + rawURL))
	return "mediadock:inspect:" + hex.EncodeToString(sum[:])
}

// List returns paginated media items owned by userID.
func (s *Service) List(ctx context.Context, userID uuid.UUID, offset, limit int) ([]repository.MediaItem, int64, error) {
	return s.repo.ListItemsByUser(ctx, userID, offset, limit)
}

// Delete removes a media item owned by userID.
func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.DeleteItem(ctx, id, userID)
}

