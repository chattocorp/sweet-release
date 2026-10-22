package core

import (
	"fmt"
	"os"
	"path/filepath"
)

// InitRepository creates repository configuration without overwriting existing files.
func InitRepository(root string) (string, error) {
	return createConfig(root, ".sweet.config.yml", "# Sweet repository configuration\n{}\n")
}

// InitPackage creates a package manifest without overwriting existing files.
func InitPackage(dir string) (string, error) {
	return createConfig(dir, ".sweet.yml", "# Sweet package configuration\nversion: 0.0.0\n")
}

func createConfig(dir, name, content string) (string, error) {
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", path, err)
	}
	_, writeErr := file.WriteString(content)
	closeErr := file.Close()
	if writeErr != nil {
		return "", fmt.Errorf("write %s: %w", path, writeErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close %s: %w", path, closeErr)
	}
	return path, nil
}
