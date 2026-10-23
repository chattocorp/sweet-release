package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangelogCommand(t *testing.T) {
	// Explicit packages need no repository configuration or historical CLI source.
	root := t.TempDir()
	dir := filepath.Join(root, "package with spaces")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".sweet.yml"), []byte("version: 9.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "CHANGELOG.md")
	if err := os.WriteFile(path, []byte("# Log\n\n## 1.0.0\n\nNew\n\n## 0.1.0\nOld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args    []string
		want    string
		success bool
	}{
		{[]string{dir, "--version", "1.0.0"}, "\nNew\n\n", true},
		{[]string{dir, "--version", "0.1.0"}, "Old\n", true},
		{[]string{dir, "--version", "2.0.0"}, "", false},
		{[]string{dir}, "\nNew\n\n", true},
		{[]string{dir + ".missing"}, "", false},
		{[]string{path}, "", false},
		{[]string{dir, dir}, "", false},
	} {
		var out, stderr bytes.Buffer
		code := run(append([]string{"changelog"}, tc.args...), &out, &stderr)
		if (code == 0) != tc.success || out.String() != tc.want {
			t.Fatalf("%v: code %d, out %q, err %s", tc.args, code, out.String(), &stderr)
		}
	}
	t.Chdir(dir)
	var latest, latestErr bytes.Buffer
	if code := run([]string{"changelog"}, &latest, &latestErr); code != 0 || latest.String() != "\nNew\n\n" {
		t.Fatalf("current package: %d, %s, %s", code, &latest, &latestErr)
	}
	t.Chdir(root)
	latest.Reset()
	latestErr.Reset()
	if code := run([]string{"changelog", "package with spaces"}, &latest, &latestErr); code != 0 || latest.String() != "\nNew\n\n" {
		t.Fatalf("relative package: %d, %s, %s", code, &latest, &latestErr)
	}
	latest.Reset()
	latestErr.Reset()
	if code := run([]string{"changelog"}, &latest, &latestErr); code == 0 || !strings.Contains(latestErr.String(), "sweet changelog <package>") {
		t.Fatalf("missing current package: %d, %s", code, &latestErr)
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"release-notes"}, &out, &stderr); code == 0 || !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("old command still accepted: %d, %s, %s", code, &out, &stderr)
	}
}
