#!/bin/bash
# Canonical copy: https://github.com/cloud-shuttle/home/blob/main/scripts/run-quality-gate.sh
# This in-repo copy is kept so CI can run without checking out the Home Repo.
# Per-repo extra ignore paths and default limits stay in this file (local overlay).
set -e

ROOT="$(pwd)"

find . -name "*.go" -not -path "*/vendor/*" -not -path "*/mock_*" -not -name "*_test.go" -not -path "*/.drover-*-workers/*" > .quality-gate-files.txt

LIMIT="${1:-30000}"

python3 "$ROOT/scripts/quality-gate.py" \
  "$ROOT" \
  --coverage "$ROOT/coverage.out" \
  --limit "$LIMIT" \
  --from-file .quality-gate-files.txt
