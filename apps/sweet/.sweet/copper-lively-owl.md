---
type: feature
---

Add `sweet github release <tag> [assets...]` to create or reuse a draft GitHub release with notes from the changelog, upload assets with replacement, and publish it as a prerelease or Latest as its version requires. The `tag.yml` workflow now adds `prerelease` and `latest` to each `tags` entry, adds a `packages` output keyed by package path, and defaults `release-token` to `GITHUB_TOKEN`, so packages can be published from the same workflow run.
