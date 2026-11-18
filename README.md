# Sweet Release

Release independently versioned packages with Markdown change files.
Sweet calculates version bumps, updates version files, and generates changelogs.

## Set up a repository

Ask your coding agent:

> Set this repo up for chattocorp/sweet-release.

### Install Sweet

From the repository root, install the CLI with [mise](https://mise.jdx.dev):

```sh
mise use github:chattocorp/sweet-release
sweet --version
```

You can also download the archive for your platform and put the `sweet` binary
on your `PATH`. [GitHub Releases](https://github.com/chattocorp/sweet-release/releases)

### Choose what gets a version

Check the repository's manifests, tags, and publishing jobs. If the whole
repository ships together, initialize one package at the root:

```sh
sweet init repo
sweet init package
```

If packages ship independently, run `sweet init repo` once, then initialize
each release directory. For example:

```sh
sweet init package apps/api
sweet init package apps/web
```

Skip initialization for manifests that already exist. Repository-wide change
types live in `.sweet.config.yml`. Each package has its own `.sweet.yml`.

### Keep existing versions and tags

Set each package's `version` to its latest release. An unreleased package starts
at `0.0.0`. List any files that must change with the version. For example, a
package released as `widget-v1.2.3` with its version in `package.json` uses:

```yaml
version: 1.2.3
tag-prefix: widget-v
version-files:
  - path: package.json
    format: json
    key: version
```

Omit `tag-prefix` to use the default tags, such as `v1.2.3` for the root or
`api@1.2.3` for `apps/api`.

### Connect GitHub Actions

| When | Reusable workflow | Result |
| --- | --- | --- |
| A pull request opens or changes | `doctor.yml` | Checks release notes and configuration |
| A commit reaches the release branch | `bump.yml` | Opens or updates release PRs |
| Checks pass on that push | `tag.yml` | Tags committed versions |

Use the same upstream commit SHA or published workflow tag for all three.
Set the push filter and Bump's `base-branch` to the repository's release branch.
[Copy the caller workflow examples](apps/docs/src/content/docs/guides/github-actions.md).

If tags already exist, land the initial manifests first. Enable Tag in a later
push so it starts with future version changes.

Bump and Tag take a `release-token` secret. Have the maintainer set
`RELEASE_PR_TOKEN` to a token with Contents and Pull requests write access
and pass it to both workflows. Keep the repository's publishing jobs.

### Check the setup

```sh
sweet status
sweet doctor
```

The status table should list each release package at its current version.
Doctor should report no release problems.

Add a release-note rule to the target repository's `AGENTS.md` or equivalent.
For each user-facing change, commit a note for each affected package. Use the
package directory (`.` for the root) and a type from `.sweet.config.yml`:

```sh
printf '%s\n' 'Fix a startup crash.' | sweet change . --type fix --stdin
```

The [coding agent guide](apps/docs/src/content/docs/guides/agents.md#4-keep-managing-releases)
includes a release policy you can copy.

## Prepare a release

For example, to release the API in a monorepo:

```sh
sweet status
sweet bump apps/api
```

Sweet calculates the bump from the API's notes, updates its version files and
changelog, and removes the consumed notes. Other packages' notes stay pending.
Use `sweet bump --all` to prepare all pending releases together. Review and
commit the result. Dependency updates between packages belong to your release
process.

## Prereleases

To ship prereleases, add a label to the package's `.sweet.yml`:

```yaml
version: 1.2.3
prerelease: beta
```

`sweet bump` then releases `1.3.0-beta.0`, `1.3.0-beta.1`, and so on. Remove the
label to release `1.3.0`; its changelog entry combines all changes since `1.2.3`.

## Tag releases

After you commit a release, preview and create its Git tags:

```sh
sweet git tag --dry-run
sweet git tag
```

Sweet creates annotated local tags for current committed package versions, such as
`api@1.2.0` (or `v1.2.0` for a root package). Each tag points to the commit
that introduced that version, even if more commits followed it. Matching tags are
kept; conflicting tags cause an error.

Use a checkout with full Git history and fetch remote tags first. Sweet does not
push tags or publish releases; those steps remain part of your release process.

See the [getting started guide](apps/docs/src/content/docs/getting-started.md)
and [documentation](docs/README.md).
