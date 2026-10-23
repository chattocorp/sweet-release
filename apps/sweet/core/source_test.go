package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryURL(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		remote bool
		want   string
	}{
		{"git@github.com:owner/repo.git", true, "https://github.com/owner/repo"},
		{"ssh://git@github.com/owner/repo.git", true, "https://github.com/owner/repo"},
		{"https://token@github.com/owner/repo.git", true, "https://github.com/owner/repo"},
		{"https://github.com/owner/repo/", true, "https://github.com/owner/repo"},
		{"https://git.example.com/owner/repo", true, ""},
		{"https://git.example.com/owner/repo", false, "https://git.example.com/owner/repo"},
		{"https://token@github.com/owner/repo", false, ""},
		{"https://github.com/owner/repo?q=secret", true, ""},
		{"https://github.com/owner/repo#fragment", true, ""},
		{"https://github.com/owner/repo%29", true, ""},
		{"file:///owner/repo", true, ""},
		{"https://github.com/owner", true, ""},
	} {
		if got := repositoryURL(tc.raw, tc.remote); got != tc.want {
			t.Errorf("repositoryURL(%q, %v) = %q, want %q", tc.raw, tc.remote, got, tc.want)
		}
	}
	if _, err := parseRepositoryConfig([]byte("repository: javascript:bad\n")); err == nil {
		t.Fatal("accepted invalid repository")
	}
	config, err := parseRepositoryConfig([]byte("repository: https://git.example.com/owner/repo.git\n"))
	if err != nil || config.Repository != "https://git.example.com/owner/repo" {
		t.Fatalf("%+v, %v", config, err)
	}
}

func sourceTestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, output, err)
	}
	return strings.TrimSpace(string(output))
}

func sourceTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	sourceTestGit(t, root, "init", "-b", "main")
	sourceTestGit(t, root, "config", "user.name", "Test")
	sourceTestGit(t, root, "config", "user.email", "test@example.com")
	sourceTestGit(t, root, "config", "commit.gpgsign", "false")
	sourceTestGit(t, root, "commit", "--allow-empty", "-m", "Initial")
	sourceTestGit(t, root, "remote", "add", "origin", "git@github.com:owner/repo.git")
	return root
}

func sourceTestWrite(t *testing.T, root, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSourceHistory(t *testing.T) {
	for _, tc := range []struct{ name, subject, merge, pr string }{
		{"squash", "feat: add API (#42)", "", "42"},
		{"merge", "feat: add API (#99)", "Merge pull request #43 from owner/feature", "43"},
		{"plain", "feat: add API", "", ""},
		{"issue", "fix: closes #44", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := sourceTestRepo(t)
			if tc.merge != "" {
				sourceTestGit(t, root, "checkout", "-b", "feature")
			}
			path := ".sweet/note.md"
			sourceTestWrite(t, root, path, "Note")
			sourceTestGit(t, root, "add", ".")
			sourceTestGit(t, root, "commit", "-m", tc.subject)
			if tc.merge != "" {
				sourceTestGit(t, root, "checkout", "main")
				sourceTestGit(t, root, "merge", "--no-ff", "feature", "-m", tc.merge)
			}
			sha := sourceTestGit(t, root, "rev-parse", "HEAD")
			// Later editorial changes must not replace the introducing commit.
			sourceTestWrite(t, root, path, "Edited note")
			sourceTestGit(t, root, "add", ".")
			sourceTestGit(t, root, "commit", "-m", "Edit note (#98)")
			repo := sourceRepository(root, RepositoryConfig{})
			want := "[" + sha[:7] + "](" + repo + "/commit/" + sha + ")"
			if tc.pr != "" {
				want = "[#" + tc.pr + "](" + repo + "/pull/" + tc.pr + ")"
			}
			if got := sourceLink(root, repo, path); got != want {
				t.Fatalf("got %q, want %q; log: %q", got, want, sourceGit(root, "log", "-1", "--first-parent", "--diff-merges=first-parent", "--diff-filter=A", "--no-renames", "--format=%H%n%s", "HEAD", "--", path))
			}
			if got := sourceLink(root, repo, ".sweet/untracked.md"); got != "" {
				t.Fatal(got)
			}
			// A shallow boundary must not be mistaken for an addition.
			clone := filepath.Join(t.TempDir(), "clone")
			sourceTestGit(t, root, "clone", "--depth=1", "file://"+root, clone)
			if got := sourceLink(clone, repo, path); got != "" {
				t.Fatal(got)
			}
			sourceTestGit(t, root, "rm", path)
			sourceTestGit(t, root, "commit", "-m", "Consume note")
			sourceTestWrite(t, root, path, "New untracked note with reused name")
			if got := sourceLink(root, repo, path); got != "" {
				t.Fatal(got)
			}
		})
	}
}

func TestBumpSourceLinks(t *testing.T) {
	root := sourceTestRepo(t)
	sourceTestWrite(t, root, ".sweet.yml", "version: 1.0.0\n")
	sourceTestWrite(t, root, ".sweet/note.md", "---\ntype: feature\n---\nAdd API.\n")
	sourceTestGit(t, root, "add", ".")
	sourceTestGit(t, root, "commit", "-m", "feat: add API (#42)")
	if _, err := Bump(root, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- Add API. ([#42](https://github.com/owner/repo/pull/42))") {
		t.Fatal(string(data))
	}
}

func TestSourceLinkMultiline(t *testing.T) {
	status := PackageStatus{NextVersion: "1.1.0", Changes: []Change{{Path: "note", Type: "feature", Body: "```go\nCall()\n```"}}}
	got := releaseEntryWithLinks(status, []ChangeType{{Name: "feature", Section: "Features"}}, map[string]string{"note": "[#42](https://github.com/owner/repo/pull/42)"})
	want := "- ```go\n  Call()\n  ```\n\n  ([#42](https://github.com/owner/repo/pull/42))\n"
	if !strings.Contains(got, want) {
		t.Fatal(got)
	}
}

func TestSourceLinkWrappedParagraph(t *testing.T) {
	const link = "[#42](https://github.com/owner/repo/pull/42)"
	for _, breaking := range []bool{false, true} {
		status := PackageStatus{NextVersion: "1.1.0", Changes: []Change{{
			Path: "note", Type: "feature", Breaking: breaking,
			Body: "\n  Show help when running `sweet` without a command.\nUse `sweet change` to write release notes. \t\n\n",
		}}}
		got := releaseEntryWithLinks(status, []ChangeType{{Name: "feature", Section: "Features"}}, map[string]string{"note": link})
		prefix := "- "
		if breaking {
			prefix += "⚠️ **BREAKING CHANGE:** "
		}
		want := "## 1.1.0\n\n### Features\n\n" + prefix + "Show help when running `sweet` without a command.\n  Use `sweet change` to write release notes. (" + link + ")\n"
		if got != want {
			t.Fatalf("breaking=%v: got %q, want %q", breaking, got, want)
		}
	}
}

func TestSourceLinkWrappedInlineCode(t *testing.T) {
	for _, body := range []string{
		"Create missing annotated local tags. Use\n`--dry-run` to preview, `--json` for automation, and `--base` with `--head` to\nselect an explicit commit range. Tags are never pushed or overwritten.",
		"The Go module is now\n`github.com/chattocorp/sweet-release/apps/sweet`; core imports use its `/core`\nsubpackage.",
		"Read the latest released changes, or use\n`--version <version>` to select a release.",
		"Show the syntax with\n``a `backtick` in code``.",
	} {
		for _, breaking := range []bool{false, true} {
			status := PackageStatus{NextVersion: "1.1.0", Changes: []Change{{Path: "note", Type: "feature", Body: body, Breaking: breaking}}}
			got := releaseEntryWithLinks(status, []ChangeType{{Name: "feature", Section: "Features"}}, map[string]string{"note": "source"})
			prefix := "- "
			if breaking {
				prefix += "⚠️ **BREAKING CHANGE:** "
			}
			want := "## 1.1.0\n\n### Features\n\n" + prefix + strings.ReplaceAll(body, "\n", "\n  ") + " (source)\n"
			if got != want {
				t.Fatalf("body %q, breaking=%v: got %q, want %q", body, breaking, got, want)
			}
		}
	}
}

func TestSourceLinkStructuredMarkdown(t *testing.T) {
	for _, body := range []string{
		"First paragraph.\n\nSecond paragraph.",
		"Example:\n```go\nCall()\n```",
		"Example:\n````go\nCall()\n````",
		"Example:\n~~~go\nCall()\n~~~",
		"Items:\n- First\n- Second",
		"Heading\n=======",
		"Example:\n\n    Call()",
		"| Column |\n| --- |\n| Value |",
	} {
		status := PackageStatus{NextVersion: "1.1.0", Changes: []Change{{Path: "note", Type: "feature", Body: body}}}
		got := releaseEntryWithLinks(status, []ChangeType{{Name: "feature", Section: "Features"}}, map[string]string{"note": "source"})
		if !strings.HasSuffix(got, "\n\n  (source)\n") {
			t.Fatalf("body %q: got %q", body, got)
		}
	}
}

func TestSourceUnicodePath(t *testing.T) {
	root := sourceTestRepo(t)
	path := "paquet-é/.sweet/note.md"
	sourceTestWrite(t, root, path, "Note")
	sourceTestGit(t, root, "add", ".")
	sourceTestGit(t, root, "commit", "-m", "feat: add note (#45)")
	if got := sourceLink(root, "https://github.com/owner/repo", path); got != "[#45](https://github.com/owner/repo/pull/45)" {
		t.Fatal(got)
	}
}
