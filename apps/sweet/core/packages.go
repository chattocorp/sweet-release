// Package core provides package discovery and initialization for Sweet.
package core

import (
	"bytes"
	"errors"
	"fmt"
	"go.yaml.in/yaml/v3"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Package identifies a release package by its repository-relative path.
type Package struct {
	Path       string  `json:"path"`
	Version    string  `json:"version"`
	Name       string  `json:"name,omitempty"`
	TagPrefix  *string `json:"tagPrefix,omitempty"`
	Prerelease string  `json:"prerelease,omitempty"`
}

// ReleasePrefix is the literal text before the version in a release tag.
func (p Package) ReleasePrefix() string {
	if p.TagPrefix != nil {
		return *p.TagPrefix
	}
	if p.Path == "." {
		return "v"
	}
	return path.Base(p.Path) + "@"
}

func (p Package) ReleaseTagName() string { return p.ReleasePrefix() + p.Version }

// TagPrefixProblems reports ambiguous release tag ownership, regardless of version.
func TagPrefixProblems(packages []Package) []Problem {
	var problems []Problem
	owners := map[string]string{}
	tags := map[string]Package{}
	for _, pkg := range packages {
		prefix := pkg.ReleasePrefix()
		if owner, ok := owners[prefix]; ok {
			problems = append(problems, Problem{Path: path.Join(pkg.Path, ".sweet.yml"), Message: fmt.Sprintf("tag prefix %q is shared by packages %q and %q; set a unique tag-prefix", prefix, owner, pkg.Path)})
		} else {
			owners[prefix] = pkg.Path
		}
		name := pkg.ReleaseTagName()
		if owner, ok := tags[name]; ok && owner.ReleasePrefix() != prefix {
			problems = append(problems, Problem{Path: path.Join(pkg.Path, ".sweet.yml"), Message: fmt.Sprintf("tag %q is shared by packages %q and %q; set a unique tag-prefix", name, owner.Path, pkg.Path)})
		}
		tags[name] = pkg
	}
	return problems
}

// DiagnosePackages returns manifest and tag-prefix problems without failing on them.
func DiagnosePackages(root string) ([]Problem, error) {
	packages, problems, err := discoverPackages(root)
	return append(problems, TagPrefixProblems(packages)...), err
}

// CheckTagPrefixes validates tag ownership in working files.
func CheckTagPrefixes(root string) error {
	packages, _, err := discoverPackages(root)
	if err != nil {
		return err
	}
	return tagPrefixError(packages)
}

func tagPrefixError(packages []Package) error {
	var errs []error
	for _, problem := range TagPrefixProblems(packages) {
		errs = append(errs, problem)
	}
	return errors.Join(errs...)
}

// DisplayName returns presentation metadata, never a package identifier.
func (p Package) DisplayName() string {
	if p.Name != "" {
		return p.Name
	}
	return p.Path
}

// RepositoryRoot returns the root of the current Git repository.
func RepositoryRoot() (string, error) {
	output, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("find Git repository root: %w", err)
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}

// DiscoverPackages finds package manifests in a Git repository.
func DiscoverPackages(root string) ([]Package, error) {
	packages, problems, err := discoverPackages(root)
	if err != nil {
		return nil, err
	}
	var errs []error
	problems = append(problems, TagPrefixProblems(packages)...)
	for _, problem := range problems {
		errs = append(errs, problem)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return packages, nil
}

func discoverPackages(root string) ([]Package, []Problem, error) {
	paths, err := repositoryFiles(root)
	if err != nil {
		return nil, nil, err
	}
	return packagesFromFiles(root, paths)
}

func repositoryFiles(root string) ([]string, error) {
	// Include tracked and new files, while respecting Git ignore rules.
	output, err := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("discover repository files: %w", err)
	}
	paths := strings.Split(string(output), "\x00")
	sort.Strings(paths)
	return paths, nil
}

func packagesFromFiles(root string, paths []string) ([]Package, []Problem, error) {
	var packages []Package
	var problems []Problem
	seen := make(map[string]bool)
	for _, path := range paths {
		if filepath.Base(path) != ".sweet.yml" || seen[path] {
			continue
		}
		seen[path] = true
		content, err := os.ReadFile(filepath.Join(root, path))
		if os.IsNotExist(err) {
			continue
		} // A tracked manifest may have been deleted locally.
		var manifest packageManifest
		if err == nil {
			manifest, err = parsePackageManifest(content)
		}
		if err != nil {
			problems = append(problems, Problem{Path: path, Message: err.Error()})
		}
		packages = append(packages, Package{Path: filepath.ToSlash(filepath.Dir(path)), Version: manifest.Version, Name: manifest.Name, TagPrefix: manifest.TagPrefix, Prerelease: manifest.Prerelease})
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Path < packages[j].Path })
	return packages, problems, nil
}

func readPackageVersion(content []byte) (string, error) {
	manifest, err := parsePackageManifest(content)
	return manifest.Version, err
}

type packageManifest struct {
	TagPrefix    *string
	Version      string
	Name         string
	VersionFiles []VersionFile
	Prerelease   string
}

func parsePackageManifest(content []byte) (packageManifest, error) {
	var result packageManifest
	var manifest struct {
		Version    yaml.Node `yaml:"version"`
		Name       yaml.Node `yaml:"name"`
		Files      yaml.Node `yaml:"version-files"`
		TagPrefix  yaml.Node `yaml:"tag-prefix"`
		Prerelease yaml.Node `yaml:"prerelease"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&manifest); err != nil {
		return result, fmt.Errorf("decode package YAML: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return result, fmt.Errorf("expected one YAML document")
	}
	version := manifest.Version
	if version.Kind != yaml.ScalarNode || version.Tag != "!!str" || strings.TrimSpace(version.Value) == "" || strings.ContainsAny(version.Value, " \t\r\n") {
		return result, fmt.Errorf("version must be a non-empty string without whitespace")
	}
	if _, err := parseVersion(version.Value); err != nil {
		return result, err
	}
	files, err := versionFilesFromNode(manifest.Files)
	if err != nil {
		return result, err
	}
	name, err := packageNameFromNode(manifest.Name)
	if err != nil {
		return result, err
	}
	var prefix *string
	if n := manifest.TagPrefix; n.Kind != 0 {
		if n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
			return result, fmt.Errorf("tag-prefix must be a string")
		}
		if err := exec.Command("git", "check-ref-format", "refs/tags/"+n.Value+version.Value).Run(); err != nil {
			return result, fmt.Errorf("tag-prefix %q produces an invalid Git tag", n.Value)
		}
		prefix = &n.Value
	}
	var prerelease string
	if n := manifest.Prerelease; n.Kind != 0 {
		if n.Kind != yaml.ScalarNode || n.Tag != "!!str" || !prereleasePattern.MatchString(n.Value) || isNumeric(n.Value) {
			return result, fmt.Errorf("prerelease must be a non-numeric label of letters, digits, and hyphens, such as beta")
		}
		prerelease = n.Value
	}
	return packageManifest{Version: version.Value, Name: name, VersionFiles: files, TagPrefix: prefix, Prerelease: prerelease}, nil
}

var prereleasePattern = regexp.MustCompile(`^[0-9A-Za-z-]+$`)

func readPackageName(content []byte) (string, error) {
	manifest, err := parsePackageManifest(content)
	return manifest.Name, err
}

func packageNameFromNode(n yaml.Node) (string, error) {
	if n.Kind == 0 {
		return "", nil
	}
	if n.Kind != yaml.ScalarNode || n.Tag != "!!str" || strings.TrimSpace(n.Value) == "" || strings.TrimSpace(n.Value) != n.Value || strings.IndexFunc(n.Value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("name must be a non-empty single-line string without surrounding whitespace")
	}
	return n.Value, nil
}
