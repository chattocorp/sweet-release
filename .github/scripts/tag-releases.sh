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

# Describe each tag. latest is true for the highest stable version among all
# local tags with the same prefix, which include the fetched remote tags.
tags=$(git tag --list | jq -R . | jq -cs --argjson plan "$plan" '
  def stable: capture("^(?<a>0|[1-9][0-9]*)\\.(?<b>0|[1-9][0-9]*)\\.(?<c>0|[1-9][0-9]*)(\\+[0-9A-Za-z.-]+)?$")
    | [.a, .b, .c | tonumber];
  . as $all | [$plan[] |
    (.name[:(.name | length) - (.version | length)]) as $prefix |
    ([$all[] | select(startswith($prefix)) | .[($prefix | length):] | stable] | max) as $highest |
    {package, version, tag: .name, commit,
     prerelease: (.version | split("+")[0] | contains("-")),
     latest: ([.version | stable][0] | . != null and . == $highest)}]')

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  printf 'tags=%s\n' "$tags" >> "$GITHUB_OUTPUT"
  # The newest tag of each package in the range, keyed by package path.
  printf 'packages=%s\n' "$(jq -c 'map({key: .package, value: .}) | from_entries' <<< "$tags")" >> "$GITHUB_OUTPUT"
fi

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    printf '## Release tags\n\n'
    jq -r 'if length == 0 then "No committed version changes." else .[] | "- `\(.name)` → `\(.commit)`" end' <<< "$plan"
  } >> "$GITHUB_STEP_SUMMARY"
fi
