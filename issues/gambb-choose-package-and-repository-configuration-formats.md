---
data:
  labels:
    - design
    - configuration
  status: open
  type: question
id: gambb
title: Choose package and repository configuration formats
---
## Context

A folder becomes a releasable package through a local manifest. Its format and exact filename are not yet agreed.

## Decisions needed

- Choose YAML, TOML, or supported alternatives and exact filenames.
- Define package metadata, including the current released version and dependency paths.
- Decide whether dependency paths are relative to the package or repository root.
- Keep repository policy separate from package metadata so the root can itself be a package.

## Proposal, not yet accepted

Use .sweet.yml for package manifests and .sweet.config.yml for repository policy, with dependency paths relative to the package.

## Completion criteria

Record the chosen formats, path rules, and minimal examples.
