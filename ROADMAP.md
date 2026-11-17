# drover-registry roadmap

Last updated 18 Nov 2026. Status: **STALE**. Versioned agent and crew package store with OCI handlers and a webhook to Muster. Last code change 25 May 2026. Becomes the bundle store in M4.

## The Drover gap program

In November 2026 Drover was compared with Vercel eve 1.0. Drover leads on isolation, the model gateway and governance, and trails on the layer people touch. The program closes five gaps across the repos: durable multi-turn sessions (drover-kelpies), a Slack-first channels layer (drover-cloud), unified OpenTelemetry tracing with a run viewer (drover-libs, Cloud console), a TypeScript client SDK (drover-cloud), and an extension bundle format (drover-muster, drover-registry). Four capabilities are deferred to Later: shared multi-person sessions, ambient channel listening, agents editing their own code, and session prewarm and migration.

Program tracking issue: in [cloud-shuttle/drover-cloud](https://github.com/cloud-shuttle/drover-cloud/issues?q=is%3Aissue+label%3Aroadmap+Tracking) (private). Sessions run on DBOS, not Temporal (ADR 0005 in drover-kelpies). Dates and day counts are estimates.

| Milestone | Due (estimate) |
|---|---|
| M0 Truth-up | 27 Nov 2026 |
| M1 Trace spine | 18 Dec 2026 |
| M2 Durable sessions | 5 Feb 2027 |
| M3 Channels and SDK | 5 Mar 2027 |
| M4 Extensions | 2 Apr 2027 |
| Later | after M4 |

## This repo in the program

### M4 Extensions (due 2 Apr 2027)

- [Store extension bundles as OCI artifacts](https://github.com/cloud-shuttle/drover-registry/issues/18)

Each item is a GitHub issue labelled `roadmap` with a `gap:*` label and a milestone: [roadmap issues in this repo](https://github.com/cloud-shuttle/drover-registry/issues?q=is%3Aissue+label%3Aroadmap).

## Proposed releases

- `v0.2.0` at M4 (OCI extension bundles)

## Existing roadmap epics

These predate the gap program and stay open:

- [#16 Epic: Deployment and operations](https://github.com/cloud-shuttle/drover-registry/issues/16)
- [#15 Epic: Signatures and supply-chain metadata](https://github.com/cloud-shuttle/drover-registry/issues/15)
- [#14 Epic: Close dreg-000: versioned package storage and distribution](https://github.com/cloud-shuttle/drover-registry/issues/14)

## How this file is kept current

Update this file in the same PR that closes a roadmap issue or moves a milestone. Status words used across Drover: ACTIVE (code changed in the last 60 days), STALE (no code change for 61 to 180 days), PROTOTYPE (experimental or scaffold), SUPERSEDED (replaced by another repo).
