package core

import (
	"net/url"
	"os/exec"
	"regexp"
	"strings"
)

var squashPR = regexp.MustCompile(` \(#([1-9][0-9]*)\)$`)
var mergePR = regexp.MustCompile(`^Merge pull request #([1-9][0-9]*) from \S+`)
var repositoryPath = regexp.MustCompile(`^/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// repositoryURL limits inferred remotes to GitHub. Explicit URLs can also name
// GitHub-compatible hosts. Credentials and Markdown delimiters never reach output.
func repositoryURL(raw string, remote bool) string {
	if remote && strings.HasPrefix(raw, "git@github.com:") {
		raw = "https://github.com/" + strings.TrimPrefix(raw, "git@github.com:")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	if remote {
		if u.Host != "github.com" || (u.Scheme != "https" && u.Scheme != "ssh") {
			return ""
		}
	} else if u.Scheme != "https" || u.User != nil {
		return ""
	}
	path := strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), ".git")
	if !repositoryPath.MatchString(path) || strings.ContainsAny(u.Host, "<>[]()\\ \t\r\n") {
		return ""
	}
	return "https://" + u.Host + path
}

func sourceRepository(root string, config RepositoryConfig) string {
	if config.Repository != "" {
		return config.Repository
	}
	return repositoryURL(sourceGit(root, "remote", "get-url", "origin"), true)
}

func sourceGit(root string, args ...string) string {
	args = append([]string{"-C", root, "--literal-pathspecs"}, args...)
	output, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(output), "\r\n")
}

// sourceLink uses the commit that added the note to the first-parent history.
// Merge diffs are explicitly against the first parent, so a feature branch's
// individual commits resolve to the merge that brought the note onto this branch.
func sourceLink(root, repository, path string) string {
	if repository == "" || sourceGit(root, "rev-parse", "--is-shallow-repository") != "false" {
		return ""
	}
	if sourceGit(root, "ls-tree", "-z", "--name-only", "HEAD", "--", path) != path+"\x00" {
		return ""
	}
	entry := sourceGit(root, "log", "-1", "--first-parent", "--diff-merges=first-parent", "--no-patch", "--diff-filter=A", "--no-renames", "--format=%H%n%s", "HEAD", "--", path)
	sha, subject, ok := strings.Cut(entry, "\n")
	if !ok || len(sha) < 7 {
		return ""
	}
	for _, pattern := range []*regexp.Regexp{squashPR, mergePR} {
		if match := pattern.FindStringSubmatch(subject); match != nil {
			return "[#" + match[1] + "](" + repository + "/pull/" + match[1] + ")"
		}
	}
	return "[" + sha[:7] + "](" + repository + "/commit/" + sha + ")"
}
