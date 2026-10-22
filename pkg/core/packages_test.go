package core

import "testing"

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
