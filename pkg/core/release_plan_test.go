package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleasePlanCommittedHistory(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, content string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) string {
		git("add", ".")
		git("commit", "-qm", message)
		return git("rev-parse", "HEAD")
	}
	git("init", "-b", "main")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	write("README.md", "Initial\n")
	initial := commit("initial")
	write("pkg/api/.sweet.yml", "version: 0.0.0\n")
	base := commit("initialize")
	plan, err := ReleasePlan(root, "pkg/api", initial, base, "v")
	if err != nil || len(plan) != 0 {
		t.Fatalf("initialization: %+v, %v", plan, err)
	}
	write("pkg/api/.sweet.yml", "version: 0.1.0\n")
	write("pkg/api/CHANGELOG.md", "# Changelog\n\n## 0.1.0\n\nFirst release.\n")
	first := commit("bump first")
	write("README.md", "Unrelated\n")
	commit("unrelated")
	write("pkg/api/.sweet.yml", "version: 0.2.0\n")
	write("pkg/api/CHANGELOG.md", "# Changelog\n\n## 0.2.0\n\nSecond release.\n\n## 0.1.0\n\nFirst release.\n")
	second := commit("bump second")
	write("README.md", "More unrelated\n")
	head := commit("after release")
	// Dirty working files must not affect committed release plans.
	write("pkg/api/.sweet.yml", "version: 99.0.0\n")
	write("pkg/api/CHANGELOG.md", "uncommitted")
	plan, err = ReleasePlan(root, "pkg/api", base, head, "pkg/api/v")
	if err != nil || len(plan) != 2 {
		t.Fatalf("plan: %+v, %v", plan, err)
	}
	if plan[0].Commit != first || plan[1].Commit != second || plan[1].Notes != "Second release." || plan[1].Tag != "pkg/api/v0.2.0" || plan[0].PreviousVersion != "0.0.0" {
		t.Fatalf("plan: %+v", plan)
	}
	retry, err := ReleasePlan(root, "pkg/api", second+"^", second, "v")
	if err != nil || len(retry) != 1 || retry[0].Commit != second {
		t.Fatalf("retry: %+v, %v", retry, err)
	}
	for _, pair := range [][2]string{{head, base}, {"--help", head}} {
		if _, err := ReleasePlan(root, "pkg/api", pair[0], pair[1], "v"); err == nil {
			t.Fatal("accepted invalid range", pair)
		}
	}
	if _, err := ReleasePlan(root, "../outside", base, head, "v"); err == nil {
		t.Fatal("accepted outside path")
	}
	if _, err := ReleasePlan(root, "pkg/api", base, head, "bad prefix"); err == nil {
		t.Fatal("accepted invalid tag")
	}
	git("restore", "pkg/api")
	// Merge history releases at the merge commit, not at its second-parent bump.
	git("checkout", "-b", "feature")
	write("pkg/api/.sweet.yml", "version: 0.3.0\n")
	write("pkg/api/CHANGELOG.md", "## 0.3.0\n\nMerged release.\n")
	feature := commit("feature bump")
	git("checkout", "main")
	git("merge", "--no-ff", "feature", "-m", "merge release")
	merged := git("rev-parse", "HEAD")
	plan, err = ReleasePlan(root, "pkg/api", head, merged, "v")
	if err != nil || len(plan) != 1 || plan[0].Commit != merged {
		t.Fatalf("merge: %+v, %v", plan, err)
	}
	if _, err := ReleasePlan(root, "pkg/api", feature, merged, "v"); err == nil {
		t.Fatal("accepted second parent as base")
	}
	for _, version := range []string{"0.2.0", "0.3.0+build", "bad"} {
		write("pkg/api/.sweet.yml", "version: "+version+"\n")
		bad := commit("invalid release")
		if _, err := ReleasePlan(root, "pkg/api", merged, bad, "v"); err == nil {
			t.Fatalf("accepted non-increase %s", version)
		}
		git("reset", "--hard", merged)
	}
	write("pkg/api/.sweet.yml", "version: 0.4.0\n")
	bad := commit("missing release notes")
	if _, err := ReleasePlan(root, "pkg/api", merged, bad, "v"); err == nil {
		t.Fatal("accepted missing changelog entry")
	}
}

func TestReleaseNotes(t *testing.T) {
	notes := "First paragraph.\n\n## Migration\n\n```md\n## 9.9.9\n```\n\nLast paragraph."
	changelog := "# Changelog\n\n## 2.0.0\n\n" + notes + "\n\n## 1.0.0\n\nOld.\n"
	got, err := releaseNotes(changelog, "2.0.0")
	if err != nil || got != notes {
		t.Fatalf("notes = %q, %v", got, err)
	}
	for _, text := range []string{"", "## 1.0.0\n", "## 2.0.0\nOne\n## 2.0.0\nTwo\n"} {
		if _, err := releaseNotes(text, "2.0.0"); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	if notes, err := releaseNotes("## 2.0.0\n", "2.0.0"); err != nil || notes != "" {
		t.Fatalf("empty notes: %q, %v", notes, err)
	}
}
