#!/usr/bin/env bash
set -euo pipefail

base=$1
head=$2

# Remote tags must be present for Sweet's conflict checks. Never force-fetch.
git fetch origin --tags
# Sweet validates the whole range before creating any missing annotated tags.
plan=$(sweet git tag --base "$base" --head "$head" --json)

# Push one tag at a time: GitHub creates no push events when more than three
# tags arrive together. Matching remote tags are no-ops, so reruns finish a
# partially pushed range. Never force-push.
while IFS= read -r name; do
  git push origin "refs/tags/$name:refs/tags/$name"
done < <(jq -r '.[].name' <<< "$plan")

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  printf 'tags=%s\n' "$(jq -c '[.[] | {package, version, tag: .name, commit}]' <<< "$plan")" >> "$GITHUB_OUTPUT"
fi

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    printf '## Release tags\n\n'
    jq -r 'if length == 0 then "No committed version changes." else .[] | "- `\(.name)` → `\(.commit)`" end' <<< "$plan"
  } >> "$GITHUB_STEP_SUMMARY"
fi
