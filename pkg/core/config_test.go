package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestChangeTypeConfiguration(t *testing.T) {
	for _, data := range []string{"types: []", "types: null", "types: fix", "types: [fix, fix]", "types: [1]", "types: ['bad name']", "types: [Fix]", "types: [", "types: [fix]\n---\ntypes: [feature]", "types: [fix]\ntypes: [feature]"} {
		if _, err := parseChangeTypes([]byte(data)); err == nil {
			t.Errorf("accepted invalid configuration: %s", data)
		}
	}
	for _, data := range []string{defaultRepositoryConfig, "{}"} {
		types, err := parseChangeTypes([]byte(data))
		if err != nil || !reflect.DeepEqual(types, []string{"fix", "feature", "chore", "refactor", "performance"}) {
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
	if err := os.WriteFile(config, []byte("types:\n  - security\n  - docs\n"), 0o644); err != nil {
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
