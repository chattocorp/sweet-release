"""Publish a builder's asset list to a verified GitHub draft."""

import importlib.util
import json
import os
from pathlib import Path

spec = importlib.util.spec_from_file_location("release", Path(__file__).with_name("github-release.py"))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


def upload(manifest, tag, commit):
    if not isinstance(manifest, list) or any(not isinstance(path, str) or not path for path in manifest):
        raise ValueError("Builder assets must be a JSON array of file paths")
    paths = [Path(path).resolve() for path in manifest]
    if any(not path.is_file() for path in paths):
        raise ValueError("Every release asset must be an existing file")
    if len({path.name for path in paths}) != len(paths):
        raise ValueError("Release asset filenames must be unique")
    if release.manage("status", tag, commit) != "draft":
        raise ValueError("Release is no longer a draft; stop before uploading")
    if paths:
        release.run("gh", "release", "upload", tag, *(str(path) for path in paths), "--clobber")


if __name__ == "__main__":
    upload(json.loads(os.environ["BUILT_ASSETS"]), os.environ["RELEASE_TAG"], os.environ["RELEASE_COMMIT"])
