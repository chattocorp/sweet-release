package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tagFixture(t *testing.T) (string, func(...string) string, func(string, string), func() string) {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := tagGit(root, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	git("init", "--initial-branch=main")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	write := func(file, text string) {
		t.Helper()
		file = filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func() string {
		t.Helper()
		git("add", ".")
		git("commit", "--allow-empty", "-m", "test: snapshot")
		return git("rev-parse", "HEAD")
	}
	return root, git, write, commit
}

func TestPlanTagsVersionsAndCommits(t *testing.T) {
	root, git, write, commit := tagFixture(t)
	write(".sweet.yml", "version: 0.0.0\n")
	write("pkg/core/.sweet.yml", "version: 0.0.0\n")
	base := commit()
	write(".sweet.yml", "version: 0.1.0\n")
	write("pkg/core/.sweet.yml", "version: 1.0.0\n")
	first := commit()
	write("pkg/core/.sweet.yml", "version: 1.1.0-beta.1\n")
	second := commit()
	write("pkg/core/.sweet/note.md", "a pending note does not release anything")
	head := commit()
	write("pkg/core/.sweet.yml", "version: 9.9.9\n") // Ignore uncommitted files.
	tags, err := PlanTags(root, base, head)
	if err != nil || len(tags) != 3 {
		t.Fatalf("tags: %+v, %v", tags, err)
	}
	for i, want := range []struct{ name, commit string }{{"v0.1.0", first}, {"pkg/core/v1.0.0", first}, {"pkg/core/v1.1.0-beta.1", second}} {
		if tags[i].Name != want.name || tags[i].Commit != want.commit || tags[i].Exists {
			t.Fatalf("tag %d: %+v", i, tags[i])
		}
	}
	if out := git("tag", "--list"); out != "" {
		t.Fatalf("planning created tags: %s", out)
	}
	git("tag", "-a", "v0.1.0", "-m", "release", first)
	git("tag", "pkg/core/v1.0.0", first) // Lightweight existing tags also work.
	tags, err = PlanTags(root, base, head)
	if err != nil || !tags[0].Exists || !tags[1].Exists || tags[2].Exists {
		t.Fatalf("rerun: %+v, %v", tags, err)
	}
	git("tag", "pkg/core/v1.1.0-beta.1", base)
	if _, err := PlanTags(root, base, head); err == nil || !strings.Contains(err.Error(), "refusing to move") {
		t.Fatalf("accepted conflicting tag: %v", err)
	}
}

func TestPlanTagsMergeUsesFirstParent(t *testing.T) {
	root, git, write, commit := tagFixture(t)
	write("pkg/api/.sweet.yml", "version: 0.0.0\n")
	base := commit()
	git("switch", "-c", "release")
	write("pkg/api/.sweet.yml", "version: 1.0.0\n")
	side := commit()
	write("pkg/api/.sweet.yml", "version: 1.1.0\n")
	commit()
	git("switch", "main")
	git("merge", "--no-ff", "release", "-m", "Merge release")
	head := git("rev-parse", "HEAD")
	tags, err := PlanTags(root, base, head)
	if err != nil || len(tags) != 1 || tags[0].Name != "pkg/api/v1.1.0" || tags[0].Commit != head {
		t.Fatalf("merge tags: %+v, %v", tags, err)
	}
	if _, err := PlanTags(root, side, head); err == nil || !strings.Contains(err.Error(), "first-parent") {
		t.Fatalf("accepted base outside first-parent history: %v", err)
	}
}

func TestPlanTagsInitialAndEmpty(t *testing.T) {
	root, _, write, commit := tagFixture(t)
	write(".sweet.yml", "version: 0.0.0\n")
	write("pkg/api/.sweet.yml", "version: 1.0.0\n")
	commit()
	write("pkg/api/.sweet.yml", "version: 2.0.0\n")
	head := commit()
	tags, err := PlanTags(root, strings.Repeat("0", 40), head)
	if err != nil || len(tags) != 1 || tags[0].Name != "pkg/api/v2.0.0" || tags[0].Commit != head {
		t.Fatalf("branch creation should use only its snapshot: %+v, %v", tags, err)
	}
	tags, err = PlanTags(root, head, head)
	if err != nil || tags == nil || len(tags) != 0 {
		t.Fatalf("empty range: %+v, %v", tags, err)
	}
	write("pkg/api/.sweet.yml", "# Changed only a comment\nversion: 2.0.0\n")
	after := commit()
	if tags, err := PlanTags(root, head, after); err != nil || len(tags) != 0 {
		t.Fatalf("comment-only change released: %+v, %v", tags, err)
	}
}

func TestPlanTagsRejectsInvalidReleases(t *testing.T) {
	for _, tc := range []struct{ name, file, body string }{
		{"bad version", "api/.sweet.yml", "version: banana\n"},
		{"bad manifest", "api/.sweet.yml", "version: [\n"},
		{"invalid tag path", "api space/.sweet.yml", "version: 1.0.0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _, write, commit := tagFixture(t)
			base := commit()
			write(tc.file, tc.body)
			head := commit()
			if _, err := PlanTags(root, base, head); err == nil {
				t.Fatal("accepted invalid release")
			}
		})
	}
}

func TestPlanTagsRejectsRepeatedVersion(t *testing.T) {
	root, _, write, commit := tagFixture(t)
	base := commit()
	for _, version := range []string{"1.0.0", "2.0.0", "1.0.0"} {
		write("api/.sweet.yml", "version: "+version+"\n")
		commit()
	}
	if _, err := PlanTags(root, base, "HEAD"); err == nil || !strings.Contains(err.Error(), "multiple commits") {
		t.Fatalf("accepted reused version: %v", err)
	}
}

func TestPlanTagsRejectsSymlinkManifest(t *testing.T) {
	root, _, write, commit := tagFixture(t)
	base := commit()
	write("target", "version: 1.0.0\n")
	if err := os.Symlink("target", filepath.Join(root, ".sweet.yml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	head := commit()
	if _, err := PlanTags(root, base, head); err == nil {
		t.Fatal("accepted symlink manifest")
	}
}

func TestPlanTagsDoesNotAcceptOptionsAsRefs(t *testing.T) {
	root, _, _, commit := tagFixture(t)
	head := commit()
	if _, err := PlanTags(root, "--all", head); err == nil {
		t.Fatal("accepted an option as a base ref")
	}
}
