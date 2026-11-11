package youtube

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/utils"
)

// Provider is a future-ready YouTube integration placeholder.
// General-purpose YouTube downloading is not enabled; only authorized creator workflows
// (e.g., YouTube Studio / Google Takeout / uploaded originals) are supported.
type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) ID() providers.PlatformID {
	return providers.PlatformYouTube
}

func (p *Provider) Name() string {
	return "YouTube (Future Authorized Integration)"
}

func (p *Provider) CanHandle(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "youtube.com" || host == "www.youtube.com" || host == "m.youtube.com" || host == "youtu.be"
}

func (p *Provider) ValidateURL(rawURL string) (*url.URL, error) {
	parsed, err := utils.ValidatePublicURL(context.Background(), rawURL, false)
	if err != nil {
		return nil, err
	}
	if !p.CanHandle(rawURL) {
		return nil, fmt.Errorf("%w: not a YouTube domain", utils.ErrInvalidURL)
	}
	return parsed, nil
}

func (p *Provider) VerifyAuthorization(_ context.Context, req providers.AuthCheckRequest) (*providers.AuthorizationResult, error) {
	if _, err := p.ValidateURL(req.URL); err != nil {
		return nil, err
	}
	return &providers.AuthorizationResult{
		Authorized:          false,
		AuthorizationMethod: "not_enabled",
		Reason:              "YouTube integration is reserved for future authorized creator APIs and does not permit general-purpose URL downloading.",
		Alternative:         "Export your own video via YouTube Studio or Google Takeout and upload it to /api/v1/conversions for format processing.",
	}, providers.ErrPlatformNotImplemented
}

func (p *Provider) FetchMetadata(_ context.Context, req providers.MetadataRequest) (*providers.MediaMetadata, error) {
	if _, err := p.ValidateURL(req.URL); err != nil {
		return nil, err
	}
	return nil, providers.ErrPlatformNotImplemented
}

func (p *Provider) CheckAvailability(_ context.Context, req providers.MetadataRequest) (*providers.AvailabilityResult, error) {
	if _, err := p.ValidateURL(req.URL); err != nil {
		return nil, err
	}
	return &providers.AvailabilityResult{
		Available:   false,
		Reason:      "YouTube integration is not yet enabled.",
		Alternative: "Use YouTube Studio download or upload your original file directly to /api/v1/conversions.",
	}, providers.ErrPlatformNotImplemented
}

func (p *Provider) ListFormats(_ context.Context, _ *providers.MediaMetadata) ([]providers.FormatOption, error) {
	return nil, providers.ErrPlatformNotImplemented
}

func (p *Provider) RetrieveMedia(_ context.Context, _ providers.RetrievalRequest) (*providers.RetrievedMedia, error) {
	return nil, providers.ErrPlatformNotImplemented
}

func (p *Provider) MapError(err error) *providers.ProviderError {
	return &providers.ProviderError{
		Provider:    providers.PlatformYouTube,
		Code:        "YOUTUBE_NOT_IMPLEMENTED",
		Message:     "YouTube downloading is not enabled. MediaDock does not bypass platform restrictions or perform unauthorized scraping.",
		Alternative: "Export your own videos via YouTube Studio or Google Takeout, or upload your original file to /api/v1/conversions.",
		HTTPStatus:  http.StatusNotImplemented,
		Cause:       err,
	}
}

