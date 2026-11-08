# Agent Context for Drover Registry

Welcome, AI Agent. This document guides work in `drover-registry`.

## Ecosystem Role

> **Part of the Drover Ecosystem**: **Drover Registry** stores and distributes versioned **agent bundles** and **crew templates**. **Drover Muster** receives webhooks on publish to register capabilities. Glossary: [`CONTEXT.md`](CONTEXT.md). Org index: [`../AGENTS.md`](../AGENTS.md).

## Stack

- **Go** (Fiber HTTP API)
- **Postgres** — package metadata and search index
- **Blob storage** — S3, GCS, or local (`internal/storage/`)
- **Auth** — Zitadel OIDC JWT; org-scoped packages (`internal/api/middleware/auth.go`)
- **OCI** — push/pull handlers (`internal/api/handlers/oci.go`)

## Layout

| Path | Role |
|------|------|
| `cmd/server/` | HTTP server entry |
| `internal/api/handlers/` | Packages, OCI routes |
| `internal/metadata/` | DB, manifests, tar handling |
| `internal/webhook/` | Muster integration |
| `migrations/` | SQL schema |

## Commands

```bash
go test ./...
make test   # if defined in Makefile
```

See [`CONTRIBUTING.md`](CONTRIBUTING.md) and [`README.md`](README.md) for Docker Compose dev setup.

## Conventions

- Preserve **tenant isolation** on every read/write; never bypass org scope in handlers.
- Treat published artifacts as **immutable**; bump version/tag on change.
- New HTTP surface: add tests in `internal/api/handlers/*_test.go`.
- Docs: Diátaxis under `docs/` when added; frontmatter per [`../docs/taxonomy.yaml`](../docs/taxonomy.yaml).

## Out of scope here

- Agent loop implementation → `drover-code`
- Capability policy engine → `drover-muster`
- Hosted job execution → `drover-cloud`
