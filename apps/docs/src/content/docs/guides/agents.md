---
title: Coding agents
description: Instructions for coding agents asked to manage releases with Sweet Release.
---

A user can hand release management to a coding agent with one sentence:

> Please manage my releases using chattocorp/sweet-release.

The rest of this page is written for that agent. Follow the steps in order, and
ask the user before making choices this page leaves open.

## 1. Install Sweet

Prefer [mise](https://mise.jdx.dev). From the repository root:

```sh
mise use github:chattocorp/sweet-release
sweet --version
```

This records Sweet in the repository's `mise.toml`, so later sessions and CI use
the same tool. Pin a version, such as `github:chattocorp/sweet-release@0.3.0`, if
the project pins its other tools.

Without mise, download the archive for the platform from
[GitHub Releases](https://github.com/chattocorp/sweet-release/releases) and put
the `sweet` executable on the `PATH`.

## 2. Set up the repository

```sh
sweet init repo
sweet init package <dir>   # once per package, or omit <dir> for a root package
```

Before you run these commands:

- **Find the packages.** Directories that are versioned and released independently,
  such as each publishable library, app, or service. Confirm the list with the user.
- **Find the current versions.** Use existing Git tags, `package.json`, `Cargo.toml`,
  or similar. Set each package's `version` in `.sweet.yml` to its latest release.
  Unreleased packages can stay at `0.0.0`.
- **Keep existing tag formats.** If the project already tags releases, set
  [`tag-prefix`](/reference/configuration/#release-tags) so new tags match.
- **Sync version files.** List files that contain the version, such as
  `package.json`, under [`version-files`](/reference/configuration/#version-files).
- **Review change types.** The defaults in `.sweet.config.yml` suit most projects.
  Change them only if the user asks.

Then validate the setup:

```sh
sweet status
sweet doctor
```

## 3. Add CI

For GitHub repositories, add the reusable workflows described in
[GitHub Actions](/guides/github-actions/):

- `doctor.yml` validates release notes in pull requests.
- `bump.yml` opens release PRs from pending notes. It needs a `RELEASE_PR_TOKEN`
  secret; tell the user to create it.
- `tag.yml` tags versions after release PRs are merged and returns the tags,
  so publishing jobs can run in the same workflow.

Pin all three to the same revision. Publishing (building artifacts, uploading
packages, deploying) is project-specific. Ask the user before adding
[publishing jobs](/guides/github-actions/#publish-releases).

## 4. Keep managing releases

Release management continues after setup, and future sessions need to know about
it. Add this section to the repository's agent instructions (`AGENTS.md`,
`CLAUDE.md`, or equivalent), adjusting the types to the repository's configuration:

````markdown
## Releases

Releases are managed with [Sweet Release](https://github.com/chattocorp/sweet-release).

- Every user-facing change needs a release note for each affected package,
  committed with the change:
  `printf 'Describe the change.\n' | sweet change <package> --type <type> --stdin`
- Types are configured in `.sweet.config.yml`. Add `breaking: true` to a note's
  frontmatter for breaking changes.
- Write notes for users of the package, not for maintainers.
- Run `sweet doctor` before finishing; it must pass.
- Do not edit versions or changelogs by hand. Release PRs do that.
````

Without CI, prepare releases locally when the user asks: `sweet status`, then
`sweet bump <package>` (or `sweet bump --all`), review and commit the result, and
run `sweet git tag`. See [Prepare a release](/guides/prepare-a-release/).

## Reference

- [Write release notes](/guides/release-notes/)
- [Configuration](/reference/configuration/)
- [GitHub Actions](/guides/github-actions/)
