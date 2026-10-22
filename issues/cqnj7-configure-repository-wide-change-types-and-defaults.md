---
data:
  labels:
    - configuration
    - changes
  status: done
  type: feature
id: cqnj7
title: Configure repository-wide change types and defaults
---
## Implemented

Repository configuration uses a `types` mapping in `.sweet.config.yml`. Each type has `bump` (major, minor, patch, or none; omitted means none) and `section` (a changelog heading; empty, null, or omitted excludes that type from the changelog). These settings are independent.

Initialization defaults: fix uses minor / Fixes, matching the requested example; feature uses minor / Features; performance uses patch / Performance; chore and refactor use none and empty sections.

The core loads ordered type policies from the target package's repository. The CLI uses their names in the fzf picker and validates `--type`. Missing configuration or an omitted types field uses built-in defaults. The former list format, empty mappings, duplicates, invalid names, unknown settings, and invalid field values produce errors with configuration file context.

Changeset frontmatter records only the selected type; repository configuration defines its release policy. Tests cover policy parsing, independent bump and section settings, custom configuration, defaults, and invalid input. Go tests, vet, and build pass.

## Follow-up scope

Release calculation and changelog generation will consume these policies in later work. Package-level overrides and merge rules remain undecided. See zf5f4 for remaining versioning decisions.
