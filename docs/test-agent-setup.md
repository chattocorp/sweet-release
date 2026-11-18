# Test the repository setup prompt

Use a fresh repository and a fresh agent session for each trial. The fixture is
a Node package at version 1.2.3 with an existing tag format. It has no remote.

## Create the fixture

```sh
repo=$(mktemp -d)
cd "$repo"
git init -b main
printf '%s\n' '{"name":"widget","version":"1.2.3"}' > package.json
git add package.json
git -c user.name=Fixture -c user.email=fixture@example.org commit -m "Release widget 1.2.3"
git tag widget-v1.2.3
```

Open the fixture in the agent. To evaluate an unpublished README, give the
agent the candidate README as a read-only attachment. For a discovery test
after publication, omit the attachment. Use this prompt in both cases:

> Set this repo up for chattocorp/sweet-release.

Record the README revision, agent/model, any follow-up questions, and the final
diff. Inspect the diff for a root package manifest, a reproducible way to
install Sweet, and release-note instructions for future agents. Setup should
preserve the published version, changelog, and tags.

## Check the outcome

Run these checks inside the disposable fixture with Sweet on your PATH.
The last commands exercise the configured release process locally.

```sh
set -eu
sweet doctor
sweet status --json | python3 -c '
import json, sys
packages = json.load(sys.stdin)
assert [(p["path"], p["version"], p.get("tagPrefix")) for p in packages] == [
    (".", "1.2.3", "widget-v")
], packages
assert not packages[0]["changes"], packages
'
test "$(git tag --list)" = widget-v1.2.3
printf '%s\n' 'Fix a startup crash.' | sweet change . --type fix --stdin
sweet doctor
sweet bump .
python3 -c 'import json; assert json.load(open("package.json"))["version"] == "1.2.4"'
grep -F 'Fix a startup crash.' CHANGELOG.md
```

A passing run shows that the agent preserved the version and tag prefix and
configured Sweet to update the package on its next release. To assess GitHub
automation, repeat in a disposable GitHub repository. Check that Doctor runs on
a pull request, Bump opens a release PR, and Tag creates `widget-v1.2.4` after
that PR merges. The local fixture covers CLI behavior only.
