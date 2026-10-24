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

release_tool=${SWEET_RELEASE_TOOL:?Set SWEET_RELEASE_TOOL to the shared release helper}
commit=$(git rev-parse HEAD)
state=$(python3 "$release_tool" status "$tag" "$commit")
if [[ "$state" == published ]]; then
  echo "$tag is already published."
  exit 0
fi
if [[ "$state" != draft ]]; then
  echo 'Prepare a draft with release-package.sh before uploading.' >&2
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
actual=$(dist/sweet_linux_amd64_v1/sweet --version)
if [[ "$actual" != "sweet $version" ]]; then
  echo "Unexpected binary version: $actual" >&2
  exit 1
fi
(cd dist && sha256sum --check checksums.txt)
# Recheck after the build: neither a moved tag nor a published draft is writable.
state=$(python3 "$release_tool" status "$tag" "$commit")
if [[ "$state" != draft ]]; then
  echo 'Release is no longer a draft; stop before uploading assets.' >&2
  exit 1
fi
gh release upload "$tag" dist/*.tar.gz dist/*.zip dist/checksums.txt --clobber
