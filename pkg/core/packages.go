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
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Package identifies a release package by its repository-relative path.
type Package struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Name    string `json:"name,omitempty"`
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
		var version string
		var name string
		if err == nil {
			var manifest packageManifest
			manifest, err = parsePackageManifest(content)
			version, name = manifest.Version, manifest.Name
		}
		if err != nil {
			problems = append(problems, Problem{Path: path, Message: err.Error()})
		}
		packages = append(packages, Package{Path: filepath.ToSlash(filepath.Dir(path)), Version: version, Name: name})
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Path < packages[j].Path })
	return packages, problems, nil
}

func readPackageVersion(content []byte) (string, error) {
	manifest, err := parsePackageManifest(content)
	return manifest.Version, err
}

type packageManifest struct {
	Version      string
	Name         string
	VersionFiles []VersionFile
}

func parsePackageManifest(content []byte) (packageManifest, error) {
	var result packageManifest
	var manifest struct {
		Version yaml.Node `yaml:"version"`
		Name    yaml.Node `yaml:"name"`
		Files   yaml.Node `yaml:"version-files"`
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
	if _, err := nextVersion(version.Value, "none"); err != nil {
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
	return packageManifest{Version: version.Value, Name: name, VersionFiles: files}, nil
}

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
