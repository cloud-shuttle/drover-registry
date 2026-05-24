package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/cloud-shuttle/drover-registry/internal/config"
	"github.com/cloud-shuttle/drover-registry/internal/metadata"
	"github.com/cloud-shuttle/drover-registry/internal/storage"
	"github.com/cloud-shuttle/drover-registry/internal/webhook"
	"github.com/gofiber/fiber/v2"
)

// PackageHandler holds dependencies for package-related endpoints.
type PackageHandler struct {
	Provider      storage.Provider
	MetaStore     *metadata.Store
	HookPublisher *webhook.Publisher
	Logger        *slog.Logger
	Cfg           config.Config
}

// NewPackageHandler creates a PackageHandler with the given dependencies.
func NewPackageHandler(provider storage.Provider, meta *metadata.Store, hook *webhook.Publisher, logger *slog.Logger, cfg config.Config) *PackageHandler {
	return &PackageHandler{
		Provider:      provider,
		MetaStore:     meta,
		HookPublisher: hook,
		Logger:        logger,
		Cfg:           cfg,
	}
}

// RegisterRoutes wires the package routes onto the given Fiber router group (usually /v1).
func (h *PackageHandler) RegisterRoutes(api fiber.Router) {
	api.Post("/packages", h.Publish)
	api.Get("/packages/:tenant/:name/:version", h.GetPackage)
	api.Get("/packages/:tenant", h.ListPackages) // legacy broad list
	api.Get("/packages/:tenant/:name", h.ListPackageVersions)
	api.Get("/search", h.Search)
}

// Publish handles artifact upload (raw body or multipart), manifest extraction, storage, DB, and webhook.
func (h *PackageHandler) Publish(c *fiber.Ctx) error {
	tenant := getTenantFromContext(c)

	// 1. Get artifact bytes
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
		body := c.Body()
		if len(body) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "empty body (send tarball as raw body or multipart 'file')"})
		}
		artifactReader = bytes.NewReader(body)
		artifactSize = int64(len(body))
	}

	// 2. Extract manifest (best effort)
	manifest, manifestRaw, err := metadata.ExtractManifestFromTarball(artifactReader)
	if err != nil {
		h.Logger.Warn("manifest extraction failed (continuing without it)", "error", err)
	}

	// Reset reader
	var finalReader io.Reader
	if rs, ok := artifactReader.(io.ReadSeeker); ok {
		rs.Seek(0, 0)
		finalReader = rs
	} else if b, ok := artifactReader.(*bytes.Reader); ok {
		b.Seek(0, 0)
		finalReader = b
	} else {
		finalReader = artifactReader
	}

	// 3. Name/version from manifest or query
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
	ref := storage.PackageRef{
		TenantID: tenant,
		Name:     name,
		Version:  version,
		Digest:   digest,
	}

	// 4. Store
	info, err := h.Provider.Put(c.Context(), ref, finalReader, artifactSize, digest)
	if err != nil {
		h.Logger.Error("storage put failed", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 5. Persist to DB
	var dbID int64
	if h.MetaStore != nil {
		pkgID, err := h.MetaStore.UpsertPackage(c.Context(), tenant, name)
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
			_ = h.MetaStore.InsertVersion(c.Context(), pkgID, mv)
			dbID = mv.ID
		}
	}

	// 6. Webhook (non-blocking)
	if h.HookPublisher != nil {
		go func() {
			_ = h.HookPublisher.Publish(context.Background(), tenant, name, version, digest, info.Size, info.StorageKey)
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
}

// GetPackage streams a specific version.
func (h *PackageHandler) GetPackage(c *fiber.Ctx) error {
	ref := storage.PackageRef{
		TenantID: c.Params("tenant"),
		Name:     c.Params("name"),
		Version:  c.Params("version"),
		Digest:   c.Query("digest", ""),
	}

	rc, info, err := h.Provider.Get(c.Context(), ref)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rc.Close()

	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Length", fmt.Sprintf("%d", info.Size))
	return c.SendStream(rc)
}

// ListPackageVersions uses the new ListVersions query.
func (h *PackageHandler) ListPackageVersions(c *fiber.Ctx) error {
	tenant := c.Params("tenant")
	name := c.Params("name")

	if h.MetaStore == nil {
		return c.JSON(fiber.Map{
			"tenant":   tenant,
			"name":     name,
			"message":  "DB not configured",
			"versions": []string{},
		})
	}

	versions, err := h.MetaStore.ListVersions(c.Context(), tenant, name)
	if err != nil {
		h.Logger.Error("ListVersions failed", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list versions"})
	}

	type VersionSummary struct {
		Version     string `json:"version"`
		Digest      string `json:"digest"`
		SizeBytes   int64  `json:"size_bytes"`
		PublishedBy string `json:"published_by,omitempty"`
		CreatedAt   string `json:"created_at"`
	}

	summaries := make([]VersionSummary, 0, len(versions))
	for _, v := range versions {
		summaries = append(summaries, VersionSummary{
			Version:     v.Version,
			Digest:      v.Digest,
			SizeBytes:   v.SizeBytes,
			PublishedBy: v.PublishedBy,
			CreatedAt:   v.CreatedAt.Format(time.RFC3339),
		})
	}

	return c.JSON(fiber.Map{
		"tenant":   tenant,
		"name":     name,
		"count":    len(summaries),
		"versions": summaries,
	})
}

// ListPackages is the legacy broad list (kept for compatibility).
func (h *PackageHandler) ListPackages(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"tenant":   c.Params("tenant"),
		"message":  "Use /packages/:tenant/:name for version listing",
		"packages": []string{},
	})
}

// getTenantFromContext is a small helper (duplicated from middleware for handler independence).
func getTenantFromContext(c *fiber.Ctx) string {
	if t, ok := c.Locals("tenantID").(string); ok && t != "" {
		return t
	}
	return "dev"
}

// Search is a simple demonstration endpoint using the new SearchByManifestField DB method.
// Example: GET /v1/search?field=type&value=crew&tenant=myorg
func (h *PackageHandler) Search(c *fiber.Ctx) error {
	if h.MetaStore == nil {
		return c.JSON(fiber.Map{"message": "DB not available", "results": []any{}})
	}

	tenant := getTenantFromContext(c)
	field := c.Query("field", "type")
	value := c.Query("value", "crew")
	limit := 20

	results, err := h.MetaStore.SearchByManifestField(c.Context(), tenant, field, value, limit)
	if err != nil {
		h.Logger.Error("SearchByManifestField failed", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "search failed"})
	}

	return c.JSON(fiber.Map{
		"tenant": tenant,
		"field":  field,
		"value":  value,
		"count":  len(results),
		"results": results,
	})
}
