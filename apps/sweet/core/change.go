package core

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// CreateChange writes a new release note in a package's .sweet directory.
// It preserves existing notes and removes incomplete files if writing fails.
func CreateChange(dir, changeType string, content io.Reader) (string, error) {
	manifest := filepath.Join(dir, ".sweet.yml")
	data, err := os.ReadFile(manifest)
	if err != nil {
		return "", fmt.Errorf("read package manifest %s: %w", manifest, err)
	}
	if _, err := parsePackageManifest(data); err != nil {
		return "", fmt.Errorf("%s: %w", manifest, err)
	}
	types, err := ChangeTypes(dir)
	if err != nil {
		return "", err
	}
	if !slices.Contains(types, changeType) {
		return "", fmt.Errorf("unknown change type %q; available types: %s", changeType, strings.Join(types, ", "))
	}
	metadata, err := yaml.Marshal(struct {
		Type string `yaml:"type"`
	}{Type: changeType})
	if err != nil {
		return "", err
	}
	content = io.MultiReader(strings.NewReader("---\n"+string(metadata)+"---\n\n"), content)
	notes := filepath.Join(dir, ".sweet")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", notes, err)
	}
	for range 100 {
		id, err := newChangeID()
		if err != nil {
			return "", err
		}
		path := filepath.Join(notes, id+".md")
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create %s: %w", path, err)
		}
		_, writeErr := io.Copy(file, content)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			os.Remove(path)
			if writeErr != nil {
				return "", fmt.Errorf("write %s: %w", path, writeErr)
			}
			return "", fmt.Errorf("close %s: %w", path, closeErr)
		}
		return path, nil
	}
	return "", fmt.Errorf("could not find an unused release note name in %s", notes)
}

func newChangeID() (string, error) {
	groups := []string{
		"amber azure blue bronze coral crimson golden green indigo ivory jade lilac lime mint navy olive orange pink plum purple red rose ruby silver teal violet white yellow cobalt copper scarlet turquoise",
		"autumn bright calm clear cool crisp dawn dusk early fair gentle happy kind lively mellow merry misty quiet rapid silent spring steady still summer sunny swift tender warm wild winter wise young",
		"badger bear birch brook cedar cloud crane deer dove elm falcon fern finch fox grove hawk heron hill lake lark leaf lynx maple moon oak otter owl pine river robin sparrow willow",
	}
	words := make([]string, len(groups))
	for i, group := range groups {
		choices := strings.Fields(group)
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(choices))))
		if err != nil {
			return "", fmt.Errorf("generate release note name: %w", err)
		}
		words[i] = choices[index.Int64()]
	}
	return strings.Join(words, "-"), nil
}
