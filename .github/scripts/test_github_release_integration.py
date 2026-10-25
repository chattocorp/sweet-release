"""Real Sweet + Git repositories; only the GitHub network boundary is replaced."""

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


SCRIPTS = Path(__file__).resolve().parent
ROOT = SCRIPTS.parents[1]


class ReleaseIntegrationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if os.name == "nt" or not shutil.which("go"):
            raise unittest.SkipTest("integration fixture requires Go and a Unix runner")
        cls.build = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.build.cleanup)
        subprocess.run(["go", "build", "-o", str(Path(cls.build.name) / "sweet"),
                        "./apps/sweet/cmd/sweet"], cwd=ROOT, check=True)

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.repo = self.root / "consumer"
        self.repo.mkdir()
        self.remote = self.root / "remote.git"
        self.bin = self.root / "bin"
        self.bin.mkdir()
        shutil.copyfile(SCRIPTS / "fixtures/gh.py", self.bin / "gh")
        (self.bin / "gh").chmod(0o755)
        self.env = dict(os.environ, PATH=os.pathsep.join([str(self.bin), self.build.name, os.environ["PATH"]]),
                        GH_REPO="example/consumer", FAKE_GH_STATE=str(self.root / "release.json"),
                        FAKE_GH_LOG=str(self.root / "gh.log"))
        subprocess.run(["git", "init", "--bare", str(self.remote)], check=True, capture_output=True)
        self.git("init", "--initial-branch=main")
        self.git("config", "user.name", "Test")
        self.git("config", "user.email", "test@example.com")
        self.git("config", "commit.gpgsign", "false")
        self.git("config", "tag.gpgsign", "false")
        self.git("remote", "add", "origin", str(self.remote))
        (self.repo / ".sweet.yml").write_text("name: Example API\nversion: 0.1.0\n")
        (self.repo / "CHANGELOG.md").write_text("# Changelog\n\n## 0.1.0\n\n- Original release.\n")
        self.git("add", ".")
        self.git("commit", "-m", "First release")
        self.commit = self.git("rev-parse", "HEAD")
        self.tag = "example@0.1.0"
        self.git("tag", "-a", self.tag, "-m", "Release")
        (self.repo / ".sweet.yml").write_text("name: Example API\nversion: 0.2.0\n")
        (self.repo / "CHANGELOG.md").write_text("# Changelog\n\n## 0.2.0\n\n- Later release.\n\n## 0.1.0\n\n- Original release.\n")
        self.git("add", ".")
        self.git("commit", "-m", "Later release")
        self.later = self.git("rev-parse", "HEAD")
        self.git("push", "origin", "main", "--tags")
        self.git("checkout", "--detach", self.commit)

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.repo), *args], stderr=subprocess.PIPE, text=True).strip()

    def action(self, phase, success=True, **extra):
        env = dict(self.env, RELEASE_PHASE=phase, RELEASE_TAG=self.tag, RELEASE_COMMIT=self.commit, **extra)
        if phase == "draft":
            env.update(RELEASE_PACKAGE=".", RELEASE_VERSION="0.1.0")
        result = subprocess.run([sys.executable, str(SCRIPTS / "github-release.py")], cwd=self.repo,
                                env=env, text=True, capture_output=True)
        self.assertEqual(result.returncode == 0, success, result.stderr)
        return result

    def record(self):
        return json.loads((self.root / "release.json").read_text())

    def mutations(self):
        path = self.root / "gh.log"
        return [args for line in path.read_text().splitlines()
                if (args := json.loads(line))[0] == "release"] if path.exists() else []

    def test_fresh_draft_upload_publish_and_retry_at_historical_commit(self):
        self.action("status")
        self.assertFalse(self.mutations())
        self.action("draft")
        record = self.record()
        self.assertEqual(record["title"], "Example API 0.1.0")
        self.assertEqual(record["target"], self.commit)
        self.assertIn("Original release", record["notes"])
        self.assertNotIn("Later release", record["notes"])
        self.assertTrue(record["isDraft"])

        asset = self.root / "artifact.tar.gz"
        asset.write_bytes(b"example artifact")
        failed = subprocess.run(["gh", "release", "upload", self.tag, str(asset)],
                                env=dict(self.env, FAKE_GH_FAILURE="upload"), capture_output=True)
        self.assertNotEqual(failed.returncode, 0)
        self.assertTrue(self.record()["isDraft"])
        subprocess.run(["gh", "release", "upload", self.tag, str(asset), "--clobber"], env=self.env, check=True)
        # Status/publish do not depend on Sweet or redundant package/version inputs.
        (self.bin / "sweet").write_text("#!/bin/sh\nexit 99\n")
        (self.bin / "sweet").chmod(0o755)
        self.action("status")
        self.action("publish")
        published = self.record()
        self.assertFalse(published["isDraft"])
        self.assertEqual(published["assets"], [asset.name])
        count = len(self.mutations())
        self.action("draft")
        self.action("publish")
        self.assertEqual(self.record(), published)
        self.assertEqual(len(self.mutations()), count)

    def test_real_remote_moved_or_deleted_prevents_publication(self):
        self.action("draft")
        count = len(self.mutations())
        subprocess.run(["git", "--git-dir", str(self.remote), "update-ref", f"refs/tags/{self.tag}", self.later], check=True)
        self.action("publish", success=False)
        subprocess.run(["git", "--git-dir", str(self.remote), "update-ref", "-d", f"refs/tags/{self.tag}"], check=True)
        self.action("publish", success=False)
        self.assertTrue(self.record()["isDraft"])
        self.assertEqual(len(self.mutations()), count)

    def test_lightweight_remote_tag(self):
        subprocess.run(["git", "--git-dir", str(self.remote), "update-ref", f"refs/tags/{self.tag}", self.commit], check=True)
        self.action("draft")
        self.assertTrue(self.record()["isDraft"])

    def test_api_failure_never_creates_a_release(self):
        self.action("draft", success=False, FAKE_GH_FAILURE="api")
        self.assertFalse(self.mutations())
        self.assertFalse((self.root / "release.json").exists())

    def upload(self, success=True, **extra):
        # Run the builder adapter and pass only its JSON asset list to the publisher.
        result_path = self.root / "build-output"
        result_path.unlink(missing_ok=True)
        built = subprocess.run([sys.executable, str(SCRIPTS / "build-assets.py")],
                               cwd=self.repo, env=dict(self.env, GITHUB_OUTPUT=str(result_path),
                               RELEASE_COMMIT=self.commit, RELEASE_VERSION="0.1.0", **extra),
                               capture_output=True, text=True)
        if built.returncode:
            self.assertFalse(success, built.stderr)
            return built
        manifest = result_path.read_text().removeprefix("assets=").strip()
        result = subprocess.run([sys.executable, str(SCRIPTS / "upload-assets.py")],
                                cwd=self.repo, env=dict(self.env, RELEASE_TAG=self.tag,
                                RELEASE_COMMIT=self.commit, BUILT_ASSETS=manifest,
                                FAKE_GH_FAILURE=extra.get("FAKE_GH_FAILURE", "")), capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, success, result.stderr)
        return result

    def test_shared_upload_failure_retry_and_publication(self):
        self.action("draft")
        dist = self.repo / "dist"
        dist.mkdir()
        (dist / "archive.tar.gz").write_bytes(b"archive")
        (dist / "checksums.txt").write_text("fixture")
        (dist / "metadata.json").write_text(json.dumps({"version": "0.1.0", "commit": self.commit}))
        (dist / "artifacts.json").write_text(json.dumps([
            {"type": "Archive", "path": "dist/archive.tar.gz",
             "extra": {"Checksum": "sha256:" + hashlib.sha256(b"archive").hexdigest()}},
            {"type": "Checksum", "path": "dist/checksums.txt"},
            {"type": "Binary", "path": "dist/unpacked-binary"}]))
        self.upload(success=False, GORELEASER="v2.18.2", FAKE_GH_FAILURE="upload")
        self.assertTrue(self.record()["isDraft"])
        (dist / "archive.tar.gz").write_bytes(b"corrupt")
        self.upload(success=False, GORELEASER="v2.18.2")
        (dist / "archive.tar.gz").write_bytes(b"archive")
        (dist / "metadata.json").write_text(json.dumps({"version": "0.0.0-SNAPSHOT", "commit": self.commit}))
        self.upload(success=False, GORELEASER="v2.18.2")
        (dist / "metadata.json").write_text(json.dumps({"version": "0.1.0", "commit": self.commit}))
        self.upload(GORELEASER="v2.18.2")
        self.assertEqual(set(self.record()["assets"]), {"archive.tar.gz", "checksums.txt"})
        self.action("publish")
        self.assertFalse(self.record()["isDraft"])
        count = len(self.mutations())
        self.upload(success=False, GORELEASER="v2.18.2")
        self.assertEqual(len(self.mutations()), count)

    def test_custom_assets_and_remote_tag_recheck(self):
        self.action("draft")
        (self.repo / "asset.zip").write_bytes(b"archive")
        self.upload(RELEASE_ASSETS="*.zip")
        count = len(self.mutations())
        subprocess.run(["git", "--git-dir", str(self.remote), "update-ref",
                        f"refs/tags/{self.tag}", self.later], check=True)
        self.upload(success=False, RELEASE_ASSETS="*.zip")
        self.assertTrue(self.record()["isDraft"])
        self.assertEqual(len(self.mutations()), count)

    def test_missing_assets_fail_before_upload(self):
        self.action("draft")
        count = len(self.mutations())
        self.upload(success=False, RELEASE_ASSETS="missing/*")
        self.upload(success=False, GORELEASER="v2.18.2")
        self.assertEqual(len(self.mutations()), count)
        self.assertTrue(self.record()["isDraft"])

    def test_notes_only_publication(self):
        self.action("draft")
        self.upload()
        self.action("publish")
        self.assertFalse(self.record()["isDraft"])
        self.assertFalse(self.record()["assets"])

    def test_tag_stage_applies_frozen_plan_without_replanning(self):
        self.git("checkout", "--detach", self.later)
        planned = [{"package": ".", "version": "0.1.0", "tag": "frozen@0.1.0", "commit": self.commit}]
        subprocess.run([sys.executable, str(SCRIPTS / "release-plan.py"), "tag"],
                       cwd=self.repo, env=dict(self.env, RELEASES=json.dumps(planned)),
                       check=True, capture_output=True)
        target = subprocess.check_output(["git", "--git-dir", str(self.remote), "rev-parse",
                                          "refs/tags/frozen@0.1.0^{commit}"], text=True).strip()
        self.assertEqual(target, self.commit)
        self.assertNotIn("v0.2.0", self.git("tag", "--list"))

    def test_pr_merge_preview_when_source_branch_is_behind_main(self):
        self.git("checkout", "-b", "feature", self.later)
        (self.repo / ".sweet.yml").write_text("name: Example API\nversion: 0.3.0\n")
        self.git("add", ".")
        self.git("commit", "-m", "Release feature")
        self.git("checkout", "main")
        (self.repo / "README.md").write_text("Concurrent main change\n")
        self.git("add", ".")
        self.git("commit", "-m", "Advance main")
        base = self.git("rev-parse", "HEAD")
        self.git("merge", "--no-ff", "feature", "-m", "Test merge")
        merge = self.git("rev-parse", "HEAD")
        output = self.root / "plan-output"
        subprocess.run([sys.executable, str(SCRIPTS / "release-plan.py"), "plan", str(Path(self.build.name) / "sweet")],
                       cwd=self.repo, env=dict(self.env, BASE=base, HEAD=merge, MODE="plan", GITHUB_OUTPUT=str(output)),
                       check=True, capture_output=True)
        planned = json.loads(output.read_text().removeprefix("releases="))
        self.assertEqual([(item["version"], item["commit"]) for item in planned], [("0.3.0", merge)])
        self.assertNotIn("v0.3.0", self.git("tag", "--list"))

    def test_duplicate_asset_names_fail_before_upload(self):
        self.action("draft")
        for folder in ("one", "two"):
            directory = self.repo / folder
            directory.mkdir()
            (directory / "artifact.zip").write_bytes(b"archive")
        count = len(self.mutations())
        self.upload(success=False, RELEASE_ASSETS="*/*.zip")
        self.assertEqual(len(self.mutations()), count)


if __name__ == "__main__":
    unittest.main()
