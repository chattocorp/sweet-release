#!/usr/bin/env python3
"""Stateful GitHub CLI transport for integration tests; never uses the network."""

import json
import os
from pathlib import Path
import sys

args = sys.argv[1:]
state_path = Path(os.environ["FAKE_GH_STATE"])
state = json.loads(state_path.read_text()) if state_path.exists() else None
with open(os.environ["FAKE_GH_LOG"], "a") as log:
    log.write(json.dumps(args) + "\n")


def option(name):
    return args[args.index(name) + 1]


if args[:2] == ["api", "graphql"]:
    if os.environ.get("FAKE_GH_FAILURE") == "api":
        sys.exit("GitHub API access denied")
    print(json.dumps({"data": {"repository": {"release": None if state is None else {
        "isDraft": state["isDraft"], "isPrerelease": state["isPrerelease"]}}}}))
    sys.exit(0)

if args[:2] == ["release", "create"]:
    assert state is None, "duplicate release creation"
    assert "--verify-tag" in args and "--draft" in args
    state = {"tag": args[2], "isDraft": True, "assets": [], "target": option("--target")}
elif args[:2] == ["release", "edit"]:
    assert state and state["isDraft"], "cannot edit missing/published release"
    if "--draft=false" in args:
        assert "--verify-tag" in args and option("--tag") == state["tag"]
        state["isDraft"] = False
elif args[:2] == ["release", "upload"]:
    assert state and state["isDraft"], "upload requires a draft"
    if os.environ.get("FAKE_GH_FAILURE") == "upload":
        sys.exit("Asset upload failed")
    for path in args[3:]:
        if path != "--clobber":
            asset = Path(path)
            assert asset.is_file()
            if asset.name not in state["assets"]:
                state["assets"].append(asset.name)
else:
    sys.exit(f"Unexpected gh invocation: {args}")

if "--notes-file" in args:
    state.update(title=option("--title"), notes=Path(option("--notes-file")).read_text(),
                 isPrerelease="--prerelease=true" in args)
state_path.write_text(json.dumps(state))
