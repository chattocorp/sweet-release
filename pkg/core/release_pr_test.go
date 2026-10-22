package core

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReleasePRStrategy(t *testing.T) {
	for _, tc := range []struct{ data, want string }{
		{"{}", "combined"},
		{"types: {fix: {bump: patch}}", "combined"},
		{"release-pr: {}", "combined"},
		{"release-pr: {strategy: combined}", "combined"},
		{"release-pr: {strategy: per-package}", "per-package"},
		{defaultRepositoryConfig, "per-package"},
	} {
		got, err := parseReleasePRStrategy([]byte(tc.data))
		if err != nil || got != tc.want {
			t.Errorf("%s: got %q, %v; want %q", tc.data, got, err, tc.want)
		}
	}
	for _, data := range []string{
		"release-pr: null", "release-pr: per-package", "release-pr: []",
		"release-pr: {strategy: all}", "release-pr: {strategy: true}",
		"release-pr: {strategy: null}", "release-pr: {strategy: ''}",
		"release-pr: {strategies: combined}", "release-pr: {strategy: combined, strategy: per-package}",
		"release-pr: {}\nrelease-pr: {}", "release-pr: {}\n---\nrelease-pr: {}",
	} {
		if _, err := parseReleasePRStrategy([]byte(data)); err == nil {
			t.Errorf("accepted %s", data)
		}
		if _, err := parseChangeTypes([]byte(data)); err == nil {
			t.Errorf("change type loading ignored invalid release-pr: %s", data)
		}
	}
}

func TestDisplayNamesDoNotIdentifyPackages(t *testing.T) {
	root, write := releasePRFixture(t)
	write(".sweet.config.yml", "release-pr: {strategy: per-package}\n")
	for _, path := range []string{"a", "b"} {
		write(path+"/.sweet.yml", "version: 1.0.0\nname: Shared Name\n")
		write(path+"/.sweet/note.md", "---\ntype: feature\n---\nNew feature.\n")
	}
	prs, err := PlanReleasePRs(root)
	if err != nil || len(prs) != 2 {
		t.Fatalf("%v, %v", prs, err)
	}
	for i, path := range []string{"a", "b"} {
		if prs[i].Title != "chore: release Shared Name 1.1.0" || prs[i].Branch != packageReleaseBranch(path) || prs[i].Packages[0] != path {
			t.Fatalf("wrong plan: %+v", prs[i])
		}
	}
	if _, err := Bump(root, nil); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "a/CHANGELOG.md"))
	if err != nil || !strings.HasPrefix(string(content), "# Shared Name Changelog\n") {
		t.Fatalf("%s, %v", content, err)
	}
}

func releasePRFixture(t *testing.T) (string, func(string, string)) {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	write := func(path, body string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, write
}

func gitWorkingTreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		files[path] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestReleasePRDetailsMatchPreparedContent(t *testing.T) {
	for _, strategy := range []string{"combined", "per-package"} {
		t.Run(strategy, func(t *testing.T) {
			root, write := releasePRFixture(t)
			write(".sweet.config.yml", "release-pr: {strategy: "+strategy+"}\n")
			for _, path := range []string{"a", "b"} {
				write(path+"/.sweet.yml", "version: 1.0.0\nname: Package "+path+"\n")
				write(path+"/.sweet/feature.md", "---\ntype: feature\n---\nNew feature.\n\n```md\n## Example\n```\n")
				write(path+"/.sweet/chore.md", "---\ntype: chore\n---\nHidden chore.\n")
			}
			write("b/CHANGELOG.md", "# Existing introduction\n\n## 1.0.0\n\nOld history.\n")
			before := gitWorkingTreeSnapshot(t, root)
			details, err := PlanReleasePRDetails(root)
			if err != nil {
				t.Fatal(err)
			}
			if after := gitWorkingTreeSnapshot(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("planning changed files")
			}
			plain, err := PlanReleasePRs(root)
			if err != nil {
				t.Fatal(err)
			}
			for i, detail := range details {
				if !reflect.DeepEqual(plain[i], detail.ReleasePR) {
					t.Fatal("plan schemas disagree")
				}
				if strings.Contains(detail.Body, "Old history") || strings.Contains(detail.Body, "Hidden chore") {
					t.Fatalf("unexpected content: %s", detail.Body)
				}
				if _, err := Bump(root, detail.Packages); err != nil {
					t.Fatal(err)
				}
				for _, release := range detail.Releases {
					data, err := os.ReadFile(filepath.Join(root, release.Path, "CHANGELOG.md"))
					if err != nil {
						t.Fatal(err)
					}
					notes, err := ChangelogRelease(string(data), release.Version)
					_, expected, _ := strings.Cut(release.Changelog, "\n")
					if err != nil || strings.TrimSpace(notes) != strings.TrimSpace(expected) {
						t.Fatalf("planned content differs: %s, %v", data, err)
					}
					if !strings.Contains(detail.Body, "## "+release.Name) || !strings.Contains(detail.Body, "Target version: **"+release.Version+"**") {
						t.Fatalf("missing metadata: %s", detail.Body)
					}
				}
			}
		})
	}
}

func TestReleasePRPlanValidatesReleaseFiles(t *testing.T) {
	for _, path := range []string{"CHANGELOG.md", ".sweet/note.md"} {
		t.Run(path, func(t *testing.T) {
			root, write := releasePRFixture(t)
			write("a/.sweet.yml", "version: 1.0.0\n")
			write("a/.sweet/note.md", "---\ntype: feature\n---\nFeature\n")
			if path == "CHANGELOG.md" {
				if err := os.Mkdir(filepath.Join(root, "a", path), 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				// Inventory follows note symlinks, but release preparation must reject them.
				write("target.md", "---\ntype: feature\n---\nFeature\n")
				if err := os.Remove(filepath.Join(root, "a", path)); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../../target.md", filepath.Join(root, "a", path)); err != nil {
					t.Skip(err)
				}
			}
			if _, err := PlanReleasePRs(root); err == nil {
				t.Fatal("planning accepted invalid release file")
			}
			if _, err := Bump(root, []string{"a"}); err == nil {
				t.Fatal("bump accepted invalid release file")
			}
		})
	}
}

func TestPlanReleasePRs(t *testing.T) {
	root, write := releasePRFixture(t)
	for _, pkg := range []string{".", "pkg/api", "pkg/api/client", "no-bump", "empty"} {
		write(filepath.Join(pkg, ".sweet.yml"), "version: 1.0.0\n")
		if pkg != "empty" {
			typ := "feature"
			if pkg == "no-bump" {
				typ = "chore"
			}
			write(filepath.Join(pkg, ".sweet/note.md"), "---\ntype: "+typ+"\n---\nA change.\n")
		}
	}
	// Missing configuration preserves combined behavior.
	prs, err := PlanReleasePRs(root)
	want := []ReleasePR{{Branch: "sweet/release", Title: "chore: prepare releases", Packages: []string{".", "pkg/api", "pkg/api/client"}}}
	if err != nil || !reflect.DeepEqual(prs, want) {
		t.Fatalf("got %+v, %v; want %+v", prs, err, want)
	}
	if _, err := InitRepository(root); err != nil {
		t.Fatal(err)
	}
	prs, err = PlanReleasePRs(root)
	if err != nil || len(prs) != 3 {
		t.Fatalf("got %+v, %v", prs, err)
	}
	for i, pkg := range want[0].Packages {
		if !reflect.DeepEqual(prs[i].Packages, []string{pkg}) || prs[i].Title != "chore: release "+pkg+" 1.1.0" {
			t.Fatalf("unexpected PR: %+v", prs[i])
		}
		manifest, err := os.ReadFile(filepath.Join(root, pkg, ".sweet.yml"))
		if err != nil || string(manifest) != "version: 1.0.0\n" {
			t.Fatalf("planning changed manifest: %q, %v", manifest, err)
		}
		if _, err := os.Stat(filepath.Join(root, pkg, "CHANGELOG.md")); !os.IsNotExist(err) {
			t.Fatalf("planning created changelog: %v", err)
		}
	}
	// New versions and notes update the existing branch identity.
	branch := prs[1].Branch
	write("pkg/api/.sweet.yml", "version: 2.0.0\n")
	prs, err = PlanReleasePRs(root)
	if err != nil || prs[1].Branch != branch || prs[1].Title != "chore: release pkg/api 2.1.0" {
		t.Fatalf("unstable branch: %+v, %v", prs, err)
	}
	write("empty/.sweet/bad.md", "malformed")
	if _, err := PlanReleasePRs(root); err == nil || !strings.Contains(err.Error(), "bad.md") {
		t.Fatalf("invalid package did not block planning: %v", err)
	}
}

func TestPlanReleasePRsEmpty(t *testing.T) {
	root, write := releasePRFixture(t)
	write(".sweet.yml", "version: 1.0.0\n")
	write(".sweet/chore.md", "---\ntype: chore\n---\nInternal work.\n")
	for _, strategy := range []string{"combined", "per-package"} {
		write(".sweet.config.yml", "release-pr: {strategy: "+strategy+"}\n")
		prs, err := PlanReleasePRs(root)
		data, _ := json.Marshal(prs)
		if err != nil || string(data) != "[]" {
			t.Fatalf("empty plan: %s, %v", data, err)
		}
	}
}

func TestReleasePRBranches(t *testing.T) {
	seen := map[string]bool{}
	for _, path := range []string{".", "root", "pkg/api", "pkg-api", "PKG/API", "with spaces", "-flag", "日本語", strings.Repeat("long/", 80)} {
		branch := packageReleaseBranch(path)
		if seen[branch] || strings.HasPrefix(branch, "sweet/release/") || len(branch) > 100 {
			t.Fatalf("invalid or colliding branch %q for %q", branch, path)
		}
		seen[branch] = true
		if err := exec.Command("git", "check-ref-format", "--branch", branch).Run(); err != nil {
			t.Fatalf("invalid Git branch %q: %v", branch, err)
		}
	}
}

func TestReleasePRSharedVersionFiles(t *testing.T) {
	root, write := releasePRFixture(t)
	write(".sweet.config.yml", "release-pr: {strategy: per-package}\n")
	write("versions.json", `{"a":"1.0.0","b":"1.0.0"}`)
	manifest := func(key string) string {
		return "version: 1.0.0\nversion-files:\n  - {path: ../versions.json, format: json, key: " + key + "}\n"
	}
	for _, pkg := range []string{"a", "b"} {
		write(pkg+"/.sweet.yml", manifest(pkg))
		write(pkg+"/.sweet/note.md", "---\ntype: feature\n---\nNew feature.\n")
	}
	if prs, err := PlanReleasePRs(root); err != nil || len(prs) != 2 {
		t.Fatalf("distinct fields in shared file: %+v, %v", prs, err)
	}
	write("b/.sweet.yml", manifest("a"))
	if _, err := PlanReleasePRs(root); err == nil || !strings.Contains(err.Error(), "same version token") {
		t.Fatalf("shared token should require combined mode: %v", err)
	}
	write(".sweet.config.yml", "release-pr: {strategy: combined}\n")
	if prs, err := PlanReleasePRs(root); err != nil || len(prs) != 1 {
		t.Fatalf("combined identical updates: %+v, %v", prs, err)
	}
	write("b/.sweet.yml", strings.Replace(manifest("a"), "version: 1.0.0", "version: 2.0.0", 1))
	if _, err := PlanReleasePRs(root); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("combined conflicting updates: %v", err)
	}
	write("b/.sweet.yml", manifest("missing"))
	if _, err := PlanReleasePRs(root); err == nil {
		t.Fatal("missing version key did not block planning")
	}
}
