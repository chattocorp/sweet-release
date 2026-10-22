---
type: feature
breaking: true
---

Consolidate the CLI and core into one Sweet Release package at `pkg/sweet`,
starting from the CLI version 0.1.0 and changelog. The Go module is now
`github.com/chattocorp/sweet-release/pkg/sweet`; core imports use its `/core`
subpackage. New releases use `pkg/sweet/v<version>` tags. Existing `pkg/cli/v`
releases remain available and can still be published. The retired core changelog
is retained as historical documentation.
