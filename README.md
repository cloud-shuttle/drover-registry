# drover-registry

Distributed Agent Registry & Crew Store for versioning and distributing agent logic and crew templates.

## Key Features
- **Secure Storage**: Supports versioned package uploads in S3/GCS with checksum hashing verification.
- **Zitadel Isolation**: Enforces OIDC scopes mapping to Zitadel Org IDs to isolate tenant packages.
- **Webhook Bindings**: Triggers callbacks to `drover-muster` to sync capability definitions upon successful package publications.

## Backlog
Work items, epics, and tasks are tracked locally in JSON Lines format inside [`.beads/issues.jsonl`](.beads/issues.jsonl) following the platform's Beads convention.
