#!/usr/bin/env bash
set -euo pipefail

# Inspect all pages before mutating anything. Only bot-owned release PRs from
# this repository are eligible; human PRs and fork branches are left alone.
gh api --paginate --slurp "repos/$GITHUB_REPOSITORY/pulls?state=all&per_page=100" > "$RUNNER_TEMP/release-pulls.json"
gh api --paginate --slurp "repos/$GITHUB_REPOSITORY/git/matching-refs/heads/sweet/release" > "$RUNNER_TEMP/release-refs.json"

jq --arg repo "$GITHUB_REPOSITORY" --argjson desired "$RELEASE_PRS" '
  flatten | map(select(.head.repo.full_name == $repo)) | group_by(.head.ref)[] |
  select(.[0].head.ref | test("^sweet/release$|^sweet/release-package-[a-z0-9-]+-[0-9a-f]{16}$")) |
  select(.[0].head.ref as $branch | $desired | all(.branch != $branch)) |
  select(any(.user.login == "github-actions[bot]" and .base.ref == "main")) |
  select(all(.state != "open" or (.user.login == "github-actions[bot]" and .base.ref == "main"))) |
  {branch: .[0].head.ref, numbers: map(select(.state == "open") | .number)}
' "$RUNNER_TEMP/release-pulls.json" | jq -sc '.' > "$RUNNER_TEMP/obsolete-release-prs.json"

while IFS= read -r number; do
  gh api --method PATCH "repos/$GITHUB_REPOSITORY/pulls/$number" -f state=closed > /dev/null
done < <(jq -r '.[].numbers[]' "$RUNNER_TEMP/obsolete-release-prs.json")

while IFS= read -r branch; do
  if jq -e --arg ref "refs/heads/$branch" 'flatten | any(.ref == $ref)' "$RUNNER_TEMP/release-refs.json" > /dev/null; then
    gh api --method DELETE "repos/$GITHUB_REPOSITORY/git/refs/heads/$branch"
  fi
done < <(jq -r '.[].branch' "$RUNNER_TEMP/obsolete-release-prs.json")
