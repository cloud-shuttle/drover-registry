package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cloud-shuttle/drover-registry/internal/api/middleware"
	"github.com/cloud-shuttle/drover-registry/internal/config"
	"github.com/cloud-shuttle/drover-registry/internal/infra"
	"github.com/cloud-shuttle/drover-registry/internal/metadata"
	appstorage "github.com/cloud-shuttle/drover-registry/internal/storage"
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

	// Real package routes (with tenant from context)
	registerPackageRoutes(api, storageProvider, db, cfg, appLogger)

	// === Basic OCI distribution support (single-layer oras push) ===
	// Supports monolithic blob upload + manifest for simple artifacts.
	// Not a full registry (no chunking, limited auth integration).
	oci := app.Group("/v2")
	oci.Get("/", func(c *fiber.Ctx) error {
		c.Set("Docker-Distribution-API-Version", "registry/2.0")
		return c.SendStatus(fiber.StatusOK)
	})

	// In-memory upload sessions for demo (in real life use Redis/DB)
	uploadSessions := make(map[string]string) // uuid -> repository name

	// Start blob upload
	oci.Post("/:name/blobs/uploads/", func(c *fiber.Ctx) error {
		name := c.Params("name")
		uuid := "u" + fmt.Sprintf("%d", time.Now().UnixNano())
		uploadSessions[uuid] = name

		location := fmt.Sprintf("/v2/%s/blobs/uploads/%s", name, uuid)
		c.Set("Location", location)
		c.Set("Docker-Upload-Uuid", uuid)
		c.Set("Docker-Distribution-API-Version", "registry/2.0")
		return c.SendStatus(fiber.StatusAccepted)
	})

	// Complete blob upload (monolithic)
	oci.Put("/:name/blobs/uploads/:uuid", func(c *fiber.Ctx) error {
		name := c.Params("name")
		uuid := c.Params("uuid")
		digest := c.Query("digest")

		if digest == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "digest query parameter required"})
		}

		// Read entire body (for small layers this is fine)
		body := c.Body()
		if len(body) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "empty blob"})
		}

		// Store using our existing provider under a synthetic tenant for OCI demo
		tenant := "oci"
		ref := appstorage.PackageRef{
			TenantID: tenant,
			Name:     name,
			Version:  digest, // use digest as "version" for uniqueness
			Digest:   digest,
		}

		_, err := storageProvider.Put(c.Context(), ref, bytes.NewReader(body), int64(len(body)), digest)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		delete(uploadSessions, uuid)

		c.Set("Docker-Distribution-API-Version", "registry/2.0")
		c.Set("Location", fmt.Sprintf("/v2/%s/blobs/%s", name, digest))
		return c.SendStatus(fiber.StatusCreated)
	})

	// Head blob (existence check)
	oci.Head("/:name/blobs/:digest", func(c *fiber.Ctx) error {
		digest := c.Params("digest")
		// Quick existence via storage (we use a synthetic ref)
		ref := appstorage.PackageRef{TenantID: "oci", Name: c.Params("name"), Version: digest, Digest: digest}
		exists, _ := storageProvider.Exists(c.Context(), ref)
		if !exists {
			return c.SendStatus(fiber.StatusNotFound)
		}
		c.Set("Docker-Distribution-API-Version", "registry/2.0")
		c.Set("Docker-Content-Digest", digest)
		return c.SendStatus(fiber.StatusOK)
	})

	// Manifest put (the final step of oras push)
	oci.Put("/:name/manifests/:ref", func(c *fiber.Ctx) error {
		name := c.Params("name")
		tag := c.Params("ref") // can be tag or digest
		body := c.Body()

		// For demo we store the manifest the same way
		digest := fmt.Sprintf("sha256:manifest-%d", time.Now().UnixNano()) // real impl would parse and compute digest
		ref := appstorage.PackageRef{
			TenantID: "oci",
			Name:     name,
			Version:  tag,
			Digest:   digest,
		}
		_, err := storageProvider.Put(c.Context(), ref, bytes.NewReader(body), int64(len(body)), digest)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		c.Set("Docker-Distribution-API-Version", "registry/2.0")
		c.Set("Docker-Content-Digest", digest)
		c.Set("Location", fmt.Sprintf("/v2/%s/manifests/%s", name, tag))
		return c.SendStatus(fiber.StatusCreated)
	})

	// Basic manifest get (for pull)
	oci.Get("/:name/manifests/:ref", func(c *fiber.Ctx) error {
		// For a real single-layer flow this would return the manifest we stored.
		// Stub for now so oras push at least succeeds; full roundtrip later.
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "OCI manifest GET not fully wired for pull yet"})
	})

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
	// Create metadata store when DB is available
	var metaStore *metadata.Store
	if db != nil {
		metaStore = metadata.NewStore(db)
	}

	// Webhook publisher to drover-muster (fire-and-forget)
	var hookPublisher *webhook.Publisher
	if cfg.MusterWebhookURL != "" {
		hookPublisher = webhook.NewPublisher(cfg)
	}

	// POST /v1/packages - real implementation with manifest extraction + Postgres
	api.Post("/packages", func(c *fiber.Ctx) error {
		tenant := middleware.GetTenant(c)

		// 1. Obtain the artifact bytes (support raw body or multipart "file")
		var artifactReader io.Reader
		var artifactSize int64
		contentType := c.Get("Content-Type")

		if strings.HasPrefix(contentType, "multipart/form-data") {
			form, err := c.MultipartForm()
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid multipart form"})
			}
			files := form.File["file"]
			if len(files) == 0 {
				files = form.File["artifact"]
			}
			if len(files) == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no 'file' field in multipart upload"})
			}
			fh := files[0]
			artifactSize = fh.Size
			f, err := fh.Open()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to open uploaded file"})
			}
			defer f.Close()
			artifactReader = f
		} else {
			// raw body
			body := c.Body()
			if len(body) == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "empty body (send tarball as raw body or multipart 'file')"})
			}
			artifactReader = bytes.NewReader(body)
			artifactSize = int64(len(body))
		}

		// 2. Try to extract manifest.json from the tarball (best effort)
		manifest, manifestRaw, err := metadata.ExtractManifestFromTarball(artifactReader)
		if err != nil {
			appLogger.Warn("manifest extraction failed (continuing without it)", "error", err)
		}

		// Reset reader because we consumed it during extraction.
		// For this milestone we support in-memory or seekable readers (multipart files are re-opened by the form lib).
		var finalReader io.Reader
		if rs, ok := artifactReader.(io.ReadSeeker); ok {
			rs.Seek(0, 0)
			finalReader = rs
		} else if b, ok := artifactReader.(*bytes.Reader); ok {
			b.Seek(0, 0)
			finalReader = b
		} else {
			// multipart file case: the original file handle from the form is still valid
			finalReader = artifactReader
		}

		// 3. Determine final name/version (prefer manifest)
		name := c.Query("name", "unnamed")
		version := c.Query("version", "v0.0.0-dev")
		if manifest != nil {
			if manifest.Name != "" {
				name = manifest.Name
			}
			if manifest.Version != "" {
				version = manifest.Version
			}
		}

		digest := c.Query("digest", fmt.Sprintf("sha256:upload-%d", time.Now().UnixNano()))
		ref := appstorage.PackageRef{
			TenantID: tenant,
			Name:     name,
			Version:  version,
			Digest:   digest,
		}

		// 4. Store the artifact
		info, err := provider.Put(c.Context(), ref, finalReader, artifactSize, digest)
		if err != nil {
			appLogger.Error("storage put failed", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// 5. Persist metadata if DB is available
		var dbID int64
		if metaStore != nil {
			pkgID, err := metaStore.UpsertPackage(c.Context(), tenant, name)
			if err == nil {
				publishedBy := "dev"
				if v := c.Locals("userID"); v != nil {
					if s, ok := v.(string); ok && s != "" {
						publishedBy = s
					}
				}
				mv := &metadata.PackageVersion{
					PackageID:   pkgID,
					Version:     version,
					Digest:      digest,
					SizeBytes:   info.Size,
					StorageKey:  info.StorageKey,
					Manifest:    manifestRaw,
					PublishedBy: publishedBy,
				}
				_ = metaStore.InsertVersion(c.Context(), pkgID, mv)
				dbID = mv.ID
			}
		}

		// 6. Fire webhook to drover-muster (non-blocking)
		if hookPublisher != nil {
			go func() {
				_ = hookPublisher.Publish(context.Background(), tenant, name, version, digest, info.Size, info.StorageKey)
			}()
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message":     "package published",
			"tenant":      tenant,
			"name":        name,
			"version":     version,
			"size":        info.Size,
			"digest":      digest,
			"storage_key": info.StorageKey,
			"db_record":   dbID > 0,
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
			"message":  "list endpoint - full metadata index coming with more queries",
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
