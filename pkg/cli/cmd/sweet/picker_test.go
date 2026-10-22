package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chattocorp/sweet-release/pkg/core"
)

func TestPickPackage(t *testing.T) {
	for _, tc := range []struct{ name, script, want, err string }{
		{"selection", "cat > \"$CANDIDATES\"\nprintf 'pkg/core\\0'", "pkg/core", ""},
		{"spaces", "printf 'pkg/a space\\0'", "pkg/a space", ""},
		{"cancel", "exit 130", "", "selection cancelled"},
		{"no selection", "exit 1", "", "selection cancelled"},
		{"failure", "exit 2", "", "select package with fzf"},
		{"unexpected path", "printf '../outside\\0'", "", "did not return an available package"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			fzf := filepath.Join(root, "fzf")
			candidates := filepath.Join(root, "candidates")
			t.Setenv("CANDIDATES", candidates)
			if err := os.WriteFile(fzf, []byte("#!/bin/sh\n"+tc.script+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			packages := []core.Package{{Path: "pkg/cli"}, {Path: "pkg/core"}, {Path: "pkg/a space"}}
			got, err := pickPackage(fzf, root, packages, io.Discard)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("error = %v, want %s", err, tc.err)
				}
				return
			}
			if err != nil || got != filepath.Join(root, tc.want) {
				t.Fatalf("path = %q, error = %v", got, err)
			}
			if tc.name == "selection" {
				data, err := os.ReadFile(candidates)
				if err != nil || string(data) != "pkg/cli\x00pkg/core\x00pkg/a space\x00" {
					t.Fatalf("candidates = %q, error = %v", data, err)
				}
			}
		})
	}
}
