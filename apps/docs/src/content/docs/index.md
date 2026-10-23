---
title: Sweet Release
description: Write change files. Review versions. Prepare releases.
---

Sweet Release is a CLI that turns Markdown change files into package versions and
changelogs. Keep release notes beside your code and release each package in a
monorepo independently.

```sh
sweet change pkg/api   # Describe a change
sweet status           # Review pending versions
sweet bump pkg/api     # Update version files and the changelog
```

## Start with a change file

Each package has a `.sweet.yml` manifest and a `.sweet/` directory for pending
notes. A note describes one change, with a type that controls its version bump
and changelog section. Notes travel with your code through review.

## Review before you release

`sweet status` shows the current version and planned bump for every package.
`sweet bump` updates local files and consumes the notes for the selected releases.
You review and commit the result. Tagging and publishing are separate steps.

## Build your workflow

- [Get started](./getting-started/) with your first package and release note.
- [Write release notes](./guides/release-notes/) interactively or from scripts.
- [Prepare a release](./guides/prepare-a-release/) for one or more packages.
- [Configure Sweet](./reference/configuration/) for your version and changelog policies.

Sweet works across languages. Configure version targets in JSON, YAML, TOML, or
plaintext files without changing how your project builds or publishes.
