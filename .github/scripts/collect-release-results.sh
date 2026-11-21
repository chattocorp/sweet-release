#!/usr/bin/env bash
set -euo pipefail

# Merge the desired.json results of all release PR jobs into one JSON array.
# download-artifact extracts a single matching artifact directly into the
# directory and several into per-artifact subdirectories, so search both.
# Missing results must stop cleanup, not be treated as no releases.
dir=$1
expected=$2

files=()
if [[ -d "$dir" ]]; then
  while IFS= read -r -d '' file; do files+=("$file"); done \
    < <(find "$dir" -name desired.json -type f -print0 | sort -z)
fi
if (( expected < 1 || ${#files[@]} != expected )); then
  echo "Expected $expected release PR results in $dir, found ${#files[@]}." >&2
  exit 1
fi
jq -sc 'add' "${files[@]}"
