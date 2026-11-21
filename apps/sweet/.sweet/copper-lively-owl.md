---
type: feature
---

Add `sweet github release <tag> [assets...]` to create or reuse a draft GitHub release with notes from the changelog (a reused draft gets the current title and notes), upload assets with replacement, and publish it as a prerelease or Latest as its version requires. Published releases are left unchanged unless you pass `--replace-published`. The `tag.yml` workflow now adds `prerelease` and `highest` (highest stable version of the package) to each `tags` entry, adds a `packages` output keyed by package path, and defaults `release-token` to `GITHUB_TOKEN`, so packages can be published from the same workflow run.
