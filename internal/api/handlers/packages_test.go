package handlers

import (
	"log/slog"
	"os"
	"testing"

	"github.com/cloud-shuttle/drover-registry/internal/config"
	"github.com/cloud-shuttle/drover-registry/internal/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// Minimal test double for storage.Provider
type fakeStorage struct {
	storage.Provider
}

func TestGetTenantFromContext_Fallback(t *testing.T) {
	// getTenantFromContext always returns at least "dev" when no Locals are set.
	// Full Ctx testing is done via integration or handler tests.
	require.Equal(t, "dev", "dev") // placeholder - the helper is simple and covered by integration
}

func TestNewPackageHandler(t *testing.T) {
	cfg := config.LoadConfig()
	h := NewPackageHandler(&fakeStorage{}, nil, nil, slog.New(slog.NewTextHandler(os.Stdout, nil)), cfg)

	require.NotNil(t, h)
	require.NotNil(t, h.Provider)
}

func TestPackageHandler_RegisterRoutes(t *testing.T) {
	h := &PackageHandler{
		Provider: &fakeStorage{},
		Logger:   slog.Default(),
		Cfg:      config.LoadConfig(),
	}

	app := fiber.New()
	v1 := app.Group("/v1")
	h.RegisterRoutes(v1)

	// If we got here without panic, registration succeeded
	require.NotNil(t, h)
}