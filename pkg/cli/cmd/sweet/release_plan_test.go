package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/chattocorp/sweet-release/pkg/core"
)

func TestReleasePlanCommand(t *testing.T) {
	t.Chdir(t.TempDir())
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	write(".sweet.yml", "version: 1.0.0\n")
	git("add", ".")
	git("commit", "-qm", "base")
	write(".sweet.yml", "version: 1.1.0\n")
	write("CHANGELOG.md", "## 1.1.0\n\nNew behavior.\n")
	git("add", ".")
	git("commit", "-qm", "release")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"release-plan", ".", "--tag-prefix", "api/v"}, &stdout, &stderr); code != 0 {
		t.Fatalf("%d: %s", code, &stderr)
	}
	var releases []core.PlannedRelease
	if err := json.Unmarshal(stdout.Bytes(), &releases); err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 || releases[0].Tag != "api/v1.1.0" || releases[0].Commit != git("rev-parse", "HEAD") || releases[0].Notes != "New behavior." {
		t.Fatalf("%+v", releases)
	}
	if git("status", "--porcelain") != "" || git("tag", "--list") != "" {
		t.Fatal("release-plan changed repository state")
	}
	stdout.Reset()
	if code := run([]string{"release-plan", ".", "--from", "HEAD"}, &stdout, &stderr); code != 0 || stdout.String() != "[]\n" {
		t.Fatalf("empty plan: %d %s %s", code, &stdout, &stderr)
	}
}
