package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/repository"
)

func registerUser(t *testing.T, env *testEnv, name, email, password string) (string, string) {
	t.Helper()
	resp, body := doJSONRequest(t, env.App, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"name":     name,
		"email":    email,
		"password": password,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register %s failed (%d): %v", email, resp.StatusCode, body)
	}
	data := body["data"].(map[string]interface{})
	tokens := data["tokens"].(map[string]interface{})
	return tokens["access_token"].(string), tokens["refresh_token"].(string)
}

func TestHealthAndReady(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})

	resp, body := doJSONRequest(t, env.App, http.MethodGet, "/api/v1/health", "", nil)
	if resp.StatusCode != http.StatusOK || body["success"] != true {
		t.Fatalf("health check failed: %d %v", resp.StatusCode, body)
	}

	resp, body = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/ready", "", nil)
	if resp.StatusCode != http.StatusOK || body["success"] != true {
		t.Fatalf("ready check failed: %d %v", resp.StatusCode, body)
	}
}

func TestAuthLifecycle_RegisterLoginRefreshLogoutSettings(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})

	access, refresh := registerUser(t, env, "Alice", "alice@example.com", "StrongPass123!")

	// Duplicate email rejected
	resp, _ := doJSONRequest(t, env.App, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"name":     "Alice2",
		"email":    "alice@example.com",
		"password": "StrongPass123!",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate email, got %d", resp.StatusCode)
	}

	// Login works
	resp, body := doJSONRequest(t, env.App, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email":    "alice@example.com",
		"password": "StrongPass123!",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: %d %v", resp.StatusCode, body)
	}

	// Wrong password rejected
	resp, _ = doJSONRequest(t, env.App, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email":    "alice@example.com",
		"password": "WrongPassword",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", resp.StatusCode)
	}

	// /auth/me works
	resp, body = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/auth/me", access, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/auth/me failed: %d %v", resp.StatusCode, body)
	}

	// Settings GET and PUT
	resp, _ = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/settings", access, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /settings failed: %d", resp.StatusCode)
	}
	resp, body = doJSONRequest(t, env.App, http.MethodPut, "/api/v1/settings", access, map[string]string{
		"theme":          "dark",
		"default_format": "mp3",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /settings failed: %d %v", resp.StatusCode, body)
	}

	// Refresh rotates token
	resp, body = doJSONRequest(t, env.App, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": refresh,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh failed: %d %v", resp.StatusCode, body)
	}
	tokens := body["data"].(map[string]interface{})
	newAccess := tokens["access_token"].(string)
	newRefresh := tokens["refresh_token"].(string)

	// Reusing old refresh token must fail
	resp, _ = doJSONRequest(t, env.App, http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": refresh,
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 when reusing rotated refresh token, got %d", resp.StatusCode)
	}

	// Logout revokes access & refresh token
	resp, _ = doJSONRequest(t, env.App, http.MethodPost, "/api/v1/auth/logout", newAccess, map[string]string{
		"refresh_token": newRefresh,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout failed: %d", resp.StatusCode)
	}

	// Revoked access token rejected on /auth/me
	resp, _ = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/auth/me", newAccess, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for revoked access token, got %d", resp.StatusCode)
	}
}

func TestDownloadPipeline_PermissionsHistoryAndExpiration(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})

	ctx, cancel := context.WithCancel(context.Background())
	env.Deps.Queue.Start(ctx)
	t.Cleanup(func() {
		cancel()
		env.Deps.Queue.Wait()
	})

	aliceToken, _ := registerUser(t, env, "Alice", "alice2@example.com", "StrongPass123!")
	bobToken, _ := registerUser(t, env, "Bob", "bob@example.com", "StrongPass123!")

	// Inspect owned Instagram URL
	resp, body := doJSONRequest(t, env.App, http.MethodPost, "/api/v1/media/inspect", aliceToken, map[string]interface{}{
		"url":          "https://www.instagram.com/reel/CxyzOwned123/",
		"access_token": "valid-ig-oauth-token",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("inspect failed: %d %v", resp.StatusCode, body)
	}
	mediaData := body["data"].(map[string]interface{})
	mediaID := mediaData["media_id"].(string)

	// Bob cannot read Alice's media item
	resp, _ = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/media/"+mediaID, bobToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when Bob accesses Alice's media, got %d", resp.StatusCode)
	}

	// Create download job (transcoded to mp3 audio extraction)
	resp, body = doJSONRequest(t, env.App, http.MethodPost, "/api/v1/downloads", aliceToken, map[string]interface{}{
		"media_id":          mediaID,
		"requested_format":  "mp3",
		"requested_quality": "audio",
		"access_token":      "valid-ig-oauth-token",
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("create download failed: %d %v", resp.StatusCode, body)
	}
	jobData := body["data"].(map[string]interface{})
	jobID := jobData["id"].(string)

	// Poll status until worker finishes
	var finalStatus string
	for i := 0; i < 40; i++ {
		resp, body = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/downloads/"+jobID+"/status", aliceToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status check failed: %d %v", resp.StatusCode, body)
		}
		st := body["data"].(map[string]interface{})["status"].(string)
		if st == "completed" || st == "failed" {
			finalStatus = st
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if finalStatus != "completed" {
		t.Fatalf("expected job status 'completed', got %q (body=%v)", finalStatus, body)
	}

	// Bob cannot view or download Alice's job
	resp, _ = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/downloads/"+jobID+"/file", bobToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when Bob downloads Alice's file, got %d", resp.StatusCode)
	}

	// Alice downloads file
	req := httptest.NewRequest(http.MethodGet, "/api/v1/downloads/"+jobID+"/file", nil)
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	dlResp, err := env.App.Test(req, -1)
	if err != nil {
		t.Fatalf("download file request failed: %v", err)
	}
	dlBytes, _ := io.ReadAll(dlResp.Body)
	_ = dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK || len(dlBytes) == 0 {
		t.Fatalf("expected non-empty downloaded file, got status=%d len=%d", dlResp.StatusCode, len(dlBytes))
	}

	// History record was created
	resp, body = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/history", aliceToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("history list failed: %d", resp.StatusCode)
	}
	histItems := body["data"].([]interface{})
	if len(histItems) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(histItems))
	}
	histID := histItems[0].(map[string]interface{})["id"].(string)

	// Bob cannot delete Alice's history
	resp, _ = doJSONRequest(t, env.App, http.MethodDelete, "/api/v1/history/"+histID, bobToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when Bob deletes Alice's history, got %d", resp.StatusCode)
	}

	// Force expiration and test cleanup
	past := time.Now().UTC().Add(-10 * time.Minute)
	env.DB.Model(&repository.DownloadJob{}).Where("id = ?", uuid.MustParse(jobID)).Update("expires_at", &past)

	cleaned, err := env.Deps.DownloaderService.CleanupExpired(context.Background())
	if err != nil || cleaned != 1 {
		t.Fatalf("expected 1 expired job cleaned, got cleaned=%d err=%v", cleaned, err)
	}

	resp, _ = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/downloads/"+jobID+"/file", aliceToken, nil)
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("expected 410 Gone for expired download, got %d", resp.StatusCode)
	}

	cancel()
}

func TestConversionEndpoint_ValidationAndProcessing(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})

	ctx, cancel := context.WithCancel(context.Background())
	env.Deps.Queue.Start(ctx)
	t.Cleanup(func() {
		cancel()
		env.Deps.Queue.Wait()
	})

	token, _ := registerUser(t, env, "Converter", "conv@example.com", "StrongPass123!")

	// 1. Reject spoofed HTML disguised as .mp4
	sendUpload := func(filename string, content []byte, targetFmt string) (*http.Response, map[string]interface{}) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write(content)
		_ = mw.WriteField("target_format", targetFmt)
		_ = mw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversions", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := env.App.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		var parsed map[string]interface{}
		_ = json.Unmarshal(raw, &parsed)
		return resp, parsed
	}

	resp, _ := sendUpload("bad.mp4", []byte("<html>not a video</html>"), "mp3")
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415 for spoofed mp4 upload, got %d", resp.StatusCode)
	}

	// 2. Valid MP4 conversion to MP3
	validMP4 := append([]byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0}, bytes.Repeat([]byte{0x01}, 64)...)
	resp, body := sendUpload("clip.mp4", validMP4, "mp3")
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 for valid conversion upload, got %d (%v)", resp.StatusCode, body)
	}

	convID := body["data"].(map[string]interface{})["id"].(string)

	var finalStatus string
	for i := 0; i < 40; i++ {
		resp, body = doJSONRequest(t, env.App, http.MethodGet, "/api/v1/conversions/"+convID+"/status", token, nil)
		st := body["data"].(map[string]interface{})["status"].(string)
		if st == "completed" || st == "failed" {
			finalStatus = st
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if finalStatus != "completed" {
		t.Fatalf("expected conversion status 'completed', got %q (%v)", finalStatus, body)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversions/"+convID+"/file", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	outResp, err := env.App.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	outBytes, _ := io.ReadAll(outResp.Body)
	_ = outResp.Body.Close()
	if outResp.StatusCode != http.StatusOK || len(outBytes) == 0 {
		t.Fatalf("expected converted output file, got status=%d len=%d", outResp.StatusCode, len(outBytes))
	}

	cancel()
}
