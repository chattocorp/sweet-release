---
data:
  labels:
    - packages
    - cli
  status: open
  type: feature
id: petk5
title: Discover releasable packages from local manifests
---
## Context

A directory becomes a package by containing a sweet manifest. The manifest stores its current released version, metadata, and dependencies expressed as paths to other packages.

## Acceptance criteria

- Discover packages in both single-package repositories and monorepos.
- Support nested packages, including when the repository root is itself a package.
- Read the released version, metadata, and dependency paths from each manifest.
- Resolve dependency references to discovered packages and report invalid manifests or unresolved references with useful file context.

## Design dependency

Use the configuration format and path rules once agreed; exact filenames remain open.
