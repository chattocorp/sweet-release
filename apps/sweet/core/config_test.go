package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepositoryConfig(t *testing.T) {
	defaults, err := parseRepositoryConfig([]byte(defaultRepositoryConfig))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ data, strategy string }{
		{"{}", "combined"},
		{"release-pr: {}", "combined"},
		{defaultRepositoryConfig, "per-package"},
	} {
		config, err := parseRepositoryConfig([]byte(tc.data))
		if err != nil || config.ReleasePRStrategy != tc.strategy || !reflect.DeepEqual(config.ChangeTypes, defaults.ChangeTypes) {
			t.Fatalf("config = %+v, error = %v", config, err)
		}
		for i := range config.ChangeTypes {
			config.ChangeTypes[i] = ChangeType{"changed", "major", "Changed"}
		}
		fresh, err := parseRepositoryConfig([]byte(tc.data))
		if err != nil || !reflect.DeepEqual(fresh.ChangeTypes, defaults.ChangeTypes) {
			t.Fatalf("defaults were shared: %+v, %v", fresh, err)
		}
	}
	config, err := parseRepositoryConfig([]byte("release-pr: {strategy: per-package}\ntypes: {security: {bump: patch}, docs: {section: Documentation}}"))
	want := []ChangeType{{"security", "patch", ""}, {"docs", "none", "Documentation"}}
	if err != nil || config.ReleasePRStrategy != "per-package" || !reflect.DeepEqual(config.ChangeTypes, want) {
		t.Fatalf("config = %+v, error = %v", config, err)
	}
	for _, data := range []string{
		"release-pr: {strategy: invalid}", "types: {}", "{}\n---\n{}",
		"types: {}\ntypes: {}", "release-pr: {}\nrelease-pr: {}",
		"types: {fix: {bump: patch, bump: minor}}", "release-pr: {unknown: true}",
	} {
		if _, err := parseRepositoryConfig([]byte(data)); err == nil {
			t.Errorf("accepted invalid configuration: %s", data)
		}
	}
}

func TestLoadRepositoryConfig(t *testing.T) {
	root := t.TempDir()
	config, err := LoadRepositoryConfig(root)
	if err != nil || config.ReleasePRStrategy != "combined" || len(config.ChangeTypes) != 5 {
		t.Fatalf("missing config = %+v, error = %v", config, err)
	}
	path := filepath.Join(root, ".sweet.config.yml")
	if err := os.WriteFile(path, []byte("types: {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRepositoryConfig(root); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing configuration path: %v", err)
	}
	// The strategy-only compatibility API does not validate unrelated policies.
	if strategy, err := LoadReleasePRStrategy(root); err != nil || strategy != "combined" {
		t.Fatalf("strategy = %q, error = %v", strategy, err)
	}
}

func TestRepositoryStatusUsesLoadedConfig(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if _, err := InitPackage(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".sweet.config.yml")
	if err := os.WriteFile(path, []byte("types: {security: {bump: patch}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateChange(root, "security", strings.NewReader("Security fix.")); err != nil {
		t.Fatal(err)
	}
	config, err := LoadRepositoryConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	// A later read would fail; the operation must use its loaded policy snapshot.
	if err := os.WriteFile(path, []byte("types: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	statuses, err := repositoryStatus(root, config)
	if err != nil || len(statuses) != 1 || statuses[0].Bump != "patch" || statuses[0].NextVersion != "0.0.1" {
		t.Fatalf("statuses = %+v, error = %v", statuses, err)
	}
}

func TestChangeTypeConfiguration(t *testing.T) {
	for _, data := range []string{"types: []", "types: null", "types: fix", "types: [fix, fix]", "types: [1]", "types: ['bad name']", "types: [Fix]", "types: [", "types: [fix]\n---\ntypes: [feature]", "types: [fix]\ntypes: [feature]"} {
		if _, err := parseChangeTypes([]byte(data)); err == nil {
			t.Errorf("accepted invalid configuration: %s", data)
		}
	}
	for _, data := range []string{defaultRepositoryConfig, "{}"} {
		types, err := parseChangeTypes([]byte(data))
		want := []ChangeType{{"fix", "minor", "Fixes"}, {"feature", "minor", "Features"}, {"chore", "none", ""}, {"refactor", "none", ""}, {"performance", "patch", "Performance"}}
		if err != nil || !reflect.DeepEqual(types, want) {
			t.Fatalf("types = %v, error = %v", types, err)
		}
	}
}

func TestRepositoryChangeTypes(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if _, err := InitRepository(root); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, ".sweet.config.yml")
	if err := os.WriteFile(config, []byte("types:\n  security:\n    bump: patch\n    section: Security\n  docs:\n    bump: none\n    section: Documentation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "nested", "package")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := InitPackage(dir); err != nil {
		t.Fatal(err)
	}
	// Configuration follows the target package, regardless of the caller's PWD.
	t.Chdir(t.TempDir())
	types, err := ChangeTypes(dir)
	if err != nil || !reflect.DeepEqual(types, []string{"security", "docs"}) {
		t.Fatalf("types = %v, error = %v", types, err)
	}
	if _, err := CreateChange(dir, "fix", strings.NewReader("body")); err == nil || !strings.Contains(err.Error(), "unknown change type") {
		t.Fatalf("unexpected error: %v", err)
	}
	files, _ := os.ReadDir(filepath.Join(dir, ".sweet"))
	if len(files) != 0 {
		t.Fatal("created a note for an unknown type")
	}
	path, err := CreateChange(dir, "security", strings.NewReader("Keep this body.\n"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "---\ntype: security\n---\n\nKeep this body.\n" {
		t.Fatalf("note = %q, error = %v", data, err)
	}
	if err := os.WriteFile(config, []byte("types: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ChangeTypes(dir); err == nil || !strings.Contains(err.Error(), config) {
		t.Fatalf("missing configuration path: %v", err)
	}
}

func TestChangeTypePolicies(t *testing.T) {
	types, err := parseChangeTypes([]byte(`types:
  breaking:
    bump: major
    section: Breaking changes
  internal:
    bump: patch
    section:
  docs:
    section: Documentation
  hidden: {}
`))
	want := []ChangeType{{"breaking", "major", "Breaking changes"}, {"internal", "patch", ""}, {"docs", "none", "Documentation"}, {"hidden", "none", ""}}
	if err != nil || !reflect.DeepEqual(types, want) {
		t.Fatalf("policies = %v, error = %v", types, err)
	}
	for _, policy := range []string{
		"types: {}", "types: {fix: null}", "types: {fix: patch}",
		"types: {fix: {bump: invalid}}", "types: {fix: {bump: 1}}",
		"types: {fix: {bump: null}}", "types: {fix: {bump: ''}}",
		"types: {fix: {section: true}}", "types: {fix: {section: []}}",
		"types: {fix: {section: \"one\\ntwo\"}}",
		"types: {fix: {bump: patch, bump: minor}}",
		"types: {fix: {sections: Fixes}}", "types: {fix: {}, fix: {}}",
		"types: {Fix: {}}", "types: {1: {}}",
	} {
		if _, err := parseChangeTypes([]byte(policy)); err == nil {
			t.Errorf("accepted %s", policy)
		}
	}
}
