package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextVersion(t *testing.T) {
	for _, tt := range []struct{ version, bump, want string }{
		{"1.2.3", "none", "1.2.3"},
		{"1.2.3", "patch", "1.2.4"},
		{"1.2.3", "minor", "1.3.0"},
		{"1.2.3", "major", "2.0.0"},
		{"0.0.0", "minor", "0.1.0"},
		{"1.2.3-beta.1+build", "patch", "1.2.4"},
		{"1.2.3-beta.1+build", "none", "1.2.3-beta.1+build"},
		{"99999999999999999999.0.0", "major", "100000000000000000000.0.0"},
	} {
		got, err := nextVersion(tt.version, tt.bump)
		if err != nil || got != tt.want {
			t.Errorf("nextVersion(%q, %q) = %q, %v; want %q", tt.version, tt.bump, got, err, tt.want)
		}
	}
	for _, version := range []string{"v1.2.3", "1.2", "01.2.3", "1.2.3-01", "1.2.3-", "1.2.3+", "foo"} {
		if _, err := nextVersion(version, "none"); err == nil {
			t.Errorf("accepted %q", version)
		}
	}
}

func TestParseChange(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		body := newline + "# Note" + newline + "---" + newline
		change, err := parseChange([]byte(strings.Join([]string{"---", "type: fix", "breaking: true", "---", ""}, newline) + body))
		if err != nil || change.Type != "fix" || !change.Breaking || change.Body != body {
			t.Fatalf("change = %#v, error = %v", change, err)
		}
	}
	for _, data := range []string{
		"", "plain text", "---\ntype: fix", "---\n---\n",
		"---\nbreaking: true\n---\n", "---\ntype: fix\nunknown: true\n---\n",
		"---\ntype: [fix]\n---\n", "---\ntype: fix\nbreaking: perhaps\n---\n",
		"---\ntype: fix\ntype: chore\n---\n",
		"---\ntype: fix\nbreaking: null\n---\n",
		"---\ntype: fix\nbreaking: yes\n---\n",
		"---\ntype: 123\n---\n",
	} {
		if _, err := parseChange([]byte(data)); err == nil {
			t.Errorf("accepted %q", data)
		}
	}
}

func TestRepositoryStatus(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, output)
		}
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
	note := func(typ string) string { return "---\ntype: " + typ + "\n---\n\nBody.\n" }
	git("init")
	write(".sweet.config.yml", "types:\n  fix: {bump: patch}\n  feature: {bump: minor}\n  chore: {bump: none}\n  breaking: {bump: major}\n")
	for _, path := range []string{".", "api", "api/client", "empty"} {
		write(filepath.Join(path, ".sweet.yml"), "version: 0.2.3\n")
	}
	write(".sweet/chore.md", note("chore"))
	write("api/.sweet/a.md", note("fix"))
	write("api/.sweet/b.md", note("feature"))
	write("api/client/.sweet/break.md", "---\ntype: chore\nbreaking: true\n---\n")
	write("api/.sweet/deleted.md", note("breaking"))
	git("add", ".")
	if err := os.Remove(filepath.Join(root, "api/.sweet/deleted.md")); err != nil {
		t.Fatal(err)
	}
	write(".gitignore", "ignored.md\na.md\n")
	write("api/.sweet/ignored.md", "invalid")
	write("api/.sweet/nested/skip.md", "invalid")
	write("api/.sweet/readme.txt", "invalid")
	write("api/.sweet/c.md", note("chore"))
	check := func(wantBumps, wantVersions []string, counts []int) {
		t.Helper()
		statuses, err := RepositoryStatus(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(statuses) != 4 {
			t.Fatalf("statuses = %#v", statuses)
		}
		for i, s := range statuses {
			if s.Bump != wantBumps[i] || s.NextVersion != wantVersions[i] || len(s.Changes) != counts[i] {
				t.Errorf("status = %#v", s)
			}
		}
		if statuses[1].Changes[0].Path != "api/.sweet/a.md" || statuses[1].Changes[0].Body != "\nBody.\n" {
			t.Errorf("change = %#v", statuses[1].Changes[0])
		}
	}
	check([]string{"none", "minor", "minor", "none"}, []string{"0.2.3", "0.3.0", "0.3.0", "0.2.3"}, []int{1, 3, 1, 0})
	write("api/client/.sweet.yml", "version: 1.2.3\n")
	write("api/.sweet/d.md", note("breaking"))
	check([]string{"none", "major", "major", "none"}, []string{"0.2.3", "1.0.0", "2.0.0", "0.2.3"}, []int{1, 4, 1, 0})
	for _, content := range []string{note("unknown"), "invalid"} {
		write("api/.sweet/bad.md", content)
		statuses, err := RepositoryStatus(root)
		if err != nil {
			t.Fatal(err)
		}
		if s := statuses[1]; len(s.Problems) != 1 || s.Problems[0].Path != "api/.sweet/bad.md" || s.Bump != "" || s.NextVersion != "" {
			t.Fatalf("status = %#v", s)
		}
	}
}

func TestRepositoryStatusManifestInventory(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, output)
		}
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
	git("init")
	write("tracked/.sweet.yml", "version: 1.2.3\n")
	write("deleted/.sweet.yml", "version: 1.2.3\n")
	git("add", ".")
	if err := os.Remove(filepath.Join(root, "deleted/.sweet.yml")); err != nil {
		t.Fatal(err)
	}
	// Ignore tracked manifests as well: Git must still include them.
	write(".gitignore", ".sweet.yml\n")
	write("ignored/.sweet.yml", "version: invalid\n")
	// An untracked package that is not ignored must still be included.
	write("fresh/.gitignore", "!.sweet.yml\n")
	write("fresh/.sweet.yml", "version: 2.0.0\n")
	write("broken/.gitignore", "!.sweet.yml\n")
	write("broken/.sweet.yml", "version: 1.2.3-01\n")
	statuses, err := RepositoryStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 3 || statuses[0].Path != "broken" || statuses[1].Path != "fresh" || statuses[2].Path != "tracked" {
		t.Fatalf("unexpected packages: %#v", statuses)
	}
	broken := statuses[0]
	if broken.Bump != "" || broken.NextVersion != "" || len(broken.Problems) != 1 || broken.Problems[0].Path != "broken/.sweet.yml" || !strings.Contains(broken.Problems[0].Message, "invalid semantic version") {
		t.Fatalf("invalid package status: %#v", broken)
	}
	for _, status := range statuses[1:] {
		if len(status.Problems) != 0 || status.Bump != "none" || status.NextVersion != status.Version {
			t.Fatalf("invalid manifest affected another package: %#v", status)
		}
	}
}
