#!/usr/bin/env bash
set -euo pipefail

tag=$1
config=$2
[[ "$tag" == pkg/cli/v* ]]
version=${tag#pkg/cli/v}
[[ "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]
[[ "$(git rev-parse HEAD)" == "$(git rev-parse --verify "refs/tags/$tag^{commit}")" ]]

# Listing fails on API/auth errors rather than treating them as a missing release.
releases=$(gh api --paginate 'repos/{owner}/{repo}/releases?per_page=100' --jq '.[] | {tag_name, draft}')
state=$(jq -sr --arg tag "$tag" 'map(select(.tag_name == $tag)) | if length == 0 then "missing" elif .[0].draft then "draft" else "published" end' <<< "$releases")
if [[ "$state" == published ]]; then
  echo "$tag is already published."
  exit 0
fi

export SWEET_RELEASE_VERSION="$version"
export GORELEASER_CURRENT_TAG="v$version"
goreleaser release --snapshot --clean --config "$config"

# Verify the actual binary, including the linker-supplied version.
[[ "$(dist/sweet_linux_amd64_v1/sweet --version)" == "sweet $version" ]]
(cd dist && sha256sum --check checksums.txt)
notes=$(mktemp)
trap 'rm -f "$notes"' EXIT
awk -v heading="## $version" '
  /^## / { if (found) exit; if ($0 == heading) { found=1; next } }
  found { print }
  END { if (!found) exit 1 }
' pkg/cli/CHANGELOG.md > "$notes"

prerelease=false
[[ "${version%%+*}" != *-* ]] || prerelease=true
if [[ "$state" == missing ]]; then
  gh release create "$tag" --verify-tag --draft --title "sweet $version" --notes-file "$notes" --prerelease="$prerelease"
fi
gh release upload "$tag" dist/*.tar.gz dist/*.zip dist/checksums.txt --clobber
gh release edit "$tag" --title "sweet $version" --notes-file "$notes" --prerelease="$prerelease" --draft=false --latest=false
