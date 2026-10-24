"""GitHub release lifecycle; build systems only upload assets to the draft."""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile


def output(*args):
    return subprocess.check_output(args, text=True).strip()


def run(*args):
    subprocess.run(args, check=True)


def manage(phase, package, version, tag, commit, latest="false"):
    if phase not in ("draft", "publish") or latest not in ("true", "false"):
        raise ValueError("phase must be draft or publish; latest must be true or false")
    if not re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?", version):
        raise ValueError("Invalid release version")
    prerelease = "-" in version.split("+", 1)[0]
    if prerelease and latest == "true":
        raise ValueError("A prerelease cannot be Latest")
    if not re.fullmatch(r"[0-9a-f]{40}|[0-9a-f]{64}", commit):
        raise ValueError("commit must be a full SHA")
    run("git", "check-ref-format", f"refs/tags/{tag}")
    if output("git", "rev-parse", "HEAD") != commit or output("git", "rev-parse", "--verify", f"refs/tags/{tag}^{{commit}}") != commit:
        raise ValueError("Checkout and release tag must match the expected commit")

    # Fail on API/auth errors. Never treat a failed read as a missing release.
    pages = json.loads(output("gh", "api", "--paginate", "--slurp", "repos/{owner}/{repo}/releases?per_page=100"))
    existing = next((release for page in pages for release in page if release["tag_name"] == tag), None)
    if existing and not existing["draft"]:
        print(f"{tag} is already published; leaving it unchanged.")
        return True

    if phase == "publish":
        if existing is None:
            raise ValueError("Prepare the draft before publishing")
        run("gh", "release", "edit", tag, "--draft=false", f"--latest={latest}")
        return True

    inventory = json.loads(output("sweet", "status", "--json"))
    metadata = next((item for item in inventory if item["path"] == package), None)
    if metadata is None or metadata["version"] != version:
        raise ValueError("Package version at the release commit does not match")
    title = f"{metadata.get('name') or package} {version}"
    # Preserve the changelog body, including its trailing newline.
    notes = subprocess.check_output(["sweet", "changelog", "--version", version, "--", package], text=True)
    with tempfile.TemporaryDirectory(prefix="sweet-release-") as temporary:
        notes_path = Path(temporary) / "notes.md"
        notes_path.write_text(notes)
        command = ["gh", "release", "edit" if existing else "create", tag,
                   "--title", title, "--notes-file", str(notes_path),
                   f"--prerelease={str(prerelease).lower()}"]
        if not existing:
            command.extend(["--verify-tag", "--draft", "--latest=false"])
        run(*command)
    return False


def main():
    published = manage(*(os.environ[f"RELEASE_{key}"] for key in
                         ("PHASE", "PACKAGE", "VERSION", "TAG", "COMMIT", "LATEST")))
    with open(os.environ["GITHUB_OUTPUT"], "a") as target:
        target.write(f"published={str(published).lower()}\n")


if __name__ == "__main__":
    main()
