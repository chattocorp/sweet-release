package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func versionTarget(format, key string) VersionFile {
	f := VersionFile{Format: format}
	if key != "" {
		f.Key = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	}
	return f
}

func TestVersionFileFormatsPreserveContent(t *testing.T) {
	for _, tt := range []struct{ name, format, key, input, want string }{
		{"JSON", "json", "version", "{\r\n\t\"version\": \"1.2.3\", \"other\": 42\r\n}\r\n", "{\r\n\t\"version\": \"2.0.0\", \"other\": 42\r\n}\r\n"},
		{"JSON nested unicode", "json", "project.version", `{"é": "✓", "project": {"version": "1.2.3"}}`, `{"é": "✓", "project": {"version": "2.0.0"}}`},
		{"JSON escaped", "json", "version", `{"version":"1.\u0032.3"}`, `{"version":"2.0.0"}`},
		{"YAML", "yaml", "project.version", "# Intro\nproject:\n  version: '1.2.3' # Keep\nother: yes\n", "# Intro\nproject:\n  version: '2.0.0' # Keep\nother: yes\n"},
		{"YAML flow", "yaml", "version", "{version: 1.2.3, other: value}\n", "{version: 2.0.0, other: value}\n"},
		{"YAML double quoted", "yaml", "version", "version: \"1.2.3\"\r\n", "version: \"2.0.0\"\r\n"},
		{"YAML BOM", "yaml", "version", "\ufeffversion: 1.2.3\n", "\ufeffversion: 2.0.0\n"},
		{"TOML table", "toml", "project.version", "# Intro\n[project]\nversion = '1.2.3' # Keep\nname = \"hello\"\n", "# Intro\n[project]\nversion = '2.0.0' # Keep\nname = \"hello\"\n"},
		{"TOML dotted", "toml", "project.version", "project.version = \"1.2.3\"\r\n", "project.version = \"2.0.0\"\r\n"},
		{"TOML inline", "toml", "project.release.version", "project = {release = {version = \"1.2.3\"}, name = 'keep'}\n", "project = {release = {version = \"2.0.0\"}, name = 'keep'}\n"},
		{"TOML multiline quotes", "toml", "version", "version = '''1.2.3'''\n", "version = '''2.0.0'''\n"},
		{"text", "text", "", " \t1.2.3\r\n", " \t2.0.0\r\n"},
		{"text no newline", "text", "", "1.2.3-beta.1+build", "2.0.0"},
		{"annotations", "annotated", "", "const Version = \"1.2.3\" // sweet-version\r\nother = '9.9.9'\r\nimage: app:v1.2.3 # sweet-version\r\n", "const Version = \"2.0.0\" // sweet-version\r\nother = '9.9.9'\r\nimage: app:v2.0.0 # sweet-version\r\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			replacements, err := versionReplacements([]byte(tt.input), versionTarget(tt.format, tt.key), "2.0.0")
			if err != nil {
				t.Fatal(err)
			}
			got, err := applyVersionReplacements([]byte(tt.input), replacements)
			if err != nil || string(got) != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestVersionFileRejectsInvalidTargets(t *testing.T) {
	for _, tt := range []struct{ format, key, input string }{
		{"json", "version", `{"version": "1.2.3",}`},
		{"json", "version", `{"version": "1.2.3", "version": "2.0.0"}`},
		{"json", "version", `{"version": 123}`},
		{"json", "version", `{"name": "a"}`},
		{"json", "items.version", `{"items": [{"version": "1.2.3"}]}`},
		{"yaml", "version", "version: 1.2.3\n---\nversion: 1.2.3\n"},
		{"yaml", "version", "version: &version 1.2.3\n"},
		{"yaml", "version", "default: &v 1.2.3\nversion: *v\n"},
		{"yaml", "version", "version: !!str 1.2.3\n"},
		{"yaml", "version", "version: |\n  1.2.3\n"},
		{"yaml", "version", "version: 1.2.3\nversion: 1.2.3\n"},
		{"yaml", "version", ""},
		{"toml", "version", "version = '1.2.3'\nversion = '1.2.3'\n"},
		{"toml", "version", "version = 123\n"},
		{"toml", "version", "name = 'a'\n"},
		{"toml", "projects.version", "[[projects]]\nversion = '1.2.3'\n"},
		{"text", "", "Release 1.2.3\n"},
		{"text", "", ""},
		{"annotated", "", "version = '1.2.3'"},
		{"annotated", "", "# sweet-version"},
		{"annotated", "", "a='1.2.3'; b='2.0.0' # sweet-version"},
		{"annotated", "", "a='1.2.3.4' # sweet-version"},
		{"annotated", "", "a='01.2.3' # sweet-version"},
		{"annotated", "", "a='release1.2.3' # sweet-version"},
	} {
		t.Run(tt.format+"/"+tt.input, func(t *testing.T) {
			if _, err := versionReplacements([]byte(tt.input), versionTarget(tt.format, tt.key), "2.0.0"); err == nil {
				t.Fatal("accepted invalid target")
			}
		})
	}
}

func TestVersionFileConfiguration(t *testing.T) {
	for _, config := range []string{
		"version-files: null", "version-files: {}", "version-files: [null]",
		"version-files: [{path: file, format: json}]",
		"version-files: [{path: file, format: json, key: []}]",
		"version-files: [{path: file, format: json, key: [123]}]",
		"version-files: [{path: file, format: json, key: a..b}]",
		"version-files: [{path: file, format: json, key: version, typo: true}]",
		"version-files: [{path: /file, format: text}]",
		"version-files: [{path: file, format: unknown}]",
		"version-files: [{path: file, format: text, key: version}]",
	} {
		if _, err := readPackageVersion([]byte("version: 1.2.3\n" + config)); err == nil {
			t.Errorf("accepted %s", config)
		}
	}
	files, err := readVersionFiles([]byte("version-files: [{path: file, format: json, key: [literal.dot, version]}]"))
	if err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"literal.dot": {"version": "1.2.3"}}`)
	replacements, err := versionReplacements(data, files[0], "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	got, err := applyVersionReplacements(data, replacements)
	if err != nil || string(got) != `{"literal.dot": {"version": "2.0.0"}}` {
		t.Fatalf("%s: %v", got, err)
	}
}

func TestBumpVersionFiles(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init").CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	write := func(path, data string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	manifest := "version: 1.2.3\nversion-files:\n  - {path: ../versions.json, format: json, key: api}\n  - {path: VERSION, format: text}\n"
	write("api/.sweet.yml", manifest)
	write("api/.sweet/fix.md", "---\ntype: fix\n---\nA fix.\n")
	write("api/VERSION", "1.2.3\n")
	write("other/.sweet.yml", "version: 2.0.0\nversion-files:\n  - {path: ../versions.json, format: json, key: other}\n")
	write("other/.sweet/fix.md", "---\ntype: fix\n---\nA fix.\n")
	write("versions.json", "{\"api\": \"1.2.3\", \"other\": \"2.0.0\"}\n")
	// A bad target on the second package must leave the first untouched.
	write("other/.sweet.yml", strings.ReplaceAll(read("other/.sweet.yml"), "key: other", "key: missing"))
	if _, err := Bump(root, nil); err == nil {
		t.Fatal("accepted missing key")
	}
	if read("api/.sweet.yml") != manifest || read("api/VERSION") != "1.2.3\n" {
		t.Fatal("partial write")
	}
	read("api/.sweet/fix.md")
	// Conflicting updates to the same field must fail before writes.
	write("other/.sweet.yml", strings.ReplaceAll(read("other/.sweet.yml"), "key: missing", "key: api"))
	if _, err := Bump(root, nil); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflict: %v", err)
	}
	write("other/.sweet.yml", strings.ReplaceAll(read("other/.sweet.yml"), "key: api", "key: other"))
	releases, err := Bump(root, nil)
	if err != nil || len(releases) != 2 {
		t.Fatalf("%v: %v", releases, err)
	}
	if got := read("versions.json"); got != "{\"api\": \"1.3.0\", \"other\": \"2.1.0\"}\n" {
		t.Fatal(got)
	}
	if read("api/VERSION") != "1.3.0\n" {
		t.Fatal("text version not updated")
	}
	read("api/CHANGELOG.md")
	if _, err := os.Stat(filepath.Join(root, "api/.sweet/fix.md")); !os.IsNotExist(err) {
		t.Fatal("note not consumed")
	}
	if releases, err := Bump(root, nil); err != nil || len(releases) != 0 {
		t.Fatalf("repeat: %v, %v", releases, err)
	}
}

func TestVersionFilePathBoundaries(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	outside := filepath.Join(t.TempDir(), "VERSION")
	if err := os.WriteFile(outside, []byte("1.2.3"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../outside", ".git/config", ".sweet.yml", ".sweet.config.yml", "CHANGELOG.md", ".sweet/note.md", "link", "missing"} {
		manifest := []byte("version-files: [{path: " + path + ", format: text}]")
		if err := planVersionFiles(root, PackageStatus{Package: Package{Path: "."}, NextVersion: "2.0.0"}, manifest, map[string]*versionFilePlan{}); err == nil {
			t.Errorf("accepted %s", path)
		}
	}
}

func TestVersionReplacementDuplicates(t *testing.T) {
	got, err := applyVersionReplacements([]byte("1.2.3"), []versionReplacement{{0, 5, "2.0.0"}, {0, 5, "2.0.0"}})
	if err != nil || string(got) != "2.0.0" {
		t.Fatalf("%s: %v", got, err)
	}
}

func TestVersionFileRollback(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "VERSION")
	if err := os.WriteFile(path, []byte("1.2.3\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	edit, err := readReleaseEdit(path, false)
	if err != nil {
		t.Fatal(err)
	}
	edit.after = []byte("2.0.0\n")
	// A missing parent makes the later write fail after the version was written.
	err = applyReleaseEdits([]releaseEdit{edit, {path: filepath.Join(root, "missing", "CHANGELOG.md"), after: []byte("new release"), mode: 0o644}})
	if err == nil {
		t.Fatal("expected write failure")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "1.2.3\n" {
		t.Fatalf("version not restored: %s, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("permissions not restored: %v", err)
	}
}
