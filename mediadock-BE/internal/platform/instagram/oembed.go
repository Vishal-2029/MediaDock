package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/utils"
)

type oEmbedResponse struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	ProviderName string `json:"provider_name"`
	ThumbnailURL string `json:"thumbnail_url"`
	HTML         string `json:"html"`
}

// FetchPublicPreview implements providers.PreviewProvider using Meta's official oEmbed API
// when INSTAGRAM_APP_ID and INSTAGRAM_APP_SECRET are configured. It returns public preview
// metadata (title, author, thumbnail) without requiring user login, while marking direct MP4
// download unavailable unless creator Graph API authorization or a licensed external source is used.
func (p *Provider) FetchPublicPreview(ctx context.Context, rawURL string) (*providers.MediaMetadata, *providers.AuthorizationResult, error) {
	if p.appID == "" || p.appSecret == "" {
		return nil, nil, providers.ErrAuthorizationRequired
	}

	parsed, err := p.validateURLWithContext(ctx, rawURL)
	if err != nil {
		return nil, nil, err
	}

	canonical := canonicalPermalink(parsed.String())
	endpoint := fmt.Sprintf("%s/instagram_oembed?url=%s&omitscript=true", p.graphAPIBase, url.QueryEscape(canonical))
	if !p.allowCustom {
		if _, err := utils.ValidatePublicURL(ctx, endpoint, true); err != nil {
			return nil, nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.appID+"|"+p.appSecret)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", providers.ErrMediaUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, nil, providers.ErrAuthorizationRequired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("%w: oEmbed status %d", providers.ErrMediaUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, nil, err
	}

	var oe oEmbedResponse
	if err := json.Unmarshal(body, &oe); err != nil {
		return nil, nil, fmt.Errorf("decode Instagram oEmbed response: %w", err)
	}

	title := strings.TrimSpace(oe.Title)
	if title == "" && strings.TrimSpace(oe.AuthorName) != "" {
		title = "Instagram Reel by @" + strings.TrimSpace(oe.AuthorName)
	}
	if title == "" {
		title = "Instagram Reel Preview"
	}
	if len(title) > 200 {
		title = title[:200]
	}

	meta := &providers.MediaMetadata{
		Provider:       providers.PlatformInstagram,
		SourceURL:      canonical,
		MediaType:      "video",
		Title:          title,
		ThumbnailURL:   strings.TrimSpace(oe.ThumbnailURL),
		Duration:       0,
		OriginalFormat: "mp4",
		FileSize:       0,
		OwnerUsername:  strings.TrimSpace(oe.AuthorName),
	}

	authRes := &providers.AuthorizationResult{
		Authorized:          false,
		PreviewOnly:         true,
		AuthorizationMethod: "meta_oembed_preview",
		AccountIdentifier:   strings.TrimSpace(oe.AuthorName),
		VerifiedAt:          time.Now().UTC(),
		Reason:              "Public preview loaded via official Meta oEmbed. Direct MP4 downloading from other accounts is not permitted by Meta Graph API without creator OAuth authorization or a licensed media provider.",
		Alternative:         "Provide the creator's OAuth token, configure a licensed external media API provider, or upload your saved video in Converters (/converters).",
	}

	return meta, authRes, nil
}

