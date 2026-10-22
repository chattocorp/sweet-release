package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitLocations(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		file string
	}{
		{"repository root from nested directory", []string{"init", "repo"}, ".sweet.config.yml"},
		{"current directory", []string{"init", "package"}, "nested/.sweet.yml"},
		{"relative directory", []string{"init", "package", ".."}, ".sweet.yml"},
		{"absolute directory", nil, ".sweet.yml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if output, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
				t.Fatalf("git init: %v: %s", err, output)
			}
			nested := filepath.Join(root, "nested")
			if err := os.Mkdir(nested, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(nested)
			args := tc.args
			if args == nil {
				args = []string{"init", "package", root}
			}
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit code = %d: %s", code, &stderr)
			}
			path := filepath.Join(root, tc.file)
			if args[1] == "package" {
				info, err := os.Stat(filepath.Join(filepath.Dir(path), ".sweet"))
				if err != nil || !info.IsDir() {
					t.Fatalf("release notes directory: %v", err)
				}
			}
			if content, err := os.ReadFile(path); err != nil || len(content) == 0 {
				t.Fatalf("config content = %q, error = %v", content, err)
			}
			// A second invocation must preserve user configuration.
			const existing = "custom: value\n"
			if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
				t.Fatal(err)
			}
			if code := run(args, &stdout, &stderr); code == 0 {
				t.Fatal("expected existing config to cause an error")
			}
			if content, err := os.ReadFile(path); err != nil || string(content) != existing {
				t.Fatalf("existing config changed: %q, error = %v", content, err)
			}
		})
	}
}

func TestInitInvalidTargets(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, args := range [][]string{
		{"init", "repo"},
		{"init", "repo", "extra"},
		{"init", "package", "missing"},
		{"init", "package", "one", "two"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code == 0 || stderr.Len() == 0 {
			t.Fatalf("args = %v: exit code = %d, stderr = %q", args, code, &stderr)
		}
	}
}
