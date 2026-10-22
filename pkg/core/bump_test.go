package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInsertRelease(t *testing.T) {
	entry := "## 1.1.0\n\n### Fixes\n\nA fix.\n"
	for _, prefix := range []string{"# Changelog\n\n", "# History\n\nIntroduction.\n\n", "# History\n\n```md\n## Example\n```\n\n"} {
		old := "## 1.0.0\n\nExisting notes.\n"
		if got := insertRelease(prefix+old, entry); got != prefix+entry+"\n"+old {
			t.Fatalf("%q", got)
		}
	}
	if got := insertRelease("Introduction.\n", entry); got != "Introduction.\n\n"+entry {
		t.Fatal(got)
	}
}

func TestReleaseEntryBreakingAndSharedSections(t *testing.T) {
	status := PackageStatus{NextVersion: "2.0.0", Changes: []Change{
		{Type: "fix", Body: "A fix."},
		{Type: "feature", Body: "A new API.", Breaking: true},
		{Type: "hidden", Body: "Hidden."},
	}}
	got := releaseEntry(status, []ChangeType{{Name: "feature", Section: "Changes"}, {Name: "fix", Section: "Changes"}})
	if strings.Count(got, "### Changes") != 1 || !strings.Contains(got, "**Breaking change.**\n\nA new API.") || strings.Contains(got, "Hidden") {
		t.Fatal(got)
	}
}

func TestBumpPreflightsAllFiles(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init").CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", output, err)
	}
	for _, name := range []string{"a", "b"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Join(dir, ".sweet"), 0o755); err != nil {
			t.Fatal(err)
		}
		for path, data := range map[string]string{".sweet.yml": "version: 1.0.0\n", ".sweet/fix.md": "---\ntype: fix\n---\nFix.\n"} {
			if err := os.WriteFile(filepath.Join(dir, path), []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.Mkdir(filepath.Join(root, "b/CHANGELOG.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Bump(root, nil); err == nil {
		t.Fatal("accepted directory as changelog")
	}
	data, err := os.ReadFile(filepath.Join(root, "a/.sweet.yml"))
	if err != nil || string(data) != "version: 1.0.0\n" {
		t.Fatalf("manifest changed: %s, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, "a/.sweet/fix.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "a/CHANGELOG.md")); !os.IsNotExist(err) {
		t.Fatalf("changelog created: %v", err)
	}
}
