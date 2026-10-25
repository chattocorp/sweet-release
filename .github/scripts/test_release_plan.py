import importlib.util
from pathlib import Path
import unittest


spec = importlib.util.spec_from_file_location("plan", Path(__file__).with_name("release-plan.py"))
plan = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plan)
BASE, HEAD = "a" * 40, "b" * 40
PR = {"pull_request": {"base": {"sha": BASE}, "head": {"sha": HEAD}}}


class ContextTests(unittest.TestCase):
    def test_pr_previews_the_merge_result_and_never_executes_in_auto_mode(self):
        self.assertEqual(plan.context("auto", "pull_request", PR, "refs/pull/1/merge", "c" * 40),
                         {"mode": "plan", "base": BASE, "head": "c" * 40})
        with self.assertRaises(ValueError):
            plan.context("release", "pull_request", PR, "refs/pull/1/merge", "c" * 40)

    def test_main_push_uses_original_range(self):
        self.assertEqual(plan.context("auto", "push", {"before": BASE}, "refs/heads/main", HEAD),
                         {"mode": "release", "base": BASE, "head": HEAD})

    def test_explicit_preview_of_main_and_release_of_historical_range(self):
        self.assertEqual(plan.context("plan", "push", {"before": BASE}, "refs/heads/main", HEAD)["mode"], "plan")
        self.assertEqual(plan.context("release", "workflow_dispatch", {}, "refs/heads/main", "c" * 40, BASE, HEAD),
                         {"mode": "release", "base": BASE, "head": HEAD})

    def test_auto_rejects_unexpected_events_and_branches(self):
        for name, ref in (("push", "refs/heads/feature"), ("push", "refs/tags/v1.0.0"),
                          ("workflow_dispatch", "refs/heads/main"), ("pull_request_target", "refs/heads/main")):
            with self.subTest(name=name, ref=ref), self.assertRaises(ValueError):
                plan.context("auto", name, {}, ref, HEAD, BASE, HEAD)

    def test_range_validation(self):
        for base, head in ((BASE, ""), ("", HEAD), ("HEAD~1", HEAD), (BASE, "0" * 40), ("bad\nmode=release", HEAD)):
            with self.subTest(base=base, head=head), self.assertRaises(ValueError):
                plan.context("release", "workflow_dispatch", {}, "refs/heads/main", HEAD, base, head)
        self.assertEqual(plan.context("release", "push", {"before": "0" * 40}, "refs/heads/main", HEAD)["base"], "0" * 40)

    def test_bad_mode_and_manual_missing_range(self):
        for mode in ("unknown", "release"):
            with self.assertRaises(ValueError):
                plan.context(mode, "workflow_dispatch", {}, "refs/heads/main", HEAD)


if __name__ == "__main__":
    unittest.main()
