package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseNotesCommand(t *testing.T) {
	// No repository, configuration, or historical CLI source is required.
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(path, []byte("# Log\n\n## 1.0.0\n\nNew\n\n## 0.1.0\nOld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args    []string
		want    string
		success bool
	}{
		{[]string{path, "--version", "1.0.0"}, "\nNew\n\n", true},
		{[]string{path, "--version", "0.1.0"}, "Old\n", true},
		{[]string{path, "--version", "2.0.0"}, "", false},
		{[]string{path}, "", false},
		{[]string{path + ".missing", "--version", "1.0.0"}, "", false},
	} {
		var out, stderr bytes.Buffer
		code := run(append([]string{"release-notes"}, tc.args...), &out, &stderr)
		if (code == 0) != tc.success || out.String() != tc.want {
			t.Fatalf("%v: code %d, out %q, err %s", tc.args, code, out.String(), &stderr)
		}
	}
}
