---
title: Getting started
description: Install Sweet Release and prepare your first package release.
---

## Install Sweet

Download the archive for your operating system and architecture from
[GitHub Releases](https://github.com/chattocorp/sweet-release/releases).
Extract it and put the `sweet` executable in a directory on your `PATH`.

```sh
sweet --version
```

## Initialize a repository and package

From a Git repository, initialize the repository configuration and the package
in the current directory:

```sh
sweet init repo
sweet init package
```

For a monorepo, initialize each package in an existing directory instead:

```sh
sweet init package pkg/api
sweet init package pkg/web
```

Repository policies live in `.sweet.config.yml`. Each package starts at version
`0.0.0` in `.sweet.yml`, with a `.sweet/` directory for notes. Initialization
preserves existing files. For an existing project, set the manifest version to
the version from which you want to calculate future releases.

## Describe a change

For a package in the current directory:

```sh
sweet change
```

Choose a change type and enter a short description. Leave the description empty
to write a longer note in `$EDITOR`. Running `sweet` without a command starts
the same workflow.

For a package elsewhere, pass its directory: `sweet change pkg/api`.

## Prepare the release

```sh
sweet status
sweet bump .
git diff
```

With the default policies, a `feature` note bumps a new package from `0.0.0` to
`0.1.0`. Sweet updates the manifest, creates `CHANGELOG.md`, and removes consumed
notes. It also updates any configured version files.

Review and commit the changes, then use your project's tagging and publishing
process. See [Prepare a release](../guides/prepare-a-release/) for package
selection and automation.
