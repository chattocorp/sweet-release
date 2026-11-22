package core

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeStatus(t *testing.T) {
	root := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	write := func(path, content string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(root, "init", "-q")
	write(".sweet.config.yml", "types:\n  fix: {bump: patch}\n  feature: {bump: minor}\n  docs: {bump: none}\n")
	write(".sweet.yml", "version: 1.2.3\n")
	write(".sweet/fix.md", "---\ntype: fix\n---\nA fix.\n")
	write("pkg/zero/.sweet.yml", "version: 0.3.0\nname: Zero\n")
	write("pkg/zero/.sweet/breaking.md", "---\ntype: fix\nbreaking: true\n---\nBreaks.\n")
	write("pkg/beta/.sweet.yml", "version: 1.3.0-beta.0\nprerelease: beta\ntag-prefix: beta-v\n")
	write("pkg/beta/.sweet/feature.md", "---\ntype: feature\n---\nNew.\n")
	write("pkg/beta/nested/.sweet.yml", "version: 2.0.0\n")
	write("pkg/beta/nested/.sweet/bad.md", "---\ntype: typo\n---\n")
	write("shared.md", "---\ntype: feature\n---\nLinked.\n")
	if err := os.Symlink("../../../shared.md", filepath.Join(root, "pkg/zero/.sweet/linked.md")); err != nil {
		t.Fatal(err)
	}
	git(root, "add", ".")
	git(root, "commit", "-q", "-m", "plan")

	plan := func(statuses []PackageStatus, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(statuses)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	checkout := filepath.Join(t.TempDir(), "checkout")
	git(root, "clone", "-q", root, checkout)
	want := plan(RepositoryStatus(checkout))
	for _, fragment := range []string{`"nextVersion":"1.2.4"`, `"nextVersion":"0.4.0"`, `"nextVersion":"1.3.0-beta.1"`, `unknown change type`, `"path":"pkg/zero/.sweet/linked.md"`} {
		if !strings.Contains(want, fragment) {
			t.Fatalf("checkout plan lacks %s: %s", fragment, want)
		}
	}

	// Working tree, index, and untracked changes must not affect the plan.
	write(".sweet.yml", "version: 9.0.0\n")
	write("pkg/zero/.sweet/untracked.md", "---\ntype: feature\n---\n")
	write("pkg/beta/.sweet/staged.md", "---\ntype: fix\n---\n")
	git(root, "add", "pkg/beta/.sweet/staged.md")
	if err := os.Remove(filepath.Join(root, ".sweet/fix.md")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "pkg/beta"))
	if got := plan(RepositoryStatus(root)); got == want {
		t.Fatal("working tree changes did not affect the working plan")
	}
	for _, rev := range []string{"HEAD", git(root, "rev-parse", "HEAD"), git(root, "rev-parse", "HEAD^{tree}")} {
		if got := plan(TreeStatus(rev)); got != want {
			t.Errorf("TreeStatus(%q) = %s\nwant %s", rev, got, want)
		}
	}

	// No work tree is needed.
	bare := filepath.Join(t.TempDir(), "bare.git")
	git(root, "clone", "-q", "--bare", root, bare)
	t.Chdir(t.TempDir())
	t.Setenv("GIT_DIR", bare)
	if got := plan(TreeStatus("HEAD")); got != want {
		t.Errorf("bare TreeStatus = %s\nwant %s", got, want)
	}

	for _, rev := range []string{"", "--output=x", "missing"} {
		if _, err := TreeStatus(rev); err == nil {
			t.Errorf("TreeStatus(%q) succeeded", rev)
		}
	}
}
