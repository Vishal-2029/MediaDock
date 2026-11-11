package instagram

import (
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

const (
	defaultGraphAPIBase = "https://graph.instagram.com"
	maxGraphPages       = 5
)

// Provider implements authorized media retrieval via the official Instagram Graph API.
// It never scrapes HTML pages, harvests session cookies, or accesses unauthorized third-party private media.
type Provider struct {
	graphAPIBase string
	httpClient   *http.Client
	defaultToken string
	allowCustom  bool // enabled only in tests when pointing to a mock Graph API server
}

// SetDefaultToken configures an optional server-side Instagram Graph API token for creator-owned media.
func (p *Provider) SetDefaultToken(token string) {
	p.defaultToken = strings.TrimSpace(token)
}

func (p *Provider) resolveToken(perRequest string) string {
	if trimmed := strings.TrimSpace(perRequest); trimmed != "" {
		return trimmed
	}
	return p.defaultToken
}

// NewProvider creates an Instagram provider backed by the official Graph API and SSRF-safe HTTP client.
func NewProvider(graphAPIBase string, httpClient *http.Client) *Provider {
	base := strings.TrimRight(strings.TrimSpace(graphAPIBase), "/")
	if base == "" {
		base = defaultGraphAPIBase
	}
	if httpClient == nil {
		httpClient = utils.NewSafeHTTPClient(60*time.Second, 5)
	}
	return &Provider{
		graphAPIBase: base,
		httpClient:   httpClient,
	}
}

// NewTestProvider creates a provider pointing to a local mock Graph API server for automated testing.
func NewTestProvider(graphAPIBase string, httpClient *http.Client) *Provider {
	p := NewProvider(graphAPIBase, httpClient)
	p.allowCustom = true
	return p
}

func (p *Provider) ID() providers.PlatformID {
	return providers.PlatformInstagram
}

func (p *Provider) Name() string {
	return "Instagram (Official Graph API)"
}

func (p *Provider) CanHandle(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "instagram.com" || host == "www.instagram.com" || host == "m.instagram.com"
}

func (p *Provider) ValidateURL(rawURL string) (*url.URL, error) {
	var parsed *url.URL
	var err error
	if p.allowCustom {
		parsed, err = url.Parse(strings.TrimSpace(rawURL))
	} else {
		parsed, err = utils.ValidatePublicURL(context.Background(), rawURL, false)
	}
	if err != nil {
		return nil, err
	}

	if strings.ToLower(parsed.Scheme) != "https" {
		return nil, fmt.Errorf("%w: Instagram URLs must use HTTPS", utils.ErrInvalidURL)
	}

	host := strings.ToLower(parsed.Hostname())
	if host != "instagram.com" && host != "www.instagram.com" && host != "m.instagram.com" {
		return nil, fmt.Errorf("%w: not an Instagram domain", utils.ErrInvalidURL)
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 {
		return nil, fmt.Errorf("%w: expected an Instagram post, reel, or video URL (e.g. /p/{code}/ or /reel/{code}/)", utils.ErrInvalidURL)
	}

	kind := strings.ToLower(segments[0])
	if kind != "p" && kind != "reel" && kind != "reels" && kind != "tv" {
		return nil, fmt.Errorf("%w: unsupported Instagram media path %q", utils.ErrInvalidURL, kind)
	}
	if strings.TrimSpace(segments[1]) == "" {
		return nil, fmt.Errorf("%w: missing Instagram media shortcode", utils.ErrInvalidURL)
	}

	return parsed, nil
}

type graphProfile struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type graphMediaItem struct {
	ID           string `json:"id"`
	Caption      string `json:"caption"`
	MediaType    string `json:"media_type"`
	MediaURL     string `json:"media_url"`
	Permalink    string `json:"permalink"`
	ThumbnailURL string `json:"thumbnail_url"`
	Username     string `json:"username"`
	Timestamp    string `json:"timestamp"`
}

type graphMediaResponse struct {
	Data   []graphMediaItem `json:"data"`
	Paging *struct {
		Next string `json:"next"`
	} `json:"paging"`
}

type graphErrorEnvelope struct {
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func (p *Provider) VerifyAuthorization(ctx context.Context, req providers.AuthCheckRequest) (*providers.AuthorizationResult, error) {
	if _, err := p.ValidateURL(req.URL); err != nil {
		return nil, err
	}

	token := p.resolveToken(req.OAuthAccessToken)
	if token == "" {
		return &providers.AuthorizationResult{
			Authorized:          false,
			AuthorizationMethod: "oauth_graph_api",
			Reason:              "Instagram does not permit general downloading of other accounts' Reels without creator authorization via Meta's official Graph API.",
			Alternative:         "To process media without login: upload a saved video/audio file in Converters, inspect a direct media URL you own, or use Instagram's 'Download Your Information' archive for your own account.",
		}, providers.ErrAuthorizationRequired
	}

	profile, err := p.fetchProfile(ctx, token)
	if err != nil {
		return nil, err
	}

	item, err := p.findAuthorizedMediaByPermalink(ctx, token, req.URL)
	if err != nil {
		return &providers.AuthorizationResult{
			Authorized:          false,
			AuthorizationMethod: "oauth_graph_api",
			AccountIdentifier:   profile.Username,
			Reason:              err.Error(),
			Alternative:         "Only media owned by or authorized for the authenticated Instagram Graph API account can be retrieved. For third-party content, request permission and the original file from the creator.",
		}, err
	}

	return &providers.AuthorizationResult{
		Authorized:          true,
		AuthorizationMethod: "oauth_graph_api",
		AccountIdentifier:   profile.Username,
		MediaIdentifier:     item.ID,
		VerifiedAt:          time.Now().UTC(),
	}, nil
}

func (p *Provider) FetchMetadata(ctx context.Context, req providers.MetadataRequest) (*providers.MediaMetadata, error) {
	parsed, err := p.ValidateURL(req.URL)
	if err != nil {
		return nil, err
	}

	token := p.resolveToken(req.OAuthAccessToken)
	if token == "" {
		return nil, providers.ErrAuthorizationRequired
	}

	item, err := p.findAuthorizedMediaByPermalink(ctx, token, parsed.String())
	if err != nil {
		return nil, err
	}

	if item.MediaURL == "" {
		return nil, fmt.Errorf("%w: Graph API did not return a downloadable media_url for this item", providers.ErrMediaUnavailable)
	}

	mediaType, origFormat := mapMediaType(item.MediaType, item.MediaURL)
	title := strings.TrimSpace(item.Caption)
	if title == "" {
		title = fmt.Sprintf("Instagram %s (%s)", mediaType, item.ID)
	}
	if len(title) > 200 {
		title = title[:200]
	}

	thumb := item.ThumbnailURL
	if thumb == "" && mediaType == "image" {
		thumb = item.MediaURL
	}

	fileSize := p.probeContentLength(ctx, item.MediaURL)

	return &providers.MediaMetadata{
		Provider:       providers.PlatformInstagram,
		SourceURL:      canonicalPermalink(parsed.String()),
		MediaID:        item.ID,
		MediaType:      mediaType,
		Title:          title,
		ThumbnailURL:   thumb,
		Duration:       0,
		OriginalFormat: origFormat,
		FileSize:       fileSize,
		DownloadURL:    item.MediaURL,
		OwnerUsername:  item.Username,
	}, nil
}

func (p *Provider) CheckAvailability(ctx context.Context, req providers.MetadataRequest) (*providers.AvailabilityResult, error) {
	meta, err := p.FetchMetadata(ctx, req)
	if err != nil {
		return &providers.AvailabilityResult{
			Available:   false,
			Reason:      err.Error(),
			Alternative: "Ensure your OAuth token is valid and you own this media, or upload the original file to /api/v1/conversions.",
		}, err
	}

	if meta.DownloadURL == "" {
		return &providers.AvailabilityResult{
			Available:   false,
			Reason:      "authorized media URL is currently unavailable",
			Alternative: "Use Instagram official data export or upload original media.",
		}, providers.ErrMediaUnavailable
	}

	return &providers.AvailabilityResult{
		Available: true,
	}, nil
}

func (p *Provider) ListFormats(_ context.Context, meta *providers.MediaMetadata) ([]providers.FormatOption, error) {
	if meta == nil {
		return nil, errors.New("media metadata is required")
	}

	if meta.MediaType == "image" {
		return []providers.FormatOption{
			{Format: meta.OriginalFormat, Quality: "original", Container: meta.OriginalFormat, Description: "Original authorized image"},
			{Format: "png", Quality: "original", Container: "png", Description: "PNG lossless image"},
			{Format: "webp", Quality: "original", Container: "webp", Description: "WebP image"},
		}, nil
	}

	return []providers.FormatOption{
		{Format: "mp4", Quality: "original", Container: "mp4", Description: "Original authorized MP4 video"},
		{Format: "mp4", Quality: "720p", Container: "mp4", Description: "Converted MP4 720p"},
		{Format: "mp4", Quality: "480p", Container: "mp4", Description: "Converted MP4 480p"},
		{Format: "mp3", Quality: "audio", Container: "mp3", Description: "Authorized audio extraction (MP3)", AudioOnly: true},
		{Format: "aac", Quality: "audio", Container: "aac", Description: "Authorized audio extraction (AAC)", AudioOnly: true},
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

	if !p.allowCustom {
		if _, err := utils.ValidatePublicURL(ctx, meta.DownloadURL, true); err != nil {
			return nil, err
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, meta.DownloadURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("download authorized Instagram media: %w", err)
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
			Provider:    providers.PlatformInstagram,
			Code:        "INSTAGRAM_AUTH_REQUIRED",
			Message:     "Instagram Reels from other accounts cannot be downloaded directly without the creator's official Meta Graph API authorization.",
			Alternative: "No login is needed for MediaDock: upload any saved video or audio file in Converters (/converters), or paste a direct media URL (.mp4, .mov, .webm, .mp3) you own or have permission to process.",
			HTTPStatus:  http.StatusForbidden,
			Cause:       err,
		}
	case errors.Is(err, providers.ErrUnauthorizedMedia):
		return &providers.ProviderError{
			Provider:    providers.PlatformInstagram,
			Code:        "INSTAGRAM_UNAUTHORIZED_MEDIA",
			Message:     "The requested Instagram media does not belong to the authorized account or cannot be accessed via the official Graph API.",
			Alternative: "MediaDock only retrieves media you own or have explicit API permission to access. Upload your original file for conversion.",
			HTTPStatus:  http.StatusForbidden,
			Cause:       err,
		}
	case errors.Is(err, providers.ErrMediaUnavailable):
		return &providers.ProviderError{
			Provider:    providers.PlatformInstagram,
			Code:        "INSTAGRAM_MEDIA_UNAVAILABLE",
			Message:     "The authorized Instagram media is unavailable or its temporary Graph API media URL expired.",
			Alternative: "Re-inspect the media URL with a fresh OAuth token.",
			HTTPStatus:  http.StatusBadGateway,
			Cause:       err,
		}
	case errors.Is(err, utils.ErrInvalidURL), errors.Is(err, utils.ErrSSRFBlocked):
		return &providers.ProviderError{
			Provider:   providers.PlatformInstagram,
			Code:       "INVALID_INSTAGRAM_URL",
			Message:    err.Error(),
			HTTPStatus: http.StatusBadRequest,
			Cause:      err,
		}
	default:
		return &providers.ProviderError{
			Provider:   providers.PlatformInstagram,
			Code:       "INSTAGRAM_PROVIDER_ERROR",
			Message:    "Unable to process Instagram request via official API.",
			HTTPStatus: http.StatusBadGateway,
			Cause:      err,
		}
	}
}

func (p *Provider) fetchProfile(ctx context.Context, token string) (*graphProfile, error) {
	endpoint := p.graphAPIBase + "/me?fields=id,username"
	var profile graphProfile
	if err := p.doGraphGET(ctx, endpoint, token, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (p *Provider) findAuthorizedMediaByPermalink(ctx context.Context, token, targetURL string) (*graphMediaItem, error) {
	want := canonicalPermalink(targetURL)
	nextURL := p.graphAPIBase + "/me/media?fields=id,caption,media_type,media_url,permalink,thumbnail_url,username,timestamp&limit=50"

	for page := 0; page < maxGraphPages && nextURL != ""; page++ {
		var resp graphMediaResponse
		if err := p.doGraphGET(ctx, nextURL, token, &resp); err != nil {
			return nil, err
		}
		for i := range resp.Data {
			if canonicalPermalink(resp.Data[i].Permalink) == want {
				return &resp.Data[i], nil
			}
		}
		if resp.Paging != nil && resp.Paging.Next != "" {
			nextURL = resp.Paging.Next
		} else {
			nextURL = ""
		}
	}

	return nil, providers.ErrUnauthorizedMedia
}

func (p *Provider) doGraphGET(ctx context.Context, endpoint, token string, out interface{}) error {
	if !p.allowCustom {
		if _, err := utils.ValidatePublicURL(ctx, endpoint, true); err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", providers.ErrMediaUnavailable, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return providers.ErrAuthorizationRequired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ge graphErrorEnvelope
		if json.Unmarshal(body, &ge) == nil && ge.Error != nil {
			if ge.Error.Code == 190 || ge.Error.Code == 10 || ge.Error.Code == 200 {
				return providers.ErrAuthorizationRequired
			}
			return fmt.Errorf("%w: %s", providers.ErrMediaUnavailable, ge.Error.Message)
		}
		return fmt.Errorf("%w: status %d", providers.ErrMediaUnavailable, resp.StatusCode)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode Instagram Graph API response: %w", err)
	}
	return nil
}

func (p *Provider) probeContentLength(ctx context.Context, rawURL string) int64 {
	if !p.allowCustom {
		if _, err := utils.ValidatePublicURL(ctx, rawURL, true); err != nil {
			return 0
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.ContentLength > 0 {
		return resp.ContentLength
	}
	return 0
}

func canonicalPermalink(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimRight(strings.ToLower(strings.TrimSpace(raw)), "/")
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	cleanPath := strings.TrimRight(u.Path, "/")
	// Normalize /reels/ to /reel/ for matching
	cleanPath = strings.Replace(cleanPath, "/reels/", "/reel/", 1)
	return "https://" + host + cleanPath
}

func mapMediaType(graphType, mediaURL string) (mediaType, ext string) {
	switch strings.ToUpper(graphType) {
	case "VIDEO", "REELS":
		return "video", "mp4"
	case "IMAGE", "CAROUSEL_ALBUM":
		ext := strings.TrimPrefix(strings.ToLower(path.Ext(mediaURL)), ".")
		if ext == "" || len(ext) > 5 {
			ext = "jpg"
		}
		return "image", ext
	default:
		return "video", "mp4"
	}
}

