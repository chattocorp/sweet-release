"""Explain the outcome of one release matrix job."""

import html
import os
from urllib.parse import quote


def message(env):
    tag = html.escape(env["RELEASE_TAG"])
    link = f"https://github.com/{env['GH_REPO']}/releases/tag/{quote(env['RELEASE_TAG'], safe='')}"
    title = f"## Release {tag}\n\n"
    if env.get("DRAFT_STATE") == "published":
        return title + f"[Already published]({link}); skipped building, uploading, and publication."
    for stage in ("prepare", "build", "upload", "publish"):
        if env.get(stage.upper() + "_OUTCOME") in ("failure", "cancelled"):
            return title + f"**{stage.capitalize()} did not complete.** See that step's logs. Correct the cause and rerun the original workflow to retain its commit range. Published releases are preserved on retry."
    if env.get("PUBLISH_OUTCOME") == "success":
        return title + f"[Published release]({link})."
    if env.get("BUILD_OUTCOME") != "success":
        return title + "Setup did not complete. See this job's logs and rerun the original workflow after correcting the cause."
    return title + "Release command completed. GitHub publication was not enabled."


if __name__ == "__main__":
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as stream:
        stream.write(message(os.environ) + "\n")
