package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

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
		name    string
		desired []map[string]string
		want    []string
		fail    bool
	}{
		{"per-package", []map[string]string{{"branch": api}}, []string{patch("1"), deleteRef("sweet/release"), deleteRef(merged)}, false},
		{"combined", []map[string]string{{"branch": "sweet/release"}}, []string{patch("2"), deleteRef(api), deleteRef(merged)}, false},
		{"no-releases", []map[string]string{}, []string{patch("1"), patch("2"), deleteRef("sweet/release"), deleteRef(api), deleteRef(merged)}, false},
		{"API-failure", []map[string]string{}, nil, true},
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
				"RELEASE_PRS="+string(data), "FAIL_READ="+fail)
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
