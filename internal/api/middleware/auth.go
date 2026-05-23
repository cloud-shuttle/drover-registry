package middleware

import (
	"strings"

	"github.com/cloud-shuttle/drover-registry/internal/config"
	"github.com/cloud-shuttle/drover-registry/internal/infra"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware provides Zitadel JWT validation with dev fallback.
type AuthMiddleware struct {
	validator   *infra.ZitadelValidator
	cfg         config.Config
}

func NewAuthMiddleware(validator *infra.ZitadelValidator, cfg config.Config) *AuthMiddleware {
	return &AuthMiddleware{validator: validator, cfg: cfg}
}

// Handler is the Fiber middleware.
// It extracts the tenant (org) ID and stores it in c.Locals("tenantID").
// Enforcement happens via RequireTenant middleware.
func (a *AuthMiddleware) Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Dev auth path (no real Zitadel)
		if a.cfg.EnableDevAuth && a.validator == nil {
			tenant := c.Get("X-Org-ID", c.Get("X-Tenant-ID", "dev"))
			if tenant == "" {
				tenant = "dev"
			}
			c.Locals("tenantID", tenant)
			c.Locals("authMode", "dev")
			return c.Next()
		}

		// Real Zitadel path
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return fiber.ErrUnauthorized
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return fiber.ErrUnauthorized
		}
		token := parts[1]

		if a.validator == nil {
			// Should not happen if config is correct, but be safe
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "auth validator not configured"})
		}

		claims, err := a.validator.Validate(c.Context(), token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token", "detail": err.Error()})
		}

		tenant := claims.GetTenantID()
		if tenant == "" {
			tenant = "unknown"
		}

		c.Locals("tenantID", tenant)
		c.Locals("userID", claims.Sub)
		c.Locals("authMode", "zitadel")
		return c.Next()
	}
}

// RequireTenant ensures a tenantID was set by auth middleware.
// Call this after the auth handler on protected routes.
func RequireTenant() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tenant, ok := c.Locals("tenantID").(string)
		if !ok || tenant == "" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "tenant/org context required (set X-Org-ID header in dev mode or provide valid Zitadel token)",
			})
		}
		return c.Next()
	}
}

// GetTenant extracts the tenant from Fiber context (convenience for handlers).
func GetTenant(c *fiber.Ctx) string {
	if t, ok := c.Locals("tenantID").(string); ok {
		return t
	}
	return "dev"
}

// DevAuthMiddleware is a standalone lightweight dev-only middleware (used when no Zitadel at all).
func DevAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tenant := c.Get("X-Org-ID", c.Get("X-Tenant-ID", "dev"))
		c.Locals("tenantID", tenant)
		c.Locals("authMode", "dev-header")
		return c.Next()
	}
}

// Optional: simple helper to extract bearer token (if needed elsewhere)
func extractBearerToken(c *fiber.Ctx) string {
	h := c.Get("Authorization")
	if h == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// ClaimsFromContext can be expanded later if we want full claims available to handlers.
func ClaimsFromContext(c *fiber.Ctx) *jwt.MapClaims {
	if claims, ok := c.Locals("claims").(*jwt.MapClaims); ok {
		return claims
	}
	return nil
}
