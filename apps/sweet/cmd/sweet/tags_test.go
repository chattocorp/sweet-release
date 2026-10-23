package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Run the real CLI in subprocesses launched by the workflow script test.
func TestTagPlannerProcess(t *testing.T) {
	if os.Getenv("SWEET_TAG_TEST_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Exit(run(os.Args[i+1:], os.Stdout, os.Stderr))
		}
	}
	os.Exit(99)
}

func TestTagWorkflowWithLocalRemote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("workflow uses Bash on Linux")
	}
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("requires %s", tool)
		}
	}
	_, source, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(source), "../../../../.github/scripts/tag-releases.sh")
	root, remote := t.TempDir(), t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(remote, "init", "--bare")
	git(root, "init", "--initial-branch=main")
	git(root, "config", "user.name", "Test")
	git(root, "config", "user.email", "test@example.com")
	git(root, "remote", "add", "origin", remote)
	write := func(file, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(version string) string {
		t.Helper()
		for _, file := range []string{".sweet.yml", "pkg/api/.sweet.yml", "apps/sweet/.sweet.yml", "apps/docs/.sweet.yml", "pkg/sweet/.sweet.yml", "pkg/cli/.sweet.yml", "pkg/core/.sweet.yml", "pkg/docs/.sweet.yml"} {
			write(filepath.Join(root, file), "version: "+version+"\n")
		}
		git(root, "add", ".")
		git(root, "commit", "-m", "test: version change")
		git(root, "push", "origin", "main")
		return git(root, "rev-parse", "HEAD")
	}
	base := commit("0.0.0")
	head := commit("0.1.0")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "sweet")
	write(wrapper, "#!/bin/sh\nexec \"$SWEET_TEST_BINARY\" -test.run='^TestTagPlannerProcess$' -- \"$@\"\n")
	invoke := func(base, head string, fail bool) {
		t.Helper()
		output := filepath.Join(t.TempDir(), "output")
		cmd := exec.Command("bash", script, wrapper, base, head)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "SWEET_TAG_TEST_PROCESS=1", "SWEET_TEST_BINARY="+bin, "GITHUB_STEP_SUMMARY=", "GITHUB_OUTPUT="+output)
		out, err := cmd.CombinedOutput()
		if (err != nil) != fail {
			t.Fatalf("tag workflow: %v: %s", err, out)
		}
		if !fail {
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var releases []struct{ Tag, Commit string }
			if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(string(data)), "cli-releases=")), &releases); err != nil {
				t.Fatal(err)
			}
			if len(releases) != 3 || !strings.HasPrefix(releases[0].Tag, "apps/sweet/v") || !strings.HasPrefix(releases[1].Tag, "pkg/cli/v") || !strings.HasPrefix(releases[2].Tag, "pkg/sweet/v") || releases[0].Commit != head || releases[1].Commit != head || releases[2].Commit != head {
				t.Fatalf("wrong publish matrix: %s", data)
			}
		}
	}
	invoke(base, head, false)
	for _, name := range []string{"v0.1.0", "pkg/api/v0.1.0"} {
		if got := git(remote, "rev-parse", "refs/tags/"+name+"^{commit}"); got != head {
			t.Fatalf("wrong tag target: %s", got)
		}
		if got := git(remote, "cat-file", "-t", "refs/tags/"+name); got != "tag" {
			t.Fatalf("tag is not annotated: %s", got)
		}
	}
	before := git(remote, "show-ref", "--tags")
	invoke(base, head, false)
	if got := git(remote, "show-ref", "--tags"); got != before {
		t.Fatal("rerun changed tags")
	}
	next := commit("0.2.0")
	git(remote, "tag", "pkg/api/v0.2.0", base)
	before = git(remote, "show-ref", "--tags")
	invoke(head, next, true)
	if got := git(remote, "show-ref", "--tags"); got != before {
		t.Fatal("conflicting tag caused a partial push")
	}
	git(remote, "tag", "-d", "pkg/api/v0.2.0")
	git(root, "tag", "-d", "pkg/api/v0.2.0")
	before = git(remote, "show-ref", "--tags")
	hook := filepath.Join(remote, "hooks/update")
	write(hook, "#!/bin/sh\nif [ \"$1\" = refs/tags/pkg/api/v0.2.0 ]; then exit 1; fi\n")
	invoke(head, next, true)
	if got := git(remote, "show-ref", "--tags"); got != before {
		t.Fatal("remote rejection caused a partial tag push")
	}
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	invoke(head, next, false)
	for _, name := range []string{"v0.2.0", "pkg/api/v0.2.0"} {
		if got := git(remote, "rev-parse", "refs/tags/"+name+"^{commit}"); got != next {
			t.Fatalf("retry did not finish tag push: %s", got)
		}
	}
}

func TestGitTagRequiresCompleteRange(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"git", "tag", "--base", "HEAD"}, &stdout, &stderr); code == 0 || stdout.Len() > 0 || !strings.Contains(stderr.String(), "missing [head]") {
		t.Fatalf("missing range: %d, %s, %s", code, &stdout, &stderr)
	}
}

func TestGitTagDryRunAndCreate(t *testing.T) {
	t.Chdir(t.TempDir())
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	if err := os.WriteFile(".sweet.yml", []byte("version: 1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "Release")
	invoke := func(args ...string) string {
		t.Helper()
		var out, stderr bytes.Buffer
		if code := run(args, &out, &stderr); code != 0 {
			t.Fatalf("%v: %d: %s", args, code, &stderr)
		}
		return out.String()
	}
	if out := invoke("git", "tag", "--dry-run"); !strings.Contains(out, "Would create v1.0.0") {
		t.Fatalf("dry run: %s", out)
	}
	if tags := git("tag", "--list"); tags != "" {
		t.Fatalf("dry run created tags: %s", tags)
	}
	if out := invoke("git", "tag"); !strings.Contains(out, "Created v1.0.0") {
		t.Fatalf("creation: %s", out)
	}
	if out := invoke("git", "tag"); out != "No new release tags.\n" {
		t.Fatalf("rerun: %s", out)
	}
}
