---
data:
  labels:
    - configuration
    - changes
  status: in_progress
  type: feature
id: cqnj7
title: Configure repository-wide change types and defaults
---
## Context

The repository root provides shared configuration in `.sweet.config.yml`.

## Acceptance criteria

- Load repository-wide configuration from the agreed filename.
- Define supported change types and their version effects.
- Apply shared defaults consistently across discovered packages.
- Report invalid configuration and unknown change types clearly.

## Implemented

- `sweet init repo` writes a `types` list with fix, feature, chore, refactor, and performance.
- `sweet change` loads types from the target package's Git repository root and offers an fzf type picker after package selection. `--type` bypasses the type picker for scripted use.
- New notes contain YAML frontmatter with the selected `type`. With `--stdin`, input is preserved as the Markdown body below the generated frontmatter.
- Missing configuration or an omitted types field uses built-in defaults. Empty lists, duplicates, invalid names, and unknown selections are rejected.
- The repository configuration now uses these defaults for dogfooding.
- Verified with Go tests, vet, build, and a real terminal flow through both fzf menus with piped input.

## Remaining decisions

Version effects for change types remain undefined. Configurable changelog headings, package overrides, and merge rules are also undecided.
