"""Build adapters produce a JSON list of files; no GitHub operations here."""

import glob
import hashlib
import json
import os
from pathlib import Path

def assets(patterns, goreleaser, version="", commit=""):
    paths = []
    if goreleaser:
        metadata = json.loads(Path("dist/metadata.json").read_text())
        if metadata["version"] != version or metadata["commit"] != commit:
            raise ValueError("GoReleaser output must match RELEASE_VERSION and RELEASE_COMMIT")
        records = json.loads(Path("dist/artifacts.json").read_text())
        for kind in ("Archive", "Checksum"):
            selected = [Path(item["path"]) for item in records if item["type"] == kind]
            if not selected:
                raise ValueError(f"GoReleaser produced no {kind} artifacts")
            paths.extend(selected)
        for item in records:
            if item["type"] == "Archive":
                algorithm, expected = item["extra"]["Checksum"].split(":", 1)
                with Path(item["path"]).open("rb") as archive:
                    actual = hashlib.file_digest(archive, algorithm).hexdigest()
                if actual != expected:
                    raise ValueError(f"Archive checksum mismatch: {item['path']}")
    for pattern in patterns.splitlines():
        if pattern.strip():
            matches = glob.glob(pattern.strip(), recursive=True)
            if not matches:
                raise ValueError(f"No assets match {pattern}")
            paths.extend(Path(path) for path in matches)
    result = sorted(set(path.resolve() for path in paths))
    if any(not path.is_file() for path in result):
        raise ValueError("Release assets must be files")
    if len({path.name for path in result}) != len(result):
        raise ValueError("Release asset filenames must be unique")
    return result


def main():
    paths = assets(os.environ.get("RELEASE_ASSETS", ""), os.environ.get("GORELEASER", ""),
                   os.environ.get("RELEASE_VERSION", ""), os.environ["RELEASE_COMMIT"])
    with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
        stream.write("assets=" + json.dumps([str(path) for path in paths]) + "\n")


if __name__ == "__main__":
    main()
