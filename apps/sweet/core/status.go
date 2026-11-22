package core

import (
	"bytes"
	"fmt"
	"math/big"
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
	return repositoryStatus(root, RepositoryConfig{})
}

// TreeStatus inventories packages as committed in a Git tree-ish: a commit,
// tag, or tree object. It reads blobs only, never the working tree or index.
func TreeStatus(rev string) ([]PackageStatus, error) {
	files, err := treeSnapshot(rev)
	if err != nil {
		return nil, err
	}
	return snapshotStatus(files, RepositoryConfig{})
}

// repositoryStatus reuses loaded settings, or loads them after package discovery.
func repositoryStatus(root string, config RepositoryConfig) ([]PackageStatus, error) {
	files, err := workingSnapshot(root)
	if err != nil {
		return nil, err
	}
	return snapshotStatus(files, config)
}

func snapshotStatus(files snapshot, config RepositoryConfig) ([]PackageStatus, error) {
	packages, manifestProblems := packagesFromFiles(files)
	if config.ChangeTypes == nil {
		var err error
		config, err = snapshotConfig(files)
		if err != nil {
			return nil, err
		}
	}
	types := config.ChangeTypes
	policies := make(map[string]string, len(types))
	for _, typ := range types {
		policies[typ.Name] = typ.Bump
	}
	notes := make(map[string][]string)
	for _, path := range files.paths {
		if isNote(path) {
			pkg := filepath.ToSlash(filepath.Dir(filepath.Dir(path)))
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
		changes, problems := changesFromFiles(files.files, notes[pkg.Path])
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
				if bumpRank[breakingBump] > bumpRank[bump] {
					bump = breakingBump
				}
			}
			if bumpRank[bump] > bumpRank[status.Bump] {
				status.Bump = bump
			}
		}
		if pkg.Version != "" {
			var err error
			status.NextVersion, err = nextVersion(pkg.Version, status.Bump, pkg.Prerelease)
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

func changesFromFiles(files fileReader, paths []string) ([]Change, []Problem) {
	var changes []Change
	var problems []Problem
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		name := path
		data, ok, err := readOptional(files, path)
		if err == nil && !ok {
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
	if err := decodeYAMLDocument(bytes.Join(lines[1:end], nil), &metadata, true, "change frontmatter"); err != nil {
		return change, err
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

// bumpRank orders bump levels from no release to a major release.
var bumpRank = map[string]int{"none": 0, "patch": 1, "minor": 2, "major": 3}

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

// parseVersion validates a semantic version and returns its regexp submatches:
// major, minor, patch, and prerelease at indexes 1 through 4.
func parseVersion(version string) ([]string, error) {
	parts := versionPattern.FindStringSubmatch(version)
	if parts == nil {
		return nil, fmt.Errorf("invalid semantic version %q", version)
	}
	for _, identifier := range strings.Split(parts[4], ".") {
		if isNumeric(identifier) && len(identifier) > 1 && identifier[0] == '0' {
			return nil, fmt.Errorf("invalid semantic version %q", version)
		}
	}
	return parts, nil
}

func isNumeric(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }

func isPrerelease(version string) bool {
	parts, err := parseVersion(version)
	return err == nil && parts[4] != ""
}

// nextVersion applies a bump to version. With a prerelease label, the result is a
// prerelease of the bumped version; without one, a prerelease is finalized.
// A prerelease already covers bumps up to its own level: 1.3.0-beta.0 includes
// minor and patch changes, so only its counter advances.
func nextVersion(version, bump, prerelease string) (string, error) {
	parts, err := parseVersion(version)
	if err != nil {
		return "", err
	}
	level, ok := bumpRank[bump]
	if !ok {
		return "", fmt.Errorf("invalid bump %q", bump)
	}
	current := parts[4]
	covered := 0
	if current != "" {
		covered = 1
		if parts[3] == "0" {
			covered = 2
			if parts[2] == "0" {
				covered = 3
			}
		}
	}
	core := parts[1:4]
	if level > covered {
		index := 3 - level
		number, _ := new(big.Int).SetString(core[index], 10)
		core[index] = number.Add(number, big.NewInt(1)).String()
		for i := index + 1; i < 3; i++ {
			core[i] = "0"
		}
	}
	next, coreChanged := strings.Join(core, "."), level > covered
	switch {
	case prerelease == "" && (coreChanged || current != ""):
		return next, nil
	case prerelease != "" && coreChanged:
		return next + "-" + prerelease + ".0", nil
	case prerelease == "" || current == "":
		return version, nil // Nothing to release.
	}
	// The version is already a prerelease of the target version.
	counter, sameChannel := strings.CutPrefix(current, prerelease+".")
	sameChannel = sameChannel && isNumeric(counter)
	if level == 0 && (sameChannel || current == prerelease) {
		return version, nil // No notes for the current channel.
	}
	if sameChannel {
		number, _ := new(big.Int).SetString(counter, 10)
		return next + "-" + prerelease + "." + number.Add(number, big.NewInt(1)).String(), nil
	}
	candidate := prerelease + ".0"
	if comparePrerelease(candidate, current) <= 0 {
		return "", fmt.Errorf("prerelease %q would not advance %s; choose a later label or remove prerelease", prerelease, version)
	}
	return next + "-" + candidate, nil
}

// comparePrerelease orders non-empty prerelease strings by semver precedence.
func comparePrerelease(a, b string) int {
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] == y[i] {
			continue
		}
		xn, yn := isNumeric(x[i]), isNumeric(y[i])
		switch {
		case xn && yn:
			p, _ := new(big.Int).SetString(x[i], 10)
			q, _ := new(big.Int).SetString(y[i], 10)
			return p.Cmp(q)
		case xn:
			return -1
		case yn:
			return 1
		}
		return strings.Compare(x[i], y[i])
	}
	return len(x) - len(y)
}
