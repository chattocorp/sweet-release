import importlib.util
import json
import os
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location("release", Path(__file__).with_name("github-release.py"))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
COMMIT = "a" * 40


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.records = []
        self.mutations = []
        self.head = self.tag_commit = COMMIT
        self.remote_commit = COMMIT
        self.remote_calls = 0
        self.move_before_write = False
        self.api_response = None
        self.version = "1.2.3"
        self.fail_read = False
        self.fail_write = False
        self.output_patch = patch.object(release, "output", side_effect=self.output)
        self.run_patch = patch.object(release, "run", side_effect=self.run_command)
        self.notes_patch = patch.object(release.subprocess, "check_output", return_value="Release notes.\n")
        for replacement in (self.output_patch, self.run_patch, self.notes_patch,
                            patch.dict(os.environ, {"GH_REPO": "example/repo"})):
            replacement.start()
            self.addCleanup(replacement.stop)

    def output(self, *args):
        if args[:2] == ("git", "rev-parse"):
            return self.head if args[-1] == "HEAD" else self.tag_commit
        if args[:2] == ("git", "ls-remote"):
            self.remote_calls += 1
            if self.move_before_write and self.remote_calls == 2:
                self.remote_commit = "b" * 40
            if self.remote_commit is None:
                raise subprocess.CalledProcessError(2, args)
            # Annotated tags must compare the peeled commit, not the tag object.
            return f"{'c' * 40}\trefs/tags/api@1.2.3\n{self.remote_commit}\trefs/tags/api@1.2.3^{{}}"
        if args[:2] == ("gh", "api"):
            if self.fail_read:
                raise subprocess.CalledProcessError(1, args)
            self.assertEqual(args[2], "graphql")
            return json.dumps(self.api_response or {"data": {"repository": {
                "release": self.records[0] if self.records else None}}})
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
            self.assertEqual(args[args.index("--target") + 1], COMMIT)
            self.records.append({"isDraft": True, "isPrerelease": "--prerelease=true" in args})
        elif "--draft=false" in args:
            self.assertIn("--verify-tag", args)
            self.assertEqual(args[args.index("--tag") + 1], "api@1.2.3")
            self.records[0]["isDraft"] = False
        elif "--prerelease=true" in args or "--prerelease=false" in args:
            self.records[0]["isPrerelease"] = "--prerelease=true" in args

    def manage(self, phase, **kwargs):
        return release.manage(phase, "api@1.2.3", COMMIT, "pkg/api",
                              kwargs.pop("version", self.version), **kwargs)

    def test_draft_retry_publish_and_published_retry(self):
        self.assertEqual(self.manage("draft"), "draft")
        self.assertTrue(self.records[0]["isDraft"])
        self.assertEqual(self.manage("draft"), "draft")
        self.assertEqual([call[2] for call in self.mutations], ["create", "edit"])
        self.assertEqual(release.manage("publish", "api@1.2.3", COMMIT), "published")
        self.assertIn("--latest=false", self.mutations[-1])
        count = len(self.mutations)
        self.assertEqual(self.manage("draft"), "published")
        self.assertEqual(self.manage("publish"), "published")
        self.assertEqual(len(self.mutations), count)

    def test_failed_publish_leaves_draft_for_retry(self):
        self.manage("draft")
        self.fail_write = True
        with self.assertRaises(subprocess.CalledProcessError):
            self.manage("publish")
        self.assertTrue(self.records[0]["isDraft"])
        self.fail_write = False
        self.assertEqual(self.manage("publish"), "published")

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
        self.records[0]["isPrerelease"] = True
        with self.assertRaisesRegex(ValueError, "cannot be Latest"):
            # The actual release flag wins, even without a caller-supplied version.
            release.manage("publish", "api@1.2.3", COMMIT, latest="true")

    def test_latest_is_explicit(self):
        self.manage("draft")
        self.manage("publish", latest="true")
        self.assertIn("--latest=true", self.mutations[-1])

    def test_automatic_latest_leaves_the_decision_to_github(self):
        self.manage("draft")
        self.manage("publish", latest="auto")
        self.assertFalse(any(arg.startswith("--latest") for arg in self.mutations[-1]))

    def test_status_needs_no_cli_or_metadata(self):
        self.assertEqual(release.manage("status", "api@1.2.3", COMMIT), "missing")
        self.records.append({"isDraft": True, "isPrerelease": False})
        self.assertEqual(release.manage("status", "api@1.2.3", COMMIT), "draft")
        self.records[0]["isDraft"] = False
        self.assertEqual(release.manage("status", "api@1.2.3", COMMIT), "published")
        self.assertFalse(self.mutations)
        self.assertFalse(any(call.args[0] == "sweet" for call in release.output.call_args_list))

    def test_inaccessible_repository_and_graphql_errors_fail_closed(self):
        for response in ({"data": {"repository": None}},
                         {"data": {"repository": {"release": None}}, "errors": [{"message": "denied"}]}):
            self.api_response = response
            with self.assertRaisesRegex(ValueError, "Cannot read"):
                self.manage("draft")
        self.assertFalse(self.mutations)

    def test_remote_tag_moved_or_deleted(self):
        for remote in ("b" * 40, None):
            self.remote_commit = remote
            with self.assertRaises((ValueError, subprocess.CalledProcessError)):
                self.manage("draft")
        self.assertFalse(self.mutations)

    def test_remote_tag_rechecked_before_mutation(self):
        for phase in ("draft", "publish"):
            with self.subTest(phase=phase):
                self.records = [{"isDraft": True, "isPrerelease": False}]
                self.remote_commit, self.remote_calls = COMMIT, 0
                self.move_before_write = True
                with self.assertRaisesRegex(ValueError, "Remote release tag"):
                    self.manage(phase)
        self.assertFalse(self.mutations)


if __name__ == "__main__":
    unittest.main()
