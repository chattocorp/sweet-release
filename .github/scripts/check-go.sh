#!/usr/bin/env bash
set -euo pipefail

# Go checks shared by CI, release PRs, and release builds.
unformatted=$(gofmt -l apps/sweet)
if [[ -n "$unformatted" ]]; then
  printf 'Run gofmt on these files:\n%s\n' "$unformatted" >&2
  exit 1
fi
go test -race ./apps/sweet/...
go vet ./apps/sweet/...
