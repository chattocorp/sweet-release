---
data:
  labels:
    - packages
    - cli
  status: in_progress
  type: feature
id: petk5
title: Discover releasable packages from local manifests
---
## Context

A directory becomes a package through a `.sweet.yml` manifest. Packages are identified by repository-relative paths, with `.` for the root; no name field is required.

## Acceptance criteria

- Discover packages in both single-package repositories and monorepos.
- Support nested packages, including when the repository root is itself a package.
- Read the released version, metadata, and dependency paths from each manifest.
- Resolve dependency references to discovered packages and report invalid manifests or unresolved references with useful file context.

## Progress

Implemented `sweet list`: lists package paths and current versions, sorted by path, from anywhere in the Git repository. Includes tracked and untracked manifests, respects Git ignore rules, and reports invalid YAML or missing versions with manifest paths. Package initialization writes `version: 0.0.0`. Tests cover nested packages, root packages, ignored files, tracked files, empty repositories, and invalid manifests. Go tests and vet pass.

## Remaining work

Dependency metadata and reference resolution remain unimplemented, pending path rules in gambb.
