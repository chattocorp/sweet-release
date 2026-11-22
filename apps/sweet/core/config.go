package core

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

const defaultRepositoryConfig = `# Sweet repository configuration
types:
  fix:
    bump: patch
    section: Fixes
  feature:
    bump: minor
    section: Features
  chore:
    bump: none
  refactor:
    bump: none
  performance:
    bump: patch
    section: Performance
`

// ChangeType defines a type's release policy. An empty Section excludes it from changelogs.
type ChangeType struct {
	Name    string
	Bump    string
	Section string
}

// RepositoryConfig contains shared release settings. ChangeTypes retains YAML order.
type RepositoryConfig struct {
	ChangeTypes []ChangeType
	Repository  string
}

type repositoryConfigNodes struct {
	Types      yaml.Node `yaml:"types"`
	Repository string    `yaml:"repository"`
}

func decodeRepositoryConfig(data []byte) (repositoryConfigNodes, error) {
	var nodes repositoryConfigNodes
	err := decodeYAMLDocument(data, &nodes, false, "repository configuration")
	return nodes, err
}

// decodeYAMLDocument decodes exactly one YAML document. Strict decoding rejects
// unknown fields.
func decodeYAMLDocument(data []byte, out any, strict bool, what string) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(strict)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", what, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected one YAML document")
	}
	return nil
}

// LoadRepositoryConfig reads and validates the repository's shared settings once.
// Missing settings use built-in types.
func LoadRepositoryConfig(dir string) (RepositoryConfig, error) {
	root, err := gitRoot(dir)
	if err != nil {
		root = dir
	}
	return snapshotConfig(snapshot{files: os.DirFS(root).(fs.ReadFileFS), configPath: filepath.Join(root, ".sweet.config.yml")})
}

// snapshotConfig reads repository settings from a snapshot's root.
func snapshotConfig(files snapshot) (RepositoryConfig, error) {
	data, ok, err := readOptional(files.files, ".sweet.config.yml")
	if err != nil {
		return RepositoryConfig{}, fmt.Errorf("read %s: %w", files.configPath, err)
	}
	if !ok {
		data = []byte("{}")
	}
	config, err := parseRepositoryConfig(data)
	if err != nil {
		return RepositoryConfig{}, fmt.Errorf("%s: %w", files.configPath, err)
	}
	return config, nil
}

func parseRepositoryConfig(data []byte) (RepositoryConfig, error) {
	nodes, err := decodeRepositoryConfig(data)
	if err != nil {
		return RepositoryConfig{}, err
	}
	types, err := changeTypes(nodes.Types)
	if err != nil {
		return RepositoryConfig{}, err
	}
	if nodes.Repository != "" && repositoryURL(nodes.Repository, false) == "" {
		return RepositoryConfig{}, fmt.Errorf("repository must be an HTTPS repository URL, for example https://github.com/owner/repo")
	}
	return RepositoryConfig{ChangeTypes: types, Repository: repositoryURL(nodes.Repository, false)}, nil
}

// CheckChangeType reports whether name is one of the configured types.
func CheckChangeType(types []ChangeType, name string) error {
	names := make([]string, len(types))
	for i, typ := range types {
		if typ.Name == name {
			return nil
		}
		names[i] = typ.Name
	}
	return fmt.Errorf("unknown change type %q; available types: %s", name, strings.Join(names, ", "))
}

var changeTypeName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var defaultPolicyCache struct {
	sync.Once
	types []ChangeType
	err   error
}

func defaultChangeTypes() ([]ChangeType, error) {
	defaultPolicyCache.Do(func() {
		nodes, err := decodeRepositoryConfig([]byte(defaultRepositoryConfig))
		if err != nil {
			defaultPolicyCache.err = err
			return
		}
		if nodes.Types.Kind == 0 {
			defaultPolicyCache.err = fmt.Errorf("default repository configuration must define types")
			return
		}
		defaultPolicyCache.types, defaultPolicyCache.err = changeTypes(nodes.Types)
	})
	return slices.Clone(defaultPolicyCache.types), defaultPolicyCache.err
}

func changeTypes(typeNode yaml.Node) ([]ChangeType, error) {
	if typeNode.Kind == 0 {
		return defaultChangeTypes()
	}
	if typeNode.Kind != yaml.MappingNode || len(typeNode.Content) == 0 {
		return nil, fmt.Errorf("types must be a non-empty mapping, for example: types: {fix: {bump: patch, section: Fixes}}")
	}
	seen := map[string]bool{}
	types := make([]ChangeType, 0, len(typeNode.Content)/2)
	for i := 0; i < len(typeNode.Content); i += 2 {
		node := typeNode.Content[i]
		name := node.Value
		if node.Tag != "!!str" || !changeTypeName.MatchString(name) {
			return nil, fmt.Errorf("invalid change type %q: use lowercase letters, digits, and hyphens, starting with a letter", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate change type %q", name)
		}
		seen[name] = true
		policy := typeNode.Content[i+1]
		if policy.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("type %q must be a mapping with bump and section settings", name)
		}
		typ := ChangeType{Name: name, Bump: "none"}
		fields := map[string]bool{}
		for j := 0; j < len(policy.Content); j += 2 {
			key, value := policy.Content[j].Value, policy.Content[j+1]
			if fields[key] {
				return nil, fmt.Errorf("type %q: duplicate setting %q", name, key)
			}
			fields[key] = true
			switch key {
			case "bump":
				if value.Tag != "!!str" {
					return nil, fmt.Errorf("type %q: bump must be none, patch, minor, or major", name)
				}
				switch value.Value {
				case "none", "patch", "minor", "major":
					typ.Bump = value.Value
				default:
					return nil, fmt.Errorf("type %q: invalid bump %q; use none, patch, minor, or major", name, value.Value)
				}
			case "section":
				if value.Tag == "!!null" {
					continue
				}
				if value.Tag != "!!str" || strings.ContainsAny(value.Value, "\r\n") {
					return nil, fmt.Errorf("type %q: section must be a single-line string or empty", name)
				}
				typ.Section = strings.TrimSpace(value.Value)
			default:
				return nil, fmt.Errorf("type %q: unknown setting %q; use bump or section", name, key)
			}
		}
		types = append(types, typ)
	}
	return types, nil
}
