#!/bin/bash
set -e

ROOT="$(pwd)"

find . -name "*.go" -not -path "*/vendor/*" -not -path "*/mock_*" -not -name "*_test.go" -not -path "*/.drover-*-workers/*" > .quality-gate-files.txt

LIMIT="${1:-30000}"

python3 "$ROOT/scripts/quality-gate.py" \
  "$ROOT" \
  --coverage "$ROOT/coverage.out" \
  --limit "$LIMIT" \
  --from-file .quality-gate-files.txt
