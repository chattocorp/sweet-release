package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisplayNamesDoNotIdentifyPackages(t *testing.T) {
	root, _, write, _ := tagFixture(t)
	for _, path := range []string{"a", "b"} {
		write(path+"/.sweet.yml", "version: 1.0.0\nname: Shared Name\n")
		write(path+"/.sweet/note.md", "---\ntype: feature\n---\nNew feature.\n")
	}
	if releases, err := Bump(root, nil); err != nil || len(releases) != 2 {
		t.Fatalf("releases: %v, %v", releases, err)
	}
	for _, path := range []string{"a", "b"} {
		content, err := os.ReadFile(filepath.Join(root, path, "CHANGELOG.md"))
		if err != nil || !strings.HasPrefix(string(content), "# Shared Name Changelog\n") {
			t.Fatalf("%s, %v", content, err)
		}
	}
}

func TestChangelogReleaseAfterBumpWithListFences(t *testing.T) {
	for _, marker := range []string{"", "- ", "* ", "+ ", "1. ", "1) ", "- 1. "} {
		for _, fence := range []string{"```", "~~~"} {
			t.Run(marker+fence, func(t *testing.T) {
				root, _, write, _ := tagFixture(t)
				write(".sweet.yml", "version: 1.0.0\n")
				write("CHANGELOG.md", "# Log\n\n## 1.0.0\n\nPrevious release.\n\n## 0.9.0\n\nOldest release.\n")
				indent := strings.Repeat(" ", len(marker))
				body := marker + fence + "md\n" + indent + "## Example\n" + indent + fence
				write(".sweet/note.md", "---\ntype: feature\n---\n"+body+"\n")
				if _, err := Bump(root, nil); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
				if err != nil {
					t.Fatal(err)
				}
				want := "\n### Features\n\n- " + strings.ReplaceAll(body, "\n", "\n  ") + "\n\n"
				for version, expected := range map[string]string{
					"1.1.0": want,
					"1.0.0": "\nPrevious release.\n\n",
					"0.9.0": "\nOldest release.\n",
				} {
					got, err := ChangelogRelease(string(data), version)
					if err != nil || got != expected {
						t.Errorf("release %s: got %q, %v; want %q", version, got, err, expected)
					}
				}
			})
		}
	}
}

func TestChangelogRelease(t *testing.T) {
	for _, tc := range []struct {
		name, data, version, want string
		fail                      bool
	}{
		{"history", "# Log\n\n## 2.0.0\n\nNew\n\n## 1.0.0\nOld\n", "2.0.0", "\nNew\n\n", false},
		{"older", "## 2.0.0\nNew\n## 1.0.0\nOld", "1.0.0", "Old", false},
		{"CRLF", "## 1.0.0\r\n\r\nNote\r\n", "1.0.0", "\r\nNote\r\n", false},
		{"fences", "```md\n## 1.0.0\n```\n## 1.0.0\n~~~md\n## example\n~~~\nBody\n## 0.1.0\nOld", "1.0.0", "~~~md\n## example\n~~~\nBody\n", false},
		{"exact", "## 1.0.0-beta.1\nNote\n", "1.0.0", "", true},
		{"missing", "# Empty\n", "1.0.0", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ChangelogRelease(tc.data, tc.version)
			if (err != nil) != tc.fail || got != tc.want {
				t.Fatalf("got %q, %v; want %q, failure %v", got, err, tc.want, tc.fail)
			}
		})
	}
}

func TestPackageChangelog(t *testing.T) {
	for _, tc := range []struct {
		name, manifest, changelog, version, want string
		fail                                     bool
	}{
		{"latest skips unreleased and fences", "version: 9.0.0", "# Log\n```\n## 8.0.0\n```\n## Unreleased\nPending\n## 1.0.0-beta.1\nNewest\n## 2.0.0\nOlder", "", "Newest\n", false},
		{"explicit older", "version: 9.0.0", "## 2.0.0\nNew\n## 1.0.0\nOld", "1.0.0", "Old", false},
		{"empty", "version: 0.0.0", "# Log", "", "", true},
		{"missing version", "version: 1.0.0", "## 1.0.0\nBody", "2.0.0", "", true},
		{"unreleased only", "version: 0.0.0", "## Unreleased\nPending", "", "", true},
		{"bad package", "version: nope", "## 1.0.0\nBody", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for file, content := range map[string]string{".sweet.yml": tc.manifest, "CHANGELOG.md": tc.changelog} {
				if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := PackageChangelog(dir, tc.version)
			if (err != nil) != tc.fail || got != tc.want {
				t.Fatalf("got %q, %v; want %q, failure %v", got, err, tc.want, tc.fail)
			}
		})
	}
	dir := t.TempDir()
	if _, err := PackageChangelog(dir, ""); err == nil {
		t.Fatal("accepted missing package manifest")
	}
	if err := os.WriteFile(filepath.Join(dir, ".sweet.yml"), []byte("version: 0.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PackageChangelog(dir, ""); err == nil {
		t.Fatal("accepted missing changelog")
	}
}
