import importlib.util
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location(
    "promote_latest", Path(__file__).with_name("promote-latest.py")
)
promotion = importlib.util.module_from_spec(spec)
spec.loader.exec_module(promotion)


def release(tag):
    return dict(tag_name=tag, draft=False, prerelease=False)


class NewTagFormatTest(unittest.TestCase):
    def test_new_format_wins_equal_version_and_orders_by_version(self):
        tags = ["apps/sweet/v1.0.0", "sweet@1.0.0", "pkg/cli/v0.9.0"]
        self.assertEqual(promotion.latest_tag(map(release, tags)), "sweet@1.0.0")
        tags.append("sweet@1.1.0")
        self.assertEqual(promotion.latest_tag(map(release, tags)), "sweet@1.1.0")


class LatestTagTests(unittest.TestCase):
    def test_numeric_version_order_ignores_publication_order(self):
        tags = ["apps/sweet/v0.9.0", "apps/sweet/v0.10.0", "apps/sweet/v0.2.0"]
        for order in (tags, list(reversed(tags))):
            self.assertEqual(promotion.latest_tag(map(release, order)), "apps/sweet/v0.10.0")

    def test_version_precedes_prefix_preference(self):
        tags = ["apps/sweet/v0.2.0", "pkg/sweet/v0.3.0", "pkg/cli/v1.0.0"]
        self.assertEqual(promotion.latest_tag(map(release, tags)), "pkg/cli/v1.0.0")

    def test_equal_versions_prefer_current_layout(self):
        tags = ["pkg/cli/v0.2.0", "apps/sweet/v0.2.0", "pkg/sweet/v0.2.0"]
        self.assertEqual(promotion.latest_tag(map(release, tags)), "apps/sweet/v0.2.0")
        self.assertEqual(
            promotion.latest_tag(map(release, [tags[0], tags[2]])), "pkg/sweet/v0.2.0"
        )

    def test_excludes_drafts_prereleases_and_unrelated_or_invalid_tags(self):
        candidates = [
            dict(tag_name="apps/sweet/v9.0.0", draft=True, prerelease=False),
            dict(tag_name="apps/sweet/v8.0.0", draft=False, prerelease=True),
        ]
        candidates += list(map(release, [
            "apps/sweet/v7.0.0-rc.1", "apps/docs/v6.0.0", "v5.0.0",
            "apps/sweet/v04.0.0", "apps/sweet/v3.0", "apps/sweet/v2.0.0+",
            "apps/sweet/v2.0.0+bad..metadata", "apps/sweet/v2.0.0\n",
        ]))
        self.assertIsNone(promotion.latest_tag(candidates))
        candidates.append(release("apps/sweet/v0.2.0"))
        self.assertEqual(promotion.latest_tag(candidates), "apps/sweet/v0.2.0")

    def test_build_metadata_has_no_version_precedence(self):
        tags = ["pkg/cli/v0.2.0+zzz", "apps/sweet/v0.2.0+aaa", "apps/sweet/v0.1.0+zzz"]
        self.assertEqual(promotion.latest_tag(map(release, tags)), "apps/sweet/v0.2.0+aaa")
        tags = ["apps/sweet/v0.2.0+bbb", "apps/sweet/v0.2.0+aaa"]
        for order in (tags, list(reversed(tags))):
            self.assertEqual(promotion.latest_tag(map(release, order)), tags[0])

    def test_no_releases(self):
        self.assertIsNone(promotion.latest_tag([]))


class PromotionTests(unittest.TestCase):
    @patch.object(promotion.subprocess, "run")
    def test_pagination_and_retry_reconcile_all_published_releases(self, run):
        pages = [[release("apps/sweet/v0.2.0")], [release("apps/sweet/v0.10.0")]]
        run.return_value = subprocess.CompletedProcess([], 0, stdout=json.dumps(pages))
        promotion.main()
        promotion.main()
        edits = [call for call in run.call_args_list if call.args[0][1] == "release"]
        self.assertEqual(len(edits), 2)
        for call in edits:
            self.assertEqual(call.args[0], ["gh", "release", "edit", "apps/sweet/v0.10.0", "--latest"])
            self.assertTrue(call.kwargs["check"])
        self.assertIn("--paginate", run.call_args_list[0].args[0])
        self.assertIn("--slurp", run.call_args_list[0].args[0])

    @patch.object(promotion.subprocess, "run")
    def test_empty_list_does_not_edit(self, run):
        run.return_value = subprocess.CompletedProcess([], 0, stdout="[[]]")
        promotion.main()
        self.assertEqual(run.call_count, 1)

    @patch.object(promotion.subprocess, "run")
    def test_api_failure_stops_promotion(self, run):
        run.side_effect = subprocess.CalledProcessError(1, "gh")
        with self.assertRaises(subprocess.CalledProcessError):
            promotion.main()
        self.assertEqual(run.call_count, 1)

    @patch.object(promotion.subprocess, "run")
    def test_failed_edit_fails_the_job(self, run):
        run.side_effect = [
            subprocess.CompletedProcess([], 0, stdout=json.dumps([[release("apps/sweet/v0.2.0")]])),
            subprocess.CalledProcessError(1, "gh"),
        ]
        with self.assertRaises(subprocess.CalledProcessError):
            promotion.main()

    @patch.object(promotion.subprocess, "run")
    def test_invalid_response_does_not_edit(self, run):
        run.return_value = subprocess.CompletedProcess([], 0, stdout="not JSON")
        with self.assertRaises(json.JSONDecodeError):
            promotion.main()
        self.assertEqual(run.call_count, 1)


if __name__ == "__main__":
    unittest.main()
