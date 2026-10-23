"""Repository-owned PR policy; Sweet supplies only status, bump, and changelogs."""

import hashlib
import json
import re
import subprocess
import sys
from pathlib import Path


def branch_for(path):
    slug = re.sub("[^a-z0-9]+", "-", path.lower()).strip("-") or "root"
    slug = slug[:48].rstrip("-")
    digest = hashlib.sha256(path.encode()).hexdigest()[:16]
    return f"sweet/release-package-{slug}-{digest}"


def matrix(packages, strategy):
    if strategy not in ("per-package", "combined"):
        raise ValueError("RELEASE_PR_STRATEGY must be per-package or combined")
    if not packages:
        return []
    if strategy == "combined":
        return [{"branch": "sweet/release", "title": "chore: prepare releases",
                 "packages": [p["path"] for p in packages]}]
    jobs = [{"branch": branch_for(p["path"]),
             "title": f"chore: release {p.get('name') or p['path']}",
             "packages": [p["path"]]} for p in packages]
    if len(jobs) > 256:
        raise ValueError("GitHub Actions supports at most 256 matrix entries")
    return jobs


def output(*args):
    return subprocess.check_output(args, text=True)


def prepare(sweet, job, destination):
    # Results and descriptions must stay outside the checkout so they cannot
    # cause an otherwise empty bump to produce a PR.
    root = Path(output("git", "rev-parse", "--show-toplevel").strip()).resolve()
    destination = Path(destination).resolve()
    if destination == root or root in destination.parents:
        raise ValueError("release results must be outside the checkout")
    if output("git", "status", "--porcelain", "--untracked-files=all"):
        raise ValueError("release preparation requires a clean checkout")
    before = {p["path"]: p for p in json.loads(output(sweet, "status", "--json"))}
    subprocess.run([sweet, "bump", "--", *job["packages"]], check=True, stdout=sys.stderr)
    changed = bool(output("git", "status", "--porcelain", "--untracked-files=all"))
    title = job["title"]
    body = "This PR prepares the following releases:\n"
    if changed:
        after = {p["path"]: p for p in json.loads(output(sweet, "status", "--json"))}
        for path in job["packages"]:
            package = after[path]
            if before[path]["version"] == package["version"]:
                continue
            name = package.get("name") or path
            version = package["version"]
            if job["branch"] != "sweet/release":
                title = f"chore: release {name} {version}"
            notes = output(sweet, "changelog", "--version", version, "--", path)
            body += f"\n## {name} {version}\n\n{notes}"
    destination.mkdir(parents=True, exist_ok=True)
    (destination / "body.md").write_text(body)
    (destination / "title.txt").write_text(title)
    # Every successful job writes a result, including a no-op. Cleanup only runs
    # after every job succeeds and consumes these actual outcomes.
    (destination / "desired.json").write_text(json.dumps([{"branch": job["branch"]}] if changed else []))
    return changed


if __name__ == "__main__":
    if sys.argv[1] == "matrix":
        print(json.dumps(matrix(json.load(sys.stdin), sys.argv[2]), separators=(",", ":")))
    elif sys.argv[1] == "prepare":
        changed = prepare(sys.argv[2], json.loads(sys.argv[3]), sys.argv[4])
        print("true" if changed else "false")
    else:
        raise ValueError("expected matrix or prepare")
