package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloud-shuttle/drover-registry/internal/config"
	"github.com/cloud-shuttle/drover-registry/internal/infra"
	appstorage "github.com/cloud-shuttle/drover-registry/internal/storage"
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

func registerPackageRoutes(api fiber.Router, provider appstorage.Provider, db *infra.DB, cfg config.Config, appLogger *slog.Logger) {
	api.Post("/packages", func(c *fiber.Ctx) error {
		tenant := c.Get("X-Org-ID", "dev")
		name := c.Query("name", "unnamed")
		version := c.Query("version", "v0.0.0-dev")
		digest := c.Query("digest", "sha256:dev")

		ref := appstorage.PackageRef{
			TenantID: tenant,
			Name:     name,
			Version:  version,
			Digest:   digest,
		}

		body := c.Body()
		if len(body) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "empty body"})
		}

		checksum := digest // in real flow we would compute or trust header
		info, err := provider.Put(c.Context(), ref, bytes.NewReader(body), int64(len(body)), checksum)
		if err != nil {
			appLogger.Error("storage put failed", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message":     "package stored",
			"tenant":      tenant,
			"name":        name,
			"version":     version,
			"size":        info.Size,
			"storage_key": info.StorageKey,
		})
	})

	api.Get("/packages/:tenant/:name/:version", func(c *fiber.Ctx) error {
		ref := appstorage.PackageRef{
			TenantID: c.Params("tenant"),
			Name:     c.Params("name"),
			Version:  c.Params("version"),
			Digest:   c.Query("digest", ""),
		}

		rc, info, err := provider.Get(c.Context(), ref)
		if err != nil {
			if errors.Is(err, appstorage.ErrNotFound) {
				return c.SendStatus(fiber.StatusNotFound)
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		defer rc.Close()

		c.Set("Content-Type", "application/octet-stream")
		c.Set("Content-Length", fmt.Sprintf("%d", info.Size))
		return c.SendStream(rc)
	})

	api.Get("/packages/:tenant", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"tenant":   c.Params("tenant"),
			"message":  "list endpoint - metadata index coming with Postgres",
			"packages": []string{},
		})
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
