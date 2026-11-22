package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusPackages(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if output, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"status"}, &stdout, &stderr); code != 0 || stdout.String() != "No packages found.\n" {
		t.Fatalf("empty repository: %d, %s, %s", code, &stdout, &stderr)
	}
	write(".sweet.yml", "version: 0.1.0\n")
	write("pkg/api/.sweet.yml", "version: 1.2.3\n")
	write("pkg/api/client/.sweet.yml", "version: 2.0.0-beta.1\n")
	write("pkg/api/.sweet/fix.md", "---\ntype: fix\n---\nA fix.\n")
	write("pkg/api/.sweet/chore.md", "---\ntype: chore\n---\nA chore.\n")
	write("ignored/.sweet.yml", "invalid: [")
	write(".gitignore", "ignored/\n")
	if output, err := exec.Command("git", "add", ".sweet.yml").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	// Tracked packages remain discoverable even if later ignored.
	write(".gitignore", "ignored/\n/.sweet.yml\n")
	t.Chdir(filepath.Join(root, "pkg/api"))
	stdout.Reset()
	if code := run([]string{"status"}, &stdout, &stderr); code != 0 {
		t.Fatalf("status: %d: %s", code, &stderr)
	}
	want := "PACKAGE VERSION BUMP NEXT . 0.1.0 pkg/api 1.2.3 patch 1.2.4 pkg/api/client 2.0.0-beta.1 none 2.0.0"
	if got := strings.Join(strings.Fields(stdout.String()), " "); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	write(filepath.Join(root, ".sweet/missing.md"), "No frontmatter.\n")
	write(".sweet/unknown.md", "---\ntype: typo\n---\n")
	write(".sweet/property.md", "---\ntype: fix\nunexpected: true\n---\n")
	stdout.Reset()
	if code := run([]string{"status"}, &stdout, &stderr); code != 0 {
		t.Fatalf("status with problems: %d: %s", code, &stderr)
	}
	for _, message := range []string{
		".sweet/missing.md: expected YAML frontmatter",
		"pkg/api/.sweet/unknown.md: unknown change type",
		"pkg/api/.sweet/property.md: decode change frontmatter",
		"field unexpected not found",
	} {
		if !strings.Contains(stdout.String(), message) {
			t.Errorf("missing %q in %s", message, &stdout)
		}
	}
	table, _, _ := strings.Cut(stdout.String(), "\nProblems:")
	if got := strings.Join(strings.Fields(table), " "); got != "PACKAGE VERSION BUMP NEXT . 0.1.0 invalid pkg/api 1.2.3 invalid pkg/api/client 2.0.0-beta.1 none 2.0.0" {
		t.Errorf("table with problems = %q", got)
	}
	write(".sweet.yml", "{}\n")
	stdout.Reset()
	if code := run([]string{"status"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "pkg/api/.sweet.yml") || !strings.Contains(stdout.String(), "Problems:") {
		t.Fatalf("invalid manifest: %d, %s, %s", code, &stdout, &stderr)
	}
}

func TestStatusRev(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(".sweet.yml", []byte("version: 1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "init")
	if err := os.WriteFile(".sweet.yml", []byte("version: 2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"status", "--json", "--rev", "HEAD"}, &stdout, &stderr); code != 0 {
		t.Fatalf("status --rev: %d: %s", code, &stderr)
	}
	if want := `[{"path":".","version":"1.0.0","changes":null,"bump":"none","nextVersion":"1.0.0","problems":null}]` + "\n"; stdout.String() != want {
		t.Fatalf("output = %s, want %s", &stdout, want)
	}
	if code := run([]string{"status", "--rev", "missing"}, &stdout, &stderr); code == 0 {
		t.Fatal("status --rev missing succeeded")
	}
}
