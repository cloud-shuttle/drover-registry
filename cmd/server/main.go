package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloud-shuttle/drover-registry/internal/api/handlers"
	"github.com/cloud-shuttle/drover-registry/internal/api/middleware"
	"github.com/cloud-shuttle/drover-registry/internal/config"
	"github.com/cloud-shuttle/drover-registry/internal/infra"
	"github.com/cloud-shuttle/drover-registry/internal/metadata"
	"github.com/cloud-shuttle/drover-registry/internal/webhook"
	"github.com/gofiber/fiber/v2"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg := config.LoadConfig()

	appLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Database (required for metadata index)
	db, err := infra.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		appLogger.Error("failed to connect to database", "error", err)
		// For early development we allow continuing without DB (storage only mode)
		appLogger.Warn("continuing in storage-only mode (no metadata persistence)")
		db = nil
	} else {
		defer db.Close()
		appLogger.Info("connected to postgres", "url", maskURL(cfg.DatabaseURL))
	}

	// Storage provider (local/s3/gcs)
	storageProvider, err := infra.NewStorageProvider(cfg)
	if err != nil {
		appLogger.Error("failed to initialize storage backend", "backend", cfg.StorageBackend, "error", err)
		os.Exit(1)
	}
	appLogger.Info("storage backend ready", "backend", cfg.StorageBackend)

	// === Auth setup (dreg-003) ===
	var authMw *middleware.AuthMiddleware
	var zitadelValidator *infra.ZitadelValidator

	if cfg.ZitadelIssuer != "" {
		var err error
		zitadelValidator, err = infra.NewZitadelValidator(cfg.ZitadelIssuer)
		if err != nil {
			appLogger.Error("failed to initialize Zitadel validator", "issuer", cfg.ZitadelIssuer, "error", err)
			// Do not crash in dev; fall back to dev auth if enabled
			if !cfg.EnableDevAuth {
				os.Exit(1)
			}
			appLogger.Warn("falling back to dev auth because Zitadel init failed")
		} else {
			appLogger.Info("Zitadel auth enabled", "issuer", cfg.ZitadelIssuer)
		}
	}

	authMw = middleware.NewAuthMiddleware(zitadelValidator, cfg)

	app := fiber.New(fiber.Config{
		AppName:      "drover-registry",
		ServerHeader: "drover-registry/0.1.0",
	})

	app.Use(recover.New())
	app.Use(fiberlogger.New())

	// Health endpoints (always available)
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "drover-registry"})
	})

	app.Get("/readyz", func(c *fiber.Ctx) error {
		if db != nil {
			rctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := db.GetPool().Ping(rctx); err != nil {
				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "not ready", "reason": "db"})
			}
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})

	// === v1 API (tenant aware via auth) ===
	api := app.Group("/v1")

	// Apply auth (Zitadel or dev header)
	if cfg.ZitadelIssuer != "" || cfg.EnableDevAuth {
		api.Use(authMw.Handler())
	} else {
		// Last resort: still require some tenant context
		api.Use(middleware.DevAuthMiddleware())
	}

	// Enforce that every request on /v1 has a tenant
	api.Use(middleware.RequireTenant())

	// Package routes (logic extracted to internal/api/handlers/packages.go)
	var meta *metadata.Store
	if db != nil {
		meta = metadata.NewStore(db)
	}
	var hook *webhook.Publisher
	if cfg.MusterWebhookURL != "" {
		hook = webhook.NewPublisher(cfg)
	}

	pkgHandler := handlers.NewPackageHandler(storageProvider, meta, hook, appLogger, cfg)
	pkgHandler.RegisterRoutes(api)

	// OCI routes (extracted to internal/api/handlers/oci.go)
	oci := app.Group("/v2")
	ociHandler := handlers.NewOCIHandler(storageProvider, appLogger)
	ociHandler.RegisterRoutes(oci)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)
	appLogger.Info("starting server", "addr", addr, "storage", cfg.StorageBackend)

	go func() {
		if err := app.Listen(addr); err != nil {
			appLogger.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	appLogger.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		appLogger.Error("shutdown error", "error", err)
	}
	appLogger.Info("server stopped")
}

// registerPackageRoutes has been replaced by handlers.PackageHandler.RegisterRoutes
// (see internal/api/handlers/packages.go). The old implementation has been removed
// as part of the extraction for better testability.

func parseLogLevel(l string) slog.Level {
	switch l {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func maskURL(u string) string {
	// naive mask for logs
	if len(u) > 20 {
		return u[:20] + "..."
	}
	return u
}
