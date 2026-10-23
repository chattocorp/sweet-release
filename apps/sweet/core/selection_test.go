package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEffectivePackageNames(t *testing.T) {
	root, write := releasePRFixture(t)
	write(".sweet.yml", "version: 1.0.0\n")
	write("apps/cli/.sweet.yml", "version: 1.0.0\nname: sweet\ndisplay-name: Sweet Release\n")
	write("path with spaces/.sweet.yml", "version: 1.0.0\n")
	for name, path := range map[string]string{".": ".", "sweet": "apps/cli", "path with spaces": "path with spaces"} {
		pkg, err := ResolvePackageName(root, name)
		if err != nil || pkg.Path != path || pkg.Name != name {
			t.Fatalf("resolve %q: %+v, %v", name, pkg, err)
		}
	}
	packages, err := RepositoryStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(packages)
	if err != nil || !strings.Contains(string(data), `"name":"sweet","displayName":"Sweet Release"`) {
		t.Fatalf("JSON: %s, %v", data, err)
	}
	if _, err := ResolvePackageName(root, "missing"); err == nil {
		t.Fatal("accepted unknown name")
	}
	write("sweet/.sweet.yml", "version: 1.0.0\ndisplay-name: Sweet Release\n")
	problems, err := CheckRepository(root)
	if err != nil || len(problems) != 2 {
		t.Fatalf("expected explicit/default name collision: %v, %v", problems, err)
	}
	if _, err := ResolvePackageName(root, "sweet"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("duplicate resolution: %v", err)
	}
}

func TestInvalidManifestPreservesPackageIdentity(t *testing.T) {
	for _, invalid := range []string{
		"version: invalid\n",
		"version: 42\n",
		"version: 1.0.0\nversion-files: invalid\n",
		"version: 1.0.0\ndisplay-name: []\n",
	} {
		t.Run(invalid, func(t *testing.T) {
			root, write := releasePRFixture(t)
			write("apps/api/.sweet.yml", "name: api\n"+invalid)
			if _, err := ResolvePackageName(root, "api"); err == nil || !strings.Contains(err.Error(), "apps/api/.sweet.yml") {
				t.Fatalf("expected selected manifest error, got %v", err)
			}
			write("api/.sweet.yml", "version: 1.0.0\n")
			if _, err := ResolvePackageName(root, "api"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("expected ambiguous name, got %v", err)
			}
			problems, err := CheckRepository(root)
			if err != nil {
				t.Fatal(err)
			}
			duplicates := 0
			for _, problem := range problems {
				if strings.Contains(problem.Message, "duplicate package name") {
					duplicates++
					if !strings.Contains(problem.Message, "api/.sweet.yml, apps/api/.sweet.yml") {
						t.Fatalf("missing conflicting manifest paths: %v", problem)
					}
				}
			}
			if duplicates != 2 {
				t.Fatalf("expected two duplicate reports: %v", problems)
			}
		})
	}
}
