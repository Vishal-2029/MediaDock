package external

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/utils"
)

// Provider is an adapter for a commercially licensed third-party media resolution API
// configured via EXTERNAL_MEDIA_API_BASE and EXTERNAL_MEDIA_API_KEY.
// It never scrapes HTML pages or harvests user credentials; it delegates resolution to
// the configured licensed upstream API and enforces SSRF + size protections locally.
type Provider struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	allowCustom bool // enabled only in tests
}

// NewProvider creates a licensed external media API provider adapter.
func NewProvider(baseURL, apiKey string, httpClient *http.Client) *Provider {
	if httpClient == nil {
		httpClient = utils.NewSafeHTTPClient(60*time.Second, 5)
	}
	return &Provider{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: httpClient,
	}
}

// NewTestProvider creates an adapter pointing to a local mock API server in automated tests.
func NewTestProvider(baseURL, apiKey string, httpClient *http.Client) *Provider {
	p := NewProvider(baseURL, apiKey, httpClient)
	p.allowCustom = true
	return p
}

func (p *Provider) Enabled() bool {
	return p.baseURL != "" && p.apiKey != ""
}

func (p *Provider) ID() providers.PlatformID {
	return providers.PlatformID("external")
}

func (p *Provider) Name() string {
	return "Licensed External Media Provider"
}

func (p *Provider) CanHandle(rawURL string) bool {
	if !p.Enabled() {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "instagram.com", "www.instagram.com", "m.instagram.com",
		"youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be",
		"facebook.com", "www.facebook.com", "m.facebook.com", "fb.watch":
		return true
	default:
		return false
	}
}

func (p *Provider) ValidateURL(rawURL string) (*url.URL, error) {
	if p.allowCustom {
		return url.Parse(strings.TrimSpace(rawURL))
	}
	parsed, err := utils.ValidatePublicURL(context.Background(), rawURL, false)
	if err != nil {
		return nil, err
	}
	if !p.CanHandle(parsed.String()) {
		return nil, fmt.Errorf("%w: URL host is not handled by external provider", utils.ErrInvalidURL)
	}
	return parsed, nil
}

func (p *Provider) VerifyAuthorization(ctx context.Context, req providers.AuthCheckRequest) (*providers.AuthorizationResult, error) {
	if !p.Enabled() {
		return nil, providers.ErrPlatformNotImplemented
	}
	if _, err := p.ValidateURL(req.URL); err != nil {
		return nil, err
	}
	return &providers.AuthorizationResult{
		Authorized:          true,
		AuthorizationMethod: "licensed_external_api",
		VerifiedAt:          time.Now().UTC(),
	}, nil
}

type inspectAPIRequest struct {
	URL string `json:"url"`
}

type inspectAPIResponse struct {
	ID            string `json:"id"`
	Platform      string `json:"platform"`
	Title         string `json:"title"`
	MediaType     string `json:"media_type"`
	ThumbnailURL  string `json:"thumbnail_url"`
	Duration      int    `json:"duration"`
	Format        string `json:"format"`
	FileSize      int64  `json:"file_size"`
	DownloadURL   string `json:"download_url"`
	OwnerUsername string `json:"owner_username"`
}

func (p *Provider) FetchMetadata(ctx context.Context, req providers.MetadataRequest) (*providers.MediaMetadata, error) {
	if !p.Enabled() {
		return nil, providers.ErrPlatformNotImplemented
	}
	parsed, err := p.ValidateURL(req.URL)
	if err != nil {
		return nil, err
	}

	endpoint := p.baseURL + "/v1/inspect"
	if !p.allowCustom {
		if _, err := utils.ValidatePublicURL(ctx, endpoint, true); err != nil {
			return nil, err
		}
	}

	payload, err := json.Marshal(inspectAPIRequest{URL: parsed.String()})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", providers.ErrMediaUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, providers.ErrAuthorizationRequired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: external provider returned status %d", providers.ErrMediaUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}

	var apiResp inspectAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("decode external provider response: %w", err)
	}
	if strings.TrimSpace(apiResp.DownloadURL) == "" {
		return nil, fmt.Errorf("%w: external provider did not return a download_url", providers.ErrMediaUnavailable)
	}

	if !p.allowCustom {
		if _, err := utils.ValidatePublicURL(ctx, apiResp.DownloadURL, true); err != nil {
			return nil, err
		}
	}

	origFmt := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(apiResp.Format)), ".")
	if origFmt == "" {
		origFmt = strings.TrimPrefix(strings.ToLower(path.Ext(apiResp.DownloadURL)), ".")
	}
	if origFmt == "" {
		origFmt = "mp4"
	}

	mediaType := strings.ToLower(strings.TrimSpace(apiResp.MediaType))
	if mediaType == "" {
		mediaType = "video"
	}
	title := strings.TrimSpace(apiResp.Title)
	if title == "" {
		title = "Resolved Media"
	}

	return &providers.MediaMetadata{
		Provider:       detectDisplayPlatform(parsed.Hostname(), apiResp.Platform),
		SourceURL:      parsed.String(),
		MediaID:        apiResp.ID,
		MediaType:      mediaType,
		Title:          title,
		ThumbnailURL:   strings.TrimSpace(apiResp.ThumbnailURL),
		Duration:       apiResp.Duration,
		OriginalFormat: origFmt,
		FileSize:       apiResp.FileSize,
		DownloadURL:    strings.TrimSpace(apiResp.DownloadURL),
		OwnerUsername:  strings.TrimSpace(apiResp.OwnerUsername),
	}, nil
}

func (p *Provider) CheckAvailability(ctx context.Context, req providers.MetadataRequest) (*providers.AvailabilityResult, error) {
	meta, err := p.FetchMetadata(ctx, req)
	if err != nil {
		return &providers.AvailabilityResult{Available: false, Reason: err.Error()}, err
	}
	return &providers.AvailabilityResult{Available: meta.DownloadURL != ""}, nil
}

func (p *Provider) ListFormats(_ context.Context, meta *providers.MediaMetadata) ([]providers.FormatOption, error) {
	if meta == nil {
		return nil, errors.New("media metadata is required")
	}
	if meta.MediaType == "image" {
		return []providers.FormatOption{
			{Format: meta.OriginalFormat, Quality: "original", Container: meta.OriginalFormat, Description: "Original image"},
		}, nil
	}
	return []providers.FormatOption{
		{Format: "mp4", Quality: "original", Container: "mp4", Description: "Original MP4 video"},
		{Format: "mp4", Quality: "720p", Container: "mp4", Description: "Converted MP4 720p"},
		{Format: "mp4", Quality: "480p", Container: "mp4", Description: "Converted MP4 480p"},
		{Format: "mp3", Quality: "audio", Container: "mp3", Description: "Audio extraction (MP3)", AudioOnly: true},
		{Format: "aac", Quality: "audio", Container: "aac", Description: "Audio extraction (AAC)", AudioOnly: true},
	}, nil
}

func (p *Provider) RetrieveMedia(ctx context.Context, req providers.RetrievalRequest) (*providers.RetrievedMedia, error) {
	meta, err := p.FetchMetadata(ctx, providers.MetadataRequest{
		URL:              req.SourceURL,
		OAuthAccessToken: req.OAuthAccessToken,
		OwnerAttested:    req.OwnerAttested,
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
		return nil, fmt.Errorf("download external media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: upstream returned status %d", providers.ErrMediaUnavailable, resp.StatusCode)
	}
	if req.MaxFileSize > 0 && resp.ContentLength > req.MaxFileSize {
		return nil, fmt.Errorf("%w: upstream file size %d exceeds limit %d", utils.ErrFileTooLarge, resp.ContentLength, req.MaxFileSize)
	}

	outFile, err := os.Create(req.DestPath)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()

	var reader io.Reader = resp.Body
	if req.MaxFileSize > 0 {
		reader = io.LimitReader(resp.Body, req.MaxFileSize+1)
	}
	written, err := io.Copy(outFile, reader)
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
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, providers.ErrAuthorizationRequired):
		return &providers.ProviderError{
			Provider:   p.ID(),
			Code:       "EXTERNAL_PROVIDER_AUTH_FAILED",
			Message:    "Configured external media provider rejected authorization credentials.",
			HTTPStatus: http.StatusBadGateway,
			Cause:      err,
		}
	case errors.Is(err, utils.ErrInvalidURL), errors.Is(err, utils.ErrSSRFBlocked):
		return &providers.ProviderError{
			Provider:   p.ID(),
			Code:       "INVALID_OR_BLOCKED_URL",
			Message:    err.Error(),
			HTTPStatus: http.StatusBadRequest,
			Cause:      err,
		}
	default:
		return &providers.ProviderError{
			Provider:   p.ID(),
			Code:       "EXTERNAL_PROVIDER_ERROR",
			Message:    "Unable to resolve media via external provider.",
			HTTPStatus: http.StatusBadGateway,
			Cause:      err,
		}
	}
}

func detectDisplayPlatform(host, hint string) providers.PlatformID {
	switch strings.ToLower(strings.TrimSpace(hint)) {
	case "instagram":
		return providers.PlatformInstagram
	case "youtube":
		return providers.PlatformYouTube
	case "facebook":
		return providers.PlatformFacebook
	}
	h := strings.ToLower(host)
	switch {
	case strings.Contains(h, "instagram.com"):
		return providers.PlatformInstagram
	case strings.Contains(h, "youtube.com"), h == "youtu.be":
		return providers.PlatformYouTube
	case strings.Contains(h, "facebook.com"), h == "fb.watch":
		return providers.PlatformFacebook
	default:
		return providers.PlatformID("external")
	}
}

