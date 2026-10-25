"""Resolve caller context, plan without writes, and apply a validated tag plan."""

import json
import os
from pathlib import Path
import re
import subprocess
import sys


def output(*args):
    return subprocess.check_output(args, text=True).strip()


def run(*args):
    subprocess.run(args, check=True)


def emit(key, value):
    with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
        stream.write(f"{key}={value}\n")


def summary(message):
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as stream:
            stream.write(message + "\n")


def context(mode, event_name, event, ref, sha, base="", head=""):
    if mode not in ("auto", "plan", "release"):
        raise ValueError("mode must be auto, plan, or release")
    if bool(base) != bool(head):
        raise ValueError("Supply both base and head, or neither")
    if mode == "auto":
        if event_name == "pull_request":
            mode = "plan"
        elif event_name == "push" and ref == "refs/heads/main":
            mode = "release"
        else:
            raise ValueError("Use an explicit mode for events other than PRs and main pushes")
    if not base:
        if event_name == "pull_request":
            if mode != "plan":
                raise ValueError("Executing from a PR requires an explicit commit range")
            # GitHub's test merge has the current base as its first parent.
            # Preview that result, even when the PR branch is behind main.
            base, head = event["pull_request"]["base"]["sha"], sha
        elif event_name == "push" and ref.startswith("refs/heads/"):
            base, head = event["before"], sha
        else:
            raise ValueError("This event requires explicit base and head SHAs")
    for value in (base, head):
        if not re.fullmatch(r"[0-9a-f]{40}|[0-9a-f]{64}", value):
            raise ValueError("base and head must be full commit SHAs")
    if not head.strip("0"):
        raise ValueError("Cannot release a deleted branch")
    return {"base": base, "head": head, "mode": mode}


def plan(sweet, base, head):
    # No tag creation, pushes, GitHub writes, or build commands in this stage.
    records = json.loads(output(sweet, "git", "tag", "--base", base, "--head", head, "--dry-run", "--json"))
    return [dict(package=item["package"], version=item["version"], tag=item["name"], commit=item["commit"])
            for item in records]


def tag(releases):
    if not releases:
        return
    run("git", "fetch", "origin", "--tags")
    missing, refs, seen = [], [], set()
    # Validate the entire plan before creating any local tags.
    for item in releases:
        name, commit = item["tag"], item["commit"]
        if name in seen or name.startswith("-"):
            raise ValueError("Release tag names must be unique and cannot start with a dash")
        seen.add(name)
        if not re.fullmatch(r"[0-9a-f]{40}|[0-9a-f]{64}", commit):
            raise ValueError("Release commit must be a full SHA")
        ref = f"refs/tags/{name}"
        run("git", "check-ref-format", ref)
        run("git", "merge-base", "--is-ancestor", commit, "HEAD")
        exists = subprocess.run(["git", "show-ref", "--verify", "--quiet", ref]).returncode
        if exists == 0:
            if output("git", "rev-parse", f"{ref}^{{commit}}") != commit:
                raise ValueError(f"Conflicting release tag: {name}")
        elif exists == 1:
            missing.append((name, commit))
        else:
            raise ValueError(f"Cannot inspect release tag: {name}")
        refs.append(f"{ref}:{ref}")
    for name, commit in missing:
        run("git", "-c", "tag.gpgSign=false", "tag", "-a", "-m", f"Release {name}", "--", name, commit)
    run("git", "push", "--atomic", "origin", *refs)
    summary(f"Tagged {len(refs)} release(s). Existing matching tags were preserved.")


def main():
    stage = sys.argv[1]
    if stage == "context":
        resolved = context(os.environ.get("MODE", "auto"), os.environ["GITHUB_EVENT_NAME"],
                           json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text()),
                           os.environ["GITHUB_REF"], os.environ["GITHUB_SHA"],
                           os.environ.get("BASE", ""), os.environ.get("HEAD", ""))
        for key, value in resolved.items():
            emit(key, value)
    elif stage == "plan":
        releases = plan(sys.argv[2], os.environ["BASE"], os.environ["HEAD"])
        emit("releases", json.dumps(releases, separators=(",", ":")))
        summary("## Release plan\n\nMode: " + os.environ["MODE"])
        summary("\n```json\n" + json.dumps(releases, indent=2) + "\n```")
        if not releases:
            summary("No committed version changes; nothing to release.")
        elif os.environ["MODE"] == "plan":
            summary("Preview only: no tags, builds, uploads, or publication.")
    elif stage == "tag":
        tag(json.loads(os.environ["RELEASES"]))
    else:
        raise ValueError("Unknown release stage")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        summary(f"Release {sys.argv[1]} failed. See this job's logs. After correcting the cause, rerun the original workflow to retain its commit range.")
        raise
