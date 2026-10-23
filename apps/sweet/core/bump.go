package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	types := config.ChangeTypes
	wanted := map[string]bool{}
	for _, path := range paths {
		wanted[filepath.ToSlash(filepath.Clean(path))] = false
	}
	var selected []PackageStatus
	var problems []error
	for _, status := range statuses {
		if _, ok := wanted[status.Path]; len(paths) > 0 && !ok {
			continue
		}
		wanted[status.Path] = true
		for _, problem := range status.Problems {
			problems = append(problems, problem)
		}
		selected = append(selected, status)
	}
	var missing []string
	for path, found := range wanted {
		if !found {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)
	for _, path := range missing {
		problems = append(problems, fmt.Errorf("%s is not a Sweet package", path))
	}
	if err := errors.Join(problems...); err != nil {
		return nil, err
	}
	var edits []releaseEdit
	versionFiles := map[string]*versionFilePlan{}
	var releases []PackageStatus
	plan := &releasePreparation{}
	for _, status := range selected {
		if len(status.Changes) == 0 || status.Bump == "none" {
			continue
		}
		manifest, err := readReleaseEdit(filepath.Join(root, status.Path, ".sweet.yml"), false)
		if err != nil {
			return nil, err
		}
		files := map[string]*versionFilePlan{}
		if err := planVersionFiles(root, status, manifest.before, files); err != nil {
			return nil, err
		}
		filePaths := make([]string, 0, len(files))
		for path := range files {
			filePaths = append(filePaths, path)
		}
		sort.Strings(filePaths)
		for _, path := range filePaths {
			file := files[path]
			if existing := versionFiles[path]; existing != nil {
				existing.replacements = append(existing.replacements, file.replacements...)
			} else {
				versionFiles[path] = file
			}
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(manifest.before, &doc); err != nil {
			return nil, err
		}
		mapping := doc.Content[0]
		foundVersion := false
		for i := 0; i < len(mapping.Content); i += 2 {
			if mapping.Content[i].Value == "version" {
				mapping.Content[i+1].Value = status.NextVersion
				foundVersion = true
			}
		}
		// A YAML merge can supply the old version. Override it explicitly.
		if !foundVersion {
			mapping.Content = append(mapping.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "version"},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: status.NextVersion})
		}
		var buf bytes.Buffer
		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)
		if err := encoder.Encode(&doc); err != nil {
			return nil, err
		}
		if err := encoder.Close(); err != nil {
			return nil, err
		}
		manifest.after = buf.Bytes()
		changelog, err := readReleaseEdit(filepath.Join(root, status.Path, "CHANGELOG.md"), true)
		if err != nil {
			return nil, err
		}
		introduction := string(changelog.before)
		if strings.TrimSpace(introduction) == "" {
			introduction = "# " + status.DisplayName() + " Changelog\n"
		}
		entry := releaseEntry(status, types)
		changelog.after = []byte(insertRelease(introduction, entry))
		edits = append(edits, manifest, changelog)
		for _, change := range status.Changes {
			edit, err := readReleaseEdit(filepath.Join(root, change.Path), false)
			if err != nil {
				return nil, err
			}
			edit.remove = true
			edits = append(edits, edit)
		}
		releases = append(releases, status)
	}
	fileEdits, err := versionFileEdits(versionFiles)
	if err != nil {
		return nil, err
	}
	edits = append(fileEdits, edits...)
	plan.edits, plan.statuses = edits, releases
	return plan, nil
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

func releaseEntry(status PackageStatus, types []ChangeType) string {
	var out strings.Builder
	fmt.Fprintf(&out, "## %s\n", status.NextVersion)
	var sections []string
	byType := map[string]string{}
	seen := map[string]bool{}
	for _, typ := range types {
		byType[typ.Name] = typ.Section
		if typ.Section != "" && !seen[typ.Section] {
			sections = append(sections, typ.Section)
			seen[typ.Section] = true
		}
	}
	for _, section := range sections {
		var bodies []string
		for _, change := range status.Changes {
			if byType[change.Type] != section {
				continue
			}
			body := strings.TrimSpace(change.Body)
			if change.Breaking {
				body = "⚠️ **BREAKING CHANGE:** " + body
			}
			if body != "" {
				lines := strings.Split(body, "\n")
				lines[0] = "- " + lines[0]
				for i := 1; i < len(lines); i++ {
					if lines[i] != "" {
						lines[i] = "  " + lines[i]
					}
				}
				bodies = append(bodies, strings.Join(lines, "\n"))
			}
		}
		if len(bodies) > 0 {
			fmt.Fprintf(&out, "\n### %s\n\n%s\n", section, strings.Join(bodies, "\n"))
		}
	}
	return out.String()
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
