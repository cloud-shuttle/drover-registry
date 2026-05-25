package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cloud-shuttle/drover-registry/internal/config"
)

// Publisher sends events to drover-muster after successful package publication.
type Publisher struct {
	url    string
	secret string
	client *http.Client
}

func NewPublisher(cfg config.Config) *Publisher {
	return &Publisher{
		url:    cfg.MusterWebhookURL,
		secret: cfg.MusterWebhookSecret,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Payload is the event sent to muster on publish.
type Payload struct {
	Event      string          `json:"event"`
	TenantID   string          `json:"tenant_id"`
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	Digest     string          `json:"digest"`
	Size       int64           `json:"size"`
	StorageKey string          `json:"storage_key,omitempty"`
	Timestamp  string          `json:"timestamp"`
	Manifest   json.RawMessage `json:"manifest,omitempty"`
}

// Publish sends the event (fire-and-forget with best effort).
func (p *Publisher) Publish(ctx context.Context, tenant, name, version, digest string, size int64, storageKey string, manifestRaw []byte) error {
	if p.url == "" {
		return nil // no-op when not configured
	}

	payload := Payload{
		Event:      "package.published",
		TenantID:   tenant,
		Name:       name,
		Version:    version,
		Digest:     digest,
		Size:       size,
		StorageKey: storageKey,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Manifest:   manifestRaw,
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Optional HMAC signature
	if p.secret != "" {
		mac := hmac.New(sha256.New, []byte(p.secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Hub-Signature-256", "sha256="+sig)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook POST failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
