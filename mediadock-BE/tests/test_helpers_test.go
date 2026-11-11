package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"github.com/mediadock/backend/internal/cache"
	"github.com/mediadock/backend/internal/config"
	"github.com/mediadock/backend/internal/modules/processing"
	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/modules/storage"
	"github.com/mediadock/backend/internal/platform/facebook"
	"github.com/mediadock/backend/internal/platform/generic"
	"github.com/mediadock/backend/internal/platform/instagram"
	"github.com/mediadock/backend/internal/platform/youtube"
	"github.com/mediadock/backend/internal/queue"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/server"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type testEnv struct {
	App        *fiber.App
	Deps       *server.Dependencies
	DB         *gorm.DB
	MiniRedis  *miniredis.Miniredis
	MockIG     *httptest.Server
	TempDir    string
}

func setupTestEnv(t *testing.T, mockProcessor processing.Processor) *testEnv {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	redisCache := cache.NewFromClient(rdb)

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() {
			_ = sqlDB.Close()
		})
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
		t.Fatalf("automigrate test db: %v", err)
	}

	store, err := storage.NewLocalStorage(tempDir, "test-signing-secret-at-least-32-chars")
	if err != nil {
		t.Fatalf("init local storage: %v", err)
	}

	// Mock official Instagram Graph API + media CDN server
	var mockIG *httptest.Server
	mockIG = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/me":
			if r.Header.Get("Authorization") != "Bearer valid-ig-oauth-token" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token","code":190}}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"17841400001","username":"creator_owner"}`))
		case "/me/media":
			if r.Header.Get("Authorization") != "Bearer valid-ig-oauth-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":            "17999900001",
						"caption":       "My Own Authorized Reel",
						"media_type":    "VIDEO",
						"media_url":     mockIG.URL + "/cdn/owned_video.mp4",
						"permalink":     "https://www.instagram.com/reel/CxyzOwned123/",
						"thumbnail_url": mockIG.URL + "/cdn/thumb.jpg",
						"username":      "creator_owner",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/cdn/owned_video.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			// Minimal MP4 ftyp header + dummy payload
			mp4Bytes := append([]byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00}, bytes.Repeat([]byte{0xAA}, 128)...)
			_, _ = w.Write(mp4Bytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(mockIG.Close)

	reg := providers.NewRegistry()
	_ = reg.Register(instagram.NewTestProvider(mockIG.URL, mockIG.Client()))
	_ = reg.Register(youtube.NewProvider())
	_ = reg.Register(facebook.NewProvider())
	_ = reg.Register(generic.NewTestProvider(mockIG.Client()))

	cfg := &config.Config{
		AppEnv:               "test",
		AppPort:              "0",
		JWTSecret:            "test-jwt-secret-key-must-be-at-least-32-chars",
		JWTAccessTTL:         15 * time.Minute,
		JWTRefreshTTL:        24 * time.Hour,
		StorageType:          "local",
		StoragePath:          tempDir,
		LocalSigningSecret:   "test-signing-secret-at-least-32-chars",
		MaxFileSize:          10 * 1024 * 1024,
		MaxProcessingTime:    30 * time.Second,
		JobRetentionTTL:      1 * time.Hour,
		HistoryRetentionDays: 30,
		WorkerConcurrency:    2,
		JobMaxRetries:        2,
		AllowedOrigins:       []string{"http://localhost:3000"},
		RateLimitMax:         1000,
		RateLimitWindow:      time.Minute,
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if mockProcessor == nil {
		mockProcessor = processing.NewFFmpegProcessor(cfg.MaxFileSize, cfg.MaxProcessingTime)
	}

	wq := queue.NewWorkerQueue(redisCache, log, cfg.WorkerConcurrency, cfg.MaxProcessingTime, cfg.JobMaxRetries)

	deps := &server.Dependencies{
		Config:    cfg,
		DB:        db,
		Redis:     redisCache,
		Logger:    log,
		Storage:   store,
		Processor: mockProcessor,
		Registry:  reg,
		Queue:     wq,
	}

	app := server.NewApp(deps)

	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})

	return &testEnv{
		App:       app,
		Deps:      deps,
		DB:        db,
		MiniRedis: mr,
		MockIG:    mockIG,
		TempDir:   tempDir,
	}
}

func doJSONRequest(t *testing.T, app *fiber.App, method, path, token string, body interface{}) (*http.Response, map[string]interface{}) {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test %s %s: %v", method, path, err)
	}

	rawResp, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var parsed map[string]interface{}
	_ = json.Unmarshal(rawResp, &parsed)
	return resp, parsed
}

type noopProcessor struct{}

func (n *noopProcessor) InspectMetadata(_ context.Context, _ string) (*processing.MediaProbeInfo, error) {
	return &processing.MediaProbeInfo{
		DurationSeconds: 12,
		FormatName:      "mp4",
		SizeBytes:       144,
		HasVideo:        true,
		HasAudio:        true,
	}, nil
}

func (n *noopProcessor) ProcessMedia(_ context.Context, opts processing.ProcessOptions) error {
	// Write a tiny valid MP4/MP3 stub output file for testing workflow without ffmpeg binary.
	data := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00}
	if opts.TargetFormat == "mp3" {
		data = []byte("ID3\x03\x00\x00\x00\x00\x00\x00dummy-mp3-frame")
	}
	return writeBytes(opts.OutputPath, data)
}

func (n *noopProcessor) GenerateThumbnail(_ context.Context, _, outputPath string) error {
	return writeBytes(outputPath, []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10})
}

func writeBytes(path string, b []byte) error {
	return os.WriteFile(path, b, 0o640)
}
