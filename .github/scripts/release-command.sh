#!/usr/bin/env bash
set -euo pipefail

# Commands may publish externally; check the checkout before handing over control.
commit=$(git rev-parse HEAD)
if [[ "$commit" != "$RELEASE_COMMIT" ]]; then
  echo 'Release command checkout does not match the planned commit.' >&2
  exit 1
fi
bash -e -o pipefail -c "$RELEASE_COMMAND"
