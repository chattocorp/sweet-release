---
data:
  labels:
    - configuration
    - changes
  status: open
  type: feature
id: cqnj7
title: Configure repository-wide change types and defaults
---
## Context

The repository root provides shared configuration. Change types such as fix and feature can be configured for the repository. This can follow the initial implementation.

## Acceptance criteria

- Load repository-wide configuration from the agreed filename.
- Define supported change types and their version effects.
- Apply shared defaults consistently across discovered packages.
- Report invalid configuration and unknown change types clearly.

## Open scope

Configurable changelog headings were proposed, but not agreed. Package-level overrides and configuration merge rules are also undecided.
