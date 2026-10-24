#!/usr/bin/env bash
set -euo pipefail

# This repository's policy. The shared release workflow only runs this command.
tooling=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
export SWEET_RELEASE_TOOL="$tooling/.github/scripts/github-release.py"
export SWEET_GO_CHECK="$tooling/.github/scripts/check-go.sh"

state=$(python3 "$SWEET_RELEASE_TOOL" status "$RELEASE_TAG" "$RELEASE_COMMIT")
if [[ "$state" == published ]]; then
  echo "$RELEASE_TAG is already published."
  exit 0
fi
RELEASE_PHASE=draft python3 "$SWEET_RELEASE_TOOL"

case "$RELEASE_PACKAGE" in
  apps/sweet|pkg/sweet|pkg/cli)
    # Install the pinned build tool if it is not already available.
    if ! command -v goreleaser > /dev/null; then
      release_bin=$(mktemp -d "$RUNNER_TEMP/sweet-build-XXXXXX")
      # The build tool can require a newer Go patch than Sweet itself.
      GOTOOLCHAIN=auto GOBIN="$release_bin" go install github.com/goreleaser/goreleaser/v2@v2.18.2
      export PATH="$release_bin:$PATH"
    fi
    bash "$tooling/.github/scripts/publish-cli.sh" "$RELEASE_TAG" "$tooling/.goreleaser.yml"
    ;;
esac

RELEASE_PHASE=publish python3 "$SWEET_RELEASE_TOOL"
