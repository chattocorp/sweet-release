---
title: Write release notes
description: Capture changes as Markdown files beside each package.
---

## Use the interactive workflow

```sh
sweet change pkg/api
```

Choose a configured change type, then enter a one-line description. An empty
description opens `$EDITOR` with the generated frontmatter. Sweet waits for the
editor to exit. Editor commands can include arguments, such as `code --wait`.

Without a package argument, Sweet uses the current directory if it is a package.
Otherwise, a searchable picker lists packages when a terminal is available.
Press Esc to cancel without creating a note.

## Select a package by name

For a package with `name: api` in its manifest, use:

```sh
sweet change --package api
```

Names resolve from any directory in the repository. When `name` is omitted,
use the repository-relative path as the name, such as `--package pkg/api`.

Positional directory arguments are relative to the current directory instead.
Do not combine `--package` with a directory argument. Unknown or ambiguous names
stop the command before it writes a note. See [Package manifest](../../reference/configuration/#package-manifest)
for name and display label configuration.

Existing malformed notes do not prevent creating a note by name. Correct those
notes before running `sweet bump`, which validates all pending notes in the selection.

## Write from a script

Pass both `--type` and `--stdin` for non-interactive use:

```sh
printf 'Add package discovery.\n' | sweet change pkg/api --type feature --stdin
printf 'Add an endpoint.\n' | sweet change --package api --type feature --stdin
```

Input is the Markdown body, without frontmatter. Sweet writes it verbatim below
the generated frontmatter. Existing notes are never overwritten.

## Note format

Notes are direct Markdown files inside a package's `.sweet/` directory:

```markdown
---
type: feature
breaking: false
---

Add package discovery.
```

`type` must match a configured repository type. `breaking` is optional and
defaults to `false`. Other frontmatter properties are rejected.

A breaking change requires at least a minor bump for `0.x` packages and a major
bump for other versions. A type configured with a major bump still requires a
major bump for a `0.x` package.

The type's `section` controls where the note appears in the changelog. A type
without a section can still trigger a version bump. Notes with no version effect
stay pending until another note causes the package to release.
