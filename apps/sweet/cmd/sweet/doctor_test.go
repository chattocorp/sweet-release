package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
)

func TestDoctor(t *testing.T) {
	for _, tt := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{name: "empty"},
		{name: "unnamed", files: map[string]string{".sweet.yml": "version: 1.0.0\n", "nested/.sweet.yml": "version: 0.1.0\n"}},
		{name: "distinct names", files: map[string]string{".sweet.yml": "name: api\nversion: 1.0.0\n", "nested/.sweet.yml": "name: other\nversion: 1.0.0\n"}},
		{name: "duplicates", files: map[string]string{".sweet.yml": "name: api\nversion: 1.0.0\n", "nested/.sweet.yml": "name: api\nversion: 1.0.0\n"}, want: []string{".sweet.yml: duplicate package name", "nested/.sweet.yml: duplicate package name", "in .sweet.yml, nested/.sweet.yml"}},
		{name: "status problems", files: map[string]string{".sweet.yml": "version: 1.0.0\n", ".sweet/bad.md": "bad note", ".sweet/unknown.md": "---\ntype: typo\n---\n", "nested/.sweet.yml": "version: nope\n"}, want: []string{"expected YAML frontmatter", "unknown change type", "invalid semantic version"}},
		{name: "ignored", files: map[string]string{".sweet.yml": "name: api\nversion: 1.0.0\n", "nested/.sweet.yml": "name: api\nversion: 1.0.0\n", ".gitignore": "nested/\n"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			if output, err := exec.Command("git", "init").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v: %s", err, output)
			}
			for path, content := range tt.files {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll("subdir", 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(filepath.Join(root, "subdir"))
			for _, asJSON := range []bool{false, true} {
				args := []string{"doctor"}
				if asJSON {
					args = append(args, "--json")
				}
				var stdout, stderr bytes.Buffer
				code := run(args, &stdout, &stderr)
				wantCode := 0
				if len(tt.want) > 0 {
					wantCode = 2
				}
				if code != wantCode {
					t.Fatalf("%v: code %d, stdout %s, stderr %s", args, code, &stdout, &stderr)
				}
				if asJSON {
					var problems []core.Problem
					if err := json.Unmarshal(stdout.Bytes(), &problems); err != nil {
						t.Fatal(err)
					}
					if problems == nil {
						t.Fatal("expected JSON array, not null")
					}
					if (len(problems) > 0) != (len(tt.want) > 0) {
						t.Fatalf("unexpected problems: %v", problems)
					}
				} else {
					for _, want := range tt.want {
						if !strings.Contains(stdout.String(), want) {
							t.Errorf("missing %q in %s", want, &stdout)
						}
					}
					if wantCode == 0 && stdout.String() != "No problems found.\n" {
						t.Fatalf("unexpected output: %s", &stdout)
					}
				}
			}
			for path, content := range tt.files {
				data, err := os.ReadFile(filepath.Join(root, path))
				if err != nil || string(data) != content {
					t.Errorf("doctor changed %s: %v", path, err)
				}
			}
		})
	}
}

func TestDoctorCommandErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, args := range [][]string{{"doctor"}, {"doctor", "extra"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 || stderr.Len() == 0 {
			t.Fatalf("%v: code %d, stderr %s", args, code, &stderr)
		}
	}
	if output, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.WriteFile(".sweet.config.yml", []byte("types: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"doctor", "--json"}, &stdout, &stderr); code != 2 || stderr.Len() == 0 || stdout.Len() != 0 {
		t.Fatalf("config error: code %d, stdout %s, stderr %s", code, &stdout, &stderr)
	}
}
