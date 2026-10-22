package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// LoadReleasePRStrategy reads the PR strategy. Omitted settings retain the
// combined behavior of existing repositories; InitRepository writes per-package.
func LoadReleasePRStrategy(dir string) (string, error) {
	data, path, err := loadRepositoryConfig(dir)
	if err != nil {
		return "", err
	}
	strategy, err := parseReleasePRStrategy(data)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return strategy, nil
}

func parseReleasePRStrategy(data []byte) (string, error) {
	var config struct {
		ReleasePR yaml.Node `yaml:"release-pr"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&config); err != nil {
		return "", fmt.Errorf("decode repository configuration: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", fmt.Errorf("expected one YAML document")
	}
	node := config.ReleasePR
	strategy := "combined"
	if node.Kind == 0 {
		return strategy, nil
	}
	if node.Kind != yaml.MappingNode {
		return "", fmt.Errorf("release-pr must be a mapping with a strategy setting")
	}
	seen := false
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Value != "strategy" || key.Tag != "!!str" {
			return "", fmt.Errorf("release-pr: unknown setting %q; use strategy", key.Value)
		}
		if seen {
			return "", fmt.Errorf("release-pr: duplicate strategy setting")
		}
		seen = true
		if value.Tag != "!!str" || (value.Value != "combined" && value.Value != "per-package") {
			return "", fmt.Errorf("release-pr strategy must be combined or per-package")
		}
		strategy = value.Value
	}
	return strategy, nil
}

// ReleasePR describes one preparation PR. Packages are repository-relative paths.
type ReleasePR struct {
	Branch   string   `json:"branch"`
	Title    string   `json:"title"`
	Packages []string `json:"packages"`
}

// PlanReleasePRs returns the desired PRs without changing files or contacting
// GitHub. Invalid packages or version targets fail the entire plan.
func PlanReleasePRs(root string) ([]ReleasePR, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	strategy, err := LoadReleasePRStrategy(root)
	if err != nil {
		return nil, err
	}
	statuses, err := RepositoryStatus(root)
	if err != nil {
		return nil, err
	}
	var problems []error
	for _, status := range statuses {
		for _, problem := range status.Problems {
			problems = append(problems, problem)
		}
	}
	if err := errors.Join(problems...); err != nil {
		return nil, err
	}
	prs := []ReleasePR{}
	combined := ReleasePR{Branch: "sweet/release", Title: "chore: prepare releases"}
	allFiles := map[string]*versionFilePlan{}
	type ownedReplacement struct {
		versionReplacement
		owner string
	}
	owners := map[string][]ownedReplacement{}
	for _, status := range statuses {
		if len(status.Changes) == 0 || status.Bump == "none" {
			continue
		}
		manifest, err := os.ReadFile(filepath.Join(root, status.Path, ".sweet.yml"))
		if err != nil {
			return nil, err
		}
		files := map[string]*versionFilePlan{}
		if err := planVersionFiles(root, status, manifest, files); err != nil {
			return nil, err
		}
		for path, file := range files {
			for _, replacement := range file.replacements {
				for _, previous := range owners[path] {
					if strategy == "per-package" && previous.owner != status.Path && replacement.start < previous.end && previous.start < replacement.end {
						return nil, fmt.Errorf("%s and %s update the same version token in %s; use release-pr strategy: combined", previous.owner, status.Path, path)
					}
				}
				owners[path] = append(owners[path], ownedReplacement{replacement, status.Path})
			}
			if existing := allFiles[path]; existing != nil {
				existing.replacements = append(existing.replacements, file.replacements...)
			} else {
				allFiles[path] = file
			}
		}
		combined.Packages = append(combined.Packages, status.Path)
		if strategy == "per-package" {
			prs = append(prs, ReleasePR{
				Branch:   packageReleaseBranch(status.Path),
				Title:    fmt.Sprintf("chore: release %s %s", status.Path, status.NextVersion),
				Packages: []string{status.Path},
			})
		}
	}
	if _, err := versionFileEdits(allFiles); err != nil {
		return nil, err
	}
	if strategy == "combined" && len(combined.Packages) > 0 {
		prs = append(prs, combined)
	}
	return prs, nil
}

var branchSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func packageReleaseBranch(path string) string {
	slug := strings.Trim(branchSlugPattern.ReplaceAllString(strings.ToLower(path), "-"), "-")
	if slug == "" {
		slug = "root"
	}
	if len(slug) > 48 {
		slug = strings.TrimRight(slug[:48], "-")
	}
	hash := sha256.Sum256([]byte(path))
	// A sibling of sweet/release avoids Git's branch/directory name conflict.
	return fmt.Sprintf("sweet/release-package-%s-%x", slug, hash[:8])
}
