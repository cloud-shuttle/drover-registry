# Drover Registry — domain glossary

Canonical language for **Drover Registry**. Shared actors (**customer**, **member**, **agent**, **platform operator**) are defined in [`../CONTEXT-MAP.md`](../CONTEXT-MAP.md). Capability governance after publish: [`../drover-muster/CONTEXT.md`](../drover-muster/CONTEXT.md). Implementation details belong in `docs/` and README, not here.

## Product role

**Drover Registry**  
Secure, tenant-isolated distribution for versioned **agent bundles**, **crew templates**, and related artifacts. OCI-compatible push/pull; immutable storage (S3/GCS/local) with Postgres metadata. On publish, webhooks notify **Drover Muster** to register declared capabilities.

## Package

**Agent bundle**  
Self-contained package: agent definition, prompts, tool references, and metadata manifest. Consumed by Drover Code, Kelpies, or Cloud bootstrap flows.

**Crew template**  
Multi-agent orchestration definition packaged for reuse across **customers**.

**Package namespace**  
Tenant-scoped name (e.g. Zitadel org id + package name + semver tag). Distinct from a **customer id** but derived from identity boundaries.

## Storage & integrity

**Immutable artifact**  
Checksummed tarball or OCI layer; new version on publish, no in-place overwrite of released tags.

**Manifest**  
Structured metadata (dependencies, capabilities, signatures) indexed in Postgres for search and policy hooks.

## Integration

**Muster webhook**  
POST to **Drover Muster** after successful publish so new tools/skills enter the **capability registry** under governance rules.

**Registry pull**  
Runtime fetch of a pinned version into a **worker instance** or local sandbox—distinct from cloning a **customer** Git **workspace**.
