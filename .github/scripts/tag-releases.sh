#!/usr/bin/env bash
set -euo pipefail

sweet=$1
base=$2
head=$3
push_tags=${PUSH_TAGS:-true}
case "$push_tags" in
  true|false) ;;
  *) echo 'PUSH_TAGS must be true or false.' >&2; exit 1 ;;
esac

# A fresh checkout plus all remote tags makes reruns safe. Never force-fetch tags.
git fetch origin --tags
args=(git tag --base "$base" --head "$head" --json)
if [[ "$push_tags" == false ]]; then args+=(--dry-run); fi
plan=$("$sweet" "${args[@]}")

refs=()
while IFS=$'\t' read -r name _commit _exists; do
  # Include matching local tags too, so retrying a rejected push can finish.
  refs+=("refs/tags/$name:refs/tags/$name")
done < <(jq -r '.[] | [.name, .commit, .exists] | @tsv' <<< "$plan")

if [[ "$push_tags" == false ]]; then
  echo 'Tag creation and pushing disabled.'
elif (( ${#refs[@]} )); then
  # Push only this plan's tags, all together. No force pushes or partial releases.
  git push --atomic origin "${refs[@]}"
else
  echo 'No new release tags.'
fi

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  printf 'releases=%s\n' "$(jq -c '[.[] | {package, version, tag: .name, commit}]' <<< "$plan")" >> "$GITHUB_OUTPUT"
fi

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    printf '## Planned releases\n\n'
    jq -r '.[] | "- `\(.name)` → `\(.commit)`"' <<< "$plan"
  } >> "$GITHUB_STEP_SUMMARY"
fi
