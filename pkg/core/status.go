package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Change is a pending package release note. Path is relative to the repository.
type Change struct {
	Path     string `json:"path"`
	Type     string `json:"type"`
	Breaking bool   `json:"breaking"`
	Body     string `json:"body"`
}

// PackageStatus describes a package and the combined effect of its pending notes.
type PackageStatus struct {
	Package
	Changes     []Change  `json:"changes"`
	Bump        string    `json:"bump"`
	NextVersion string    `json:"nextVersion"`
	Problems    []Problem `json:"problems"`
}

// Problem identifies a file that prevents a reliable release calculation.
type Problem struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (p Problem) Error() string { return p.Path + ": " + p.Message }

// RepositoryStatus inventories all packages, including those without changes.
// It reads tracked and untracked notes, respecting Git ignore rules.
// Packages with file problems have no calculated bump or next version.
func RepositoryStatus(root string) ([]PackageStatus, error) {
	paths, err := repositoryFiles(root)
	if err != nil {
		return nil, err
	}
	packages, manifestProblems, err := packagesFromFiles(root, paths)
	if err != nil {
		return nil, err
	}
	types, err := LoadChangeTypes(root)
	if err != nil {
		return nil, err
	}
	policies := make(map[string]string, len(types))
	for _, typ := range types {
		policies[typ.Name] = typ.Bump
	}
	rank := map[string]int{"none": 0, "patch": 1, "minor": 2, "major": 3}
	notes := make(map[string][]string)
	for _, path := range paths {
		dir := filepath.Dir(path)
		if filepath.Base(dir) == ".sweet" && filepath.Ext(path) == ".md" {
			pkg := filepath.ToSlash(filepath.Dir(dir))
			notes[pkg] = append(notes[pkg], path)
		}
	}
	packageProblems := make(map[string][]Problem)
	for _, problem := range manifestProblems {
		pkg := filepath.ToSlash(filepath.Dir(problem.Path))
		packageProblems[pkg] = append(packageProblems[pkg], problem)
	}
	statuses := make([]PackageStatus, 0, len(packages))
	for _, pkg := range packages {
		changes, problems := changesFromFiles(root, notes[pkg.Path])
		status := PackageStatus{Package: pkg, Changes: changes, Bump: "none", Problems: problems}
		status.Problems = append(status.Problems, packageProblems[pkg.Path]...)
		for _, change := range changes {
			bump, ok := policies[change.Type]
			if !ok {
				status.Problems = append(status.Problems, Problem{Path: change.Path, Message: fmt.Sprintf("unknown change type %q", change.Type)})
				continue
			}
			if change.Breaking {
				breakingBump := "major"
				if strings.HasPrefix(pkg.Version, "0.") {
					breakingBump = "minor"
				}
				if rank[breakingBump] > rank[bump] {
					bump = breakingBump
				}
			}
			if rank[bump] > rank[status.Bump] {
				status.Bump = bump
			}
		}
		if pkg.Version != "" {
			status.NextVersion, err = nextVersion(pkg.Version, status.Bump)
			if err != nil {
				status.Problems = append(status.Problems, Problem{Path: filepath.ToSlash(filepath.Join(pkg.Path, ".sweet.yml")), Message: err.Error()})
			}
		}
		if len(status.Problems) > 0 {
			status.Bump, status.NextVersion = "", ""
			sort.SliceStable(status.Problems, func(i, j int) bool { return status.Problems[i].Path < status.Problems[j].Path })
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// ReadChanges reads direct .sweet/*.md children in filename order.
// Missing note directories and locally deleted tracked notes are treated as empty.
func ReadChanges(root string, pkg Package) ([]Change, error) {
	changes, problems, err := readChanges(root, pkg)
	if err != nil {
		return nil, err
	}
	var errs []error
	for _, problem := range problems {
		errs = append(errs, problem)
	}
	return changes, errors.Join(errs...)
}

func readChanges(root string, pkg Package) ([]Change, []Problem, error) {
	dir := filepath.Join(root, pkg.Path)
	output, err := exec.Command("git", "-C", dir, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".sweet/").Output()
	if err != nil {
		return nil, nil, fmt.Errorf("read changes for %s: %w", pkg.Path, err)
	}
	paths := strings.Split(string(output), "\x00")
	sort.Strings(paths)
	var names []string
	for _, path := range paths {
		if filepath.Dir(path) == ".sweet" && filepath.Ext(path) == ".md" {
			names = append(names, filepath.ToSlash(filepath.Join(pkg.Path, path)))
		}
	}
	changes, problems := changesFromFiles(root, names)
	return changes, problems, nil
}

func changesFromFiles(root string, paths []string) ([]Change, []Problem) {
	var changes []Change
	var problems []Problem
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		name := path
		data, err := os.ReadFile(filepath.Join(root, path))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			problems = append(problems, Problem{Path: name, Message: err.Error()})
			continue
		}
		change, err := parseChange(data)
		if err != nil {
			problems = append(problems, Problem{Path: name, Message: err.Error()})
			continue
		}
		change.Path = name
		changes = append(changes, change)
	}
	return changes, problems
}

func parseChange(data []byte) (Change, error) {
	var change Change
	lines := bytes.SplitAfter(data, []byte("\n"))
	isDelimiter := func(line []byte) bool {
		return string(bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))) == "---"
	}
	if !isDelimiter(lines[0]) {
		return change, fmt.Errorf("expected YAML frontmatter starting with ---")
	}
	end := 1
	for end < len(lines) && !isDelimiter(lines[end]) {
		end++
	}
	if end == len(lines) {
		return change, fmt.Errorf("missing closing --- for YAML frontmatter")
	}
	var metadata struct {
		Type     yaml.Node `yaml:"type"`
		Breaking yaml.Node `yaml:"breaking"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(bytes.Join(lines[1:end], nil)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&metadata); err != nil {
		return change, fmt.Errorf("decode change frontmatter: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return change, fmt.Errorf("expected one YAML frontmatter document")
	}
	if metadata.Type.Kind != yaml.ScalarNode || metadata.Type.Tag != "!!str" || strings.TrimSpace(metadata.Type.Value) == "" {
		return change, fmt.Errorf("change type must be a non-empty string")
	}
	if metadata.Breaking.Kind != 0 {
		if metadata.Breaking.Kind != yaml.ScalarNode || metadata.Breaking.Tag != "!!bool" {
			return change, fmt.Errorf("breaking must be true or false")
		}
		if err := metadata.Breaking.Decode(&change.Breaking); err != nil {
			return change, fmt.Errorf("decode breaking: %w", err)
		}
	}
	change.Type = metadata.Type.Value
	change.Body = string(bytes.Join(lines[end+1:], nil))
	return change, nil
}

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

func nextVersion(version, bump string) (string, error) {
	parts := versionPattern.FindStringSubmatch(version)
	if parts == nil {
		return "", fmt.Errorf("invalid semantic version %q", version)
	}
	for _, identifier := range strings.Split(parts[4], ".") {
		if len(identifier) > 1 && identifier[0] == '0' && strings.Trim(identifier, "0123456789") == "" {
			return "", fmt.Errorf("invalid semantic version %q", version)
		}
	}
	if bump == "none" {
		return version, nil
	}
	index := map[string]int{"major": 1, "minor": 2, "patch": 3}[bump]
	if index == 0 {
		return "", fmt.Errorf("invalid bump %q", bump)
	}
	number, _ := new(big.Int).SetString(parts[index], 10)
	parts[index] = number.Add(number, big.NewInt(1)).String()
	for i := index + 1; i <= 3; i++ {
		parts[i] = "0"
	}
	return strings.Join(parts[1:4], "."), nil
}
