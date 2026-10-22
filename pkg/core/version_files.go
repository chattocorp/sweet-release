package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
	"go.yaml.in/yaml/v3"
)

// VersionFile is an explicit destination for a package's next version.
type VersionFile struct {
	Path   string    `yaml:"path"`
	Format string    `yaml:"format"`
	Key    yaml.Node `yaml:"key"`
}

func readVersionFiles(data []byte) ([]VersionFile, error) {
	var manifest struct {
		Files yaml.Node `yaml:"version-files"`
	}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if manifest.Files.Kind == 0 {
		return nil, nil
	}
	if manifest.Files.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("version-files must be a list")
	}
	var files []VersionFile
	for _, node := range manifest.Files.Content {
		// Decode entries strictly while allowing unrelated package metadata.
		encoded, err := yaml.Marshal(node)
		if err != nil {
			return nil, err
		}
		decoder := yaml.NewDecoder(bytes.NewReader(encoded))
		decoder.KnownFields(true)
		var file VersionFile
		if err := decoder.Decode(&file); err != nil {
			return nil, fmt.Errorf("version-files: %w", err)
		}
		if strings.TrimSpace(file.Path) == "" || filepath.IsAbs(file.Path) {
			return nil, fmt.Errorf("version-files path must be a non-empty relative path")
		}
		switch file.Format {
		case "json", "yaml", "toml":
			if _, err := file.keyParts(); err != nil {
				return nil, fmt.Errorf("%s: %w", file.Path, err)
			}
		case "text", "annotated":
			if file.Key.Kind != 0 {
				return nil, fmt.Errorf("%s: key is only supported for json, yaml, and toml", file.Path)
			}
		default:
			return nil, fmt.Errorf("%s: format must be json, yaml, toml, text, or annotated", file.Path)
		}
		files = append(files, file)
	}
	return files, nil
}

func (f VersionFile) keyParts() ([]string, error) {
	var keys []string
	switch f.Key.Kind {
	case yaml.ScalarNode:
		if f.Key.Tag == "!!str" {
			keys = strings.Split(f.Key.Value, ".")
		}
	case yaml.SequenceNode:
		for _, part := range f.Key.Content {
			if part.Kind != yaml.ScalarNode || part.Tag != "!!str" {
				return nil, fmt.Errorf("key segments must be strings")
			}
			keys = append(keys, part.Value)
		}
	}
	if len(keys) == 0 || slices.Contains(keys, "") {
		return nil, fmt.Errorf("key must be a dotted string or a non-empty list of non-empty string segments")
	}
	return keys, nil
}

type versionReplacement struct {
	start, end int
	value      string
}

type versionFilePlan struct {
	edit         releaseEdit
	replacements []versionReplacement
}

func planVersionFiles(root string, status PackageStatus, manifest []byte, plans map[string]*versionFilePlan) error {
	targets, err := readVersionFiles(manifest)
	if err != nil {
		return err
	}
	for _, target := range targets {
		path := filepath.Join(root, status.Path, target.Path)
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%s: version-file path must stay inside the repository", target.Path)
		}
		for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
			if part == ".git" || part == ".sweet" || part == ".sweet.yml" || part == ".sweet.config.yml" || part == "CHANGELOG.md" {
				return fmt.Errorf("%s: version-file target is reserved for Git or Sweet", rel)
			}
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if resolved != path {
			return fmt.Errorf("%s: version-file paths must not contain symlinks", rel)
		}
		plan := plans[path]
		if plan == nil {
			edit, err := readReleaseEdit(path, false)
			if err != nil {
				return err
			}
			plan = &versionFilePlan{edit: edit}
			plans[path] = plan
		}
		replacements, err := versionReplacements(plan.edit.before, target, status.NextVersion)
		if err != nil {
			return fmt.Errorf("%s (%s): %w", rel, status.Path, err)
		}
		plan.replacements = append(plan.replacements, replacements...)
	}
	return nil
}

func versionFileEdits(plans map[string]*versionFilePlan) ([]releaseEdit, error) {
	var paths []string
	for path := range plans {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var edits []releaseEdit
	for _, path := range paths {
		plan := plans[path]
		data, err := applyVersionReplacements(plan.edit.before, plan.replacements)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		plan.edit.after = data
		edits = append(edits, plan.edit)
	}
	return edits, nil
}

func versionReplacements(data []byte, target VersionFile, version string) ([]versionReplacement, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("version files must use UTF-8 encoding")
	}
	keys, _ := target.keyParts() // Validated when the manifest is read.
	switch target.Format {
	case "json", "yaml":
		replacement, err := yamlVersionReplacement(data, keys, version, target.Format == "json")
		return []versionReplacement{replacement}, err
	case "toml":
		replacement, err := tomlVersionReplacement(data, keys, version)
		return []versionReplacement{replacement}, err
	case "text":
		old := strings.TrimSpace(string(data))
		if _, err := nextVersion(old, "none"); err != nil {
			return nil, fmt.Errorf("text file must contain only a semantic version: %w", err)
		}
		start := bytes.Index(data, []byte(old))
		return []versionReplacement{{start, start + len(old), version}}, nil
	case "annotated":
		return annotatedReplacements(data, version)
	}
	return nil, fmt.Errorf("unsupported version-file format %q", target.Format)
}

// YAML nodes also provide source positions for JSON, after strict JSON validation.
// Only the scalar token is replaced, preserving all other bytes in the document.
func yamlVersionReplacement(data []byte, keys []string, version string, isJSON bool) (versionReplacement, error) {
	var zero versionReplacement
	if isJSON && !json.Valid(data) {
		return zero, fmt.Errorf("invalid JSON")
	}
	var doc yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&doc); err != nil {
		return zero, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return zero, fmt.Errorf("expected one YAML document")
	}
	// Decoding validates duplicate mapping keys as well as YAML syntax.
	var decoded any
	if err := doc.Decode(&decoded); err != nil {
		return zero, err
	}
	if len(doc.Content) != 1 {
		return zero, fmt.Errorf("expected a mapping")
	}
	node := doc.Content[0]
	for _, key := range keys {
		if node.Anchor != "" {
			return zero, fmt.Errorf("version target must not be inside an anchored mapping")
		}
		if node.Kind != yaml.MappingNode {
			return zero, fmt.Errorf("key %q does not address a mapping", key)
		}
		var child *yaml.Node
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Tag == "!!str" && node.Content[i].Value == key {
				child = node.Content[i+1]
			}
		}
		if child == nil {
			return zero, fmt.Errorf("missing key %q", key)
		}
		node = child
	}
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" || node.Anchor != "" || node.Style&(yaml.TaggedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return zero, fmt.Errorf("version target must be a plain or quoted string without anchors, aliases, or explicit tags")
	}
	if _, err := nextVersion(node.Value, "none"); err != nil {
		return zero, err
	}
	start := 0
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		start = 3 // YAML source columns exclude the UTF-8 byte order mark.
	}
	for line := 1; line < node.Line; line++ {
		n := bytes.IndexByte(data[start:], '\n')
		if n < 0 {
			return zero, fmt.Errorf("invalid scalar position")
		}
		start += n + 1
	}
	for column := 1; column < node.Column; column++ {
		_, size := utf8.DecodeRune(data[start:])
		start += size
	}
	end := start + len(node.Value)
	value := version
	if data[start] == '"' || data[start] == '\'' {
		quote := data[start]
		end = start + 1
		for end < len(data) {
			if quote == '"' && data[end] == '\\' {
				end += 2
				continue
			}
			if data[end] == quote {
				if quote == '\'' && end+1 < len(data) && data[end+1] == quote {
					end += 2
					continue
				}
				end++
				break
			}
			end++
		}
		value = string(quote) + version + string(quote)
	}
	return versionReplacement{start, end, value}, nil
}

func tomlVersionReplacement(data []byte, keys []string, version string) (versionReplacement, error) {
	var zero versionReplacement
	var decoded map[string]any
	if err := toml.Unmarshal(data, &decoded); err != nil {
		return zero, err
	}
	var value any = decoded
	for _, key := range keys {
		mapping, ok := value.(map[string]any)
		if !ok {
			return zero, fmt.Errorf("key %q does not address a table", key)
		}
		value, ok = mapping[key]
		if !ok {
			return zero, fmt.Errorf("missing key %q", key)
		}
	}
	old, ok := value.(string)
	if !ok {
		return zero, fmt.Errorf("version target must be a string")
	}
	if _, err := nextVersion(old, "none"); err != nil {
		return zero, err
	}
	var parser unstable.Parser
	parser.Reset(data)
	var table []string
	var matches []versionReplacement
	var visit func(*unstable.Node, []string)
	visit = func(node *unstable.Node, prefix []string) {
		path := slices.Clone(prefix)
		for it := node.Key(); it.Next(); {
			path = append(path, string(it.Node().Data))
		}
		v := node.Value()
		if v.Kind == unstable.InlineTable {
			for it := v.Children(); it.Next(); {
				visit(it.Node(), path)
			}
		} else if slices.Equal(path, keys) && v.Kind == unstable.String {
			raw := parser.Raw(v.Raw)
			quote := string(raw[:1])
			if bytes.HasPrefix(raw, []byte("\"\"\"")) || bytes.HasPrefix(raw, []byte("'''")) {
				quote = string(raw[:3])
			}
			matches = append(matches, versionReplacement{int(v.Raw.Offset), int(v.Raw.Offset + v.Raw.Length), quote + version + quote})
		}
	}
	for parser.NextExpression() {
		node := parser.Expression()
		switch node.Kind {
		case unstable.Table, unstable.ArrayTable:
			table = nil
			for it := node.Key(); it.Next(); {
				table = append(table, string(it.Node().Data))
			}
		case unstable.KeyValue:
			visit(node, table)
		}
	}
	if err := parser.Error(); err != nil {
		return zero, err
	}
	if len(matches) != 1 {
		return zero, fmt.Errorf("key must identify exactly one string field")
	}
	return matches[0], nil
}

var annotationMarker = regexp.MustCompile(`\bsweet-version\b`)
var embeddedVersion = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?`)

func annotatedReplacements(data []byte, version string) ([]versionReplacement, error) {
	var replacements []versionReplacement
	offset := 0
	for number, line := range bytes.SplitAfter(data, []byte("\n")) {
		if annotationMarker.Match(line) {
			matches := embeddedVersion.FindAllIndex(line, -1)
			if len(matches) != 1 {
				return nil, fmt.Errorf("line %d: sweet-version requires exactly one semantic version", number+1)
			}
			match := matches[0]
			old := string(line[match[0]:match[1]])
			if _, err := nextVersion(old, "none"); err != nil {
				return nil, fmt.Errorf("line %d: %w", number+1, err)
			}
			// Do not replace a substring of another token (for example 1.2.3.4).
			isVersionChar := func(c byte) bool {
				return strings.ContainsRune("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz._+-", rune(c))
			}
			start := match[0]
			if start > 0 && line[start-1] == 'v' {
				start--
			}
			if (start > 0 && isVersionChar(line[start-1])) || (match[1] < len(line) && isVersionChar(line[match[1]])) {
				return nil, fmt.Errorf("line %d: version must be a complete token", number+1)
			}
			replacements = append(replacements, versionReplacement{offset + match[0], offset + match[1], version})
		}
		offset += len(line)
	}
	if len(replacements) == 0 {
		return nil, fmt.Errorf("no sweet-version annotations found")
	}
	return replacements, nil
}

func applyVersionReplacements(data []byte, replacements []versionReplacement) ([]byte, error) {
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start < replacements[j].start })
	var out bytes.Buffer
	end := 0
	var previous versionReplacement
	for i, replacement := range replacements {
		if i > 0 && replacement == previous {
			continue
		}
		if replacement.start < end {
			return nil, fmt.Errorf("conflicting version-file updates")
		}
		out.Write(data[end:replacement.start])
		out.WriteString(replacement.value)
		end, previous = replacement.end, replacement
	}
	out.Write(data[end:])
	return out.Bytes(), nil
}
