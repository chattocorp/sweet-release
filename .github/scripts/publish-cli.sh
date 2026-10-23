#!/usr/bin/env bash
set -euo pipefail

tag=$1
config=$2
metadata=${SWEET_METADATA:?Set SWEET_METADATA to the current sweet binary}
case "$tag" in
  apps/sweet/v*) package=apps/sweet ;;
  pkg/sweet/v*) package=pkg/sweet ;;
  pkg/cli/v*) package=pkg/cli ;;
  *) echo "Unsupported release tag: $tag" >&2; exit 1 ;;
esac
version=${tag#"$package/v"}
[[ "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]
[[ "$(git rev-parse HEAD)" == "$(git rev-parse --verify "refs/tags/$tag^{commit}")" ]]
name=$("$metadata" status --json | jq -er --arg package "$package" '.[] | select(.path == $package) | if (.name // "") == "" then .path else .name end')

# Listing fails on API/auth errors rather than treating them as a missing release.
releases=$(gh api --paginate 'repos/{owner}/{repo}/releases?per_page=100' --jq '.[] | {tag_name, draft}')
state=$(jq -sr --arg tag "$tag" 'map(select(.tag_name == $tag)) | if length == 0 then "missing" elif .[0].draft then "draft" else "published" end' <<< "$releases")
if [[ "$state" == published ]]; then
  echo "$tag is already published."
  exit 0
fi

export SWEET_RELEASE_VERSION="$version"
export GORELEASER_CURRENT_TAG="v$version"
# GoReleaser does not expand templates in builds.dir. Adapt the retained
# configuration outside the checkout when publishing the historical CLI layout.
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT
awk -v directory="$package" '{ if ($0 == "    dir: apps/sweet") $0 = "    dir: " directory; print }' "$config" > "$temporary/goreleaser.yml"
goreleaser release --snapshot --clean --config "$temporary/goreleaser.yml"

# Verify the actual binary, including the linker-supplied version.
[[ "$(dist/sweet_linux_amd64_v1/sweet --version)" == "sweet $version" ]]
(cd dist && sha256sum --check checksums.txt)
notes="$temporary/notes.md"
"$metadata" changelog show "$package" --version "$version" > "$notes"

prerelease=false
[[ "${version%%+*}" != *-* ]] || prerelease=true
if [[ "$state" == missing ]]; then
  gh release create "$tag" --verify-tag --draft --title "$name $version" --notes-file "$notes" --prerelease="$prerelease"
fi
gh release upload "$tag" dist/*.tar.gz dist/*.zip dist/checksums.txt --clobber
gh release edit "$tag" --title "$name $version" --notes-file "$notes" --prerelease="$prerelease" --draft=false --latest=false
