package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloud-shuttle/drover-registry/internal/config"
	"github.com/cloud-shuttle/drover-registry/internal/infra"
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

	// === v1 API (tenant aware) ===
	api := app.Group("/v1")

	// TODO (next): add auth middleware here (Zitadel + dev header)
	// api.Use(authMiddleware)

	// Basic publish/fetch routes (dreg-002 starter)
	// These will be moved to internal/api/handlers once we have real implementations
	registerPackageRoutes(api, storageProvider, db, cfg, appLogger)

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

func registerPackageRoutes(api fiber.Router, sp interface{}, db *infra.DB, cfg config.Config, appLogger *slog.Logger) {
	// Placeholder routes for publish / fetch (will be replaced with real handlers)
	api.Post("/packages", func(c *fiber.Ctx) error {
		// TODO: real multipart upload + manifest parsing + storage.Put + DB insert
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "publish endpoint wired (stub)",
			"tenant":  c.Get("X-Org-ID", "dev"),
			"name":    "example-package",
			"version": "v0.1.0-stub",
		})
	})

	api.Get("/packages/:tenant/:name/:version", func(c *fiber.Ctx) error {
		// TODO: lookup in DB or storage, stream back
		return c.JSON(fiber.Map{
			"tenant":  c.Params("tenant"),
			"name":    c.Params("name"),
			"version": c.Params("version"),
			"status":  "stub - will stream tarball",
		})
	})

	// Simple list for a tenant
	api.Get("/packages/:tenant", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"tenant": c.Params("tenant"), "packages": []string{}})
	})
}

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
