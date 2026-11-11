package generic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/utils"
)

// Provider handles direct user-owned media URLs (e.g., personal CDN/cloud object URLs)
// where the user attests ownership or permission and the target serves a valid media MIME type.
type Provider struct {
	httpClient  *http.Client
	allowCustom bool // only enabled in tests
}

func NewProvider(httpClient *http.Client) *Provider {
	if httpClient == nil {
		httpClient = utils.NewSafeHTTPClient(60*time.Second, 3)
	}
	return &Provider{httpClient: httpClient}
}

func NewTestProvider(httpClient *http.Client) *Provider {
	p := NewProvider(httpClient)
	p.allowCustom = true
	return p
}

func (p *Provider) ID() providers.PlatformID {
	return providers.PlatformGeneric
}

func (p *Provider) Name() string {
	return "Generic Direct Authorized Media"
}

func (p *Provider) CanHandle(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	ext := strings.ToLower(path.Ext(u.Path))
	return utils.AllowedMediaExtensions[ext]
}

func (p *Provider) ValidateURL(rawURL string) (*url.URL, error) {
	if p.allowCustom {
		parsed, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", utils.ErrInvalidURL, err)
		}
		return parsed, nil
	}
	parsed, err := utils.ValidatePublicURL(context.Background(), rawURL, false)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	if !utils.AllowedMediaExtensions[ext] {
		return nil, fmt.Errorf("%w: direct URL must point to a supported media file extension", utils.ErrInvalidURL)
	}
	return parsed, nil
}

func (p *Provider) VerifyAuthorization(_ context.Context, req providers.AuthCheckRequest) (*providers.AuthorizationResult, error) {
	if _, err := p.ValidateURL(req.URL); err != nil {
		return nil, err
	}
	if !req.OwnerAttested {
		return &providers.AuthorizationResult{
			Authorized:          false,
			AuthorizationMethod: "owner_attestation",
			Reason:              "Explicit ownership or permission attestation (owner_attested=true) is required for direct media URLs.",
			Alternative:         "Confirm you own or have permission to process this media, or upload the file directly to /api/v1/conversions.",
		}, providers.ErrAuthorizationRequired
	}
	return &providers.AuthorizationResult{
		Authorized:          true,
		AuthorizationMethod: "owner_attestation",
		VerifiedAt:          time.Now().UTC(),
	}, nil
}

func (p *Provider) FetchMetadata(ctx context.Context, req providers.MetadataRequest) (*providers.MediaMetadata, error) {
	parsed, err := p.ValidateURL(req.URL)
	if err != nil {
		return nil, err
	}
	if !req.OwnerAttested {
		return nil, providers.ErrAuthorizationRequired
	}
	if !p.allowCustom {
		if _, err := utils.ValidatePublicURL(ctx, parsed.String(), true); err != nil {
			return nil, err
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodHead, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", providers.ErrMediaUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: upstream status %d", providers.ErrMediaUnavailable, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	mediaMIME, _, _ := mime.ParseMediaType(contentType)
	if mediaMIME != "" && !utils.AllowedMediaMIMEs[mediaMIME] && mediaMIME != "application/octet-stream" {
		return nil, fmt.Errorf("%w: remote content-type %q is not a supported media type", utils.ErrUnsupportedFileType, mediaMIME)
	}

	ext := strings.TrimPrefix(strings.ToLower(path.Ext(parsed.Path)), ".")
	mediaType := classifyType(ext)
	title := utils.SanitizeFilename(path.Base(parsed.Path))

	var fileSize int64
	if resp.ContentLength > 0 {
		fileSize = resp.ContentLength
	}

	return &providers.MediaMetadata{
		Provider:       providers.PlatformGeneric,
		SourceURL:      parsed.String(),
		MediaType:      mediaType,
		Title:          title,
		Duration:       0,
		OriginalFormat: ext,
		FileSize:       fileSize,
		DownloadURL:    parsed.String(),
	}, nil
}

func (p *Provider) CheckAvailability(ctx context.Context, req providers.MetadataRequest) (*providers.AvailabilityResult, error) {
	if _, err := p.FetchMetadata(ctx, req); err != nil {
		return &providers.AvailabilityResult{
			Available: false,
			Reason:    err.Error(),
		}, err
	}
	return &providers.AvailabilityResult{Available: true}, nil
}

func (p *Provider) ListFormats(_ context.Context, meta *providers.MediaMetadata) ([]providers.FormatOption, error) {
	if meta == nil {
		return nil, errors.New("metadata required")
	}
	if meta.MediaType == "audio" {
		return []providers.FormatOption{
			{Format: meta.OriginalFormat, Quality: "original", Container: meta.OriginalFormat, Description: "Original audio", AudioOnly: true},
			{Format: "mp3", Quality: "audio", Container: "mp3", Description: "MP3 audio", AudioOnly: true},
			{Format: "aac", Quality: "audio", Container: "aac", Description: "AAC audio", AudioOnly: true},
			{Format: "wav", Quality: "audio", Container: "wav", Description: "WAV lossless audio", AudioOnly: true},
		}, nil
	}
	if meta.MediaType == "image" {
		return []providers.FormatOption{
			{Format: meta.OriginalFormat, Quality: "original", Container: meta.OriginalFormat, Description: "Original image"},
		}, nil
	}
	return []providers.FormatOption{
		{Format: meta.OriginalFormat, Quality: "original", Container: meta.OriginalFormat, Description: "Original video"},
		{Format: "mp4", Quality: "720p", Container: "mp4", Description: "MP4 720p"},
		{Format: "mp4", Quality: "480p", Container: "mp4", Description: "MP4 480p"},
		{Format: "mp3", Quality: "audio", Container: "mp3", Description: "Audio extraction (MP3)", AudioOnly: true},
	}, nil
}

func (p *Provider) RetrieveMedia(ctx context.Context, req providers.RetrievalRequest) (*providers.RetrievedMedia, error) {
	meta, err := p.FetchMetadata(ctx, providers.MetadataRequest{
		URL:           req.SourceURL,
		OwnerAttested: req.OwnerAttested,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, meta.DownloadURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d", providers.ErrMediaUnavailable, resp.StatusCode)
	}
	if req.MaxFileSize > 0 && resp.ContentLength > req.MaxFileSize {
		return nil, fmt.Errorf("%w: file size %d exceeds limit %d", utils.ErrFileTooLarge, resp.ContentLength, req.MaxFileSize)
	}

	f, err := os.Create(req.DestPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader = resp.Body
	if req.MaxFileSize > 0 {
		reader = io.LimitReader(resp.Body, req.MaxFileSize+1)
	}
	written, err := io.Copy(f, reader)
	if err != nil {
		return nil, err
	}
	if req.MaxFileSize > 0 && written > req.MaxFileSize {
		_ = os.Remove(req.DestPath)
		return nil, fmt.Errorf("%w: downloaded bytes exceed limit %d", utils.ErrFileTooLarge, req.MaxFileSize)
	}

	return &providers.RetrievedMedia{
		FilePath:    req.DestPath,
		Format:      meta.OriginalFormat,
		ContentType: resp.Header.Get("Content-Type"),
		FileSize:    written,
	}, nil
}

func (p *Provider) MapError(err error) *providers.ProviderError {
	switch {
	case errors.Is(err, providers.ErrAuthorizationRequired):
		return &providers.ProviderError{
			Provider:    providers.PlatformGeneric,
			Code:        "OWNERSHIP_ATTESTATION_REQUIRED",
			Message:     "Explicit ownership or permission attestation is required to process direct media URLs.",
			Alternative: "Set owner_attested=true if you own or are authorized to use this media.",
			HTTPStatus:  http.StatusForbidden,
			Cause:       err,
		}
	case errors.Is(err, utils.ErrSSRFBlocked), errors.Is(err, utils.ErrInvalidURL):
		return &providers.ProviderError{
			Provider:   providers.PlatformGeneric,
			Code:       "INVALID_OR_BLOCKED_URL",
			Message:    err.Error(),
			HTTPStatus: http.StatusBadRequest,
			Cause:      err,
		}
	case errors.Is(err, utils.ErrUnsupportedFileType):
		return &providers.ProviderError{
			Provider:   providers.PlatformGeneric,
			Code:       "UNSUPPORTED_MEDIA_TYPE",
			Message:    err.Error(),
			HTTPStatus: http.StatusUnsupportedMediaType,
			Cause:      err,
		}
	default:
		return &providers.ProviderError{
			Provider:   providers.PlatformGeneric,
			Code:       "GENERIC_PROVIDER_ERROR",
			Message:    "Could not retrieve authorized direct media.",
			HTTPStatus: http.StatusBadGateway,
			Cause:      err,
		}
	}
}

func classifyType(ext string) string {
	switch ext {
	case "mp3", "aac", "m4a", "wav", "flac", "ogg":
		return "audio"
	case "jpg", "jpeg", "png", "webp":
		return "image"
	default:
		return "video"
	}
}

