#!/usr/bin/env bash
set -euo pipefail

# Only this repository needs Node, pnpm, and Go checks after a bump.
# The docs site shares the Sweet Release version.
if jq -e '.packages | index("apps/sweet") != null' <<< "$RELEASE_JOB" > /dev/null; then
  package_manager=$(jq -r .packageManager apps/docs/package.json)
  npx --yes --package=node@24 --package="$package_manager" -c '
    set -e
    pnpm --dir apps/docs install --lockfile-only --no-frozen-lockfile --ignore-scripts
    pnpm --dir apps/docs install --frozen-lockfile
    pnpm --dir apps/docs run check
    pnpm --dir apps/docs run build
  '
fi
bash "$(dirname "$0")/check-go.sh"
go build -o "$RUNNER_TEMP/sweet-verified" ./apps/sweet/cmd/sweet
