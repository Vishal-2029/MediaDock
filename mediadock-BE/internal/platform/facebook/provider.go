package facebook

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/utils"
)

// Provider is a future-ready Facebook integration placeholder.
// General-purpose scraping or unauthorized downloads are never implemented.
type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) ID() providers.PlatformID {
	return providers.PlatformFacebook
}

func (p *Provider) Name() string {
	return "Facebook (Future Authorized Integration)"
}

func (p *Provider) CanHandle(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "facebook.com" || host == "www.facebook.com" || host == "m.facebook.com" || host == "fb.watch"
}

func (p *Provider) ValidateURL(rawURL string) (*url.URL, error) {
	parsed, err := utils.ValidatePublicURL(context.Background(), rawURL, false)
	if err != nil {
		return nil, err
	}
	if !p.CanHandle(rawURL) {
		return nil, fmt.Errorf("%w: not a Facebook domain", utils.ErrInvalidURL)
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
		Reason:              "Facebook integration is reserved for future authorized Meta Graph API access and is not enabled yet.",
		Alternative:         "Use Facebook's official 'Download Your Information' export or upload your original media file to /api/v1/conversions.",
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
		Reason:      "Facebook integration is not yet enabled.",
		Alternative: "Export your own media from Facebook Account Center or upload the original file to /api/v1/conversions.",
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
		Provider:    providers.PlatformFacebook,
		Code:        "FACEBOOK_NOT_IMPLEMENTED",
		Message:     "Facebook downloading is not enabled. MediaDock only supports authorized API integrations.",
		Alternative: "Export your own media via Facebook's official tools or upload your original file directly to /api/v1/conversions.",
		HTTPStatus:  http.StatusNotImplemented,
		Cause:       err,
	}
}

