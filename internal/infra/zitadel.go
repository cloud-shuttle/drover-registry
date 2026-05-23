package infra

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

type ZitadelValidator struct {
	issuer string
	jwks   *keyfunc.JWKS
}

func NewZitadelValidator(issuer string) (*ZitadelValidator, error) {
	if issuer == "" {
		return nil, fmt.Errorf("zitadel issuer URL is required")
	}

	jwksURL := issuer + "/.well-known/jwks.json"
	log.Printf("Zitadel validator initialized with JWKS endpoint: %s", jwksURL)

	options := keyfunc.Options{
		RefreshInterval:    time.Hour,
		RefreshRateLimit:   time.Minute * 5,
		RefreshTimeout:     time.Second * 10,
		RefreshErrorHandler: func(err error) {
			log.Printf("JWKS refresh error: %s", err.Error())
		},
	}
	jwks, err := keyfunc.Get(jwksURL, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWKS from %s: %w", jwksURL, err)
	}

	return &ZitadelValidator{
		issuer: issuer,
		jwks:   jwks,
	}, nil
}

func (z *ZitadelValidator) Validate(ctx context.Context, tokenString string) (*ZitadelClaims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("token is empty")
	}

	claims := &ZitadelClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, z.jwks.Keyfunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if claims.Issuer != z.issuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", z.issuer, claims.Issuer)
	}

	return claims, nil
}

// ZitadelClaims captures the claims we care about for tenant isolation.
// Zitadel puts the org ID under "urn:zitadel:iam:org:id" or a custom "org_id" claim depending on setup.
// We support both common patterns.
type ZitadelClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	// Common Zitadel org claim
	OrgID string `json:"org_id"`
	// Alternative Zitadel claim (used in many real deployments)
	UrnZitadelIamOrgID string `json:"urn:zitadel:iam:org:id"`
	Roles              []string `json:"roles"`
	jwt.RegisteredClaims
}

func (c *ZitadelClaims) GetTenantID() string {
	if c.UrnZitadelIamOrgID != "" {
		return c.UrnZitadelIamOrgID
	}
	if c.OrgID != "" {
		return c.OrgID
	}
	// Fallback: use subject as tenant (not ideal but works for single-org dev)
	return c.Sub
}
