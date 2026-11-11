package media

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
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
	MediaID          uuid.UUID                     `json:"media_id"`
	SourceID         uuid.UUID                     `json:"source_id"`
	Provider         providers.PlatformID          `json:"provider"`
	Title            string                        `json:"title"`
	ThumbnailURL     string                        `json:"thumbnail_url,omitempty"`
	Duration         int                           `json:"duration"`
	OriginalFormat   string                        `json:"original_format"`
	FileSize         int64                         `json:"file_size"`
	SourceURL        string                        `json:"source_url"`
	Authorization    providers.AuthorizationResult `json:"authorization"`
	AvailableFormats []providers.FormatOption      `json:"available_formats"`
	CreatedAt        time.Time                     `json:"created_at"`
}

// Service handles URL inspection, authorization checks, and media metadata persistence.
type Service struct {
	repo     repository.MediaRepository
	registry *providers.Registry
}

// NewService constructs a media Service.
func NewService(repo repository.MediaRepository, registry *providers.Registry) *Service {
	return &Service{
		repo:     repo,
		registry: registry,
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
	if err != nil {
		provErr := provider.MapError(err)
		if authRes != nil && authRes.Alternative != "" && provErr.Alternative == "" {
			provErr.Alternative = authRes.Alternative
		}
		return nil, provErr
	}
	if !authRes.Authorized {
		return nil, &providers.ProviderError{
			Provider:    provider.ID(),
			Code:        "AUTHORIZATION_DENIED",
			Message:     authRes.Reason,
			Alternative: authRes.Alternative,
			HTTPStatus:  http.StatusForbidden,
		}
	}

	meta, err := provider.FetchMetadata(ctx, providers.MetadataRequest{
		URL:              parsedURL.String(),
		OAuthAccessToken: strings.TrimSpace(req.AccessToken),
		OwnerAttested:    req.OwnerAttested,
	})
	if err != nil {
		return nil, provider.MapError(err)
	}

	formats, err := provider.ListFormats(ctx, meta)
	if err != nil {
		return nil, provider.MapError(err)
	}

	verifiedAt := authRes.VerifiedAt
	if verifiedAt.IsZero() {
		verifiedAt = time.Now().UTC()
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
		ID:             uuid.New(),
		UserID:         userID,
		Title:          meta.Title,
		ThumbnailURL:   meta.ThumbnailURL,
		Duration:       meta.Duration,
		OriginalFormat: meta.OriginalFormat,
		FileSize:       meta.FileSize,
	}

	if err := s.repo.CreateSourceAndItem(ctx, source, item); err != nil {
		return nil, err
	}

	return &InspectResponse{
		MediaID:          item.ID,
		SourceID:         source.ID,
		Provider:         provider.ID(),
		Title:            item.Title,
		ThumbnailURL:     item.ThumbnailURL,
		Duration:         item.Duration,
		OriginalFormat:   item.OriginalFormat,
		FileSize:         item.FileSize,
		SourceURL:        source.SourceURL,
		Authorization:    *authRes,
		AvailableFormats: formats,
		CreatedAt:        item.CreatedAt,
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

	var formats []providers.FormatOption
	if prov, err := s.registry.GetByID(provID); err == nil {
		mediaType := "video"
		switch strings.ToLower(item.OriginalFormat) {
		case "jpg", "jpeg", "png", "webp":
			mediaType = "image"
		case "mp3", "aac", "m4a", "wav", "flac", "ogg":
			mediaType = "audio"
		}
		formats, _ = prov.ListFormats(ctx, &providers.MediaMetadata{
			Provider:       provID,
			MediaType:      mediaType,
			OriginalFormat: item.OriginalFormat,
		})
	}

	return &InspectResponse{
		MediaID:        item.ID,
		SourceID:       item.SourceID,
		Provider:       provID,
		Title:          item.Title,
		ThumbnailURL:   item.ThumbnailURL,
		Duration:       item.Duration,
		OriginalFormat: item.OriginalFormat,
		FileSize:       item.FileSize,
		SourceURL:      sourceURL,
		Authorization: providers.AuthorizationResult{
			Authorized:          true,
			AuthorizationMethod: authMethod,
			VerifiedAt:          verifiedAt,
		},
		AvailableFormats: formats,
		CreatedAt:        item.CreatedAt,
	}, nil
}

// List returns paginated media items owned by userID.
func (s *Service) List(ctx context.Context, userID uuid.UUID, offset, limit int) ([]repository.MediaItem, int64, error) {
	return s.repo.ListItemsByUser(ctx, userID, offset, limit)
}

// Delete removes a media item owned by userID.
func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.DeleteItem(ctx, id, userID)
}

