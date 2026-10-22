package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Bump prepares releases for repository-relative package paths. An empty selection
// selects all packages. It validates and prepares all edits before writing files.
func Bump(root string, paths []string) ([]PackageStatus, error) {
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
	for path, found := range wanted {
		if !found {
			problems = append(problems, fmt.Errorf("%s is not a Sweet package", path))
		}
	}
	if err := errors.Join(problems...); err != nil {
		return nil, err
	}
	var edits []releaseEdit
	versionFiles := map[string]*versionFilePlan{}
	var releases []PackageStatus
	for _, status := range selected {
		if len(status.Changes) == 0 || status.Bump == "none" {
			continue
		}
		manifest, err := readReleaseEdit(filepath.Join(root, status.Path, ".sweet.yml"), false)
		if err != nil {
			return nil, err
		}
		if err := planVersionFiles(root, status, manifest.before, versionFiles); err != nil {
			return nil, err
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
		changelog.after = []byte(insertRelease(introduction, releaseEntry(status, types)))
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
	if err := applyReleaseEdits(edits); err != nil {
		return nil, err
	}
	return releases, nil
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
				body = "**Breaking change.**\n\n" + body
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
			fmt.Fprintf(&out, "\n### %s\n\n%s\n", section, strings.Join(bodies, "\n\n"))
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
	offset := 0
	fence := ""
	for _, line := range strings.SplitAfter(existing, "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == "" {
				fence = ""
			}
		} else if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			char := trimmed[:1]
			fence = trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, char))]
		} else if strings.HasPrefix(line, "## ") {
			return existing[:offset] + entry + "\n" + existing[offset:]
		}
		offset += len(line)
	}
	return strings.TrimRight(existing, "\r\n") + "\n\n" + entry
}
