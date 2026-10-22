#!/usr/bin/env bash
set -euo pipefail

# Retained release tooling must also check tags from before module consolidation.
if [[ "${1:-}" == version-file ]]; then
  if [[ -f go.mod ]]; then
    echo go.mod
  else
    test -f pkg/cli/go.mod
    echo pkg/cli/go.mod
  fi
  exit 0
fi

if [[ -f go.mod ]]; then
  go test ./...
  go vet ./...
else
  go test ./pkg/cli/... ./pkg/core/...
  go vet ./pkg/cli/... ./pkg/core/...
fi
