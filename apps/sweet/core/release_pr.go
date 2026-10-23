package core

import (
	"crypto/sha256"
	"fmt"
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
	config, err := decodeRepositoryConfig(data)
	if err != nil {
		return "", err
	}
	return releasePRStrategy(config.ReleasePR)
}

func releasePRStrategy(node yaml.Node) (string, error) {
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

// ReleasePRDetails adds planned content without changing the plan command schema.
type ReleasePRDetails struct {
	ReleasePR
	Releases []PlannedRelease `json:"releases"`
	Body     string           `json:"body"`
}

// PlanReleasePRs returns the desired PRs without changing files or contacting
// GitHub. Invalid packages or version targets fail the entire plan.
func PlanReleasePRs(root string) ([]ReleasePR, error) {
	details, err := PlanReleasePRDetails(root)
	if err != nil {
		return nil, err
	}
	prs := make([]ReleasePR, 0, len(details))
	for _, detail := range details {
		prs = append(prs, detail.ReleasePR)
	}
	return prs, nil
}

// PlanReleasePRDetails validates all edits and renders descriptions without writes.
func PlanReleasePRDetails(root string) ([]ReleasePRDetails, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	config, err := LoadRepositoryConfig(root)
	if err != nil {
		return nil, err
	}
	strategy := config.ReleasePRStrategy
	plan, err := prepareReleases(root, nil, strategy == "per-package", config)
	if err != nil {
		return nil, err
	}
	prs := []ReleasePRDetails{}
	combined := ReleasePR{Branch: "sweet/release", Title: "chore: prepare releases"}
	for i, status := range plan.statuses {
		combined.Packages = append(combined.Packages, status.Path)
		if strategy == "per-package" {
			pr := ReleasePR{
				Branch:   packageReleaseBranch(status.Path),
				Title:    fmt.Sprintf("chore: release %s %s", status.DisplayName(), status.NextVersion),
				Packages: []string{status.Path},
			}
			prs = append(prs, describeReleasePR(pr, plan.content[i:i+1]))
		}
	}
	if strategy == "combined" && len(combined.Packages) > 0 {
		prs = append(prs, describeReleasePR(combined, plan.content))
	}
	return prs, nil
}

func describeReleasePR(pr ReleasePR, releases []PlannedRelease) ReleasePRDetails {
	var body strings.Builder
	body.WriteString("This PR prepares the following releases:\n")
	for _, release := range releases {
		_, notes, _ := strings.Cut(release.Changelog, "\n")
		fmt.Fprintf(&body, "\n## %s\n\nTarget version: **%s**\n%s", release.DisplayName, release.Version, notes)
	}
	return ReleasePRDetails{pr, releases, body.String()}
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
