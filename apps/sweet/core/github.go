package core

import (
	"bytes"
	"fmt"
	"math/big"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitHubReleaseOptions configures PublishGitHubRelease.
type GitHubReleaseOptions struct {
	Tag    string
	Assets []string
	// Title and Notes replace the defaults when non-empty.
	Title string
	Notes string
	// Draft stops after creating the release and uploading assets.
	Draft bool
	// Latest overrides the automatic Latest selection when non-nil.
	Latest *bool
}

// GitHubReleaseResult describes what PublishGitHubRelease did.
type GitHubReleaseResult struct {
	Tag        string
	Package    string
	Version    string
	Prerelease bool
	Latest     bool
	// State is the release state before this run: missing, draft, or published.
	State     string
	Published bool
}

// githubReleases is the GitHub API surface used to publish a release.
type githubReleases interface {
	// state returns missing, draft, or published.
	state(tag string) (string, error)
	create(tag, title, notes string, prerelease bool) error
	upload(tag string, assets []string) error
	publish(tag string, prerelease, latest bool) error
	// tags lists remote tag names that start with prefix.
	tags(prefix string) ([]string, error)
}

// PublishGitHubRelease creates or reuses a draft GitHub release for a Sweet tag,
// uploads assets with replacement, and publishes it unless opts.Draft is set.
// It uses the gh CLI with its usual authentication (GH_TOKEN) and repository
// detection (GH_REPO or the Git remote).
func PublishGitHubRelease(root string, opts GitHubReleaseOptions) (GitHubReleaseResult, error) {
	return publishGitHubRelease(root, ghCLI{}, opts)
}

func publishGitHubRelease(root string, gh githubReleases, opts GitHubReleaseOptions) (GitHubReleaseResult, error) {
	packages, err := DiscoverPackages(root)
	if err != nil {
		return GitHubReleaseResult{}, err
	}
	pkg, version, err := packageForTag(packages, opts.Tag)
	if err != nil {
		return GitHubReleaseResult{}, err
	}
	result := GitHubReleaseResult{Tag: opts.Tag, Package: pkg.Path, Version: version, Prerelease: isPrerelease(version)}
	if opts.Latest != nil && *opts.Latest && result.Prerelease {
		return result, fmt.Errorf("prerelease %s cannot be the latest release", opts.Tag)
	}
	if result.State, err = gh.state(opts.Tag); err != nil {
		return result, err
	}
	if result.State == "missing" {
		notes := opts.Notes
		if notes == "" {
			if notes, err = PackageChangelog(filepath.Join(root, filepath.FromSlash(pkg.Path)), version); err != nil {
				return result, err
			}
		}
		title := opts.Title
		if title == "" {
			title = opts.Tag
			if pkg.Name != "" {
				title = pkg.Name + " " + version
			}
		}
		if err := gh.create(opts.Tag, title, notes, result.Prerelease); err != nil {
			return result, err
		}
	}
	if len(opts.Assets) > 0 {
		if err := gh.upload(opts.Tag, opts.Assets); err != nil {
			return result, err
		}
	}
	if opts.Draft {
		return result, nil
	}
	switch {
	case opts.Latest != nil:
		result.Latest = *opts.Latest
	case !result.Prerelease && (pkg.Path == "." || len(packages) == 1):
		// GitHub has one Latest release per repository. Only the main package
		// (the root package, or the only package) claims it automatically.
		tags, err := gh.tags(pkg.ReleasePrefix())
		if err != nil {
			return result, err
		}
		result.Latest = isHighestStable(version, pkg.ReleasePrefix(), tags)
	}
	if err := gh.publish(opts.Tag, result.Prerelease, result.Latest); err != nil {
		return result, err
	}
	result.Published = true
	return result, nil
}

// packageForTag finds the package whose tag prefix is followed by a version.
func packageForTag(packages []Package, tag string) (Package, string, error) {
	var matches []Package
	var version string
	for _, pkg := range packages {
		rest, ok := strings.CutPrefix(tag, pkg.ReleasePrefix())
		if _, err := parseVersion(rest); ok && err == nil {
			matches = append(matches, pkg)
			version = rest
		}
	}
	switch len(matches) {
	case 0:
		return Package{}, "", fmt.Errorf("tag %q does not belong to any package", tag)
	case 1:
		return matches[0], version, nil
	}
	return Package{}, "", fmt.Errorf("tag %q matches packages %q and %q", tag, matches[0].Path, matches[1].Path)
}

// isHighestStable reports whether version is stable and no stable tag with the
// prefix has a higher version. Build metadata is ignored for precedence.
func isHighestStable(version, prefix string, tags []string) bool {
	own, err := parseVersion(version)
	if err != nil || own[4] != "" {
		return false
	}
	for _, tag := range tags {
		rest, ok := strings.CutPrefix(tag, prefix)
		other, err := parseVersion(rest)
		if !ok || err != nil || other[4] != "" {
			continue
		}
		for i := 1; i <= 3; i++ {
			a, _ := new(big.Int).SetString(own[i], 10)
			b, _ := new(big.Int).SetString(other[i], 10)
			if c := b.Cmp(a); c != 0 {
				if c > 0 {
					return false
				}
				break
			}
		}
	}
	return true
}

// ghCLI implements githubReleases with the gh command.
type ghCLI struct{}

func (ghCLI) run(stdin string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh %s %s: %w: %s", args[0], args[1], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func (g ghCLI) state(tag string) (string, error) {
	out, err := g.run("", "release", "view", tag, "--json", "isDraft", "--jq", ".isDraft")
	switch {
	case err != nil && strings.Contains(err.Error(), "release not found"):
		return "missing", nil
	case err != nil:
		return "", err
	case strings.TrimSpace(out) == "true":
		return "draft", nil
	}
	return "published", nil
}

func (g ghCLI) create(tag, title, notes string, prerelease bool) error {
	args := []string{"release", "create", tag, "--verify-tag", "--draft", "--latest=false", "--title", title, "--notes-file", "-"}
	if prerelease {
		args = append(args, "--prerelease")
	}
	_, err := g.run(notes, args...)
	return err
}

func (g ghCLI) upload(tag string, assets []string) error {
	_, err := g.run("", append([]string{"release", "upload", tag, "--clobber", "--"}, assets...)...)
	return err
}

func (g ghCLI) publish(tag string, prerelease, latest bool) error {
	_, err := g.run("", "release", "edit", tag, "--draft=false", fmt.Sprintf("--prerelease=%t", prerelease), fmt.Sprintf("--latest=%t", latest))
	return err
}

func (g ghCLI) tags(prefix string) ([]string, error) {
	segments := strings.Split(prefix, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	out, err := g.run("", "api", "--paginate", "repos/{owner}/{repo}/git/matching-refs/tags/"+strings.Join(segments, "/"), "--jq", ".[].ref")
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, ref := range strings.Fields(out) {
		tags = append(tags, strings.TrimPrefix(ref, "refs/tags/"))
	}
	return tags, nil
}
