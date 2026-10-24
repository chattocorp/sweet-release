---
title: sweet-release issues
description: Track planned work, bugs, and open questions for sweet-release. Store context and acceptance criteria in each document's Markdown body.
schema:
  $schema: https://json-schema.org/draft/2020-12/schema
  type: object
  properties:
    type:
      type: string
      enum: [feature, bug, task, question]
      description: The kind of work or question tracked by this issue.
    status:
      type: string
      enum: [open, in_progress, blocked, done, cancelled]
      description: The current work state. New issues normally start as open.
    priority:
      type: string
      enum: [low, normal, high, urgent]
      description: Optional priority for ordering work.
    labels:
      type: array
      items:
        type: string
        minLength: 1
      uniqueItems: true
      description: Optional labels for related areas of work.
  required: [type, status]
  additionalProperties: false
---

# sweet-release issues

Track project work, bugs, and open questions in this collection. Search for an
existing issue before creating one. Read relevant documents before changing them.
Refer to issues by stored ID, not filename.

## Issue content

Required data fields are `type` and `status`. Priority and labels are optional.
Keep context, acceptance criteria, related issue IDs, and unresolved decisions in
the Markdown body. Distinguish agreed requirements from proposals; unanswered
design questions are not accepted requirements.

## Workflow

New issues normally start as `open`. Set `in_progress` when work starts, `blocked`
when a concrete obstacle prevents progress, and `done` when acceptance criteria
are met. Record blockers or completion evidence in the body. Use `cancelled` for
work that is no longer wanted, with the reason. Prefer these states over deleting
issue history.

After writes, run `means doctor --path issues --json` from the repository root
and address diagnostics. Report affected issue IDs and changes. Keep documents
and `MEANS.md` in Git; leave the ignored `issues/.means/` cache untracked.
