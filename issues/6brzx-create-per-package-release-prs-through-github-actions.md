---
data:
  labels:
    - github
    - releases
  status: open
  type: feature
id: 6brzx
title: Create per-package release PRs through GitHub Actions
---
## Context

sweet must run in a GitHub Action and create a release PR for each package that needs a release.

## Acceptance criteria

- Provide a CLI workflow that runs in GitHub Actions.
- Create separate release PRs for eligible packages.
- Reconcile an existing package release PR on subsequent runs rather than creating duplicates.
- Include the proposed version and package release notes in the release changes.
- Keep each package's changes scoped to its release and follow the agreed dependency timing.

## Decisions still needed

Define when manifests advance their released version, when consumed change files are removed, and what merge triggers. Tags, GitHub Releases, and artifact publication were proposed but are not yet agreed. Define these lifecycle details before implementing finalization.
