package core

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
)

// PlannedRelease describes an already committed version increase, not a pending bump.
type PlannedRelease struct {
	Package         string `json:"package"`
	Version         string `json:"version"`
	PreviousVersion string `json:"previous_version"`
	Commit          string `json:"commit"`
	Tag             string `json:"tag"`
	Notes           string `json:"notes"`
}

// ReleasePlan reads committed files along the first-parent history from from
// (exclusive) to to (inclusive). It never changes files, tags, or remote state.
func ReleasePlan(root, path, from, to, tagPrefix string) ([]PlannedRelease, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	if filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, "../") {
		return nil, fmt.Errorf("package path must stay inside the repository")
	}
	base, err := resolveCommit(root, from)
	if err != nil {
		return nil, err
	}
	head, err := resolveCommit(root, to)
	if err != nil {
		return nil, err
	}
	releases := []PlannedRelease{}
	if base == head {
		return releases, nil
	}
	commits, err := releaseGit(root, "rev-list", "--first-parent", "--reverse", base+".."+head)
	if err != nil {
		return nil, err
	}
	list := strings.Fields(string(commits))
	if len(list) == 0 {
		return nil, fmt.Errorf("from must be a first-parent ancestor of to")
	}
	parent, err := resolveCommit(root, list[0]+"^")
	if err != nil || parent != base {
		return nil, fmt.Errorf("from must be a first-parent ancestor of to")
	}
	manifestPath := filepath.ToSlash(filepath.Join(path, ".sweet.yml"))
	previous, err := committedVersion(root, base, manifestPath)
	if err != nil {
		return nil, err
	}
	knownPackage := previous != ""
	for _, commit := range list {
		current, err := committedVersion(root, commit, manifestPath)
		if err != nil {
			return nil, err
		}
		knownPackage = knownPackage || current != ""
		if current == previous || current == "" || previous == "" {
			previous = current
			continue // Adding or removing a package is not a release.
		}
		if semver.Compare("v"+current, "v"+previous) <= 0 {
			return nil, fmt.Errorf("%s at %s: version must increase (%s -> %s)", path, commit, previous, current)
		}
		tag := tagPrefix + current
		if _, err := releaseGit(root, "check-ref-format", "refs/tags/"+tag); err != nil {
			return nil, fmt.Errorf("invalid release tag %q", tag)
		}
		changelog, err := releaseGit(root, "show", commit+":"+filepath.ToSlash(filepath.Join(path, "CHANGELOG.md")))
		if err != nil {
			return nil, fmt.Errorf("%s at %s: read changelog: %w", path, commit, err)
		}
		notes, err := releaseNotes(string(changelog), current)
		if err != nil {
			return nil, fmt.Errorf("%s at %s: %w", path, commit, err)
		}
		releases = append(releases, PlannedRelease{Package: path, Version: current, PreviousVersion: previous, Commit: commit, Tag: tag, Notes: notes})
		previous = current
	}
	if !knownPackage {
		return nil, fmt.Errorf("%s is not a package in the requested history", path)
	}
	return releases, nil
}

func releaseGit(root string, args ...string) ([]byte, error) {
	output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func resolveCommit(root, ref string) (string, error) {
	output, err := releaseGit(root, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	return strings.TrimSpace(string(output)), err
}

func committedVersion(root, commit, path string) (string, error) {
	entries, err := releaseGit(root, "ls-tree", "--name-only", "-z", commit, "--", path)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	data, err := releaseGit(root, "show", commit+":"+path)
	if err != nil {
		return "", err
	}
	version, err := readPackageVersion(data)
	if err == nil {
		_, err = nextVersion(version, "none")
	}
	if err != nil {
		return "", fmt.Errorf("%s at %s: %w", path, commit, err)
	}
	return version, nil
}

// Only semantic-version H2 headings delimit releases; other headings and fenced
// code within a note remain part of the release body.
func releaseNotes(changelog, version string) (string, error) {
	start, end, offset, count := -1, len(changelog), 0, 0
	fence := ""
	for _, line := range strings.SplitAfter(changelog, "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == "" {
				fence = ""
			}
		} else if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, trimmed[:1]))]
		} else if strings.HasPrefix(line, "## ") {
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if _, err := nextVersion(heading, "none"); err == nil {
				if start >= 0 && end == len(changelog) {
					end = offset
				}
				if heading == version {
					count++
					start = offset + len(line)
				}
			}
		}
		offset += len(line)
	}
	if count != 1 {
		return "", fmt.Errorf("changelog must contain exactly one '## %s' release heading", version)
	}
	return strings.TrimSpace(changelog[start:end]), nil
}
