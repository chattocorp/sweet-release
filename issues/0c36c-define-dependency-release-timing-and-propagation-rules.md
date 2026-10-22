---
data:
  labels:
    - design
    - dependencies
  status: open
  type: question
id: 0c36c
title: Define dependency release timing and propagation rules
---
## Agreed behavior

Dependencies are paths to other packages in the monorepo. A dependency release must cause a dependent package to release. GitHub automation creates per-package release PRs.

## Decisions needed

- Prepare PRs for the full dependency chain together, or wait until a dependency release completes?
- What version bump does a propagated release require, including when the dependency has breaking changes?
- How should propagation combine with a package's own changes and travel through multiple dependency levels?
- How are cycles handled?
- How is an already processed dependency release recorded to avoid repeated releases?

## Proposal, not yet accepted

Wait for dependency release completion, use a patch bump by default, combine with pending changes using the largest bump, and reject cycles initially.

## Completion criteria

Record rules with examples for a chain, multiple dependencies, and a package with its own pending changes.
