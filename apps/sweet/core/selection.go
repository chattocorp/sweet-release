package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ResolvePackageName resolves an explicit name or a repository-relative path
// default. Duplicate names are ambiguous even if one package is invalid.
func ResolvePackageName(root, name string) (Package, error) {
	packages, manifestProblems, err := discoverPackages(root)
	if err != nil {
		return Package{}, err
	}
	var matches []Package
	var paths []string
	for _, pkg := range packages {
		if pkg.Name == name {
			matches = append(matches, pkg)
			paths = append(paths, pkg.Path)
		}
	}
	if len(matches) == 0 {
		return Package{}, fmt.Errorf("unknown package name %q", name)
	}
	if len(matches) > 1 {
		return Package{}, fmt.Errorf("ambiguous package name %q: %s", name, strings.Join(paths, ", "))
	}
	var problems []error
	for _, problem := range manifestProblems {
		if filepath.ToSlash(filepath.Dir(problem.Path)) == matches[0].Path {
			problems = append(problems, problem)
		}
	}
	if err := errors.Join(problems...); err != nil {
		return Package{}, err
	}
	return matches[0], nil
}
