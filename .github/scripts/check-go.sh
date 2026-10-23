#!/usr/bin/env bash
set -euo pipefail

# Retained release tooling must also check historical source layouts.
if [[ "${1:-}" == version-file ]]; then
  if [[ -f apps/sweet/go.mod ]]; then
    echo apps/sweet/go.mod
  elif [[ -f pkg/sweet/go.mod ]]; then
    echo pkg/sweet/go.mod
  elif [[ -f go.mod ]]; then
    echo go.mod
  else
    test -f pkg/cli/go.mod
    echo pkg/cli/go.mod
  fi
  exit 0
fi

if [[ -f apps/sweet/go.mod ]]; then
  go test ./apps/sweet/...
  go vet ./apps/sweet/...
elif [[ -f pkg/sweet/go.mod ]]; then
  go test ./pkg/sweet/...
  go vet ./pkg/sweet/...
elif [[ -f go.mod ]]; then
  go test ./...
  go vet ./...
else
  go test ./pkg/cli/... ./pkg/core/...
  go vet ./pkg/cli/... ./pkg/core/...
fi
