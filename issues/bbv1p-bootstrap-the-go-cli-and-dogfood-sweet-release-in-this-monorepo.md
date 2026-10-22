---
data:
  labels:
    - bootstrap
    - cli
    - dogfood
  status: in_progress
  type: task
id: bbv1p
title: Bootstrap the Go CLI and dogfood sweet-release in this monorepo
---
## Context

sweet-release is a Go release tool with a sweet CLI. This repository must use it for its own releases and support monorepo development from the start. A Node-based documentation site will join the repository later.

## Acceptance criteria

- Establish a Go module and a runnable sweet CLI with a basic build and test workflow.
- Organize the repository so it can contain the Go tool and a future Node documentation site.
- Give the tool a package manifest and use local change files for its own release notes once those formats are defined.
- Once GitHub release automation exists, configure it to release this project.

## Implementation progress

- Added a root Go module and cmd/sweet as the initial layout. Internal packages can be added when release logic needs them.
- Adopted Cobra for the CLI surface as requested. A fresh root command handles help, version, and argument validation; errors go to stderr with a nonzero exit status.
- Added Cobra v1.10.2 and dependency checksums. CLI tests cover help, version, unknown commands, and unknown flags.
- Pinned the installed Go 1.27.3 toolchain and added mise build, test, and check tasks.
- Verified CLI tests, go vet, the binary build, and help/version smoke checks.
- The module path is currently sweet-release because no Git remote is configured; choose the canonical import path when the repository destination is known.

## Remaining work

- Add the tool's package manifest and change files after format decisions (gambb, n734g).
- Configure self-releases after GitHub automation exists (6brzx).
