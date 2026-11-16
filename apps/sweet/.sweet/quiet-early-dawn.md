---
type: feature
---

Release prereleases by setting `prerelease: <label>` in a package's `.sweet.yml`, such as `1.3.0-beta.0` from `1.2.3`. Remove the label to finalize; the final changelog entry combines all changes since the last stable release. Bumping an existing prerelease now keeps its base version when the bump is already covered, so `2.0.0-beta.1` finalizes to `2.0.0` instead of `3.0.0`.

A package whose manifest version is already a prerelease but has no `prerelease` label is now finalized on its next bump. Add the label to keep releasing prereleases.
