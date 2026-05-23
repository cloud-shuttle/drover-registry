package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloud-shuttle/drover-registry/internal/infra"
	"github.com/jackc/pgx/v5"
)

// Package represents a logical package (name within a tenant).
type Package struct {
	ID        int64     `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// PackageVersion is one specific version of a package.
type PackageVersion struct {
	ID           int64           `json:"id"`
	PackageID    int64           `json:"package_id"`
	Version      string          `json:"version"`
	Digest       string          `json:"digest"`
	SizeBytes    int64           `json:"size_bytes"`
	StorageKey   string          `json:"storage_key"`
	Manifest     json.RawMessage `json:"manifest"`
	PublishedBy  string          `json:"published_by"`
	CreatedAt    time.Time       `json:"created_at"`
}

// Store provides metadata persistence over Postgres.
type Store struct {
	db *infra.DB
}

func NewStore(db *infra.DB) *Store {
	return &Store{db: db}
}

// UpsertPackage ensures the (tenant, name) pair exists and returns its ID.
func (s *Store) UpsertPackage(ctx context.Context, tenantID, name string) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("database not available")
	}

	const q = `
		INSERT INTO packages (tenant_id, name)
		VALUES ($1, $2)
		ON CONFLICT (tenant_id, name) DO UPDATE SET updated_at = NOW()
		RETURNING id
	`
	var id int64
	err := s.db.QueryRow(ctx, q, tenantID, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert package: %w", err)
	}
	return id, nil
}

// InsertVersion records a new version (assumes package exists).
func (s *Store) InsertVersion(ctx context.Context, pkgID int64, v *PackageVersion) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}

	manifestJSON := v.Manifest
	if manifestJSON == nil {
		manifestJSON = []byte("{}")
	}

	const q = `
		INSERT INTO package_versions
			(package_id, version, digest, size_bytes, storage_key, manifest, published_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (package_id, version) DO UPDATE
			SET digest = EXCLUDED.digest,
			    size_bytes = EXCLUDED.size_bytes,
			    storage_key = EXCLUDED.storage_key,
			    manifest = EXCLUDED.manifest,
			    published_by = EXCLUDED.published_by
		RETURNING id, created_at
	`
	return s.db.QueryRow(ctx, q,
		pkgID, v.Version, v.Digest, v.SizeBytes, v.StorageKey, manifestJSON, v.PublishedBy,
	).Scan(&v.ID, &v.CreatedAt)
}

// GetVersionByNameVersion is a convenience lookup (used by fetch if we want to resolve without digest).
func (s *Store) GetVersionByNameVersion(ctx context.Context, tenantID, name, version string) (*PackageVersion, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	const q = `
		SELECT pv.id, pv.package_id, pv.version, pv.digest, pv.size_bytes, pv.storage_key, pv.manifest, pv.published_by, pv.created_at
		FROM package_versions pv
		JOIN packages p ON p.id = pv.package_id
		WHERE p.tenant_id = $1 AND p.name = $2 AND pv.version = $3
		LIMIT 1
	`
	row := s.db.QueryRow(ctx, q, tenantID, name, version)

	var pv PackageVersion
	var manifestRaw []byte
	err := row.Scan(&pv.ID, &pv.PackageID, &pv.Version, &pv.Digest, &pv.SizeBytes, &pv.StorageKey, &manifestRaw, &pv.PublishedBy, &pv.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	pv.Manifest = manifestRaw
	return &pv, nil
}

// ListVersions returns all versions for a given package in a tenant (newest first).
func (s *Store) ListVersions(ctx context.Context, tenantID, name string) ([]PackageVersion, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	const q = `
		SELECT pv.id, pv.package_id, pv.version, pv.digest, pv.size_bytes, pv.storage_key, pv.manifest, pv.published_by, pv.created_at
		FROM package_versions pv
		JOIN packages p ON p.id = pv.package_id
		WHERE p.tenant_id = $1 AND p.name = $2
		ORDER BY pv.created_at DESC
	`
	rows, err := s.db.Query(ctx, q, tenantID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []PackageVersion
	for rows.Next() {
		var pv PackageVersion
		var manifestRaw []byte
		if err := rows.Scan(&pv.ID, &pv.PackageID, &pv.Version, &pv.Digest, &pv.SizeBytes, &pv.StorageKey, &manifestRaw, &pv.PublishedBy, &pv.CreatedAt); err != nil {
			return nil, err
		}
		pv.Manifest = manifestRaw
		versions = append(versions, pv)
	}
	return versions, nil
}

// GetVersionByDigest looks up a specific version using the content digest (very useful for OCI-like resolution).
func (s *Store) GetVersionByDigest(ctx context.Context, tenantID, name, digest string) (*PackageVersion, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	const q = `
		SELECT pv.id, pv.package_id, pv.version, pv.digest, pv.size_bytes, pv.storage_key, pv.manifest, pv.published_by, pv.created_at
		FROM package_versions pv
		JOIN packages p ON p.id = pv.package_id
		WHERE p.tenant_id = $1 AND p.name = $2 AND pv.digest = $3
		LIMIT 1
	`
	row := s.db.QueryRow(ctx, q, tenantID, name, digest)

	var pv PackageVersion
	var manifestRaw []byte
	err := row.Scan(&pv.ID, &pv.PackageID, &pv.Version, &pv.Digest, &pv.SizeBytes, &pv.StorageKey, &manifestRaw, &pv.PublishedBy, &pv.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	pv.Manifest = manifestRaw
	return &pv, nil
}

// SearchByManifestField is a simple example of querying inside the JSONB manifest (e.g. by type or label).
func (s *Store) SearchByManifestField(ctx context.Context, tenantID, field, value string, limit int) ([]PackageVersion, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not available")
	}
	if limit <= 0 {
		limit = 20
	}

	// Simple containment query: manifest @> '{"type": "crew"}' or similar
	// For flexibility we use a jsonb_path_exists or @> with constructed object.
	q := `
		SELECT pv.id, pv.package_id, pv.version, pv.digest, pv.size_bytes, pv.storage_key, pv.manifest, pv.published_by, pv.created_at
		FROM package_versions pv
		JOIN packages p ON p.id = pv.package_id
		WHERE p.tenant_id = $1
		  AND pv.manifest @> $2::jsonb
		ORDER BY pv.created_at DESC
		LIMIT $3
	`

	condition := map[string]string{field: value}
	condJSON, _ := json.Marshal(condition)

	rows, err := s.db.Query(ctx, q, tenantID, condJSON, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PackageVersion
	for rows.Next() {
		var pv PackageVersion
		var manifestRaw []byte
		if err := rows.Scan(&pv.ID, &pv.PackageID, &pv.Version, &pv.Digest, &pv.SizeBytes, &pv.StorageKey, &manifestRaw, &pv.PublishedBy, &pv.CreatedAt); err != nil {
			return nil, err
		}
		pv.Manifest = manifestRaw
		results = append(results, pv)
	}
	return results, nil
}

