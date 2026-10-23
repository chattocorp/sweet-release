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

## Write from a script

Pass both `--type` and `--stdin` for non-interactive use:

```sh
printf 'Add package discovery.\n' | sweet change pkg/api --type feature --stdin
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

In the generated changelog, breaking notes appear first in each section and start
with an inline warning. The existing order is preserved within the breaking and
non-breaking groups:

```markdown
- ⚠️ **BREAKING CHANGE:** Replace the old API with the new API.
```

The type's `section` controls where the note appears in the changelog. A type
without a section can still trigger a version bump. Notes with no version effect
stay pending until another note causes the package to release.

## Links to commits and pull requests

`sweet bump` adds a source link to each committed note when it can identify the
repository and the note's history. No network requests or GitHub API token are
needed. For example:

```markdown
- Add package discovery. ([#123](https://github.com/owner/repo/pull/123))
```

Sweet uses the commit that added the note on the current branch's first-parent
history. With a regular merge, this is the merge commit. It recognizes PR numbers
in subjects that end with ` (#123)` or start with
`Merge pull request #123 from ...`. Otherwise, it links to the commit using its
short SHA. Later edits to a note do not change its source link. For a single
paragraph, the link stays inline, including when the source text wraps across
lines, including lines that start with inline code such as `--version`. For
structured Markdown or multiple paragraphs, the link appears in a
separate paragraph within the changelog item.

Commit the note with the change it describes. These links follow the note's
history, not the history of the code described by the note. Commit subjects are
conventions: Sweet does not verify that a referenced PR exists. A renamed note
is treated as a new addition at its new path.

Sweet infers the repository URL from a GitHub `origin` remote. You can set a
[`repository` URL](/reference/configuration/#changelog-source-repository) to
override it or use a GitHub-compatible host. Missing or unsupported remotes,
uncommitted notes, and unavailable history produce notes without source links.
Shallow repositories also omit source links. In GitHub Actions, set
`fetch-depth: 0` on the checkout that runs `sweet bump` to provide full history.
