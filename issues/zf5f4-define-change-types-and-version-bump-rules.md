---
data:
  labels:
    - design
    - versioning
  status: open
  type: question
id: zf5f4
title: Define change types and version bump rules
---
## Agreed behavior

Change frontmatter specifies a type such as fix or feature, not patch, minor, or major. Breaking changes use breaking: true. Repository configuration maps each type to a bump (major, minor, patch, or none) and an independent changelog section. Empty sections exclude changes from changelogs without changing their version effect.

## Current configuration defaults

- fix: minor, section Fixes (matches the requested configuration example).
- feature: minor, section Features.
- performance: patch, section Performance.
- chore and refactor: none, no changelog section.

Unknown types are rejected. Missing bump settings mean none. Policy parsing is implemented in cqnj7; version calculation is not yet implemented.

## Decisions needed

- Define the version effect of breaking: true, including for 0.x versions.
- Define how multiple pending changes combine into one next version.
- Define behavior for packages without a previous release.

## Proposal, not yet accepted

Select the largest required bump. Breaking changes override the normal type effect.

## Completion criteria

Record remaining version rules and representative version examples.
