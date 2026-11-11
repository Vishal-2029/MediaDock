package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mediadock/backend/internal/cache"
	"github.com/mediadock/backend/internal/config"
	"github.com/mediadock/backend/internal/database"
	"github.com/mediadock/backend/internal/modules/processing"
	"github.com/mediadock/backend/internal/modules/storage"
	"github.com/mediadock/backend/internal/queue"
	"github.com/mediadock/backend/internal/server"
	"github.com/mediadock/backend/internal/utils"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err.Error())
		os.Exit(1)
	}

	log := utils.NewLogger(cfg.AppEnv)
	log.Info("starting MediaDock backend", "env", cfg.AppEnv, "port", cfg.AppPort)

	db, err := database.ConnectPostgres(cfg.DatabaseURL, cfg.IsProduction())
	if err != nil {
		log.Error("failed to connect to PostgreSQL", "error", err.Error())
		os.Exit(1)
	}

	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()

	if err := database.RunMigrations(rootCtx, db, log); err != nil {
		log.Error("database migration failed", "error", err.Error())
		os.Exit(1)
	}

	redisClient, err := cache.ConnectRedis(cfg.RedisURL)
	if err != nil {
		log.Error("failed to connect to Redis", "error", err.Error())
		os.Exit(1)
	}
	defer redisClient.Close()

	store, err := storage.NewFromConfig(rootCtx, cfg)
	if err != nil {
		log.Error("failed to initialize storage", "error", err.Error())
		os.Exit(1)
	}

	registry, err := server.BuildDefaultRegistry(cfg)
	if err != nil {
		log.Error("failed to initialize provider registry", "error", err.Error())
		os.Exit(1)
	}

	processor := processing.NewFFmpegProcessor(cfg.MaxFileSize, cfg.MaxProcessingTime)
	workerQueue := queue.NewWorkerQueue(
		redisClient,
		log,
		cfg.WorkerConcurrency,
		cfg.MaxProcessingTime+30*time.Second,
		cfg.JobMaxRetries,
	)

	deps := &server.Dependencies{
		Config:    cfg,
		DB:        db,
		Redis:     redisClient,
		Logger:    log,
		Storage:   store,
		Processor: processor,
		Registry:  registry,
		Queue:     workerQueue,
	}

	app := server.NewApp(deps)

	// Start Redis-backed background worker pool.
	workerQueue.Start(rootCtx)

	// Start periodic expiration and retention cleanup loop.
	go runCleanupLoop(rootCtx, deps, log)

	// Start HTTP server in a goroutine and listen for termination signals.
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- app.Listen(":" + cfg.AppPort)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("shutdown signal received", "signal", sig.String())
	case err := <-serverErrCh:
		if err != nil {
			log.Error("http server stopped unexpectedly", "error", err.Error())
		}
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Error("http server shutdown error", "error", err.Error())
	}

	cancelRoot()
	workerQueue.Wait()

	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}

	log.Info("MediaDock backend stopped cleanly")
}

func runCleanupLoop(ctx context.Context, deps *server.Dependencies, log *slog.Logger) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
			if deps.DownloaderService != nil {
				if _, err := deps.DownloaderService.CleanupExpired(cleanCtx); err != nil {
					log.Warn("download cleanup error", "error", err.Error())
				}
			}
			if deps.ConversionService != nil {
				if _, err := deps.ConversionService.CleanupExpired(cleanCtx); err != nil {
					log.Warn("conversion cleanup error", "error", err.Error())
				}
			}
			if deps.HistoryService != nil {
				if _, err := deps.HistoryService.ApplyRetention(cleanCtx); err != nil {
					log.Warn("history retention error", "error", err.Error())
				}
			}
			cancel()
		}
	}
}

