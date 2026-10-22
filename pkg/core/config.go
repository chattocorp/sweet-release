package core

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

const defaultRepositoryConfig = `# Sweet repository configuration
release-pr:
  strategy: per-package
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
	data, path, err := loadRepositoryConfig(dir)
	if err != nil {
		return nil, err
	}
	types, err := parseChangeTypes(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return types, nil
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
	if _, err := parseReleasePRStrategy(data); err != nil {
		return nil, err
	}
	var config struct {
		Types yaml.Node `yaml:"types"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("decode repository configuration: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("expected one YAML document")
	}
	if config.Types.Kind == 0 {
		return parseChangeTypes([]byte(defaultRepositoryConfig))
	}
	if config.Types.Kind != yaml.MappingNode || len(config.Types.Content) == 0 {
		return nil, fmt.Errorf("types must be a non-empty mapping, for example: types: {fix: {bump: minor, section: Fixes}}")
	}
	valid := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	seen := map[string]bool{}
	types := make([]ChangeType, 0, len(config.Types.Content)/2)
	for i := 0; i < len(config.Types.Content); i += 2 {
		node := config.Types.Content[i]
		name := node.Value
		if node.Tag != "!!str" || !valid.MatchString(name) {
			return nil, fmt.Errorf("invalid change type %q: use lowercase letters, digits, and hyphens, starting with a letter", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate change type %q", name)
		}
		seen[name] = true
		policy := config.Types.Content[i+1]
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
