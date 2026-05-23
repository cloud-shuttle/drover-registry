# drover-registry

**Distributed Agent Registry & Crew Store** — versioned storage and distribution for agent logic, crew templates, and MCP-compatible artifacts in the Drover ecosystem.

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev/)
[![Fiber](https://img.shields.io/badge/Fiber-v2-4A90E2)](https://gofiber.io/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## What is Drover Registry?

It provides a secure, tenant-isolated package registry for:

- **Agent bundles** (self-contained agent definitions + prompts + tools)
- **Crew templates** (multi-agent orchestration definitions)
- **Versioned artifacts** consumable by Drover, Drover Code, and compatible runtimes

Packages are stored as immutable, checksummed tarballs (or OCI artifacts) in S3/GCS with a metadata layer for search, dependency resolution, and governance hooks.

## Key Features (Planned)

- **Secure Storage**: Versioned uploads to S3/GCS (or local for dev) with SHA-256 verification on write/read.
- **Zitadel Isolation**: OIDC JWT validation + org-scoped tenants. Packages are namespaced by Zitadel org ID (e.g. `cloud-shuttle/researcher-kelpie:v1.2.0`).
- **OCI Compliance**: Push/pull via `oras` / Docker clients (`oras push ...`).
- **Webhook Integration**: On successful publish → POST to `drover-muster` to register new capabilities/tools declared in the package.
- **High-performance streaming**: Range requests, compression, CDN-friendly URLs for fast agent bootstrap in microVMs.
- **Metadata & Search**: Postgres-backed index of manifests, dependencies, signatures.

## Architecture Overview

```
┌──────────────┐     ┌────────────────────┐     ┌─────────────────┐
│  Drover /    │────▶│  drover-registry   │────▶│  S3 / GCS /     │
│  Agent Runtimes│     │  (this service)    │     │  Local Storage  │
└──────────────┘     └────────────────────┘     └─────────────────┘
                            │
                            │ webhook (on publish)
                            ▼
                     ┌──────────────────┐
                     │  drover-muster   │  (capability governance)
                     └──────────────────┘
                            │
                            │ Zitadel OIDC
                            ▼
                     ┌──────────────────┐
                     │  Zitadel IAM     │
                     └──────────────────┘
```

See [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) (to be written) and the backlog in `.beads/issues.jsonl`.

## Current Status

Early scaffold. See `.beads/issues.jsonl` for the detailed task breakdown (dreg-000 epic etc.).

Implemented so far:
- Basic Fiber HTTP server with `/healthz` + `/readyz`
- Project layout matching Drover ecosystem conventions
- MIT license + contributing guide

Next up (dreg-001):
- Storage abstraction + local disk backend + hash verification

## Quick Start (Dev)

```bash
git clone https://github.com/cloud-shuttle/drover-registry.git
cd drover-registry

# Run the server (will listen :8080)
go run ./cmd/server
```

In another terminal:

```bash
curl http://localhost:8080/healthz
```

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) and upcoming `docs/DEVELOPMENT.md`.

### Local Stack (planned)

```bash
docker compose up -d   # postgres + minio + ...
make migrate
make run
```

## Integration Points

- **Publishers**: `oras`, custom CLI, CI pipelines, Drover `drover publish` (future)
- **Consumers**: Drover workers, agent runtimes, `drover-muster` sync
- **Auth**: Zitadel-issued JWTs with `urn:zitadel:iam:org:id:<org>` scope claim

## Backlog & Roadmap

Work items live in `.beads/issues.jsonl`:

- `dreg-000` Epic: Versioned Package Storage & Distribution Pipeline
- `dreg-001` Storage provider engine configuration
- `dreg-002` Version indexing & metadata API parser
- `dreg-003` Zitadel OIDC authentication & scope checks
- `dreg-004` Webhook publisher for Muster synchronization
- `dreg-005` High-performance package streaming endpoint

Contributions that close beads items are especially welcome.

## Related Projects

- [cloud-shuttle/drover](https://github.com/cloud-shuttle/drover) — The Parallel Coordinator (Go)
- [cloud-shuttle/drover-muster](https://github.com/cloud-shuttle/drover-muster) — Capability Registry & Governance (Go + Fiber)
- [cloud-shuttle/drover-libs](https://github.com/cloud-shuttle/drover-libs) — Shared libraries (private)

## License

MIT — see [LICENSE](./LICENSE)

---

Built with ❤️ by [Cloud Shuttle](https://cloudshuttle.com.au) for the autonomous engineering future.
