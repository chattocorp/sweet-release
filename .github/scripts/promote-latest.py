#!/usr/bin/env python3
"""Reconcile GitHub Latest with the highest published stable CLI version."""

import json
import re
import subprocess


PREFIXES = ("pkg/cli/v", "pkg/sweet/v", "apps/sweet/v", "sweet@")
STABLE_VERSION = re.compile(
    r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?"
)


def latest_tag(releases):
    candidates = []
    for release in releases:
        if release["draft"] or release["prerelease"]:
            continue
        tag = release["tag_name"]
        for priority, prefix in enumerate(PREFIXES):
            if not tag.startswith(prefix):
                continue
            version = STABLE_VERSION.fullmatch(tag[len(prefix):])
            if version:
                # Build metadata has no version precedence. Prefer the current
                # package layout for equal versions, then a deterministic tag.
                candidates.append((tuple(map(int, version.groups())), priority, tag))
    return max(candidates)[2] if candidates else None


def main():
    # The workflow holds the shared promotion lock before this fresh read.
    result = subprocess.run(
        ["gh", "api", "--paginate", "--slurp",
         "repos/{owner}/{repo}/releases?per_page=100"],
        check=True, stdout=subprocess.PIPE, text=True,
    )
    pages = json.loads(result.stdout)
    tag = latest_tag(release for page in pages for release in page)
    if tag is None:
        print("No published stable CLI releases to promote.")
        return
    subprocess.run(["gh", "release", "edit", tag, "--latest"], check=True)
    print(f"Latest CLI release: {tag}")


if __name__ == "__main__":
    main()
