package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorWarnsAndCommandsRejectTagCollisions(t *testing.T) {
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
	if code := run([]string{"doctor"}, &out, &stderr); code != 0 || !strings.Contains(out.String(), "Warning:") || !strings.Contains(out.String(), "tag-prefix") {
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
	if code := run([]string{"doctor"}, &out, &stderr); code != 0 || !strings.Contains(out.String(), "No package problems") {
		t.Fatalf("doctor: %d %s %s", code, &out, &stderr)
	}
}
