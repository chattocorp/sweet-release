package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
)

func TestPrereleaseWorkflow(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	git := func(args ...string) {
		t.Helper()
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, output, err)
		}
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	invoke := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v: code %d, stdout %s, stderr %s", args, code, &stdout, &stderr)
		}
		return stdout.String()
	}
	status := func() core.PackageStatus {
		t.Helper()
		var statuses []core.PackageStatus
		if err := json.Unmarshal([]byte(invoke("status", "--json")), &statuses); err != nil || len(statuses) != 1 {
			t.Fatalf("status: %v, %v", statuses, err)
		}
		return statuses[0]
	}
	commit := func(message string) {
		t.Helper()
		git("add", "-A")
		git("commit", "-m", message)
	}
	git("init")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	write("api/package.json", `{"version": "1.2.3"}`+"\n")
	write("api/.sweet.yml", "version: 1.2.3\nprerelease: beta\nversion-files:\n  - {path: package.json, format: json, key: version}\n")
	write("api/.sweet/a.md", "---\ntype: feature\n---\nNew API.\n")
	commit("Start beta")

	if got := status(); got.Prerelease != "beta" || got.NextVersion != "1.3.0-beta.0" {
		t.Fatalf("status = %+v", got)
	}
	if out := invoke("bump", "--all"); !strings.Contains(out, "1.2.3 -> 1.3.0-beta.0") {
		t.Fatal(out)
	}
	if manifest := read("api/.sweet.yml"); !strings.Contains(manifest, "version: 1.3.0-beta.0") || !strings.Contains(manifest, "prerelease: beta") {
		t.Fatalf("manifest = %s", manifest)
	}
	if got := read("api/package.json"); !strings.Contains(got, `"1.3.0-beta.0"`) {
		t.Fatalf("package.json = %s", got)
	}
	commit("Release beta")
	if out := invoke("git", "tag"); !strings.Contains(out, "api@1.3.0-beta.0") {
		t.Fatal(out)
	}
	if out := invoke("bump", "--all"); !strings.Contains(out, "No packages need a release") {
		t.Fatal(out)
	}

	// Removing the label finalizes without notes.
	write("api/.sweet.yml", strings.Replace(read("api/.sweet.yml"), "prerelease: beta\n", "", 1))
	commit("Finalize")
	got := status()
	if got.Bump != "none" || got.NextVersion != "1.3.0" {
		t.Fatalf("status = %+v", got)
	}
	if table := strings.Join(strings.Fields(invoke("status")), " "); table != "PACKAGE VERSION BUMP NEXT api 1.3.0-beta.0 none 1.3.0" {
		t.Fatal(table)
	}
	if picker := bumpPicker([]core.PackageStatus{got}); !reflect.DeepEqual(picker.options, []string{"api"}) {
		t.Fatalf("picker options = %v", picker.options)
	}
	if out := invoke("bump", "api"); !strings.Contains(out, "1.3.0-beta.0 -> 1.3.0 (0 notes consumed)") {
		t.Fatal(out)
	}
	if got := read("api/package.json"); !strings.Contains(got, `"1.3.0"`) {
		t.Fatalf("package.json = %s", got)
	}
	if notes := invoke("changelog", "--version", "1.3.0", "--", "api"); !strings.Contains(notes, "### Features\n\n- New API.") {
		t.Fatalf("final notes = %q", notes)
	}
	commit("Release final")
	if out := invoke("git", "tag", "--dry-run"); !strings.Contains(out, "Would create api@1.3.0 ") {
		t.Fatal(out)
	}
}
