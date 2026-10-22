---
data:
  labels:
    - versioning
    - cli
  status: open
  type: feature
id: 0y7fs
title: Calculate package releases from pending changes
---
## Context

The release calculation combines a package's released version with pending change types and breaking flags, plus dependency propagation under the agreed rules.

## Acceptance criteria

- Calculate the next version using the agreed version rules.
- Combine multiple changes for a package into one proposed release.
- Retain the notes and reasons that caused the proposed release.
- Make the calculation verifiable locally before GitHub writes occur.

## Proposed interface, not yet accepted

A sweet status command could show proposed versions and reasons. The calculation should be separate from GitHub operations.

## Design dependency

Default version rules and dependency timing must be settled before their behavior is finalized.
