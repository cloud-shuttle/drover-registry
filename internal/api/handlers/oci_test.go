package handlers

import (
	"log/slog"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func TestNewOCIHandler(t *testing.T) {
	h := NewOCIHandler(&fakeStorage{}, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	require.NotNil(t, h)
	require.NotNil(t, h.uploadSessions)
}

func TestOCIHandler_Ping(t *testing.T) {
	h := NewOCIHandler(&fakeStorage{}, slog.Default())

	app := fiber.New()
	v2g := app.Group("/v2")
	h.RegisterRoutes(v2g)

	// Simple smoke test - just ensure the route is registered without panic
	require.NotNil(t, h)
}