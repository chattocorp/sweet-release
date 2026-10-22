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
)

// Package identifies a release package by its repository-relative path.
type Package struct {
	Path    string
	Version string
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
	// Include tracked and new files, while respecting Git ignore rules.
	output, err := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return nil, nil, fmt.Errorf("discover package manifests: %w", err)
	}
	var packages []Package
	var problems []Problem
	seen := make(map[string]bool)
	for _, path := range strings.Split(string(output), "\x00") {
		if filepath.Base(path) != ".sweet.yml" || seen[path] {
			continue
		}
		seen[path] = true
		content, err := os.ReadFile(filepath.Join(root, path))
		if os.IsNotExist(err) {
			continue
		} // A tracked manifest may have been deleted locally.
		var version string
		if err == nil {
			version, err = readPackageVersion(content)
		}
		if err != nil {
			problems = append(problems, Problem{Path: path, Message: err.Error()})
		}
		packages = append(packages, Package{Path: filepath.ToSlash(filepath.Dir(path)), Version: version})
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Path < packages[j].Path })
	return packages, problems, nil
}

func readPackageVersion(content []byte) (string, error) {
	var manifest struct {
		Version yaml.Node `yaml:"version"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&manifest); err != nil {
		return "", fmt.Errorf("decode package YAML: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", fmt.Errorf("expected one YAML document")
	}
	version := manifest.Version
	if version.Kind != yaml.ScalarNode || version.Tag != "!!str" || strings.TrimSpace(version.Value) == "" || strings.ContainsAny(version.Value, " \t\r\n") {
		return "", fmt.Errorf("version must be a non-empty string without whitespace")
	}
	if _, err := readVersionFiles(content); err != nil {
		return "", err
	}
	return version.Value, nil
}
