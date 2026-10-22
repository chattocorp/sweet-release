package core

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCreateChange(t *testing.T) {
	dir := t.TempDir()
	if _, err := InitPackage(dir); err != nil {
		t.Fatal(err)
	}
	// Packages created before the notes directory was introduced also work.
	if err := os.Remove(filepath.Join(dir, ".sweet")); err != nil {
		t.Fatal(err)
	}
	const body = "---\ntype: fix\n---\n\nKeep **Markdown** intact.\n"
	paths := map[string]bool{}
	for range 20 {
		path, err := CreateChange(dir, "fix", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if paths[path] {
			t.Fatalf("reused filename: %s", path)
		}
		paths[path] = true
		if !regexp.MustCompile(`^[a-z]+-[a-z]+-[a-z]+\.md$`).MatchString(filepath.Base(path)) {
			t.Fatalf("unexpected name: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "---\ntype: fix\n---\n\n"+body {
			t.Fatalf("content = %q, error = %v", data, err)
		}
	}
}

type brokenReader struct{}

func (brokenReader) Read(p []byte) (int, error) { return 0, errors.New("input failed") }

func TestCreateChangeFailure(t *testing.T) {
	dir := t.TempDir()
	if _, err := CreateChange(dir, "fix", strings.NewReader("note")); err == nil {
		t.Fatal("accepted a directory without a manifest")
	}
	if _, err := os.Stat(filepath.Join(dir, ".sweet")); !os.IsNotExist(err) {
		t.Fatal("created notes directory without a package")
	}
	if _, err := InitPackage(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateChange(dir, "fix", brokenReader{}); err == nil {
		t.Fatal("ignored read failure")
	}
	files, err := os.ReadDir(filepath.Join(dir, ".sweet"))
	if err != nil || len(files) != 0 {
		t.Fatalf("partial notes remain: %v, %v", files, err)
	}
}

func TestInitPackageNotesConflict(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".sweet"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InitPackage(dir); err == nil {
		t.Fatal("accepted notes path that is a file")
	}
	if _, err := os.Stat(filepath.Join(dir, ".sweet.yml")); !os.IsNotExist(err) {
		t.Fatal("partial manifest remains")
	}
	data, err := os.ReadFile(filepath.Join(dir, ".sweet"))
	if err != nil || string(data) != "existing" {
		t.Fatalf("existing file changed: %q, %v", data, err)
	}
}
