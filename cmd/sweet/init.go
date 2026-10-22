package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a repository or package",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "repo",
		Short: "Create .sweet.config.yml at the Git repository root",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
			if err != nil {
				return fmt.Errorf("find Git repository root: %w", err)
			}
			return createConfig(cmd, strings.TrimSuffix(string(output), "\n"), ".sweet.config.yml", "# Sweet repository configuration\n{}\n")
		},
	}, &cobra.Command{
		Use:   "package [dir]",
		Short: "Create .sweet.yml in the current or given directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			return createConfig(cmd, dir, ".sweet.yml", "# Sweet package configuration\n{}\n")
		},
	})
	return cmd
}

func createConfig(cmd *cobra.Command, dir, name, content string) error {
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	_, writeErr := file.WriteString(content)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write %s: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", path, closeErr)
	}
	cmd.Printf("Created %s\n", path)
	return nil
}
