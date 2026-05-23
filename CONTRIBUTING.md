# Contributing to Drover Registry

Thank you for your interest in contributing to **Drover Registry** — the Distributed Agent Registry & Crew Store for the Drover ecosystem.

## Development Setup

### Prerequisites

- Go 1.25+
- Docker & Docker Compose (for local Postgres, MinIO, etc.)
- Git
- (Optional) Zitadel instance or test OIDC provider for auth testing

### Clone and Run

```bash
git clone https://github.com/cloud-shuttle/drover-registry.git
cd drover-registry

# Build
go build -o drover-registry ./cmd/server

# Run with local config (see docs/DEVELOPMENT.md)
go run ./cmd/server
```

## Project Structure

```
drover-registry/
├── cmd/server/           # HTTP server entrypoint (Fiber)
├── internal/
│   ├── api/              # HTTP handlers, routes, middleware
│   ├── auth/             # Zitadel OIDC/JWT validation, tenant scoping
│   ├── config/           # Env + config loading
│   ├── db/               # Postgres migrations, queries, models
│   ├── metadata/         # Version indexing, package manifests
│   ├── storage/          # Pluggable backends: local, s3, gcs
│   │   ├── local/
│   │   ├── s3/
│   │   └── gcs/
│   └── webhook/          # Publisher to drover-muster
├── pkg/
│   ├── oci/              # OCI artifact helpers (future)
│   └── types/            # Shared DTOs, manifests
├── docs/                 # Architecture, API, deployment guides
├── examples/             # Sample agent/crew packages
└── deploy/               # Docker, k8s manifests
```

## Key Tasks & Backlog

Tasks are tracked in `.beads/issues.jsonl` (Beads format, compatible with Drover tooling).

Current priorities (see `.beads/`):
- dreg-001: Storage provider engine (local + S3 + GCS + hash verify)
- dreg-002: Version indexing & metadata parser
- dreg-003: Zitadel auth & tenant isolation
- ...

Run `drover status --tree` (if drover installed) or inspect the jsonl directly.

## Code Style

- Follow standard Go (gofmt, go vet)
- Use Fiber for HTTP (v2)
- Wrap errors: `fmt.Errorf("...: %w", err)`
- Structured logging (slog or logrus? decide on first PR)
- All public packages documented
- Add tests for new storage providers, auth middleware, parsers

## Running Tests

```bash
go test ./...
go test -race ./...
```

Integration tests will use testcontainers for Postgres + MinIO.

## Submitting Changes

1. Create feature branch from main
2. Implement + test
3. Update relevant docs / .beads if task complete
4. Open PR to cloud-shuttle/drover-registry
5. Ensure `make ci` or equivalent passes (lint, test, build)

## License

MIT — see [LICENSE](./LICENSE)

---

Built by [Cloud Shuttle](https://cloudshuttle.com.au) as part of the Drover ecosystem.
