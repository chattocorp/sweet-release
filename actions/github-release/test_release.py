import importlib.util
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location("release", Path(__file__).with_name("release.py"))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
COMMIT = "a" * 40


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.records = []
        self.mutations = []
        self.head = self.tag_commit = COMMIT
        self.version = "1.2.3"
        self.fail_read = False
        self.fail_write = False
        self.output_patch = patch.object(release, "output", side_effect=self.output)
        self.run_patch = patch.object(release, "run", side_effect=self.run_command)
        self.notes_patch = patch.object(release.subprocess, "check_output", return_value="Release notes.\n")
        for replacement in (self.output_patch, self.run_patch, self.notes_patch):
            replacement.start()
            self.addCleanup(replacement.stop)

    def output(self, *args):
        if args[:2] == ("git", "rev-parse"):
            return self.head if args[-1] == "HEAD" else self.tag_commit
        if args[:2] == ("gh", "api"):
            if self.fail_read:
                raise subprocess.CalledProcessError(1, args)
            # Include an unrelated page to exercise pagination.
            return json.dumps([[{"tag_name": "other@1.0.0", "draft": False}], self.records])
        if args == ("sweet", "status", "--json"):
            return json.dumps([{"path": "pkg/api", "name": "API", "version": self.version}])
        self.fail(f"Unexpected command: {args}")

    def run_command(self, *args):
        if args[:2] == ("git", "check-ref-format"):
            return
        self.assertEqual(args[:2], ("gh", "release"))
        if self.fail_write:
            raise subprocess.CalledProcessError(1, args)
        self.mutations.append(args)
        if "--notes-file" in args:
            self.assertEqual(Path(args[args.index("--notes-file") + 1]).read_text(), "Release notes.\n")
            self.assertEqual(args[args.index("--title") + 1], f"API {self.version}")
        if args[2] == "create":
            self.assertIn("--verify-tag", args)
            self.assertIn("--draft", args)
            self.records.append({"tag_name": args[3], "draft": True})
        elif "--draft=false" in args:
            self.records[0]["draft"] = False

    def manage(self, phase, **kwargs):
        return release.manage(phase, "pkg/api", kwargs.pop("version", self.version),
                              "api@1.2.3", COMMIT, **kwargs)

    def test_draft_retry_publish_and_published_retry(self):
        self.assertFalse(self.manage("draft"))
        self.assertTrue(self.records[0]["draft"])
        self.assertFalse(self.manage("draft"))
        self.assertEqual([call[2] for call in self.mutations], ["create", "edit"])
        self.assertTrue(self.manage("publish"))
        self.assertIn("--latest=false", self.mutations[-1])
        count = len(self.mutations)
        self.assertTrue(self.manage("draft"))
        self.assertTrue(self.manage("publish"))
        self.assertEqual(len(self.mutations), count)

    def test_failed_publish_leaves_draft_for_retry(self):
        self.manage("draft")
        self.fail_write = True
        with self.assertRaises(subprocess.CalledProcessError):
            self.manage("publish")
        self.assertTrue(self.records[0]["draft"])
        self.fail_write = False
        self.assertTrue(self.manage("publish"))

    def test_missing_draft_cannot_be_published(self):
        with self.assertRaisesRegex(ValueError, "Prepare the draft"):
            self.manage("publish")
        self.assertFalse(self.mutations)

    def test_api_failure_does_not_create_release(self):
        self.fail_read = True
        with self.assertRaises(subprocess.CalledProcessError):
            self.manage("draft")
        self.assertFalse(self.mutations)

    def test_checkout_and_tag_must_match(self):
        for attribute in ("head", "tag_commit"):
            with self.subTest(attribute=attribute):
                setattr(self, attribute, "b" * 40)
                with self.assertRaisesRegex(ValueError, "expected commit"):
                    self.manage("draft")
                setattr(self, attribute, COMMIT)
        self.assertFalse(self.mutations)

    def test_metadata_mismatch_does_not_create_release(self):
        with self.assertRaisesRegex(ValueError, "Package version"):
            self.manage("draft", version="9.0.0")
        self.assertFalse(self.mutations)

    def test_missing_changelog_does_not_create_release(self):
        with patch.object(release.subprocess, "check_output", side_effect=subprocess.CalledProcessError(1, "sweet")):
            with self.assertRaises(subprocess.CalledProcessError):
                self.manage("draft")
        self.assertFalse(self.mutations)

    def test_failed_draft_creation_does_not_publish(self):
        self.fail_write = True
        with self.assertRaises(subprocess.CalledProcessError):
            self.manage("draft")
        self.assertFalse(self.records)
        self.assertFalse(self.mutations)

    def test_prerelease_and_build_metadata(self):
        for version, prerelease in [("1.2.3-rc.1+build", True), ("1.2.3+build-with-dash", False)]:
            with self.subTest(version=version):
                self.version = version
                self.manage("draft")
                self.assertIn(f"--prerelease={str(prerelease).lower()}", self.mutations[-1])
        self.version = "1.2.3-rc.1"
        with self.assertRaisesRegex(ValueError, "cannot be Latest"):
            self.manage("publish", latest="true")

    def test_latest_is_explicit(self):
        self.manage("draft")
        self.manage("publish", latest="true")
        self.assertIn("--latest=true", self.mutations[-1])


if __name__ == "__main__":
    unittest.main()
