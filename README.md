# Sweet Release

- Write release notes as Markdown files alongside each package.
- Manage independently versioned packages in a single repository.
- Configure change types, version bumps, and changelog sections.
- Preview pending releases with `sweet status`.
- Update package versions and generate changelogs with `sweet bump`.
- Copy bumped versions into JSON, YAML, TOML, and plaintext files.
- Use interactive pickers or explicit arguments and piped input.

## How it works

Sweet tracks each package's version in `.sweet.yml` and its pending changes in
`.sweet/*.md`. Repository-wide policies live in `.sweet.config.yml`.

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

See [AGENTS.md](AGENTS.md) for development instructions and detailed behavior.
