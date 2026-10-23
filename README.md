# Sweet Release

- Write release notes as Markdown files alongside each package.
- Manage independently versioned packages in a single repository.
- Configure change types, version bumps, and changelog sections.
- Preview pending releases with `sweet status`.
- Update package versions and generate changelogs with `sweet bump`.
- Copy bumped versions into JSON, YAML, TOML, and plaintext files.
- Create local release tags with `sweet git tag`.
- Use interactive pickers or explicit arguments and piped input.

## How it works

Sweet tracks each package's version in `.sweet.yml` and its pending changes in
`.sweet/*.md`. Repository-wide policies live in `.sweet.config.yml`.

An optional `name` in `.sweet.yml` supplies a display name for changelogs and
releases. Names need not be unique; paths still identify packages and determine tags.

```sh
sweet init repo               # Initialize repository configuration
sweet init package pkg/api    # Initialize an existing package directory
sweet change pkg/api          # Write a release note
sweet status                  # Preview version bumps across the repository
sweet bump pkg/api            # Prepare the package's next release
```

Each note has a change type and a Markdown body, with an optional breaking-change
flag. Sweet uses these notes and the configured policies to calculate the next
semantic version. Running `sweet` without a command starts the change workflow.

`sweet bump` updates `.sweet.yml`, configured `version-files`, and `CHANGELOG.md`,
then removes the consumed change files. It changes local files only: committing,
tagging, and publishing are separate steps. Dependency propagation is not yet
supported.

After committing a release, run `sweet git tag --dry-run` to preview its tags,
then `sweet git tag` to create them locally. Sweet targets the commits that
introduced the current versions. Push tags and publish through your own release
process.

This repository's [Release PR workflow](.github/workflows/release-pr.yml) runs
Sweet as notes reach `main` and keeps release preparation PRs up to date.
Choose one PR per package or one combined PR in `.sweet.config.yml`:

```yaml
release-pr:
  strategy: per-package # or combined
```

`sweet init repo` selects `per-package`. Existing configs that omit the setting
keep combined PRs. After a preparation PR reaches `main` and CI passes, the
tagging workflow creates `<package-directory>/v<version>` tags (or `v<version>`
for a root package). CLI releases then publish Linux, macOS, and Windows binaries
with checksums on the repository's [Releases page](https://github.com/chattocorp/sweet-release/releases).

The [documentation site](apps/docs) contains setup and usage guides.
See [AGENTS.md](AGENTS.md) for development instructions and detailed behavior.

The CLI is in `apps/sweet`, with Go module
`github.com/chattocorp/sweet-release/apps/sweet` and release tags
`apps/sweet/v<version>`. Core imports now use the module's `/core` subpackage.
Existing `pkg/sweet/v` and `pkg/cli/v` releases remain available on the Releases page.
The Astro and Starlight documentation site is a separate package in `apps/docs`.
