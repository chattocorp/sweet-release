package core

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// CheckRepository reports status problems and duplicate effective package names.
// It does not change files or require packages to have names.
func CheckRepository(root string) ([]Problem, error) {
	packages, err := RepositoryStatus(root)
	if err != nil {
		return nil, err
	}
	problems := make([]Problem, 0)
	names := make(map[string][]string)
	for _, pkg := range packages {
		problems = append(problems, pkg.Problems...)
		if pkg.Name != "" {
			names[pkg.Name] = append(names[pkg.Name], filepath.ToSlash(filepath.Join(pkg.Path, ".sweet.yml")))
		}
	}
	for _, pkg := range packages {
		paths := names[pkg.Name]
		if len(paths) > 1 {
			problems = append(problems, Problem{
				Path:    filepath.ToSlash(filepath.Join(pkg.Path, ".sweet.yml")),
				Message: fmt.Sprintf("duplicate package name %q in %s", pkg.Name, strings.Join(paths, ", ")),
			})
		}
	}
	sort.SliceStable(problems, func(i, j int) bool { return problems[i].Path < problems[j].Path })
	return problems, nil
}
