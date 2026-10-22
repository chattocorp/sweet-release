---
data:
  labels:
    - dependencies
    - versioning
  status: open
  type: feature
id: k50s1
title: Propagate releases to dependent packages
---
## Context

Manifest dependencies exist specifically to trigger releases of dependent packages when their dependencies release.

## Acceptance criteria

- Build the package dependency graph from declared paths.
- Trigger dependent releases under the agreed timing and bump rules.
- Apply the agreed rules across multiple dependency levels.
- Combine propagated releases with a dependent package's own pending changes.
- Explain which dependency caused each propagated release.
- Handle repeat runs and cycles according to the agreed policy.

## Design dependency

Release timing, propagated bumps, cycle handling, and recording consumed dependency releases remain open questions.
