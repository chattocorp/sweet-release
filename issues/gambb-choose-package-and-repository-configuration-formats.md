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
## Accepted decisions

- Call each releasable unit a package.
- Use YAML: `.sweet.yml` for package manifests and `.sweet.config.yml` for repository policy.
- Keep repository policy separate from package metadata so the root can itself be a package.
- `sweet init repo` creates `.sweet.config.yml` at the Git repository root, including when invoked from a subdirectory.
- `sweet init package [dir]` creates `.sweet.yml` in the current directory or the given existing directory.
- Init commands preserve existing files. Repository configuration starts as an empty YAML mapping.
- Package manifests contain the current `version`; no name field is required. `sweet list` identifies packages by repository-relative path, using `.` for the root.

## Current implementation defaults

Package initialization writes `version: 0.0.0`. The version must be a non-empty YAML string without whitespace. Semantic version validation is deferred to version calculation.

## Decisions needed

- Define dependency metadata.
- Decide whether dependency paths are relative to the package or repository root. Package-relative paths remain a proposal.

## Implementation evidence

Both init commands and `sweet list` are implemented. Go tests and vet pass.

## Completion criteria

Record the remaining dependency metadata and path rules and examples.
