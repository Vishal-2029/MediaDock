package tests

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mediadock/backend/internal/modules/providers"
)

func TestProviderRegistry_Selection(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})
	reg := env.Deps.Registry

	cases := []struct {
		url      string
		expected providers.PlatformID
		wantErr  bool
	}{
		{"https://www.instagram.com/reel/CxyzOwned123/", providers.PlatformInstagram, false},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", providers.PlatformYouTube, false},
		{"https://www.facebook.com/watch/?v=12345", providers.PlatformFacebook, false},
		{"https://cdn.example.com/my-video.mp4", providers.PlatformGeneric, false},
		{"https://example.com/some-webpage", "", true},
	}

	for _, tc := range cases {
		p, err := reg.Resolve(tc.url)
		if tc.wantErr {
			if err == nil {
				t.Errorf("expected error resolving %q, got provider %s", tc.url, p.ID())
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error resolving %q: %v", tc.url, err)
			continue
		}
		if p.ID() != tc.expected {
			t.Errorf("Resolve(%q) = %s, want %s", tc.url, p.ID(), tc.expected)
		}
	}
}

func TestInstagramProvider_AuthorizationMetadataAndRetrieval(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})
	ctx := context.Background()

	igProv, err := env.Deps.Registry.GetByID(providers.PlatformInstagram)
	if err != nil {
		t.Fatalf("get instagram provider: %v", err)
	}

	ownedURL := "https://www.instagram.com/reel/CxyzOwned123/"
	unownedURL := "https://www.instagram.com/reel/SomeoneElse999/"

	// 1. Missing OAuth token -> authorization required
	_, err = igProv.VerifyAuthorization(ctx, providers.AuthCheckRequest{URL: ownedURL})
	if !errors.Is(err, providers.ErrAuthorizationRequired) {
		t.Fatalf("expected ErrAuthorizationRequired when token missing, got: %v", err)
	}

	// 2. Valid OAuth token for unowned media -> unauthorized
	_, err = igProv.VerifyAuthorization(ctx, providers.AuthCheckRequest{
		URL:              unownedURL,
		OAuthAccessToken: "valid-ig-oauth-token",
	})
	if !errors.Is(err, providers.ErrUnauthorizedMedia) {
		t.Fatalf("expected ErrUnauthorizedMedia for third-party media, got: %v", err)
	}

	// 3. Valid OAuth token for owned media -> authorized + metadata + formats + download
	authRes, err := igProv.VerifyAuthorization(ctx, providers.AuthCheckRequest{
		URL:              ownedURL,
		OAuthAccessToken: "valid-ig-oauth-token",
	})
	if err != nil || !authRes.Authorized {
		t.Fatalf("expected authorized result, got err=%v res=%+v", err, authRes)
	}

	meta, err := igProv.FetchMetadata(ctx, providers.MetadataRequest{
		URL:              ownedURL,
		OAuthAccessToken: "valid-ig-oauth-token",
	})
	if err != nil {
		t.Fatalf("FetchMetadata failed: %v", err)
	}
	if meta.Title != "My Own Authorized Reel" || meta.OriginalFormat != "mp4" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}

	formats, err := igProv.ListFormats(ctx, meta)
	if err != nil || len(formats) == 0 {
		t.Fatalf("expected available formats, got err=%v len=%d", err, len(formats))
	}

	dest := filepath.Join(env.TempDir, "ig-download.mp4")
	retrieved, err := igProv.RetrieveMedia(ctx, providers.RetrievalRequest{
		SourceURL:        ownedURL,
		OAuthAccessToken: "valid-ig-oauth-token",
		DestPath:         dest,
		MaxFileSize:      10 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("RetrieveMedia failed: %v", err)
	}
	if retrieved.FileSize <= 0 {
		t.Fatalf("expected downloaded bytes > 0")
	}
}

func TestFutureProviders_ExplicitComplianceErrors(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})
	ctx := context.Background()

	for _, id := range []providers.PlatformID{providers.PlatformYouTube, providers.PlatformFacebook} {
		p, err := env.Deps.Registry.GetByID(id)
		if err != nil {
			t.Fatalf("missing provider %s: %v", id, err)
		}
		target := "https://www.youtube.com/watch?v=abc"
		if id == providers.PlatformFacebook {
			target = "https://www.facebook.com/watch/?v=123"
		}
		res, err := p.VerifyAuthorization(ctx, providers.AuthCheckRequest{URL: target})
		if !errors.Is(err, providers.ErrPlatformNotImplemented) {
			t.Errorf("%s expected ErrPlatformNotImplemented, got: %v", id, err)
		}
		if res == nil || res.Alternative == "" {
			t.Errorf("%s expected non-empty alternative guidance", id)
		}
	}
}

