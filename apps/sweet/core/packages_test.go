package core

import "testing"

func TestDifferentPrefixesCannotProduceSameTag(t *testing.T) {
	prefix := "api@1.0.0+"
	packages := []Package{
		{Path: "apps/api", Version: "1.0.0+2.0.0"},
		{Path: "services/other", Version: "2.0.0", TagPrefix: &prefix},
	}
	if problems := TagPrefixProblems(packages); len(problems) != 1 {
		t.Fatalf("missed identical tags: %v", problems)
	}
}

func TestTagPrefixManifest(t *testing.T) {
	for _, raw := range []string{"null", "123", "[]", "'bad prefix@'", "'../bad@'", "'bad..name@'", "'bad@{'"} {
		if _, err := parsePackageManifest([]byte("version: 1.0.0\ntag-prefix: " + raw)); err == nil {
			t.Errorf("accepted prefix %s", raw)
		}
	}
	for _, prefix := range []string{"", "custom@", "old/path/v", "@scope/pkg@"} {
		manifest, err := parsePackageManifest([]byte("version: 1.0.0\ntag-prefix: '" + prefix + "'"))
		if err != nil {
			t.Fatal(err)
		}
		pkg := Package{Path: "apps/api", Version: manifest.Version, TagPrefix: manifest.TagPrefix}
		if got := pkg.ReleaseTagName(); got != prefix+"1.0.0" {
			t.Fatal(got)
		}
	}
}

func TestPackageManifestCompatibility(t *testing.T) {
	manifest, err := parsePackageManifest([]byte("# metadata remains allowed\nversion: &version '1.2.3-beta.1+build'\nname: &name Sweet Release\ncustom: {value: *version}\nversion-files:\n  - {path: VERSION, format: text}\n"))
	if err != nil || manifest.Version != "1.2.3-beta.1+build" || manifest.Name != "Sweet Release" || len(manifest.VersionFiles) != 1 {
		t.Fatalf("manifest = %#v, error = %v", manifest, err)
	}
}

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
