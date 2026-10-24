---
name: sweet-issues
description: Create, find, and update issues in sweet-release's local Means collection. Use when tracking project work, bugs, open questions, or progress in issues/.
---

# sweet-release issues

Use the Means collection at `issues/` in this checkout. Run commands from the
repository root, or pass the absolute path to this checkout's `issues/`.
Means uses the exact `--path`; it does not search parent directories.

Read `issues/MEANS.md` before use. It is the source of truth for the collection's
schema and workflow. Follow its guidance for issue content, status changes, and
validation. Use `means --help` or command-specific help as needed.

## Find and read

```sh
means list --path issues --limit 0 --json
means search --path issues --filter status=open --limit 0 --json
means read <id> --path issues --json
```

Check for existing issues before creating one. Read relevant documents before
changing them. Refer to issues by stored ID, not filename. Inspect diagnostics
as well as command exit codes.

## Create

Use `means create` so Means generates the immutable ID. Example:

```sh
means create --path issues --input - --json <<'JSON'
{
  "frontmatter": {
    "title": "Discover releasable packages",
    "data": { "type": "feature", "status": "open" }
  },
  "body": "Discover packages from their manifests.\n\n## Acceptance criteria\n\n- Find nested packages even when the repository root is a package.\n"
}
JSON
```

## Update and close

Read the issue with `--json` to obtain its current hash. Use
`means update <id> --path issues --input - --json` with an object such as:

```json
{
  "hash": "<hash returned by read>",
  "frontmatter": { "data": { "status": "in_progress" } }
}
```

Omitted metadata stays unchanged. Omit `body` to preserve it; supplying it
replaces the entire body. If the hash conflicts, read again and review changes
before retrying. Do not retry a successful create: it creates another issue.

Follow the status workflow and validation steps in `issues/MEANS.md`.
