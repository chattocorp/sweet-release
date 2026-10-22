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
- Init commands preserve existing files. Initial files contain a descriptive comment and an empty YAML mapping (`{}`), pending metadata decisions.

## Decisions needed

- Define package metadata, including the current released version and dependency paths.
- Decide whether dependency paths are relative to the package or repository root. Package-relative paths remain a proposal.

## Implementation evidence

Both init commands are implemented. Go tests cover locations, existing-file preservation, and invalid targets and arguments. `go test ./...` and `go vet ./...` pass.

## Completion criteria

Record the remaining metadata and dependency-path rules and examples.
