package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/cache"
	"github.com/mediadock/backend/internal/middleware"
	"github.com/mediadock/backend/internal/modules/storage"
	"github.com/mediadock/backend/internal/queue"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/utils"
	"github.com/redis/go-redis/v9"
)

func TestOpenAPISpecEndpoint(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil)
	resp, err := env.App.Test(req, -1)
	if err != nil {
		t.Fatalf("GET /api/v1/openapi.yaml failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "MediaDock REST API") {
		t.Fatalf("expected OpenAPI spec title in response")
	}
}

func TestHttpOnlyCookieAuthAndProfileUpdate(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})
	registerUser(t, env, "Cookie User", "cookie@example.com", "StrongPass123!")

	loginPayload, _ := json.Marshal(map[string]string{
		"email":    "cookie@example.com",
		"password": "StrongPass123!",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginPayload))
	loginReq.Header.Set("Content-Type", "application/json")

	loginResp, err := env.App.Test(loginReq, -1)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	_ = loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on login, got %d", loginResp.StatusCode)
	}

	var accessCookie, refreshCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == "mediadock_access" {
			accessCookie = c
		}
		if c.Name == "mediadock_refresh" {
			refreshCookie = c
		}
	}
	if accessCookie == nil || refreshCookie == nil || !accessCookie.HttpOnly || !refreshCookie.HttpOnly {
		t.Fatalf("expected HttpOnly access and refresh cookies to be set on login")
	}

	// GET /auth/me using only the HttpOnly cookie
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.AddCookie(accessCookie)
	meResp, err := env.App.Test(meReq, -1)
	if err != nil {
		t.Fatalf("GET /auth/me failed: %v", err)
	}
	var meBody map[string]interface{}
	_ = json.NewDecoder(meResp.Body).Decode(&meBody)
	_ = meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from cookie auth /auth/me, got %d: %v", meResp.StatusCode, meBody)
	}

	// PUT /auth/me updates display name
	updatePayload, _ := json.Marshal(map[string]string{"name": "Updated Cookie User"})
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/auth/me", bytes.NewReader(updatePayload))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.AddCookie(accessCookie)
	putResp, err := env.App.Test(putReq, -1)
	if err != nil {
		t.Fatalf("PUT /auth/me failed: %v", err)
	}
	var putBody map[string]interface{}
	_ = json.NewDecoder(putResp.Body).Decode(&putBody)
	_ = putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from PUT /auth/me, got %d: %v", putResp.StatusCode, putBody)
	}
	updatedName := putBody["data"].(map[string]interface{})["name"].(string)
	if updatedName != "Updated Cookie User" {
		t.Fatalf("expected updated name 'Updated Cookie User', got %q", updatedName)
	}

	// POST /auth/logout clears cookies and revokes access token
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(accessCookie)
	logoutReq.AddCookie(refreshCookie)
	logoutResp, err := env.App.Test(logoutReq, -1)
	if err != nil {
		t.Fatalf("POST /auth/logout failed: %v", err)
	}
	_ = logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", logoutResp.StatusCode)
	}

	// Access cookie should now be rejected
	meAgainReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meAgainReq.AddCookie(accessCookie)
	meAgainResp, _ := env.App.Test(meAgainReq, -1)
	_ = meAgainResp.Body.Close()
	if meAgainResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout revocation, got %d", meAgainResp.StatusCode)
	}
}

func TestMediaLibraryListDeleteAndDownloadsListIsolation(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})
	aliceToken, _ := registerUser(t, env, "Alice", "alice2@example.com", "StrongPass123!")
	bobToken, _ := registerUser(t, env, "Bob", "bob2@example.com", "StrongPass123!")

	// Alice inspects an authorized Instagram reel
	resp, body := doJSONRequest(t, env.App, http.MethodPost, "/api/v1/media/inspect", aliceToken, map[string]interface{}{
		"url":          "https://www.instagram.com/reel/CxyzOwned123/",
		"access_token": "valid-ig-oauth-token",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("inspect failed: %d %v", resp.StatusCode, body)
	}
	mediaID := body["data"].(map[string]interface{})["media_id"].(string)

	// Alice creates a download job
	resp, body = doJSONRequest(t, env.App, http.MethodPost, "/api/v1/downloads", aliceToken, map[string]interface{}{
		"media_id":          mediaID,
		"requested_format":  "mp4",
		"requested_quality": "original",
		"access_token":      "valid-ig-oauth-token",
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("create download failed: %d %v", resp.StatusCode, body)
	}

	// GET /api/v1/media returns 1 item for Alice and 0 for Bob
	resp, body = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/media", aliceToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /media failed: %d %v", resp.StatusCode, body)
	}
	if items := body["data"].([]interface{}); len(items) != 1 {
		t.Fatalf("expected 1 media item for Alice, got %d", len(items))
	}

	resp, body = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/media", bobToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /media for Bob failed: %d", resp.StatusCode)
	}
	if items := body["data"].([]interface{}); len(items) != 0 {
		t.Fatalf("expected 0 media items for Bob, got %d", len(items))
	}

	// GET /api/v1/downloads returns Alice's job and 0 for Bob
	resp, body = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/downloads", aliceToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /downloads failed: %d %v", resp.StatusCode, body)
	}
	if jobs := body["data"].([]interface{}); len(jobs) != 1 {
		t.Fatalf("expected 1 download job for Alice, got %d", len(jobs))
	}

	resp, body = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/downloads", bobToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /downloads for Bob failed: %d", resp.StatusCode)
	}
	if jobs := body["data"].([]interface{}); len(jobs) != 0 {
		t.Fatalf("expected 0 download jobs for Bob, got %d", len(jobs))
	}

	// Bob cannot delete Alice's media item (404)
	resp, _ = doJSONRequest(t, env.App, http.MethodDelete, "/api/v1/media/"+mediaID, bobToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when Bob deletes Alice's media, got %d", resp.StatusCode)
	}

	// Alice deletes her media item
	resp, _ = doJSONRequest(t, env.App, http.MethodDelete, "/api/v1/media/"+mediaID, aliceToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 when Alice deletes her media, got %d", resp.StatusCode)
	}
}

func TestRateLimitMiddlewareReturns429(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	redisCache := cache.NewFromClient(rdb)

	app := fiber.New()
	app.Use(middleware.RateLimit(redisCache, 3, time.Minute))
	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	for i := 0; i < 3; i++ {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/ping", nil), -1)
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/ping", nil), -1)
	if err != nil {
		t.Fatalf("4th request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests on 4th request, got %d", resp.StatusCode)
	}
}

func TestSafeHTTPClientBlocksRedirectToLocalhost(t *testing.T) {
	client := utils.NewSafeHTTPClient(5*time.Second, 3)

	// Directly calling CheckRedirect simulates a server redirecting to loopback.
	redirectReq := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/secret", nil)
	prevReq := httptest.NewRequest(http.MethodGet, "https://example.com/public", nil)

	err := client.CheckRedirect(redirectReq, []*http.Request{prevReq})
	if err == nil || !errors.Is(err, utils.ErrSSRFBlocked) {
		t.Fatalf("expected ErrSSRFBlocked on redirect to 127.0.0.1, got %v", err)
	}
}

func TestLocalStorageSignedURLVerification(t *testing.T) {
	store, err := storage.NewLocalStorage(t.TempDir(), "super-secret-signing-key-32-bytes!!")
	if err != nil {
		t.Fatalf("NewLocalStorage failed: %v", err)
	}

	key := "downloads/user1/clip.mp4"
	qs, err := store.SignedURL(context.Background(), key, 5*time.Minute)
	if err != nil {
		t.Fatalf("SignedURL failed: %v", err)
	}

	parsed, err := url.ParseQuery(strings.TrimPrefix(qs, "?"))
	if err != nil {
		t.Fatalf("parse signed query failed: %v", err)
	}
	expStr := parsed.Get("exp")
	sig := parsed.Get("sig")

	if err := store.VerifySignedToken(key, expStr, sig); err != nil {
		t.Fatalf("expected valid signed token, got %v", err)
	}
	if err := store.VerifySignedToken(key, expStr, "tampered-signature"); !errors.Is(err, storage.ErrInvalidSigned) {
		t.Fatalf("expected ErrInvalidSigned for tampered signature, got %v", err)
	}
	if err := store.VerifySignedToken(key, "1000000", sig); !errors.Is(err, storage.ErrInvalidSigned) {
		t.Fatalf("expected ErrInvalidSigned for expired timestamp, got %v", err)
	}
}

func TestHistoryApplyRetentionDeletesOldRecordsOnly(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})

	oldRec := &repository.DownloadHistory{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		JobID:        uuid.New(),
		FileName:     "old-clip.mp4",
		DownloadedAt: time.Now().UTC().AddDate(0, 0, -120),
	}
	recentRec := &repository.DownloadHistory{
		ID:           uuid.New(),
		UserID:       oldRec.UserID,
		JobID:        uuid.New(),
		FileName:     "recent-clip.mp4",
		DownloadedAt: time.Now().UTC().AddDate(0, 0, -5),
	}
	if err := env.DB.Create(oldRec).Error; err != nil {
		t.Fatalf("create old history record: %v", err)
	}
	if err := env.DB.Create(recentRec).Error; err != nil {
		t.Fatalf("create recent history record: %v", err)
	}

	deleted, err := env.Deps.HistoryService.ApplyRetention(context.Background())
	if err != nil {
		t.Fatalf("ApplyRetention failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 record deleted by retention, got %d", deleted)
	}

	var remaining int64
	if err := env.DB.Model(&repository.DownloadHistory{}).Where("id = ?", recentRec.ID).Count(&remaining).Error; err != nil {
		t.Fatalf("count remaining history: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected recent record to remain, got count %d", remaining)
	}
}

func TestWorkerQueueMovesExhaustedJobToDLQAndStripsToken(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	redisCache := cache.NewFromClient(rdb)

	wq := queue.NewWorkerQueue(redisCache, slog.New(slog.NewTextHandler(io.Discard, nil)), 1, 5*time.Second, 2)
	wq.RegisterHandler(queue.JobTypeDownload, func(ctx context.Context, payload queue.JobPayload) error {
		return errors.New("simulated processing failure")
	})

	payload := queue.JobPayload{
		ID:         uuid.New(),
		Type:       queue.JobTypeDownload,
		Attempt:    2,
		MaxRetries: 2,
		Token:      "sensitive-ephemeral-token",
		EnqueuedAt: time.Now().UTC(),
	}

	wq.ProcessOne(context.Background(), payload)

	items, err := rdb.LRange(context.Background(), "mediadock:jobs:dlq", 0, -1).Result()
	if err != nil {
		t.Fatalf("read DLQ failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 job in DLQ, got %d", len(items))
	}

	var dlqPayload queue.JobPayload
	if err := json.Unmarshal([]byte(items[0]), &dlqPayload); err != nil {
		t.Fatalf("unmarshal DLQ payload: %v", err)
	}
	if dlqPayload.ID != payload.ID {
		t.Fatalf("expected DLQ job ID %s, got %s", payload.ID, dlqPayload.ID)
	}
	if dlqPayload.Token != "" {
		t.Fatalf("expected ephemeral token to be stripped in DLQ, got %q", dlqPayload.Token)
	}
}

func TestGuestWorkspaceNoLoginRequired(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})

	// Settings GET and PUT work without any auth header or login cookie
	resp, body := doJSONRequest(t, env.App, http.MethodGet, "/api/v1/settings", "", nil)
	if resp.StatusCode != http.StatusOK || body["success"] != true {
		t.Fatalf("expected guest GET /settings 200, got %d (%v)", resp.StatusCode, body)
	}

	resp, body = doJSONRequest(t, env.App, http.MethodPut, "/api/v1/settings", "", map[string]string{
		"theme":          "light",
		"default_format": "mp3",
	})
	if resp.StatusCode != http.StatusOK || body["success"] != true {
		t.Fatalf("expected guest PUT /settings 200, got %d (%v)", resp.StatusCode, body)
	}

	// Media list, download list, and history list work without login
	for _, path := range []string{"/api/v1/media", "/api/v1/downloads", "/api/v1/history"} {
		resp, body = doJSONRequest(t, env.App, http.MethodGet, path, "", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected guest GET %s 200, got %d (%v)", path, resp.StatusCode, body)
		}
	}

	// Unauthenticated third-party Instagram Reel returns clear 403 compliance notice (not 401 login error)
	resp, body = doJSONRequest(t, env.App, http.MethodPost, "/api/v1/media/inspect", "", map[string]interface{}{
		"url": "https://www.instagram.com/reel/Dfj0Z-OMD19/",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 compliance error for unauthenticated Instagram reel, got %d (%v)", resp.StatusCode, body)
	}

	// Authorized creator media inspect works without app login
	resp, body = doJSONRequest(t, env.App, http.MethodPost, "/api/v1/media/inspect", "", map[string]interface{}{
		"url":          "https://www.instagram.com/reel/CxyzOwned123/",
		"access_token": "valid-ig-oauth-token",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected guest creator media inspect 200, got %d (%v)", resp.StatusCode, body)
	}
}
