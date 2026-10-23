#!/usr/bin/env bash
set -euo pipefail

sweet=$1
base=$2
head=$3

# A fresh checkout plus all remote tags makes reruns safe. Never force-fetch tags.
git fetch origin --tags
plan=$("$sweet" git tag --base "$base" --head "$head" --json)

refs=()
while IFS=$'\t' read -r name commit exists; do
  # Include matching local tags too, so retrying a rejected push can finish.
  refs+=("refs/tags/$name:refs/tags/$name")
done < <(jq -r '.[] | [.name, .commit, .exists] | @tsv' <<< "$plan")

if (( ${#refs[@]} )); then
  # Push only this plan's tags, all together. No force pushes or partial releases.
  git push --atomic origin "${refs[@]}"
else
  echo 'No new release tags.'
fi

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  printf 'cli-releases=%s\n' "$(jq -c '[.[] | select(.package == "apps/sweet" or .package == "pkg/sweet" or .package == "pkg/cli") | {tag: .name, commit: .commit}]' <<< "$plan")" >> "$GITHUB_OUTPUT"
fi

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    printf '## Release tags\n\n'
    jq -r '.[] | "- `\(.name)` → `\(.commit)`"' <<< "$plan"
  } >> "$GITHUB_STEP_SUMMARY"
fi
