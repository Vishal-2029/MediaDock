package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/cache"
	"github.com/mediadock/backend/internal/config"
	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/modules/storage"
	"github.com/mediadock/backend/internal/platform/external"
	"github.com/mediadock/backend/internal/platform/instagram"
	"github.com/mediadock/backend/internal/queue"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/server"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSignedGuestCookieIsolation(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})

	// First unauthenticated request gets a signed mediadock_guest_id cookie
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	resp1, err := env.App.Test(req1, -1)
	if err != nil {
		t.Fatalf("first guest GET /history: %v", err)
	}
	_ = resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on guest history, got %d", resp1.StatusCode)
	}

	var guestCookie1 *http.Cookie
	for _, c := range resp1.Cookies() {
		if c.Name == "mediadock_guest_id" {
			guestCookie1 = c
			break
		}
	}
	if guestCookie1 == nil || !strings.Contains(guestCookie1.Value, ".") {
		t.Fatalf("expected signed mediadock_guest_id cookie, got %+v", guestCookie1)
	}

	// Browser 1 inspects a reel with OAuth token using its signed guest cookie
	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"url":          "https://www.instagram.com/reel/CxyzOwned123/",
		"access_token": "valid-ig-oauth-token",
	})
	inspReq := httptest.NewRequest(http.MethodPost, "/api/v1/media/inspect", bytes.NewReader(bodyBytes))
	inspReq.Header.Set("Content-Type", "application/json")
	inspReq.AddCookie(guestCookie1)
	inspResp, err := env.App.Test(inspReq, -1)
	if err != nil {
		t.Fatalf("guest inspect: %v", err)
	}
	_ = inspResp.Body.Close()
	if inspResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on guest inspect, got %d", inspResp.StatusCode)
	}

	// Browser 1 sees its own 1 media item when presenting guestCookie1
	listReq1 := httptest.NewRequest(http.MethodGet, "/api/v1/media", nil)
	listReq1.AddCookie(guestCookie1)
	listResp1, _ := env.App.Test(listReq1, -1)
	var listBody1 map[string]interface{}
	_ = json.NewDecoder(listResp1.Body).Decode(&listBody1)
	_ = listResp1.Body.Close()
	if items := listBody1["data"].([]interface{}); len(items) != 1 {
		t.Fatalf("expected Browser 1 guest to see 1 media item, got %d", len(items))
	}

	// Browser 2 (no cookie) gets a distinct guest workspace with 0 media items
	listReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/media", nil)
	listResp2, _ := env.App.Test(listReq2, -1)
	var listBody2 map[string]interface{}
	_ = json.NewDecoder(listResp2.Body).Decode(&listBody2)
	_ = listResp2.Body.Close()
	if items := listBody2["data"].([]interface{}); len(items) != 0 {
		t.Fatalf("expected isolated Browser 2 guest to see 0 media items, got %d", len(items))
	}

	// Tampered guest cookie must be rejected and rotated to a fresh empty guest workspace
	tampered := &http.Cookie{Name: "mediadock_guest_id", Value: uuid.NewString() + ".deadbeef"}
	tamperReq := httptest.NewRequest(http.MethodGet, "/api/v1/media", nil)
	tamperReq.AddCookie(tampered)
	tamperResp, _ := env.App.Test(tamperReq, -1)
	var tamperBody map[string]interface{}
	_ = json.NewDecoder(tamperResp.Body).Decode(&tamperBody)
	_ = tamperResp.Body.Close()
	if items := tamperBody["data"].([]interface{}); len(items) != 0 {
		t.Fatalf("expected tampered guest cookie to see 0 items, got %d", len(items))
	}
	rotated := false
	for _, c := range tamperResp.Cookies() {
		if c.Name == "mediadock_guest_id" && c.Value != tampered.Value {
			rotated = true
		}
	}
	if !rotated {
		t.Fatal("expected tampered mediadock_guest_id cookie to be rotated")
	}
}

func TestConfigValidateProductionPlaceholders(t *testing.T) {
	base := config.Config{
		AppEnv:            "production",
		DatabaseURL:       "postgres://user:pass@db:5432/mediadock",
		RedisURL:          "redis://redis:6379/0",
		JWTSecret:         "change-this-to-a-long-random-secret-at-least-32-chars",
		LocalSigningSecret: "a-real-random-signing-secret-that-is-over-32-chars",
		StorageType:       "local",
		MaxFileSize:       1024,
		MaxProcessingTime: time.Minute,
		WorkerConcurrency: 2,
	}
	if err := base.Validate(); err == nil {
		t.Fatal("expected production validation error when JWTSecret uses placeholder")
	}

	base.JWTSecret = "a-real-production-jwt-secret-that-is-well-over-32-chars"
	base.LocalSigningSecret = "change-this-signing-secret-in-production"
	if err := base.Validate(); err == nil {
		t.Fatal("expected production validation error when LocalSigningSecret uses placeholder")
	}

	base.LocalSigningSecret = "a-real-random-signing-secret-that-is-over-32-chars"
	if err := base.Validate(); err != nil {
		t.Fatalf("expected valid production config to pass, got %v", err)
	}
}

func TestQueueTokenEncryptionAndDLQStripping(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	redisCache := cache.NewFromClient(rdb)

	wq := queue.NewWorkerQueue(redisCache, slog.New(slog.NewTextHandler(io.Discard, nil)), 1, 5*time.Second, 1)
	wq.SetTokenSecret("super-secret-queue-encryption-key-32chars")

	jobID := uuid.New()
	secretToken := "ig-oauth-secret-value-123"
	if err := wq.Enqueue(context.Background(), queue.JobTypeDownload, jobID, secretToken); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	rawItems, err := rdb.LRange(context.Background(), "mediadock:jobs:pending", 0, -1).Result()
	if err != nil || len(rawItems) != 1 {
		t.Fatalf("expected 1 queued item, err=%v len=%d", err, len(rawItems))
	}
	if strings.Contains(rawItems[0], secretToken) {
		t.Fatal("expected queued Redis payload not to contain plaintext OAuth token")
	}

	var storedPayload queue.JobPayload
	if err := json.Unmarshal([]byte(rawItems[0]), &storedPayload); err != nil {
		t.Fatalf("unmarshal stored payload: %v", err)
	}
	if !strings.HasPrefix(storedPayload.Token, "enc:v1:") {
		t.Fatalf("expected encrypted enc:v1 token prefix, got %q", storedPayload.Token)
	}

	// Handler must receive the decrypted plaintext token and failing job moves to DLQ with token stripped
	var seenToken string
	wq.RegisterHandler(queue.JobTypeDownload, func(_ context.Context, payload queue.JobPayload) error {
		seenToken = payload.Token
		return errors.New("intentional worker failure")
	})

	wq.ProcessOne(context.Background(), storedPayload)
	if seenToken != secretToken {
		t.Fatalf("expected handler to receive decrypted token %q, got %q", secretToken, seenToken)
	}

	dlqItems, err := rdb.LRange(context.Background(), "mediadock:jobs:dlq", 0, -1).Result()
	if err != nil || len(dlqItems) != 1 {
		t.Fatalf("expected 1 DLQ item, err=%v len=%d", err, len(dlqItems))
	}
	if strings.Contains(dlqItems[0], secretToken) || strings.Contains(dlqItems[0], "enc:v1:") {
		t.Fatalf("expected DLQ payload to strip all tokens, got %s", dlqItems[0])
	}
}

func TestInstagramOEmbedPreviewOnlyBlocksDownload(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	redisCache := cache.NewFromClient(rdb)

	tempDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(tempDir+"/oembed.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(
		&repository.User{},
		&repository.UserSettings{},
		&repository.MediaSource{},
		&repository.MediaItem{},
		&repository.DownloadJob{},
		&repository.DownloadHistory{},
		&repository.ConversionJob{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	mockMeta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/instagram_oembed":
			if r.Header.Get("Authorization") != "Bearer app-123|app-secret-xyz" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"title": "Public creator showcase reel",
				"author_name": "public_creator",
				"provider_name": "Instagram",
				"thumbnail_url": "https://cdn.example.com/public-thumb.jpg",
				"html": "<blockquote></blockquote>"
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(mockMeta.Close)

	igProv := instagram.NewTestProvider(mockMeta.URL, mockMeta.Client())
	igProv.SetAppCredentials("app-123", "app-secret-xyz")
	reg := providers.NewRegistry()
	_ = reg.Register(igProv)

	store, _ := storage.NewLocalStorage(tempDir, "test-signing-secret-at-least-32-chars")
	cfg := &config.Config{
		AppEnv:             "test",
		JWTSecret:          "test-jwt-secret-key-must-be-at-least-32-chars",
		JWTAccessTTL:       15 * time.Minute,
		JWTRefreshTTL:      24 * time.Hour,
		StorageType:        "local",
		StoragePath:        tempDir,
		LocalSigningSecret: "test-signing-secret-at-least-32-chars",
		MaxFileSize:        10 * 1024 * 1024,
		MaxProcessingTime:  30 * time.Second,
		JobRetentionTTL:    time.Hour,
		WorkerConcurrency:  1,
		JobMaxRetries:      1,
		AllowedOrigins:     []string{"http://localhost:5173"},
		RateLimitMax:       100,
		RateLimitWindow:    time.Minute,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	wq := queue.NewWorkerQueue(redisCache, log, 1, time.Minute, 1)
	app := server.NewApp(&server.Dependencies{
		Config:    cfg,
		DB:        db,
		Redis:     redisCache,
		Logger:    log,
		Storage:   store,
		Processor: &noopProcessor{},
		Registry:  reg,
		Queue:     wq,
	})

	// Unauthenticated public Reel inspect succeeds with preview-only metadata (downloadable=false)
	resp, body := doJSONRequest(t, app, http.MethodPost, "/api/v1/media/inspect", "", map[string]interface{}{
		"url": "https://www.instagram.com/reel/Dfj0Z-OMD19/",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 oEmbed preview, got %d: %v", resp.StatusCode, body)
	}
	data := body["data"].(map[string]interface{})
	if data["downloadable"] != false {
		t.Fatalf("expected downloadable=false for oEmbed preview, got %v", data["downloadable"])
	}
	if data["thumbnail_url"] != "https://cdn.example.com/public-thumb.jpg" {
		t.Fatalf("unexpected thumbnail_url: %v", data["thumbnail_url"])
	}
	mediaID := data["media_id"].(string)

	var guestCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "mediadock_guest_id" {
			guestCookie = c
			break
		}
	}
	if guestCookie == nil {
		t.Fatal("expected signed mediadock_guest_id cookie on preview inspect")
	}

	// A separate browser without cookie must not see this guest media item (404)
	otherResp, _ := doJSONRequest(t, app, http.MethodPost, "/api/v1/downloads", "", map[string]interface{}{
		"media_id":         mediaID,
		"requested_format": "mp4",
	})
	_ = otherResp.Body.Close()
	if otherResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 across distinct guest workspaces, got %d", otherResp.StatusCode)
	}

	// Attempting to queue a download for preview-only item in the same browser session must return 403 PREVIEW_ONLY_MEDIA
	dlPayload, _ := json.Marshal(map[string]interface{}{
		"media_id":         mediaID,
		"requested_format": "mp4",
	})
	dlReq := httptest.NewRequest(http.MethodPost, "/api/v1/downloads", bytes.NewReader(dlPayload))
	dlReq.Header.Set("Content-Type", "application/json")
	dlReq.AddCookie(guestCookie)
	dlResp, err := app.Test(dlReq, -1)
	if err != nil {
		t.Fatalf("POST /downloads preview-only: %v", err)
	}
	var dlBody map[string]interface{}
	_ = json.NewDecoder(dlResp.Body).Decode(&dlBody)
	_ = dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 PREVIEW_ONLY_MEDIA, got %d: %v", dlResp.StatusCode, dlBody)
	}
	errObj := dlBody["error"].(map[string]interface{})
	if errObj["code"] != "PREVIEW_ONLY_MEDIA" {
		t.Fatalf("expected PREVIEW_ONLY_MEDIA code, got %v", errObj["code"])
	}
}

func TestExternalProviderInspectAndDownload(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	redisCache := cache.NewFromClient(rdb)

	tempDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(tempDir+"/ext.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(
		&repository.User{},
		&repository.UserSettings{},
		&repository.MediaSource{},
		&repository.MediaItem{},
		&repository.DownloadJob{},
		&repository.DownloadHistory{},
		&repository.ConversionJob{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	mp4Sample := append([]byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00}, bytes.Repeat([]byte{0x55}, 64)...)

	var mockExt *httptest.Server
	mockExt = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/inspect":
			if r.Header.Get("Authorization") != "Bearer licensed-api-key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":             "ext-reel-1",
				"platform":       "instagram",
				"title":          "Licensed Reel Download",
				"media_type":     "video",
				"thumbnail_url":  mockExt.URL + "/thumb.jpg",
				"duration":       18,
				"format":         "mp4",
				"file_size":      len(mp4Sample),
				"download_url":   mockExt.URL + "/video.mp4",
				"owner_username": "licensed_creator",
			})
		case "/video.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write(mp4Sample)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(mockExt.Close)

	reg := providers.NewRegistry()
	_ = reg.Register(external.NewTestProvider(mockExt.URL, "licensed-api-key", mockExt.Client()))

	store, _ := storage.NewLocalStorage(tempDir, "test-signing-secret-at-least-32-chars")
	cfg := &config.Config{
		AppEnv:             "test",
		JWTSecret:          "test-jwt-secret-key-must-be-at-least-32-chars",
		JWTAccessTTL:       15 * time.Minute,
		JWTRefreshTTL:      24 * time.Hour,
		StorageType:        "local",
		StoragePath:        tempDir,
		LocalSigningSecret: "test-signing-secret-at-least-32-chars",
		MaxFileSize:        10 * 1024 * 1024,
		MaxProcessingTime:  30 * time.Second,
		JobRetentionTTL:    time.Hour,
		WorkerConcurrency:  1,
		JobMaxRetries:      1,
		AllowedOrigins:     []string{"http://localhost:5173"},
		RateLimitMax:       100,
		RateLimitWindow:    time.Minute,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	wq := queue.NewWorkerQueue(redisCache, log, 1, time.Minute, 1)
	deps := &server.Dependencies{
		Config:    cfg,
		DB:        db,
		Redis:     redisCache,
		Logger:    log,
		Storage:   store,
		Processor: &noopProcessor{},
		Registry:  reg,
		Queue:     wq,
	}
	app := server.NewApp(deps)

	// 1. Inspect public Instagram URL via licensed external adapter (no user login required)
	resp, body := doJSONRequest(t, app, http.MethodPost, "/api/v1/media/inspect", "", map[string]interface{}{
		"url": "https://www.instagram.com/reel/Dfj0Z-OMD19/",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 inspect via external provider, got %d: %v", resp.StatusCode, body)
	}
	var guestCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "mediadock_guest_id" {
			guestCookie = c
			break
		}
	}
	data := body["data"].(map[string]interface{})
	if data["downloadable"] != true {
		t.Fatalf("expected downloadable=true, got %v", data["downloadable"])
	}
	mediaID := data["media_id"].(string)

	// 2. Queue download with same guest cookie
	dlBodyBytes, _ := json.Marshal(map[string]interface{}{
		"media_id":          mediaID,
		"requested_format":  "mp4",
		"requested_quality": "original",
	})
	dlReq := httptest.NewRequest(http.MethodPost, "/api/v1/downloads", bytes.NewReader(dlBodyBytes))
	dlReq.Header.Set("Content-Type", "application/json")
	dlReq.AddCookie(guestCookie)
	dlResp, err := app.Test(dlReq, -1)
	if err != nil {
		t.Fatalf("POST /downloads: %v", err)
	}
	var dlParsed map[string]interface{}
	_ = json.NewDecoder(dlResp.Body).Decode(&dlParsed)
	_ = dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 queued download, got %d: %v", dlResp.StatusCode, dlParsed)
	}
	jobID := dlParsed["data"].(map[string]interface{})["id"].(string)

	// 3. Process queued download synchronously via WorkerQueue
	rawItems, err := rdb.LRange(context.Background(), "mediadock:jobs:pending", 0, -1).Result()
	if err != nil || len(rawItems) != 1 {
		t.Fatalf("expected 1 queued download job, got %d (err=%v)", len(rawItems), err)
	}
	var qPayload queue.JobPayload
	_ = json.Unmarshal([]byte(rawItems[0]), &qPayload)
	wq.ProcessOne(context.Background(), qPayload)

	// 4. Download the completed MP4 artifact
	fileReq := httptest.NewRequest(http.MethodGet, "/api/v1/downloads/"+jobID+"/file", nil)
	fileReq.AddCookie(guestCookie)
	fileResp, err := app.Test(fileReq, -1)
	if err != nil {
		t.Fatalf("GET /downloads/:id/file: %v", err)
	}
	fileBytes, _ := io.ReadAll(fileResp.Body)
	_ = fileResp.Body.Close()
	if fileResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 file stream, got %d (%s)", fileResp.StatusCode, string(fileBytes))
	}
	if !bytes.Equal(fileBytes, mp4Sample) {
		t.Fatalf("downloaded bytes (%d) did not match expected MP4 payload (%d)", len(fileBytes), len(mp4Sample))
	}
}

func TestConversionThumbnailEndpoint(t *testing.T) {
	env := setupTestEnv(t, &noopProcessor{})

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("target_format", "mp4")
	_ = mw.WriteField("target_resolution", "720p")
	part, err := mw.CreateFormFile("file", "clip.mp4")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	mp4Bytes := append([]byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00}, bytes.Repeat([]byte{0xAB}, 64)...)
	_, _ = part.Write(mp4Bytes)
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversions", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := env.App.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /conversions: %v", err)
	}
	var parsed map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 conversion accepted, got %d: %v", resp.StatusCode, parsed)
	}

	var guestCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "mediadock_guest_id" {
			guestCookie = c
			break
		}
	}
	convID := parsed["data"].(map[string]interface{})["id"].(string)

	rawItems, err := env.MiniRedis.List("mediadock:jobs:pending")
	if err != nil || len(rawItems) != 1 {
		t.Fatalf("expected 1 queued conversion job, got %d (err=%v)", len(rawItems), err)
	}
	var payload queue.JobPayload
	_ = json.Unmarshal([]byte(rawItems[0]), &payload)
	env.Deps.Queue.ProcessOne(context.Background(), payload)

	thumbReq := httptest.NewRequest(http.MethodGet, "/api/v1/conversions/"+convID+"/thumbnail", nil)
	thumbReq.AddCookie(guestCookie)
	thumbResp, err := env.App.Test(thumbReq, -1)
	if err != nil {
		t.Fatalf("GET /conversions/:id/thumbnail: %v", err)
	}
	thumbBytes, _ := io.ReadAll(thumbResp.Body)
	_ = thumbResp.Body.Close()
	if thumbResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 thumbnail JPEG, got %d", thumbResp.StatusCode)
	}
	if thumbResp.Header.Get("Content-Type") != "image/jpeg" || !bytes.HasPrefix(thumbBytes, []byte{0xFF, 0xD8}) {
		t.Fatalf("expected JPEG thumbnail bytes, got Content-Type=%q len=%d", thumbResp.Header.Get("Content-Type"), len(thumbBytes))
	}
}
