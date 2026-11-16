package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Bump prepares releases for repository-relative package paths. An empty selection
// selects all packages. It validates and prepares all edits before writing files.
func Bump(root string, paths []string) ([]PackageStatus, error) {
	plan, err := prepareReleases(root, paths)
	if err != nil {
		return nil, err
	}
	if err := applyReleaseEdits(plan.edits); err != nil {
		return nil, err
	}
	return plan.statuses, nil
}

type releasePreparation struct {
	statuses []PackageStatus
	edits    []releaseEdit
}

func prepareReleases(root string, paths []string) (*releasePreparation, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	config, err := LoadRepositoryConfig(root)
	if err != nil {
		return nil, err
	}
	statuses, err := repositoryStatus(root, config)
	if err != nil {
		return nil, err
	}
	selected, err := selectReleases(statuses, paths)
	if err != nil {
		return nil, err
	}
	plan := &releasePreparation{}
	var edits []releaseEdit
	versionFiles := map[string]*versionFilePlan{}
	repository := sourceRepository(root, config)
	for _, status := range selected {
		if status.NextVersion == status.Version {
			continue
		}
		if err := planVersionFiles(root, status, versionFiles); err != nil {
			return nil, err
		}
		manifest, err := manifestEdit(filepath.Join(root, status.Path, ".sweet.yml"), status.NextVersion)
		if err != nil {
			return nil, err
		}
		changelog, err := changelogEdit(root, repository, status, config.ChangeTypes)
		if err != nil {
			return nil, err
		}
		edits = append(edits, manifest, changelog)
		for _, change := range status.Changes {
			edit, err := readReleaseEdit(filepath.Join(root, change.Path), false)
			if err != nil {
				return nil, err
			}
			edit.remove = true
			edits = append(edits, edit)
		}
		plan.statuses = append(plan.statuses, status)
	}
	fileEdits, err := versionFileEdits(versionFiles)
	if err != nil {
		return nil, err
	}
	plan.edits = append(fileEdits, edits...)
	return plan, nil
}

// selectReleases picks the statuses for the given package paths, or all of them
// without paths. It fails on problems in selected packages, on ambiguous tag
// prefixes anywhere, and on paths that are not packages.
func selectReleases(statuses []PackageStatus, paths []string) ([]PackageStatus, error) {
	wanted := map[string]bool{}
	for _, path := range paths {
		wanted[filepath.ToSlash(filepath.Clean(path))] = false
	}
	var selected []PackageStatus
	var problems []Problem
	packages := make([]Package, len(statuses))
	for i, status := range statuses {
		packages[i] = status.Package
		if _, ok := wanted[status.Path]; len(paths) > 0 && !ok {
			continue
		}
		wanted[status.Path] = true
		problems = append(problems, status.Problems...)
		selected = append(selected, status)
	}
	// Tag ownership is repository-wide, so any ambiguity blocks every release.
	problems = append(problems, TagPrefixProblems(packages)...)
	var missing []string
	for path, found := range wanted {
		if !found {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)
	errs := []error{joinProblems(problems)}
	for _, path := range missing {
		errs = append(errs, fmt.Errorf("%s is not a Sweet package", path))
	}
	return selected, errors.Join(errs...)
}

// manifestEdit sets the manifest's version, keeping its other settings.
func manifestEdit(path, version string) (releaseEdit, error) {
	edit, err := readReleaseEdit(path, false)
	if err != nil {
		return edit, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(edit.before, &doc); err != nil {
		return edit, err
	}
	mapping := doc.Content[0]
	foundVersion := false
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "version" {
			mapping.Content[i+1].Value = version
			foundVersion = true
		}
	}
	// A YAML merge can supply the old version. Override it explicitly.
	if !foundVersion {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "version"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: version})
	}
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return edit, err
	}
	if err := encoder.Close(); err != nil {
		return edit, err
	}
	edit.after = buf.Bytes()
	return edit, nil
}

// changelogEdit inserts the release entry into the package's changelog.
func changelogEdit(root, repository string, status PackageStatus, types []ChangeType) (releaseEdit, error) {
	edit, err := readReleaseEdit(filepath.Join(root, status.Path, "CHANGELOG.md"), true)
	if err != nil {
		return edit, err
	}
	introduction := string(edit.before)
	if strings.TrimSpace(introduction) == "" {
		introduction = "# " + status.DisplayName() + " Changelog\n"
	}
	links := make(map[string]string)
	for _, change := range status.Changes {
		links[change.Path] = sourceLink(root, repository, change.Path)
	}
	entry := releaseEntryWithLinks(status, types, links, prereleaseHistory(status, string(edit.before))...)
	edit.after = []byte(insertRelease(introduction, entry))
	return edit, nil
}

func applyReleaseEdits(edits []releaseEdit) error {
	for i, edit := range edits {
		if err := edit.apply(); err != nil {
			// Include the failed write: it might have truncated an existing file.
			errs := []error{err}
			for j := i; j >= 0; j-- {
				if rollbackErr := edits[j].restore(); rollbackErr != nil {
					errs = append(errs, fmt.Errorf("restore %s: %w", edits[j].path, rollbackErr))
				}
			}
			return errors.Join(errs...)
		}
	}
	return nil
}

type releaseEdit struct {
	path           string
	before, after  []byte
	mode           os.FileMode
	exists, remove bool
}

func readReleaseEdit(path string, optional bool) (releaseEdit, error) {
	e := releaseEdit{path: path, mode: 0o644}
	info, err := os.Lstat(path)
	if optional && os.IsNotExist(err) {
		return e, nil
	}
	if err != nil {
		return e, err
	}
	if !info.Mode().IsRegular() {
		return e, fmt.Errorf("%s must be a regular file", path)
	}
	e.exists, e.mode = true, info.Mode().Perm()
	e.before, err = os.ReadFile(path)
	return e, err
}

func (e releaseEdit) apply() error {
	if e.remove {
		return os.Remove(e.path)
	}
	return os.WriteFile(e.path, e.after, e.mode)
}

func (e releaseEdit) restore() error {
	if e.exists {
		return os.WriteFile(e.path, e.before, e.mode)
	}
	err := os.Remove(e.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// prereleaseHistory returns the bodies of the prerelease entries at the top of
// the changelog, newest first, when status finalizes a prerelease.
func prereleaseHistory(status PackageStatus, changelog string) []string {
	if !isPrerelease(status.Version) || isPrerelease(status.NextVersion) {
		return nil
	}
	var bodies []string
	// Stop at the first other heading, typically the last stable release.
	for _, heading := range changelogHeadings(changelog) {
		if !isPrerelease(heading.title) {
			break
		}
		if body, err := ChangelogRelease(changelog, heading.title); err == nil {
			bodies = append(bodies, body)
		}
	}
	return bodies
}

// releaseEntryWithLinks renders a changelog entry for pending notes, merged with
// the items of earlier changelog entries in history (e.g. finalized prereleases).
func releaseEntryWithLinks(status PackageStatus, types []ChangeType, links map[string]string, history ...string) string {
	sections := newReleaseSections(types)
	section := map[string]string{}
	for _, typ := range types {
		section[typ.Name] = typ.Section
	}
	for _, change := range status.Changes {
		body := strings.TrimSpace(change.Body)
		if change.Breaking {
			body = breakingMarker + body
		}
		if section[change.Type] == "" || body == "" {
			continue
		}
		lines := strings.Split(body, "\n")
		if link := links[change.Path]; link != "" {
			if len(lines) == 1 || isProse(body) {
				last := len(lines) - 1
				lines[last] = strings.TrimRight(lines[last], " \t\r") + " (" + link + ")"
			} else {
				lines = append(lines, "", "("+link+")")
			}
		}
		lines[0] = "- " + lines[0]
		for i := 1; i < len(lines); i++ {
			if lines[i] != "" {
				lines[i] = "  " + lines[i]
			}
		}
		sections.add(section[change.Type], strings.Join(lines, "\n"))
	}
	for _, body := range history {
		sections.addEntry(body)
	}
	return sections.render(status.NextVersion)
}

const breakingMarker = "⚠️ **BREAKING CHANGE:** "

// releaseSections collects changelog items by section, breaking changes first.
type releaseSections struct {
	order           []string
	breaking, other map[string][]string
}

func newReleaseSections(types []ChangeType) *releaseSections {
	r := &releaseSections{breaking: map[string][]string{}, other: map[string][]string{}}
	seen := map[string]bool{}
	for _, typ := range types {
		if typ.Section != "" && !seen[typ.Section] {
			r.order = append(r.order, typ.Section)
			seen[typ.Section] = true
		}
	}
	return r
}

func (r *releaseSections) add(section, item string) {
	if section == "" && !slices.Contains(r.order, "") {
		r.order = append([]string{""}, r.order...) // Unsectioned items come first.
	} else if !slices.Contains(r.order, section) {
		r.order = append(r.order, section)
	}
	if strings.HasPrefix(item, "- "+breakingMarker) {
		r.breaking[section] = append(r.breaking[section], item)
	} else {
		r.other[section] = append(r.other[section], item)
	}
}

// addEntry adds the items of a rendered changelog entry body. Items start with
// "- " at column zero; indented and blank lines continue the current item.
func (r *releaseSections) addEntry(body string) {
	section := ""
	var item []string
	flush := func() {
		if text := strings.TrimRight(strings.Join(item, "\n"), " \t\r\n"); text != "" {
			r.add(section, text)
		}
		item = nil
	}
	for _, line := range strings.Split(body, "\n") {
		switch {
		case strings.HasPrefix(line, "### "):
			flush()
			section = strings.TrimSpace(line[4:])
		case strings.HasPrefix(line, "- "):
			flush()
			item = []string{line}
		case item != nil || strings.TrimSpace(line) != "":
			item = append(item, line)
		}
	}
	flush()
}

func (r *releaseSections) render(version string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "## %s\n", version)
	for _, section := range r.order {
		items := append(slices.Clone(r.breaking[section]), r.other[section]...)
		if len(items) == 0 {
			continue
		}
		if section == "" {
			fmt.Fprintf(&out, "\n%s\n", strings.Join(items, "\n"))
		} else {
			fmt.Fprintf(&out, "\n### %s\n\n%s\n", section, strings.Join(items, "\n"))
		}
	}
	return out.String()
}

// Keep attribution inline for prose, including multiple soft-wrapped paragraphs,
// but leave structured Markdown untouched.
func isProse(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") ||
			strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") ||
			strings.ContainsAny(trimmed[:1], "#>|<[") || changelogListMarker.MatchString(trimmed) ||
			strings.Trim(trimmed, "-*_ =\t") == "" {
			return false
		}
	}
	return true
}

// Keep the document introduction above releases and insert before the first H2.
// Fenced code blocks in the introduction may themselves contain headings.
func insertRelease(existing, entry string) string {
	if strings.TrimSpace(existing) == "" {
		return "# Changelog\n\n" + entry
	}
	if headings := changelogHeadings(existing); len(headings) > 0 {
		offset := headings[0].start
		return existing[:offset] + entry + "\n" + existing[offset:]
	}
	return strings.TrimRight(existing, "\r\n") + "\n\n" + entry
}
