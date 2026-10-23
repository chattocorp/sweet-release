---
type: feature
breaking: true
---

Remove `sweet release-pr plan` and the `release-pr` configuration setting.
Release PR automation now uses `sweet status --json` and `sweet bump`, with
PR policy owned by the workflow. Existing `release-pr` settings have no effect
and can be removed.
