package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/mediadock/backend/docs"
	"github.com/mediadock/backend/internal/cache"
	"github.com/mediadock/backend/internal/config"
	"github.com/mediadock/backend/internal/database"
	"github.com/mediadock/backend/internal/middleware"
	"github.com/mediadock/backend/internal/modules/auth"
	"github.com/mediadock/backend/internal/modules/conversion"
	"github.com/mediadock/backend/internal/modules/downloader"
	"github.com/mediadock/backend/internal/modules/history"
	"github.com/mediadock/backend/internal/modules/media"
	"github.com/mediadock/backend/internal/modules/processing"
	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/modules/storage"
	"github.com/mediadock/backend/internal/modules/users"
	"github.com/mediadock/backend/internal/platform/facebook"
	"github.com/mediadock/backend/internal/platform/generic"
	"github.com/mediadock/backend/internal/platform/instagram"
	"github.com/mediadock/backend/internal/platform/youtube"
	"github.com/mediadock/backend/internal/queue"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/utils"
	"gorm.io/gorm"
)

// Dependencies bundles core services so both main.go and integration tests can construct the server.
type Dependencies struct {
	Config            *config.Config
	DB                *gorm.DB
	Redis             *cache.RedisClient
	Logger            *slog.Logger
	Storage           storage.Storage
	Processor         processing.Processor
	Registry          *providers.Registry
	Queue             *queue.WorkerQueue
	DownloaderService *downloader.Service
	ConversionService *conversion.Service
	HistoryService    *history.Service
}

// BuildDefaultRegistry creates the production Provider Registry with all platform integrations.
func BuildDefaultRegistry(cfg *config.Config) (*providers.Registry, error) {
	reg := providers.NewRegistry()
	safeClient := utils.NewSafeHTTPClient(60*time.Second, 5)

	igProvider := instagram.NewProvider(cfg.InstagramGraphBase, safeClient)
	igProvider.SetDefaultToken(cfg.InstagramAccessToken)
	if err := reg.Register(igProvider); err != nil {
		return nil, err
	}
	if err := reg.Register(youtube.NewProvider()); err != nil {
		return nil, err
	}
	if err := reg.Register(facebook.NewProvider()); err != nil {
		return nil, err
	}
	if err := reg.Register(generic.NewProvider(safeClient)); err != nil {
		return nil, err
	}
	return reg, nil
}

// NewApp wires all repositories, services, handlers, middlewares, and /api/v1 routes into a Fiber app.
func NewApp(deps *Dependencies) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "MediaDock API",
		BodyLimit:    int(deps.Config.MaxFileSize),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			msg := "internal server error"
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				code = fiberErr.Code
				msg = fiberErr.Message
			} else if deps.Logger != nil {
				deps.Logger.Error("unhandled request error", "path", c.Path(), "error", err.Error())
			}
			return utils.JSONError(c, code, "HTTP_ERROR", msg)
		},
	})

	app.Use(recover.New())
	app.Use(middleware.CORS(deps.Config.AllowedOrigins))
	if deps.Logger != nil {
		app.Use(middleware.RequestLogger(deps.Logger))
	}
	app.Use(middleware.RateLimit(deps.Redis, deps.Config.RateLimitMax, deps.Config.RateLimitWindow))

	validator := utils.NewValidator()

	userRepo := repository.NewUserRepository(deps.DB)
	mediaRepo := repository.NewMediaRepository(deps.DB)
	downloadRepo := repository.NewDownloadJobRepository(deps.DB)
	conversionRepo := repository.NewConversionJobRepository(deps.DB)
	historyRepo := repository.NewHistoryRepository(deps.DB)

	authSvc := auth.NewService(userRepo, deps.Redis, deps.Config)
	usersSvc := users.NewService(userRepo)
	mediaSvc := media.NewService(mediaRepo, deps.Registry)
	historySvc := history.NewService(historyRepo, deps.Config.HistoryRetentionDays, deps.Logger)
	downloaderSvc := downloader.NewService(
		downloadRepo,
		mediaRepo,
		deps.Registry,
		deps.Processor,
		deps.Storage,
		historySvc,
		deps.Queue,
		deps.Config,
		deps.Logger,
	)
	conversionSvc := conversion.NewService(
		conversionRepo,
		deps.Processor,
		deps.Storage,
		deps.Queue,
		deps.Config,
		deps.Logger,
	)

	deps.HistoryService = historySvc
	deps.DownloaderService = downloaderSvc
	deps.ConversionService = conversionSvc

	if deps.Queue != nil {
		deps.Queue.RegisterHandler(queue.JobTypeDownload, downloaderSvc.ProcessJob)
		deps.Queue.RegisterHandler(queue.JobTypeConversion, conversionSvc.ProcessJob)
	}

	authHandler := auth.NewHandler(authSvc, validator)
	usersHandler := users.NewHandler(usersSvc, validator)
	mediaHandler := media.NewHandler(mediaSvc, validator)
	downloaderHandler := downloader.NewHandler(downloaderSvc, validator)
	conversionHandler := conversion.NewHandler(conversionSvc, validator)
	historyHandler := history.NewHandler(historySvc)

	authRequired := middleware.RequireAuth(authSvc)
	workspaceAuth := middleware.OptionalAuthOrGuest(authSvc)

	v1 := app.Group("/api/v1")

	// Health & Readiness & OpenAPI Spec
	v1.Get("/openapi.yaml", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/yaml; charset=utf-8")
		return c.Send(docs.OpenAPISpec)
	})
	v1.Get("/health", func(c *fiber.Ctx) error {
		return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{
			"status":  "ok",
			"service": "mediadock-api",
		})
	})
	v1.Get("/ready", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 3*time.Second)
		defer cancel()

		dbErr := database.Ping(ctx, deps.DB)
		redisErr := deps.Redis.Ping(ctx)
		if dbErr != nil || redisErr != nil {
			details := map[string]string{}
			if dbErr != nil {
				details["postgres"] = "unavailable"
			}
			if redisErr != nil {
				details["redis"] = "unavailable"
			}
			return utils.JSONError(c, fiber.StatusServiceUnavailable, "NOT_READY", "dependencies are not ready", details)
		}
		return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{
			"status":   "ready",
			"postgres": "ok",
			"redis":    "ok",
		})
	})

	// Authentication routes
	authGroup := v1.Group("/auth")
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/refresh", authHandler.Refresh)
	authGroup.Post("/logout", authRequired, authHandler.Logout)
	authGroup.Get("/me", authRequired, authHandler.Me)
	authGroup.Put("/me", authRequired, authHandler.UpdateMe)

	// Media inspection & library routes (no login required; isolates by authenticated user if provided)
	mediaGroup := v1.Group("/media", workspaceAuth)
	mediaGroup.Get("/", mediaHandler.List)
	mediaGroup.Post("/inspect", mediaHandler.Inspect)
	mediaGroup.Get("/:id", mediaHandler.GetByID)
	mediaGroup.Delete("/:id", mediaHandler.Delete)

	// Download job routes
	downloadsGroup := v1.Group("/downloads", workspaceAuth)
	downloadsGroup.Get("/", downloaderHandler.List)
	downloadsGroup.Post("/", downloaderHandler.Create)
	downloadsGroup.Get("/:id", downloaderHandler.GetByID)
	downloadsGroup.Get("/:id/status", downloaderHandler.GetStatus)
	downloadsGroup.Get("/:id/file", downloaderHandler.GetFile)
	downloadsGroup.Delete("/:id", downloaderHandler.Delete)

	// History routes
	historyGroup := v1.Group("/history", workspaceAuth)
	historyGroup.Get("/", historyHandler.List)
	historyGroup.Get("/:id", historyHandler.GetByID)
	historyGroup.Delete("/:id", historyHandler.Delete)

	// Conversion routes
	convGroup := v1.Group("/conversions", workspaceAuth)
	convGroup.Post("/", conversionHandler.Create)
	convGroup.Get("/:id", conversionHandler.GetByID)
	convGroup.Get("/:id/status", conversionHandler.GetStatus)
	convGroup.Get("/:id/file", conversionHandler.GetFile)

	// Settings routes
	settingsGroup := v1.Group("/settings", workspaceAuth)
	settingsGroup.Get("/", usersHandler.GetSettings)
	settingsGroup.Put("/", usersHandler.UpdateSettings)

	return app
}

