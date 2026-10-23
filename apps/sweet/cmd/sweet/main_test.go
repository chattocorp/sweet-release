package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chattocorp/sweet-release/apps/sweet"
)

func TestRun(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, tc := range []struct {
		name        string
		args        []string
		code        int
		output      string
		errorOutput string
	}{
		{name: "no arguments", code: 2, errorOutput: "not a Sweet package"},
		{name: "help", args: []string{"--help"}, output: "Usage:"},
		{name: "short help", args: []string{"-h"}, output: "Usage:"},
		{name: "version", args: []string{"--version"}, output: "sweet " + sweet.Version + "\n"},
		{name: "unknown command", args: []string{"release"}, code: 2, errorOutput: "unknown command"},
		{name: "unknown flag", args: []string{"--unknown"}, code: 2, errorOutput: "unknown flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tc.args, &stdout, &stderr); code != tc.code {
				t.Fatalf("exit code = %d, want %d", code, tc.code)
			}
			if tc.code == 0 {
				if !strings.Contains(stdout.String(), tc.output) || stderr.Len() != 0 {
					t.Fatalf("stdout = %q, stderr = %q", &stdout, &stderr)
				}
			} else if stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.errorOutput) {
				t.Fatalf("stdout = %q, stderr = %q", &stdout, &stderr)
			}
		})
	}
}
