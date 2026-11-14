package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorAndCommandsRejectTagCollisions(t *testing.T) {
	t.Chdir(t.TempDir())
	if out, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for _, dir := range []string{"apps/api", "services/api"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".sweet.yml"), []byte("version: 1.0.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"doctor"}, &out, &stderr); code != 2 || !strings.Contains(stderr.String(), "tag-prefix") {
		t.Fatalf("doctor: %d %s %s", code, &out, &stderr)
	}
	for _, args := range [][]string{{"status"}, {"status", "--json"}, {"bump", "apps/api"}, {"change"}, {"changelog", "apps/api"}, {"git", "tag"}, {"init", "repo"}} {
		out.Reset()
		stderr.Reset()
		if code := run(args, &out, &stderr); code == 0 || !strings.Contains(stderr.String(), "tag prefix") {
			t.Fatalf("%v: %d %s %s", args, code, &out, &stderr)
		}
	}
	if err := os.WriteFile("services/api/.sweet.yml", []byte("version: 1.0.0\ntag-prefix: service-api@\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"doctor"}, &out, &stderr); code != 0 || !strings.Contains(out.String(), "No release problems") {
		t.Fatalf("doctor: %d %s %s", code, &out, &stderr)
	}
}

func TestDoctorValidatesReleasesWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name, manifest, note, want string
	}{
		{name: "clean"},
		{name: "valid note", note: "---\ntype: feature\n---\nNew feature.\n"},
		{name: "unknown type", note: "---\ntype: feat\n---\nNew feature.\n", want: `.sweet/note.md: unknown change type "feat"`},
		{name: "parse error", note: "No frontmatter.\n", want: ".sweet/note.md: expected YAML frontmatter"},
		{name: "invalid metadata", note: "---\ntype: fix\nbreaking: nope\n---\n", want: ".sweet/note.md: breaking must be true or false"},
		{name: "invalid version", manifest: "version: nope\n", want: ".sweet.yml: invalid semantic version"},
		{name: "missing version file", manifest: "version: 1.0.0\nversion-files: [{path: VERSION, format: text}]\n", note: "---\ntype: fix\n---\nFix.\n", want: "VERSION"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if out, err := exec.Command("git", "init").CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			manifest := tc.manifest
			if manifest == "" {
				manifest = "version: 1.0.0\n"
			}
			files := map[string]string{".sweet.yml": manifest}
			if tc.note != "" {
				files[".sweet/note.md"] = tc.note
			}
			for path, data := range files {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, stderr bytes.Buffer
			code := run([]string{"doctor"}, &out, &stderr)
			if tc.want == "" {
				if code != 0 || !strings.Contains(out.String(), "No release problems found.") {
					t.Fatalf("doctor: %d %s %s", code, &out, &stderr)
				}
			} else if code != 2 || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("doctor: %d %s %s; want %q", code, &out, &stderr, tc.want)
			}
			for path, want := range files {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatalf("doctor changed %s: %q, %v", path, got, err)
				}
			}
			if _, err := os.Stat("CHANGELOG.md"); !os.IsNotExist(err) {
				t.Fatalf("doctor created changelog: %v", err)
			}
		})
	}
}
