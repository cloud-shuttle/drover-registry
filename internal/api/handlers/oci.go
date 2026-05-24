package handlers

import (
	"bytes"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloud-shuttle/drover-registry/internal/storage"
	"github.com/gofiber/fiber/v2"
)

// OCIHandler provides basic single-layer OCI distribution support for oras/docker.
type OCIHandler struct {
	Provider storage.Provider
	Logger   *slog.Logger

	// Simple in-memory sessions for demo (replace with Redis/DB in production)
	uploadSessions map[string]string // uuid -> repository name
}

// NewOCIHandler creates a new OCI handler.
func NewOCIHandler(provider storage.Provider, logger *slog.Logger) *OCIHandler {
	return &OCIHandler{
		Provider:       provider,
		Logger:         logger,
		uploadSessions: make(map[string]string),
	}
}

// RegisterRoutes mounts the minimal OCI v2 endpoints on the given group (usually /v2).
func (h *OCIHandler) RegisterRoutes(oci fiber.Router) {
	oci.Get("/", h.Ping)

	oci.Post("/:name/blobs/uploads/", h.StartBlobUpload)
	oci.Put("/:name/blobs/uploads/:uuid", h.CompleteBlobUpload)
	oci.Head("/:name/blobs/:digest", h.HeadBlob)

	oci.Put("/:name/manifests/:ref", h.PutManifest)
	oci.Get("/:name/manifests/:ref", h.GetManifest)
}

func (h *OCIHandler) Ping(c *fiber.Ctx) error {
	c.Set("Docker-Distribution-API-Version", "registry/2.0")
	return c.SendStatus(fiber.StatusOK)
}

func (h *OCIHandler) StartBlobUpload(c *fiber.Ctx) error {
	name := c.Params("name")
	uuid := fmt.Sprintf("u%d", time.Now().UnixNano())
	h.uploadSessions[uuid] = name

	location := fmt.Sprintf("/v2/%s/blobs/uploads/%s", name, uuid)
	c.Set("Location", location)
	c.Set("Docker-Upload-Uuid", uuid)
	c.Set("Docker-Distribution-API-Version", "registry/2.0")
	return c.SendStatus(fiber.StatusAccepted)
}

func (h *OCIHandler) CompleteBlobUpload(c *fiber.Ctx) error {
	name := c.Params("name")
	uuid := c.Params("uuid")
	digest := c.Query("digest")

	if digest == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "digest query parameter required"})
	}

	body := c.Body()
	if len(body) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "empty blob"})
	}

	tenant := getTenantFromContext(c) // respects auth / X-Org-ID
	ref := storage.PackageRef{
		TenantID: tenant,
		Name:     name,
		Version:  digest,
		Digest:   digest,
	}

	_, err := h.Provider.Put(c.Context(), ref, bytes.NewReader(body), int64(len(body)), digest)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	delete(h.uploadSessions, uuid)

	c.Set("Docker-Distribution-API-Version", "registry/2.0")
	c.Set("Location", fmt.Sprintf("/v2/%s/blobs/%s", name, digest))
	return c.SendStatus(fiber.StatusCreated)
}

func (h *OCIHandler) HeadBlob(c *fiber.Ctx) error {
	digest := c.Params("digest")
	name := c.Params("name")
	tenant := getTenantFromContext(c)

	ref := storage.PackageRef{TenantID: tenant, Name: name, Version: digest, Digest: digest}
	exists, _ := h.Provider.Exists(c.Context(), ref)
	if !exists {
		return c.SendStatus(fiber.StatusNotFound)
	}

	c.Set("Docker-Distribution-API-Version", "registry/2.0")
	c.Set("Docker-Content-Digest", digest)
	return c.SendStatus(fiber.StatusOK)
}

func (h *OCIHandler) PutManifest(c *fiber.Ctx) error {
	name := c.Params("name")
	tag := c.Params("ref")
	body := c.Body()

	tenant := getTenantFromContext(c)
	digest := fmt.Sprintf("sha256:manifest-%d", time.Now().UnixNano())

	ref := storage.PackageRef{
		TenantID: tenant,
		Name:     name,
		Version:  tag,
		Digest:   digest,
	}

	_, err := h.Provider.Put(c.Context(), ref, bytes.NewReader(body), int64(len(body)), digest)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	c.Set("Docker-Distribution-API-Version", "registry/2.0")
	c.Set("Docker-Content-Digest", digest)
	c.Set("Location", fmt.Sprintf("/v2/%s/manifests/%s", name, tag))
	return c.SendStatus(fiber.StatusCreated)
}

func (h *OCIHandler) GetManifest(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "OCI manifest GET not fully wired for pull yet"})
}
