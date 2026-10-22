package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBumpWorkflow(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if output, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", output, err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	invoke := func(want int, args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != want {
			t.Fatalf("%v: code %d, stdout %s, stderr %s", args, code, &stdout, &stderr)
		}
		return stdout.String() + stderr.String()
	}
	note := func(typ, body string) string { return "---\ntype: " + typ + "\n---\n\n" + body + "\n" }
	write(".sweet.config.yml", "types:\n  feature: {bump: minor, section: Added}\n  fix: {bump: patch, section: Fixes}\n  docs: {bump: none, section: Added}\n  chore: {bump: none}\n")
	write("api/.sweet.yml", "# Package metadata\nversion: 1.2.3 # Current version\ncustom: retained\n")
	write("api/.sweet/a.md", note("feature", "New API.\n\n```go\ncall()\n```"))
	write("api/.sweet/b.md", note("docs", "Updated guide."))
	write("api/.sweet/c.md", note("chore", "Hidden housekeeping."))
	write("api/.sweet/keep.txt", "Keep this file.")
	write("other/.sweet.yml", "version: 0.1.0\n")
	write("other/.sweet/a.md", note("chore", "Not a release."))
	write("broken/.sweet.yml", "version: 1.0.0\n")
	write("broken/.sweet/a.md", "invalid")
	invoke(2, "bump", "--all")
	if strings.Contains(read("api/.sweet.yml"), "1.3.0") {
		t.Fatal("modified valid package despite invalid selection")
	}
	invoke(2, "bump", "api", "--all")
	invoke(2, "bump")
	invoke(0, "bump", "api") // An unrelated invalid package does not block this release.
	manifest := read("api/.sweet.yml")
	for _, want := range []string{"version: 1.3.0", "# Package metadata", "# Current version", "custom: retained"} {
		if !strings.Contains(manifest, want) {
			t.Errorf("missing %q in %s", want, manifest)
		}
	}
	changelog := read("api/CHANGELOG.md")
	want := "# Changelog\n\n## 1.3.0\n\n### Added\n\nNew API.\n\n```go\ncall()\n```\n\nUpdated guide.\n"
	if changelog != want {
		t.Fatalf("changelog = %q, want %q", changelog, want)
	}
	for _, name := range []string{"a", "b", "c"} {
		if _, err := os.Stat("api/.sweet/" + name + ".md"); !os.IsNotExist(err) {
			t.Fatalf("note %s remains: %v", name, err)
		}
	}
	read("api/.sweet/keep.txt")
	read("other/.sweet/a.md")
	if out := invoke(0, "bump", "api"); !strings.Contains(out, "No packages need a release") {
		t.Fatal(out)
	}
	if read("api/CHANGELOG.md") != changelog {
		t.Fatal("repeat bump changed changelog")
	}
	write("api/.sweet/fix.md", note("fix", "Fixed API."))
	t.Chdir(filepath.Join(root, "api"))
	invoke(0, "bump")
	if got := read("CHANGELOG.md"); !strings.HasSuffix(got, strings.TrimPrefix(changelog, "# Changelog\n\n")) || !strings.Contains(got, "## 1.3.1\n\n### Fixes") {
		t.Fatal(got)
	}
	t.Chdir(root)
	write("broken/.sweet/a.md", note("fix", "Fixed."))
	invoke(0, "bump", "--all")
	read("other/.sweet/a.md")
	if read("other/.sweet.yml") != "version: 0.1.0\n" {
		t.Fatal("bumped notes with no version effect")
	}
}
