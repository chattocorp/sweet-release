package core

import "testing"

func TestPackageManifestCompatibility(t *testing.T) {
	manifest, err := parsePackageManifest([]byte("# metadata remains allowed\nversion: &version '1.2.3-beta.1+build'\nname: sweet\ndisplay-name: &name Sweet Release\ncustom: {value: *version}\nversion-files:\n  - {path: VERSION, format: text}\n"))
	if err != nil || manifest.Version != "1.2.3-beta.1+build" || manifest.Name != "sweet" || manifest.DisplayName != "Sweet Release" || len(manifest.VersionFiles) != 1 {
		t.Fatalf("manifest = %#v, error = %v", manifest, err)
	}
}

func TestPackageDisplayName(t *testing.T) {
	for _, raw := range []string{"null", "123", "[]", "''", "'  '", "' leading'", "\"two\\nlines\""} {
		if _, err := readPackageVersion([]byte("version: 1.0.0\ndisplay-name: " + raw)); err == nil {
			t.Errorf("accepted name %s", raw)
		}
	}
	name, err := readPackageName([]byte("name: sweet\nversion: 1.0.0\n"))
	if err != nil || name != "sweet" {
		t.Fatalf("%q, %v", name, err)
	}
	for _, p := range []Package{{Path: "pkg/cli"}, {Path: "."}} {
		if p.DisplayName() != p.Path {
			t.Fatal("missing path fallback")
		}
	}
	if got := (Package{Path: "pkg/cli", Name: "sweet"}).DisplayName(); got != "sweet" {
		t.Fatalf("name fallback = %q", got)
	}
	if got := (Package{Path: "pkg/cli", Name: "sweet", Label: "Sweet Release"}).DisplayName(); got != "Sweet Release" {
		t.Fatalf("display label = %q", got)
	}
}

func TestMachinePackageNames(t *testing.T) {
	for _, raw := range []string{"null", "123", "[]", "''", "Sweet Release", "API", "1api", "pkg/api", "api_name", "' leading'"} {
		if _, err := parsePackageManifest([]byte("version: 1.0.0\nname: " + raw)); err == nil {
			t.Errorf("accepted invalid machine name %s", raw)
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
