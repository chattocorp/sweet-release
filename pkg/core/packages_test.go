package core

import "testing"

func TestPackageDisplayName(t *testing.T) {
	for _, raw := range []string{"null", "123", "[]", "''", "'  '", "' leading'", "\"two\\nlines\""} {
		if _, err := readPackageVersion([]byte("version: 1.0.0\nname: " + raw)); err == nil {
			t.Errorf("accepted name %s", raw)
		}
	}
	name, err := readPackageName([]byte("name: Sweet Release\nversion: 1.0.0\n"))
	if err != nil || name != "Sweet Release" {
		t.Fatalf("%q, %v", name, err)
	}
	for _, p := range []Package{{Path: "pkg/cli"}, {Path: "."}} {
		if p.DisplayName() != p.Path {
			t.Fatal("missing path fallback")
		}
	}
}

func TestPackageVersion(t *testing.T) {
	for _, content := range []string{"", "{}", "version: null", "version: 1", "version: []", "version: ''", "version: 'a b'", "version: [", "version: 1.0.0\nversion: 2.0.0", "version: 1.0.0\n---\nversion: 2.0.0"} {
		if version, err := readPackageVersion([]byte(content)); err == nil {
			t.Errorf("accepted %q as %q", content, version)
		}
	}
	version, err := readPackageVersion([]byte("# Package\nversion: '1.2.3'\n"))
	if err != nil || version != "1.2.3" {
		t.Fatalf("version = %q, error = %v", version, err)
	}
}
