package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrereleaseManifest(t *testing.T) {
	manifest, err := parsePackageManifest([]byte("version: 1.0.0\nprerelease: beta\n"))
	if err != nil || manifest.Prerelease != "beta" {
		t.Fatalf("manifest = %#v, error = %v", manifest, err)
	}
	for _, value := range []string{"''", "1", "'1'", "beta.1", "'be ta'", "[beta]"} {
		if _, err := parsePackageManifest([]byte("version: 1.0.0\nprerelease: " + value + "\n")); err == nil {
			t.Errorf("accepted prerelease %s", value)
		}
	}
}

func TestPrereleaseCycle(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init").CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", output, err)
	}
	write := func(path, data string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bump := func(want string) {
		t.Helper()
		releases, err := Bump(root, nil)
		if err != nil {
			t.Fatal(err)
		}
		if want == "" {
			if len(releases) != 0 {
				t.Fatalf("unexpected release %s", releases[0].NextVersion)
			}
			return
		}
		if len(releases) != 1 || releases[0].NextVersion != want {
			t.Fatalf("releases = %#v, want %s", releases, want)
		}
	}
	write("CHANGELOG.md", "# Changelog\n\n## 1.2.3\n\n### Fixes\n\n- Old fix.\n")
	write(".sweet.yml", "version: 1.2.3\nprerelease: beta\n")
	write(".sweet/a.md", "---\ntype: feature\n---\nFirst feature.\n")
	bump("1.3.0-beta.0")
	bump("") // No notes, same channel.
	write(".sweet/b.md", "---\ntype: fix\n---\nA fix.\n")
	write(".sweet/c.md", "---\ntype: feature\nbreaking: true\n---\nRemove the old API.\n\nUse the new one.\n")
	bump("2.0.0-beta.0")
	write(".sweet.yml", "version: 2.0.0-beta.0\nprerelease: rc\n")
	bump("2.0.0-rc.0") // Switching channels releases without notes.
	write(".sweet.yml", "version: 2.0.0-rc.0\n")
	write(".sweet/d.md", "---\ntype: fix\n---\nLast fix.\n")
	bump("2.0.0")
	data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	final, err := ChangelogRelease(string(data), "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	want := `
### Fixes

- Last fix.
- A fix.

### Features

- ⚠️ **BREAKING CHANGE:** Remove the old API.

  Use the new one.
- First feature.

`
	if final != want {
		t.Fatalf("final entry = %q, want %q", final, want)
	}
	for _, heading := range []string{"## 2.0.0-rc.0", "## 2.0.0-beta.0", "## 1.3.0-beta.0", "## 1.2.3"} {
		if !strings.Contains(string(data), heading+"\n") {
			t.Errorf("changelog lost %s:\n%s", heading, data)
		}
	}
	bump("") // Stable with no notes.
}

func TestPrereleaseHistoryMerge(t *testing.T) {
	changelog := "# Changelog\n\n" +
		"## 1.3.0-beta.1\n\nUnsectioned remark.\n\n### Fixes\n\n- Fix.\n  More detail.\n\n  ```sh\n  - not an item\n  ```\n\n### Custom\n\n- Hand-written.\n" +
		"## 1.3.0-beta.0\n\n### Features\n\n- Feature.\n- ⚠️ **BREAKING CHANGE:** Old breaking.\n" +
		"## 1.2.3 (2026-01-01)\n\n### Fixes\n\n- Stable fix.\n" +
		"## 1.2.3-beta.0\n\n### Fixes\n\n- Too old.\n"
	status := PackageStatus{Package: Package{Version: "1.3.0-beta.1"}, NextVersion: "1.3.0", Changes: []Change{
		{Type: "feature", Body: "New feature."},
		{Type: "fix", Body: "New breaking fix.", Breaking: true},
	}}
	types := []ChangeType{{Name: "fix", Section: "Fixes"}, {Name: "feature", Section: "Features"}}
	got := releaseEntryWithLinks(status, types, nil, prereleaseHistory(status, changelog)...)
	want := "## 1.3.0\n\nUnsectioned remark.\n" +
		"\n### Fixes\n\n- ⚠️ **BREAKING CHANGE:** New breaking fix.\n- Fix.\n  More detail.\n\n  ```sh\n  - not an item\n  ```\n" +
		"\n### Features\n\n- ⚠️ **BREAKING CHANGE:** Old breaking.\n- New feature.\n- Feature.\n" +
		"\n### Custom\n\n- Hand-written.\n"
	if got != want {
		t.Fatalf("entry =\n%s\nwant\n%s", got, want)
	}
	status.NextVersion = "1.3.0-beta.2"
	if history := prereleaseHistory(status, changelog); history != nil {
		t.Fatalf("prerelease bump merged history: %q", history)
	}
}
