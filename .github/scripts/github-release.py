"""Internal GitHub release lifecycle for this repository's release policy."""

import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


def output(*args):
    return subprocess.check_output(args, text=True).strip()


def run(*args):
    subprocess.run(args, check=True)


def get_release(tag):
    """One targeted lookup; a null release is distinct from an inaccessible repo."""
    owner, name = os.environ["GH_REPO"].split("/")
    query = """query($owner:String!,$name:String!,$tag:String!) {
      repository(owner:$owner,name:$name) {
        release(tagName:$tag) { isDraft isPrerelease }
      }
    }"""
    response = json.loads(output("gh", "api", "graphql", "-f", f"query={query}",
                                 "-f", f"owner={owner}", "-f", f"name={name}", "-f", f"tag={tag}"))
    repository = (response.get("data") or {}).get("repository")
    if response.get("errors") or repository is None:
        raise ValueError("Cannot read releases from the target repository")
    return repository["release"]


def verify_remote_tag(tag, commit):
    ref = f"refs/tags/{tag}"
    refs = dict(line.split()[::-1] for line in output(
        "git", "ls-remote", "--exit-code", "origin", ref, ref + "^{}").splitlines())
    if refs.get(ref + "^{}", refs.get(ref)) != commit:
        raise ValueError("Remote release tag does not match the expected commit")


def manage(phase, tag, commit, package="", version="", latest="false"):
    if phase not in ("status", "draft", "publish") or latest not in ("true", "false"):
        raise ValueError("phase must be status, draft or publish; latest must be true or false")
    if not re.fullmatch(r"[0-9a-f]{40}|[0-9a-f]{64}", commit):
        raise ValueError("commit must be a full SHA")
    run("git", "check-ref-format", f"refs/tags/{tag}")
    if tag.startswith("-"):
        raise ValueError("Release tags cannot start with a dash")
    if output("git", "rev-parse", "HEAD") != commit or output("git", "rev-parse", "--verify", f"refs/tags/{tag}^{{commit}}") != commit:
        raise ValueError("Checkout and release tag must match the expected commit")

    verify_remote_tag(tag, commit)
    existing = get_release(tag)
    state = "missing" if existing is None else "draft" if existing["isDraft"] else "published"
    if phase == "status" or state == "published":
        return state

    if phase == "publish":
        if existing is None:
            raise ValueError("Prepare the draft before publishing")
        if existing["isPrerelease"] and latest == "true":
            raise ValueError("A prerelease cannot be Latest")
        verify_remote_tag(tag, commit)
        # --verify-tag on edit needs --tag; --target also prevents default-branch
        # fallback if the tag disappears between the remote check and the request.
        run("gh", "release", "edit", tag, "--tag", tag, "--verify-tag", "--target", commit,
            "--draft=false", f"--latest={latest}")
        return "published"

    if not package or not re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?", version):
        raise ValueError("Draft preparation requires a package and valid version")
    prerelease = "-" in version.split("+", 1)[0]
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
            command.extend(["--verify-tag", "--target", commit, "--draft", "--latest=false"])
        verify_remote_tag(tag, commit)
        run(*command)
    return "draft"


def main():
    # The asset builder uses this same read-only path before building and uploading.
    if len(sys.argv) == 4 and sys.argv[1] == "status":
        print(manage("status", sys.argv[2], sys.argv[3]))
        return
    state = manage(os.environ["RELEASE_PHASE"], os.environ["RELEASE_TAG"],
                   os.environ["RELEASE_COMMIT"], os.environ.get("RELEASE_PACKAGE", ""),
                   os.environ.get("RELEASE_VERSION", ""), os.environ.get("RELEASE_LATEST", "false"))
    print(state)


if __name__ == "__main__":
    main()
