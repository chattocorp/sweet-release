package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Use the retained hook outside the checkout, as the publishing workflow does.
func TestRetainedReleaseChecks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release hooks use Bash")
	}
	_, source, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(source), "../../../../.github/scripts/check-go.sh")
	for _, legacy := range []bool{false, true} {
		name := "root"
		patterns := "./..."
		if legacy {
			name = "historical"
			patterns = "./pkg/cli/... ./pkg/core/..."
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			mod := "go.mod"
			if legacy {
				mod = "pkg/cli/go.mod"
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, mod)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, mod), []byte("module example.test/release\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "go"), []byte("#!/bin/sh\necho \"$*\"\n[ -z \"$FAIL_TEST\" ] || [ \"$1\" != test ]\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			selector := exec.Command("bash", script, "version-file")
			selector.Dir = dir
			selected, err := selector.CombinedOutput()
			if err != nil || strings.TrimSpace(string(selected)) != mod {
				t.Fatalf("Go version file: %s, %v; want %s", selected, err, mod)
			}
			cmd := exec.Command("bash", script)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			want := "test " + patterns + "\nvet " + patterns + "\n"
			if err != nil || string(out) != want {
				t.Fatalf("checks: %s, %v; want %s", out, err, want)
			}
			cmd = exec.Command("bash", script)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "FAIL_TEST=1")
			out, err = cmd.CombinedOutput()
			if err == nil || strings.Contains(string(out), "vet ") {
				t.Fatalf("failed tests must stop release checks: %s, %v", out, err)
			}
		})
	}
}
