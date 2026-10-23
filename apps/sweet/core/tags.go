package core

import (
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strings"
)

// ReleaseTag identifies a version transition on the first-parent release history.
type ReleaseTag struct {
	Package string `json:"package"`
	Version string `json:"version"`
	Name    string `json:"name"`
	Commit  string `json:"commit"`
	Exists  bool   `json:"exists"`
}

func tagGit(root string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func tagCommit(root, ref string) (string, error) {
	out, err := tagGit(root, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	return strings.TrimSpace(out), err
}

// PlanTags plans tags for version changes between two commits, without writing
// tags. Fetch remote tags first so existing-tag checks include the remote state.
// A zero base (GitHub's branch-creation event) examines only the head snapshot.
func PlanTags(root, base, head string) ([]ReleaseTag, error) {
	head, err := tagCommit(root, head)
	if err != nil {
		return nil, err
	}
	commits := []string{head}
	previous := map[string]string{}
	zeroBase := (len(base) == 40 || len(base) == 64) && strings.Trim(base, "0") == ""
	if !zeroBase {
		base, err = tagCommit(root, base)
		if err != nil {
			return nil, err
		}
		history, err := tagGit(root, "rev-list", "--first-parent", head)
		if err != nil {
			return nil, err
		}
		commits = nil
		found := false
		for _, commit := range strings.Fields(history) {
			if commit == base {
				found = true
				break
			}
			commits = append(commits, commit)
		}
		if !found {
			return nil, fmt.Errorf("base is not on head's first-parent history; refusing to tag a rewritten or unrelated history")
		}
		for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
			commits[i], commits[j] = commits[j], commits[i]
		}
		previous, err = tagVersions(root, base)
		if err != nil {
			return nil, err
		}
	}
	tags := []ReleaseTag{}
	seen := map[string]string{}
	for _, commit := range commits {
		versions, err := tagVersions(root, commit)
		if err != nil {
			return nil, err
		}
		var packages []string
		for pkg := range versions {
			packages = append(packages, pkg)
		}
		sort.Strings(packages)
		for _, pkg := range packages {
			version := versions[pkg]
			if version == previous[pkg] || version == "0.0.0" {
				continue
			}
			name := path.Join(pkg, "v"+version)
			if _, err := tagGit(root, "check-ref-format", "refs/tags/"+name); err != nil {
				return nil, fmt.Errorf("package %q produces an invalid tag %q: %w", pkg, name, err)
			}
			if earlier, ok := seen[name]; ok && earlier != commit {
				return nil, fmt.Errorf("tag %s is requested for multiple commits", name)
			}
			seen[name] = commit
			tag := ReleaseTag{Package: pkg, Version: version, Name: name, Commit: commit}
			refs, err := tagGit(root, "for-each-ref", "--format=%(refname)", "refs/tags/"+name)
			if err != nil {
				return nil, err
			}
			for _, ref := range strings.Fields(refs) {
				if ref != "refs/tags/"+name {
					continue
				}
				target, err := tagCommit(root, ref)
				if err != nil {
					return nil, err
				}
				if target != commit {
					return nil, fmt.Errorf("tag %s already points to %s, expected %s; refusing to move it", name, target, commit)
				}
				tag.Exists = true
			}
			tags = append(tags, tag)
		}
		previous = versions
	}
	return tags, nil
}

func tagVersions(root, commit string) (map[string]string, error) {
	tree, err := tagGit(root, "ls-tree", "-r", "-z", commit)
	if err != nil {
		return nil, err
	}
	versions := map[string]string{}
	for _, record := range strings.Split(tree, "\x00") {
		metadata, file, ok := strings.Cut(record, "\t")
		if !ok || path.Base(file) != ".sweet.yml" {
			continue
		}
		fields := strings.Fields(metadata)
		if len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755") {
			return nil, fmt.Errorf("%s at %s must be a regular package manifest", file, commit)
		}
		content, err := tagGit(root, "cat-file", "blob", fields[2])
		if err != nil {
			return nil, err
		}
		manifest, err := parsePackageManifestNames([]byte(content), true)
		version := manifest.Version
		if err == nil {
			_, err = nextVersion(version, "none")
		}
		if err != nil {
			return nil, fmt.Errorf("%s at %s: %w", file, commit, err)
		}
		versions[path.Dir(file)] = version
	}
	return versions, nil
}
