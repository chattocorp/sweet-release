package core

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
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
    bump: minor
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
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&nodes); err != nil {
		return nodes, fmt.Errorf("decode repository configuration: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nodes, fmt.Errorf("expected one YAML document")
	}
	return nodes, nil
}

// LoadRepositoryConfig reads and validates the repository's shared settings once.
// Missing settings use built-in types.
func LoadRepositoryConfig(dir string) (RepositoryConfig, error) {
	data, path, err := loadRepositoryConfig(dir)
	if err != nil {
		return RepositoryConfig{}, err
	}
	config, err := parseRepositoryConfig(data)
	if err != nil {
		return RepositoryConfig{}, fmt.Errorf("%s: %w", path, err)
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

// ChangeTypes loads shared types from the package's Git repository root.
// Missing configuration or an omitted types field uses the built-in defaults.
func ChangeTypes(dir string) ([]string, error) {
	config, err := LoadChangeTypes(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(config))
	for i, typ := range config {
		names[i] = typ.Name
	}
	return names, nil
}

// LoadChangeTypes loads release policies in their configured display order.
func LoadChangeTypes(dir string) ([]ChangeType, error) {
	config, err := LoadRepositoryConfig(dir)
	return config.ChangeTypes, err
}

func loadRepositoryConfig(dir string) ([]byte, string, error) {
	root := dir
	if output, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output(); err == nil {
		root = strings.TrimSuffix(string(output), "\n")
	}
	path := filepath.Join(root, ".sweet.config.yml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data = []byte("{}")
	} else if err != nil {
		return nil, path, fmt.Errorf("read %s: %w", path, err)
	}
	return data, path, nil
}

func parseChangeTypes(data []byte) ([]ChangeType, error) {
	config, err := parseRepositoryConfig(data)
	return config.ChangeTypes, err
}

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
		return nil, fmt.Errorf("types must be a non-empty mapping, for example: types: {fix: {bump: minor, section: Fixes}}")
	}
	valid := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	seen := map[string]bool{}
	types := make([]ChangeType, 0, len(typeNode.Content)/2)
	for i := 0; i < len(typeNode.Content); i += 2 {
		node := typeNode.Content[i]
		name := node.Value
		if node.Tag != "!!str" || !valid.MatchString(name) {
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
