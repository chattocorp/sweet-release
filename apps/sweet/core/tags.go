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
// An all-zero base examines only the head snapshot.
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
			if err := checkReleaseTag(root, &tag); err != nil {
				return nil, err
			}
			tags = append(tags, tag)
		}
		previous = versions
	}
	return tags, nil
}

func checkReleaseTag(root string, tag *ReleaseTag) error {
	tag.Exists = false
	if _, err := tagGit(root, "check-ref-format", "refs/tags/"+tag.Name); err != nil {
		return err
	}
	refs, err := tagGit(root, "for-each-ref", "--format=%(refname)", "refs/tags/"+tag.Name)
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(refs) {
		if ref != "refs/tags/"+tag.Name {
			continue
		}
		target, err := tagCommit(root, ref)
		if err != nil {
			return err
		}
		if target != tag.Commit {
			return fmt.Errorf("tag %s already points to %s, expected %s; refusing to move it", tag.Name, target, tag.Commit)
		}
		tag.Exists = true
	}
	return nil
}

// PlanCurrentTags finds the first-parent introduction of each current release.
// It ignores working files and does not backfill previous package versions.
func PlanCurrentTags(root string) ([]ReleaseTag, error) {
	shallow, err := tagGit(root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(shallow) == "true" {
		return nil, fmt.Errorf("tagging current releases requires full Git history; fetch with --unshallow first")
	}
	head, err := tagCommit(root, "HEAD")
	if err != nil {
		return nil, err
	}
	versions, err := tagVersions(root, head)
	if err != nil {
		return nil, err
	}
	pending := map[string]ReleaseTag{}
	for pkg, version := range versions {
		if version != "0.0.0" {
			pending[pkg] = ReleaseTag{Package: pkg, Version: version, Name: path.Join(pkg, "v"+version), Commit: head}
		}
	}
	history, err := tagGit(root, "rev-list", "--first-parent", head)
	if err != nil {
		return nil, err
	}
	tags := []ReleaseTag{}
	for _, commit := range strings.Fields(history)[1:] {
		if len(pending) == 0 {
			break
		}
		previous, err := tagVersions(root, commit)
		if err != nil {
			return nil, err
		}
		for pkg, tag := range pending {
			if previous[pkg] != tag.Version {
				tags = append(tags, tag)
				delete(pending, pkg)
			} else {
				tag.Commit = commit
				pending[pkg] = tag
			}
		}
	}
	for _, tag := range pending {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Package < tags[j].Package })
	for i := range tags {
		if err := checkReleaseTag(root, &tags[i]); err != nil {
			return nil, err
		}
	}
	return tags, nil
}

// CreateReleaseTags validates the complete plan before creating annotated local
// tags. Existing tags are retained. Git failures can leave earlier tags created;
// rerunning the command safely completes the remaining tags.
func CreateReleaseTags(root string, tags []ReleaseTag) error {
	for i := range tags {
		if err := checkReleaseTag(root, &tags[i]); err != nil {
			return err
		}
	}
	for _, tag := range tags {
		if !tag.Exists {
			if _, err := tagGit(root, "tag", "--annotate", "--message", "Release "+tag.Name, "--", tag.Name, tag.Commit); err != nil {
				return err
			}
		}
	}
	return nil
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
		version, err := readPackageVersion([]byte(content))
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
