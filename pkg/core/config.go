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

const defaultRepositoryConfig = "# Sweet repository configuration\ntypes:\n  - fix\n  - feature\n  - chore\n  - refactor\n  - performance\n"

// ChangeTypes loads shared types from the package's Git repository root.
// Missing configuration or an omitted types field uses the built-in defaults.
func ChangeTypes(dir string) ([]string, error) {
	root := dir
	if output, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output(); err == nil {
		root = strings.TrimSuffix(string(output), "\n")
	}
	path := filepath.Join(root, ".sweet.config.yml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data = []byte(defaultRepositoryConfig)
	} else if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	types, err := parseChangeTypes(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return types, nil
}

func parseChangeTypes(data []byte) ([]string, error) {
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
	if config.Types.Kind != yaml.SequenceNode || len(config.Types.Content) == 0 {
		return nil, fmt.Errorf("types must be a non-empty list of names")
	}
	valid := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	seen := map[string]bool{}
	types := make([]string, 0, len(config.Types.Content))
	for _, node := range config.Types.Content {
		name := node.Value
		if node.Tag != "!!str" || !valid.MatchString(name) {
			return nil, fmt.Errorf("invalid change type %q: use lowercase letters, digits, and hyphens, starting with a letter", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate change type %q", name)
		}
		seen[name] = true
		types = append(types, name)
	}
	return types, nil
}
