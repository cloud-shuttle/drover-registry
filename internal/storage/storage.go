package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFound         = errors.New("object not found")
	ErrChecksumMismatch = errors.New("checksum mismatch")
)

// PackageRef identifies a stored artifact (package version).
// Example: Name="researcher-kelpie", Version="v1.2.0", Digest="sha256:..."
type PackageRef struct {
	TenantID string // Zitadel org ID or tenant slug for isolation
	Name     string // e.g. "researcher-kelpie"
	Version  string // semver or tag, e.g. "v1.2.0"
	Digest   string // content digest (sha256:abc123...)
}

// ObjectInfo contains metadata returned after Put/Get operations.
type ObjectInfo struct {
	Ref        PackageRef
	Size       int64
	Checksum   string // sha256 hex
	StoredAt   time.Time
	StorageKey string // internal key/path in backend
}

// Provider is the pluggable storage abstraction for drover-registry.
// All implementations MUST verify checksums on write (and optionally on read).
type Provider interface {
	// Put writes the content for the given ref. The caller provides a pre-computed
	// checksum (hex-encoded sha256). The implementation must verify the stream
	// matches the checksum before persisting.
	Put(ctx context.Context, ref PackageRef, r io.Reader, size int64, checksum string) (*ObjectInfo, error)

	// Get returns a reader for the stored object. Caller is responsible for closing.
	// Implementations should support range requests via ctx or by wrapping.
	Get(ctx context.Context, ref PackageRef) (io.ReadCloser, *ObjectInfo, error)

	// Delete removes the object (idempotent).
	Delete(ctx context.Context, ref PackageRef) error

	// Exists reports whether the object is present.
	Exists(ctx context.Context, ref PackageRef) (bool, error)

	// Head returns metadata without opening the full stream.
	Head(ctx context.Context, ref PackageRef) (*ObjectInfo, error)

	// ListVersions returns all known versions for a name within a tenant (for listing).
	// Note: full listing/search is handled by the metadata layer; this is storage-level.
	ListVersions(ctx context.Context, tenantID, name string) ([]string, error)
}

// Config holds backend-specific settings (populated from env / config).
type Config struct {
	Backend string // "local", "s3", "gcs"

	// Local backend
	LocalRoot string // e.g. "./storage" or "/var/lib/drover-registry"

	// S3 / GCS common
	Bucket string
	Region string

	// S3 specific (AWS or MinIO)
	Endpoint        string // for MinIO or custom S3-compatible
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool // true for MinIO

	// GCS specific
	ProjectID          string
	CredentialsFile    string // path to service account JSON
}
