package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeGH logs each invocation and keeps the release's draft state in a file.
const fakeGH = `#!/bin/sh
printf '%s\n' "$*" >> "$GH_LOG"
case "$1 $2" in
"release view")
  if [ -f "$GH_STATE" ]; then cat "$GH_STATE"; else echo 'release not found' >&2; exit 1; fi ;;
"release create")
  cat > "$GH_NOTES"; echo true > "$GH_STATE" ;;
"release edit")
  echo false > "$GH_STATE" ;;
"api "*)
  printf 'refs/tags/v0.9.0\nrefs/tags/v1.0.0\n' ;;
esac
`

func TestGitHubReleaseCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh is a shell script")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	if out, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for file, text := range map[string]string{
		".sweet.yml":   "name: App\nversion: 1.0.0\n",
		"CHANGELOG.md": "## 1.0.0\n\nNew\n",
	} {
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeGH), 0o755); err != nil {
		t.Fatal(err)
	}
	log, state, notes := filepath.Join(dir, "log"), filepath.Join(bin, "state"), filepath.Join(bin, "notes")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_LOG", log)
	t.Setenv("GH_STATE", state)
	t.Setenv("GH_NOTES", notes)
	invoke := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"github", "release"}, args...), &stdout, &stderr); code != 0 {
			t.Fatalf("%v: %d: %s", args, code, &stderr)
		}
		return stdout.String()
	}
	calls := func() string {
		t.Helper()
		data, _ := os.ReadFile(log)
		os.Remove(log)
		return string(data)
	}

	out := invoke("v1.0.0", "a.tgz", "b.zip")
	if out != "Created draft release v1.0.0\nUploaded 2 assets\nPublished v1.0.0 (prerelease: false, latest: true)\n" {
		t.Fatalf("output: %q", out)
	}
	want := "release view v1.0.0 --json isDraft --jq .isDraft\n" +
		"release create v1.0.0 --verify-tag --draft --latest=false --title App 1.0.0 --notes-file -\n" +
		"release upload v1.0.0 --clobber -- a.tgz b.zip\n" +
		"api --paginate repos/{owner}/{repo}/git/matching-refs/tags/v --jq .[].ref\n" +
		"release edit v1.0.0 --draft=false --prerelease=false --latest=true\n"
	if got := calls(); got != want {
		t.Fatalf("gh calls:\n%s\nwant:\n%s", got, want)
	}
	if data, _ := os.ReadFile(notes); string(data) != "\nNew\n" {
		t.Fatalf("notes: %q", data)
	}

	// Rerun after publication: only the state lookup, nothing changes.
	if out := invoke("v1.0.0", "a.tgz"); !strings.HasPrefix(out, "Left published release v1.0.0 unchanged") {
		t.Fatalf("rerun: %q", out)
	}
	if got := calls(); got != "release view v1.0.0 --json isDraft --jq .isDraft\n" {
		t.Fatalf("rerun calls: %s", got)
	}
	// --replace-published: no new release, assets replaced, publication reapplied.
	if out := invoke("v1.0.0", "--replace-published", "a.tgz"); !strings.HasPrefix(out, "Reused published release v1.0.0\nUploaded 1 assets\nPublished") {
		t.Fatalf("replace: %q", out)
	}
	if got := calls(); strings.Contains(got, "release create") || !strings.Contains(got, "release upload v1.0.0 --clobber -- a.tgz\n") || !strings.Contains(got, "release edit") {
		t.Fatalf("replace calls: %s", got)
	}

	// A draft run against an existing draft only uploads; --latest=false skips the tag lookup.
	if err := os.WriteFile(state, []byte("true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := invoke("v1.0.0", "--draft", "c.txt"); out != "Reused draft release v1.0.0\nUploaded 1 assets\n" {
		t.Fatalf("draft: %q", out)
	}
	if got := calls(); strings.Contains(got, "release edit") {
		t.Fatalf("draft published: %s", got)
	}
	invoke("v1.0.0", "--latest=false")
	if got := calls(); strings.Contains(got, "api ") || !strings.Contains(got, "--latest=false\n") {
		t.Fatalf("explicit latest: %s", got)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"github", "release", "nope@1.0.0"}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "does not belong to any package") {
		t.Fatalf("unknown tag: %d %s", code, &stderr)
	}
}
