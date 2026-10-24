#!/usr/bin/env bash
set -euo pipefail

tag=$1
config=$2
case "$tag" in
  sweet@*) package=apps/sweet ;;
  apps/sweet/v*) package=apps/sweet ;;
  pkg/sweet/v*) package=pkg/sweet ;;
  pkg/cli/v*) package=pkg/cli ;;
  *) echo "Unsupported release tag: $tag" >&2; exit 1 ;;
esac
version=${tag#"$package/v"}
if [[ "$tag" == sweet@* ]]; then version=${tag#sweet@}; fi
[[ "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]
[[ "$(git rev-parse HEAD)" == "$(git rev-parse --verify "refs/tags/$tag^{commit}")" ]]

# Listing fails on API/auth errors rather than treating them as a missing release.
releases=$(gh api --paginate 'repos/{owner}/{repo}/releases?per_page=100' --jq '.[] | {tag_name, draft}')
state=$(jq -sr --arg tag "$tag" 'map(select(.tag_name == $tag)) | if length == 0 then "missing" elif .[0].draft then "draft" else "published" end' <<< "$releases")
if [[ "$state" == published ]]; then
  echo "$tag is already published."
  exit 0
fi
if [[ "$state" != draft ]]; then
  echo 'Prepare a draft with the Sweet GitHub Release action before uploading.' >&2
  exit 1
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
gh release upload "$tag" dist/*.tar.gz dist/*.zip dist/checksums.txt --clobber
