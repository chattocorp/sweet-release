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
	want := "PACKAGE VERSION . 0.1.0 pkg/api 1.2.3 pkg/api/client 2.0.0-beta.1"
	if got := strings.Join(strings.Fields(stdout.String()), " "); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	write(".sweet.yml", "{}\n")
	stdout.Reset()
	if code := run([]string{"status"}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "pkg/api/.sweet.yml") || stdout.Len() != 0 {
		t.Fatalf("invalid manifest: %d, %s, %s", code, &stdout, &stderr)
	}
}
