package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// The shared Go checks must stop at the first failing step.
func TestReleaseChecks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release hooks use Bash")
	}
	_, source, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(source), "../../../../.github/scripts/check-go.sh")
	dir := t.TempDir()
	// Stub go and gofmt: go records its arguments; FAIL_STEP makes a step fail.
	stubs := map[string]string{
		"go":    "#!/bin/sh\necho \"$1\"\n[ \"$FAIL_STEP\" != \"$1\" ]\n",
		"gofmt": "#!/bin/sh\n[ \"$FAIL_STEP\" != gofmt ] || echo apps/sweet/unformatted.go\n",
	}
	for name, content := range stubs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct{ fail, want string }{
		{"", "test\nvet\n"},
		{"gofmt", "Run gofmt on these files:\napps/sweet/unformatted.go\n"},
		{"test", "test\n"},
	} {
		cmd := exec.Command("bash", script)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "FAIL_STEP="+tt.fail)
		out, err := cmd.CombinedOutput()
		if (err != nil) != (tt.fail != "") || string(out) != tt.want {
			t.Errorf("FAIL_STEP=%q: %q, %v; want %q", tt.fail, out, err, tt.want)
		}
	}
}
