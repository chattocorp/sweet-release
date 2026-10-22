package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chattocorp/sweet-release/pkg/core"
)

func TestReleasePRPlanCommand(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	t.Chdir(root)
	invoke := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v: %d: %s", args, code, &stderr)
		}
		return stdout.String()
	}
	invoke("init", "repo")
	for _, path := range []string{"-api", "nested/client with spaces"} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		invoke("init", "package", "--", path)
		if err := os.WriteFile(filepath.Join(path, ".sweet/note.md"), []byte("---\ntype: feature\n---\nFeature.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(filepath.Join(root, "nested"))
	var prs []core.ReleasePR
	if err := json.Unmarshal([]byte(invoke("release-pr", "plan")), &prs); err != nil || len(prs) != 2 {
		t.Fatalf("invalid JSON plan: %+v, %v", prs, err)
	}
	if prs[0].Packages[0] != "-api" || prs[1].Packages[0] != "nested/client with spaces" {
		t.Fatalf("paths are not repository-relative: %+v", prs)
	}
	var details []core.ReleasePRDetails
	if err := json.Unmarshal([]byte(invoke("release-pr", "plan", "--details")), &details); err != nil || len(details) != len(prs) {
		t.Fatalf("invalid details: %+v, %v", details, err)
	}
	for i, detail := range details {
		if detail.Branch != prs[i].Branch || len(detail.Releases) != 1 || !strings.Contains(detail.Body, "Feature.") {
			t.Fatalf("wrong details: %+v", detail)
		}
	}
	t.Chdir(root)
	for _, pr := range prs {
		invoke(append([]string{"bump", "--"}, pr.Packages...)...)
	}
	if got := invoke("release-pr", "plan"); got != "[]\n" {
		t.Fatalf("consumed notes still planned: %s", got)
	}
	if err := os.WriteFile(".sweet.config.yml", []byte("release-pr: {strategy: invalid}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"release-pr", "plan"}, &stdout, &stderr); code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "strategy") {
		t.Fatalf("invalid config: %d, %s, %s", code, &stdout, &stderr)
	}
}
