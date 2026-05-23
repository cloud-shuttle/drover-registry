-- +migrate Up
-- Initial schema for drover-registry package metadata (dreg-002)

CREATE TABLE IF NOT EXISTS packages (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   TEXT NOT NULL,                    -- Zitadel org ID or tenant slug
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_packages_tenant_name ON packages (tenant_id, name);

CREATE TABLE IF NOT EXISTS package_versions (
    id            BIGSERIAL PRIMARY KEY,
    package_id    BIGINT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    version       TEXT NOT NULL,                  -- e.g. "v1.2.0" or "1.2.0"
    digest        TEXT NOT NULL,                  -- "sha256:abc123..."
    size_bytes    BIGINT NOT NULL,
    storage_key   TEXT NOT NULL,                  -- key in S3/GCS/local
    manifest      JSONB NOT NULL,                 -- the parsed PackageManifest
    published_by  TEXT,                           -- subject from JWT or "dev"
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (package_id, version)
);

CREATE INDEX IF NOT EXISTS idx_versions_package ON package_versions (package_id);
CREATE INDEX IF NOT EXISTS idx_versions_digest ON package_versions (digest);
CREATE INDEX IF NOT EXISTS idx_versions_tenant_lookup 
    ON package_versions USING gin ((manifest->'name'), (manifest->'type'));
