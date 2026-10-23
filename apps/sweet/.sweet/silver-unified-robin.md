---
type: feature
breaking: true
---

Consolidate the CLI and core into one Sweet Release package at `apps/sweet`,
starting from the CLI version 0.1.0 and changelog. The Go module is now
`github.com/chattocorp/sweet-release/apps/sweet`; core imports use its `/core`
subpackage. New releases use `apps/sweet/v<version>` tags. Existing `pkg/sweet/v` and `pkg/cli/v`
releases remain available and can still be published. The retired core changelog
is retained as historical documentation.
