---
data:
  labels:
    - changes
    - packages
  status: open
  type: feature
id: n734g
title: Read package-local Markdown change files
---
## Context

Each package accumulates release notes in its own .sweet/ subfolder. Release intent comes from these files, not Git commit messages.

## Acceptance criteria

- Read Markdown documents with frontmatter from each package's .sweet/ folder.
- Preserve the Markdown release-note body.
- Read a change type such as fix or feature and an optional breaking: true flag.
- Associate each change with its owning package.
- Report malformed frontmatter or invalid fields with file context.
- Do not infer releases from commit messages.

## Open details

Define the exact frontmatter schema and file naming rules during implementation. Change types express intent rather than SemVer bump names.
