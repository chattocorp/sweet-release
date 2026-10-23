# Sweet Release

Release independently versioned packages with Markdown change files.
Sweet calculates version bumps, updates version files, and generates changelogs.

Download the CLI from [GitHub Releases](https://github.com/chattocorp/sweet-release/releases).

```sh
sweet init repo
sweet init package
sweet change
sweet status
sweet bump .
```

Review and commit the changes, then tag and publish through your release process.

## A monorepo Sweet Release story

Your monorepo has a Go API in `apps/api` and a JavaScript frontend in `apps/web`.
They share a repository, but each has its own version and release schedule.
From the repository root, initialize Sweet and both existing directories:

```sh
sweet init repo
sweet init package apps/api
sweet init package apps/web
```

Each package gets a `.sweet.yml` manifest and a `.sweet/` directory for Markdown
change notes. Set each manifest's version to its current release version.

You add an API feature and update the frontend to use it. Record a note for each
package, choose a change type, and commit the notes with your code:

```sh
sweet change apps/api
sweet change apps/web
```

When the API is ready to ship, inspect pending releases and prepare just the API:

```sh
sweet status
sweet bump apps/api
```

Sweet calculates the API's version bump from its notes, updates its manifest and
any configured version files, writes its changelog, and removes the consumed notes.
The frontend's notes stay pending until you run `sweet bump apps/web`. To prepare
all packages with pending version bumps together, use `sweet bump --all`.

Review and commit each release, then tag and publish through your release process.
Dependency updates between packages remain part of that process; Sweet does not
propagate releases to dependent packages.

## Tag releases

After you commit a release, preview and create its Git tags:

```sh
sweet git tag --dry-run
sweet git tag
```

Sweet creates annotated local tags for current committed package versions, such as
`apps/api/v1.2.0` (or `v1.2.0` for a root package). Each tag points to the commit
that introduced that version, even if more commits followed it. Matching tags are
kept; conflicting tags cause an error.

Use a checkout with full Git history and fetch remote tags first. Sweet does not
push tags or publish releases; those steps remain part of your release process.

See the [getting started guide](apps/docs/src/content/docs/getting-started.md)
and [documentation](docs/README.md).
