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

Change frontmatter specifies a type such as fix or feature, not patch, minor, or major. Breaking changes use breaking: true. Types will be configurable at repository level.

## Decisions needed

- Choose built-in types and their version effects.
- Define the version effect of breaking: true, including for 0.x versions.
- Define how multiple pending changes combine into one next version.
- Define behavior for unknown types and packages without a previous release.

## Proposal, not yet accepted

Map fix to patch and feature to minor, and select the largest required bump. Breaking changes override the normal type effect.

## Completion criteria

Record default rules and representative version examples.
