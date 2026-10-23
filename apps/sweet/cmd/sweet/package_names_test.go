package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageNameSelection(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if out, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	if err := os.MkdirAll("apps/cli", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("apps/cli/.sweet.yml", []byte("name: sweet\ndisplay-name: Sweet Release\nversion: 1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "apps"))
	for _, args := range [][]string{
		{"change", "--package", "sweet", "--type", "feature", "--stdin"},
		{"bump", "--package", "sweet"},
	} {
		cmd := newRootCommand()
		cmd.SetArgs(args)
		cmd.SetIn(strings.NewReader("New feature.\n"))
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v, %s", args, err, &output)
		}
	}
	data, err := os.ReadFile("cli/CHANGELOG.md")
	if err != nil || !strings.Contains(string(data), "# Sweet Release Changelog") || !strings.Contains(string(data), "1.1.0") {
		t.Fatalf("changelog: %s, %v", data, err)
	}
	if err := os.WriteFile("cli/.sweet/broken.md", []byte("Missing frontmatter.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCommand()
	cmd.SetArgs([]string{"change", "--package", "sweet", "--type", "fix", "--stdin"})
	cmd.SetIn(strings.NewReader("Another fix.\n"))
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("existing malformed note blocked change: %v", err)
	}
	notes, err := filepath.Glob("cli/.sweet/*.md")
	if err != nil || len(notes) != 2 {
		t.Fatalf("expected old and new notes: %v, %v", notes, err)
	}
	cmd = newRootCommand()
	cmd.SetArgs([]string{"bump", "--package", "sweet"})
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "broken.md") {
		t.Fatalf("expected bump to reject malformed note: %v", err)
	}
	manifest, err := os.ReadFile("cli/.sweet.yml")
	if err != nil || !strings.Contains(string(manifest), "version: 1.1.0") {
		t.Fatalf("failed bump changed version: %s, %v", manifest, err)
	}
	for _, args := range [][]string{
		{"bump", "--package", "missing"},
		{"bump", "--package", "sweet", "--all"},
		{"bump", "--package", "sweet", "cli"},
		{"change", "--package", "sweet", "cli"},
		{"change", "--package", "", "--stdin", "--type", "fix"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("accepted %v: %s", args, &stdout)
		}
	}
}
