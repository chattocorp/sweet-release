import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("summary", Path(__file__).with_name("release-summary.py"))
summary = importlib.util.module_from_spec(spec)
spec.loader.exec_module(summary)


class SummaryTests(unittest.TestCase):
    def test_published_and_retry_have_links(self):
        env = {"RELEASE_TAG": "pkg/api@1.0.0", "GH_REPO": "owner/repo"}
        for status in ({"PUBLISH_OUTCOME": "success"}, {"DRAFT_STATE": "published"}):
            self.assertIn("https://github.com/owner/repo/releases/tag/pkg%2Fapi%401.0.0", summary.message(dict(env, **status)))
        self.assertIn("skipped", summary.message(dict(env, DRAFT_STATE="published")))

    def test_failed_stage_explains_retry(self):
        for stage in ("prepare", "build", "upload", "publish"):
            result = summary.message({"RELEASE_TAG": "v1.0.0", "GH_REPO": "owner/repo", stage.upper() + "_OUTCOME": "failure"})
            self.assertIn(stage.capitalize() + " did not complete", result)
            self.assertIn("original workflow", result)


if __name__ == "__main__":
    unittest.main()
