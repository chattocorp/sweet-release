package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestReleasePRWorkflowMatrix(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("workflow requires python3")
	}
	_, source, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(source), "../../../../.github/scripts/release-pr.py")
	paths := []string{".", "root", "pkg/api", "pkg-api", "PKG/API", "with spaces", "-flag", "日本語", strings.Repeat("long/", 80)}
	var inventory []map[string]string
	for _, path := range paths {
		inventory = append(inventory, map[string]string{"path": path})
	}
	invoke := func(packages []map[string]string, strategy string) ([]byte, error) {
		data, _ := json.Marshal(packages)
		cmd := exec.Command("python3", script, "matrix", strategy)
		cmd.Stdin = bytes.NewReader(data)
		return cmd.CombinedOutput()
	}
	data, err := invoke(inventory, "per-package")
	if err != nil {
		t.Fatalf("matrix: %v: %s", err, data)
	}
	var jobs []struct{ Branch string }
	if err := json.Unmarshal(data, &jobs); err != nil || len(jobs) != len(paths) {
		t.Fatalf("jobs: %s, %v", data, err)
	}
	seen := map[string]bool{}
	for _, job := range jobs {
		if seen[job.Branch] || len(job.Branch) > 100 || !strings.HasPrefix(job.Branch, "sweet/release-package-") {
			t.Fatalf("invalid or colliding branch: %s", job.Branch)
		}
		seen[job.Branch] = true
		if err := exec.Command("git", "check-ref-format", "--branch", job.Branch).Run(); err != nil {
			t.Fatal(err)
		}
	}
	// Preserve the previous planner's branch convention so existing PRs are reused.
	if jobs[0].Branch != "sweet/release-package-root-cdb4ee2aea69cc6a" {
		t.Fatalf("root branch changed: %s", jobs[0].Branch)
	}
	if out, err := invoke(nil, "per-package"); err != nil || string(out) != "[]\n" {
		t.Fatalf("empty inventory: %s, %v", out, err)
	}
	if _, err := invoke(inventory, "invalid"); err == nil {
		t.Fatal("accepted invalid workflow strategy")
	}
	oversized := make([]map[string]string, 257)
	for i := range oversized {
		oversized[i] = inventory[0]
	}
	if out, err := invoke(oversized, "per-package"); err == nil || !strings.Contains(string(out), "at most 256") {
		t.Fatalf("oversized matrix: %s, %v", out, err)
	}
}

// releasePRHarness runs release-pr.py against a Git repository with this test
// binary standing in for the sweet CLI.
type releasePRHarness struct {
	t                       *testing.T
	root, temporary, script string
	sweet                   string
	env                     []string
}

func newReleasePRHarness(t *testing.T) *releasePRHarness {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("workflow uses a shell wrapper on Linux")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("workflow requires python3")
	}
	_, source, _, _ := runtime.Caller(0)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	h := &releasePRHarness{t: t, root: t.TempDir(), temporary: t.TempDir(),
		script: filepath.Join(filepath.Dir(source), "../../../../.github/scripts/release-pr.py"),
		env:    append(os.Environ(), "SWEET_TAG_TEST_PROCESS=1", "SWEET_TEST_BINARY="+binary)}
	h.sweet = filepath.Join(h.temporary, "sweet")
	h.write(h.sweet, "#!/bin/sh\nexec \"$SWEET_TEST_BINARY\" -test.run='^TestTagPlannerProcess$' -- \"$@\"\n")
	h.git("init")
	h.git("config", "user.name", "Test")
	h.git("config", "user.email", "test@example.com")
	return h
}

func (h *releasePRHarness) invoke(input string, name string, args ...string) ([]byte, error) {
	h.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = h.root, h.env, strings.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		h.t.Logf("%s %v: %s", name, args, &stderr)
	}
	return out, err
}

func (h *releasePRHarness) git(args ...string) {
	h.t.Helper()
	if _, err := h.invoke("", "git", args...); err != nil {
		h.t.Fatal(err)
	}
}

func (h *releasePRHarness) write(path, content string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		h.t.Fatal(err)
	}
}

func TestReleasePRPreparation(t *testing.T) {
	for _, tc := range []struct {
		name, strategy, selected string
		invalid, changed         bool
	}{
		{"root", "per-package", ".", false, true},
		{"spaces", "per-package", "nested/client with spaces", false, true},
		{"leading-dash", "per-package", "-flag", false, true},
		{"idle", "per-package", "idle", false, false},
		{"combined", "combined", "", false, true},
		{"invalid", "per-package", ".", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newReleasePRHarness(t)
			root, temporary, script, sweet := h.root, h.temporary, h.script, h.sweet
			invoke, git, write := h.invoke, h.git, h.write
			for _, path := range []string{".", "nested/client with spaces", "-flag", "idle"} {
				write(filepath.Join(root, path, ".sweet.yml"), "version: 0.0.0\n")
				if path == "." {
					write(filepath.Join(root, path, ".sweet.yml"), "version: 0.0.0\nname: Sweet Release\n")
				}
				if path != "idle" {
					kind := "feature"
					if tc.invalid && path == "." {
						kind = "unknown"
					}
					write(filepath.Join(root, path, ".sweet/note.md"), "---\ntype: "+kind+"\n---\nNew functionality.\n")
				}
			}
			git("add", ".")
			git("commit", "-m", "Initial packages")
			inventory, err := invoke("", sweet, "status", "--json")
			if err != nil {
				t.Fatal(err)
			}
			data, err := invoke(string(inventory), "python3", script, "matrix", tc.strategy)
			if err != nil {
				t.Fatal(err)
			}
			var jobs []struct {
				Branch, Title string
				Packages      []string
			}
			if err := json.Unmarshal(data, &jobs); err != nil {
				t.Fatal(err)
			}
			for _, job := range jobs {
				if tc.strategy != "combined" && job.Packages[0] != tc.selected {
					continue
				}
				jobData, _ := json.Marshal(map[string]any{"branch": job.Branch, "title": job.Title, "packages": job.Packages})
				destination := filepath.Join(temporary, "result")
				out, err := invoke("", "python3", script, "prepare", sweet, string(jobData), destination)
				if tc.invalid {
					if err == nil {
						t.Fatal("invalid note did not fail preparation")
					}
					if _, err := os.Stat(destination); !os.IsNotExist(err) {
						t.Fatal("failed job wrote cleanup results")
					}
					return
				}
				if err != nil || (strings.TrimSpace(string(out)) == "true") != tc.changed {
					t.Fatalf("prepare: %s, %v", out, err)
				}
				data, err := os.ReadFile(filepath.Join(destination, "desired.json"))
				if err != nil {
					t.Fatal(err)
				}
				var desired []map[string]string
				if err := json.Unmarshal(data, &desired); err != nil || (len(desired) == 1) != tc.changed {
					t.Fatalf("cleanup result: %s, %v", data, err)
				}
				if tc.changed {
					wantTitle := "chore: prepare releases"
					if tc.strategy == "per-package" {
						name := tc.selected
						if name == "." {
							name = "Sweet Release"
						}
						wantTitle = "chore: release " + name + " 0.1.0"
					}
					title, err := os.ReadFile(filepath.Join(destination, "title.txt"))
					if err != nil || string(title) != wantTitle {
						t.Fatalf("release title: %s, want %s, %v", title, wantTitle, err)
					}
					body, err := os.ReadFile(filepath.Join(destination, "body.md"))
					if err != nil || !strings.Contains(string(body), "New functionality.") || !strings.Contains(string(body), "0.1.0") {
						t.Fatalf("release body: %s, %v", body, err)
					}
					for _, path := range job.Packages {
						if path == "idle" {
							continue
						}
						name := path
						if path == "." {
							name = "Sweet Release"
						}
						if !strings.Contains(string(body), "\n## "+name+" 0.1.0\n") {
							t.Fatalf("missing package and version heading: %s", body)
						}
					}
					if strings.Contains(string(body), "Target version:") {
						t.Fatalf("unexpected target version line: %s", body)
					}
					git("add", ".")
					git("commit", "-m", "Prepared release")
					out, err = invoke("", "python3", script, "prepare", sweet, string(jobData), destination)
					if err != nil || strings.TrimSpace(string(out)) != "false" {
						t.Fatalf("consumed notes produced another PR: %s, %v", out, err)
					}
				}
				return
			}
			t.Fatal("missing package job")
		})
	}
}

// Exercise the actual cleanup script with a local GitHub transport. No network
// calls or repository mutations are made by these tests.
func TestReleasePRCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the workflow runs in Bash on Linux")
	}
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("workflow test requires %s", tool)
		}
	}
	_, source, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(source), "../../../../.github/scripts/cleanup-release-prs.sh")
	const repo = "example/repo"
	const api = "sweet/release-package-api-0123456789abcdef"
	const merged = "sweet/release-package-merged-0123456789abcdef"
	const human = "sweet/release-package-human-0123456789abcdef"
	const protected = "sweet/release-package-protected-0123456789abcdef"
	const otherBase = "sweet/release-package-other-0123456789abcdef"
	pr := func(number int, branch, state, author, headRepo, base string) map[string]any {
		return map[string]any{
			"number": number, "state": state,
			"user": map[string]string{"login": author},
			"head": map[string]any{"ref": branch, "repo": map[string]string{"full_name": headRepo}},
			"base": map[string]string{"ref": base},
		}
	}
	pulls := []any{
		pr(1, "sweet/release", "open", "github-actions[bot]", repo, "main"),
		pr(2, api, "open", "github-actions[bot]", repo, "main"),
		pr(3, merged, "closed", "github-actions[bot]", repo, "main"),
		pr(4, human, "open", "human", repo, "main"),
		pr(5, "sweet/release", "open", "github-actions[bot]", "fork/repo", "main"),
		pr(6, "feature/user-work", "open", "github-actions[bot]", repo, "main"),
		pr(7, protected, "closed", "github-actions[bot]", repo, "main"),
		pr(8, protected, "open", "human", repo, "main"),
		pr(9, otherBase, "open", "github-actions[bot]", repo, "develop"),
	}
	patch := func(n string) string { return "api --method PATCH repos/" + repo + "/pulls/" + n + " -f state=closed" }
	deleteRef := func(branch string) string { return "api --method DELETE repos/" + repo + "/git/refs/heads/" + branch }
	for _, tc := range []struct {
		name         string
		desired      []map[string]string
		want         []string
		fail         bool
		author, base string
	}{
		{"per-package", []map[string]string{{"branch": api}}, []string{patch("1"), deleteRef("sweet/release"), deleteRef(merged)}, false, "", ""},
		{"combined", []map[string]string{{"branch": "sweet/release"}}, []string{patch("2"), deleteRef(api), deleteRef(merged)}, false, "", ""},
		{"no-releases", []map[string]string{}, []string{patch("1"), patch("2"), deleteRef("sweet/release"), deleteRef(api), deleteRef(merged)}, false, "", ""},
		{"API-failure", []map[string]string{}, nil, true, "", ""},
		{"personal-token", []map[string]string{}, []string{patch("1"), patch("2"), deleteRef("sweet/release"), deleteRef(api), deleteRef(merged)}, false, "release-maintainer", "stable/next"},
		{"app-token", []map[string]string{}, []string{patch("1"), patch("2"), deleteRef("sweet/release"), deleteRef(api), deleteRef(merged)}, false, "release-app[bot]", "stable/next"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name string, data []byte, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), data, mode); err != nil {
					t.Fatal(err)
				}
			}
			// Two pages verify that candidates are collected across pagination.
			data, _ := json.Marshal([]any{pulls[:3], pulls[3:]})
			if tc.author != "" {
				data = bytes.ReplaceAll(data, []byte(`"github-actions[bot]"`), []byte(`"`+tc.author+`"`))
				data = bytes.ReplaceAll(data, []byte(`"main"`), []byte(`"`+tc.base+`"`))
			}
			write("pulls.json", data, 0o644)
			var refs []map[string]string
			for _, branch := range []string{"sweet/release", api, merged, human, protected, otherBase} {
				refs = append(refs, map[string]string{"ref": "refs/heads/" + branch})
			}
			data, _ = json.Marshal([]any{refs})
			write("refs.json", data, 0o644)
			write("gh", []byte(`#!/usr/bin/env bash
set -euo pipefail
case "$*" in
  'api --paginate --slurp '*'/pulls?'*)
    if [[ "$FAIL_READ" == true ]]; then exit 1; fi
    cat "$FIXTURE/pulls.json" ;;
  'api --paginate --slurp '*'/git/matching-refs/'*) cat "$FIXTURE/refs.json" ;;
  'api --method PATCH '*|'api --method DELETE '*) printf '%s\n' "$*" >> "$FIXTURE/mutations" ;;
  *) printf 'Unexpected gh call: %s\n' "$*" >&2; exit 1 ;;
esac
`), 0o755)
			data, _ = json.Marshal(tc.desired)
			fail := "false"
			if tc.fail {
				fail = "true"
			}
			cmd := exec.Command("bash", script)
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"FIXTURE="+dir, "RUNNER_TEMP="+dir, "GITHUB_REPOSITORY="+repo,
				"RELEASE_PRS="+string(data), "FAIL_READ="+fail,
				"RELEASE_PR_AUTHOR="+tc.author, "BASE_BRANCH="+tc.base)
			out, err := cmd.CombinedOutput()
			if (err != nil) != tc.fail {
				t.Fatalf("cleanup: %v: %s", err, out)
			}
			data, err = os.ReadFile(filepath.Join(dir, "mutations"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			var got []string
			if len(data) > 0 {
				got = strings.Split(strings.TrimSpace(string(data)), "\n")
			}
			slices.Sort(got)
			slices.Sort(tc.want)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("mutations:\n%v\nwant:\n%v", got, tc.want)
			}
		})
	}
}

func TestReleasePRPrerelease(t *testing.T) {
	h := newReleasePRHarness(t)
	manifest := filepath.Join(h.root, "api/.sweet.yml")
	h.write(filepath.Join(h.root, "api/.sweet/a.md"), "---\ntype: feature\n---\nBeta feature.\n")
	h.write(manifest, "version: 1.2.3\nprerelease: beta\n")
	h.git("add", ".")
	h.git("commit", "-m", "Start beta")
	prepare := func(wantTitle string, wantBody ...string) {
		t.Helper()
		job := `{"branch":"sweet/release-package-api","title":"chore: release api","packages":["api"]}`
		destination := filepath.Join(h.temporary, "result")
		out, err := h.invoke("", "python3", h.script, "prepare", h.sweet, job, destination)
		if err != nil {
			t.Fatal(err)
		}
		if wantTitle == "" {
			if strings.TrimSpace(string(out)) != "false" {
				t.Fatalf("unexpected PR: %s", out)
			}
			return
		}
		title, _ := os.ReadFile(filepath.Join(destination, "title.txt"))
		body, _ := os.ReadFile(filepath.Join(destination, "body.md"))
		if strings.TrimSpace(string(out)) != "true" || string(title) != wantTitle {
			t.Fatalf("prepare = %s, title = %q, want %q", out, title, wantTitle)
		}
		for _, want := range wantBody {
			if !strings.Contains(string(body), want) {
				t.Fatalf("body missing %q:\n%s", want, body)
			}
		}
		h.git("add", "-A")
		h.git("commit", "-m", wantTitle)
	}
	prepare("chore: release api 1.3.0-beta.0", "## api 1.3.0-beta.0", "- Beta feature.")
	prepare("") // No new notes for the current channel.
	h.write(filepath.Join(h.root, "api/.sweet/b.md"), "---\ntype: fix\n---\nBeta fix.\n")
	h.git("add", "-A")
	h.git("commit", "-m", "Add fix")
	prepare("chore: release api 1.3.0-beta.1", "- Beta fix.")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	h.write(manifest, strings.Replace(string(data), "prerelease: beta\n", "", 1))
	h.git("commit", "-am", "Finalize")
	prepare("chore: release api 1.3.0", "## api 1.3.0", "- Beta feature.", "- Beta fix.")
	prepare("")
}

// Results arrive flat for one artifact and in subdirectories for several.
func TestCollectReleaseResults(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the workflow runs in Bash on Linux")
	}
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("workflow test requires %s", tool)
		}
	}
	_, source, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(source), "../../../../.github/scripts/collect-release-results.sh")
	write := func(file, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	single := t.TempDir()
	write(filepath.Join(single, "desired.json"), `[{"branch":"a"}]`)
	multiple := t.TempDir()
	write(filepath.Join(multiple, "result-0", "desired.json"), `[{"branch":"a"}]`)
	write(filepath.Join(multiple, "result-1", "desired.json"), `[]`)
	write(filepath.Join(multiple, "result-2", "desired.json"), `[{"branch":"c"}]`)
	for _, tc := range []struct {
		name, dir, expected, want string
	}{
		{"single artifact", single, "1", `[{"branch":"a"}]`},
		{"several artifacts", multiple, "3", `[{"branch":"a"},{"branch":"c"}]`},
		{"no results", t.TempDir(), "1", ""},
		{"missing directory", filepath.Join(single, "missing"), "1", ""},
		{"missing result", multiple, "4", ""},
		{"zero expected", t.TempDir(), "0", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := exec.Command("bash", script, tc.dir, tc.expected).Output()
			if (err != nil) != (tc.want == "") || strings.TrimSpace(string(out)) != tc.want {
				t.Fatalf("collect: %v: %q", err, out)
			}
		})
	}
}
