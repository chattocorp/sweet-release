package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chattocorp/sweet-release/pkg/core"
	"github.com/spf13/cobra"
)

func newChangeCommand() *cobra.Command {
	var stdin bool
	var changeType string
	cmd := &cobra.Command{
		Use:   "change [package]",
		Short: "Create a release note in a package directory",
		Long:  "Create a release note in a package directory (the current directory by default).\nOpen it with $EDITOR, or use --stdin to read Markdown from standard input.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			dir, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(dir, ".sweet.yml")); os.IsNotExist(err) {
				selected := ""
				if len(args) == 0 {
					selected, err = selectChangePackage(cmd)
					if err != nil {
						return err
					}
				}
				if selected == "" {
					return missingChangePackage(dir, len(args) == 0, stdin)
				}
				dir = selected
			} else if err != nil {
				return fmt.Errorf("check package %s: %w", dir, err)
			}
			editor := os.Getenv("EDITOR")
			if !stdin && strings.TrimSpace(editor) == "" {
				return fmt.Errorf("set EDITOR or use --stdin to supply a release note")
			}
			types, err := core.ChangeTypes(dir)
			if err != nil {
				return err
			}
			selectedType := changeType
			if selectedType == "" {
				selectedType, err = selectChangeType(cmd, types)
				if err != nil {
					return err
				}
			}
			if !slices.Contains(types, selectedType) {
				return fmt.Errorf("unknown change type %q; available types: %s", selectedType, strings.Join(types, ", "))
			}
			var content io.Reader = strings.NewReader("")
			if stdin {
				content = cmd.InOrStdin()
			}
			path, err := core.CreateChange(dir, selectedType, content)
			if err != nil {
				return err
			}
			if !stdin {
				// Interpret EDITOR as a shell command (for example, code --wait).
				// Pass the filename separately so paths cannot become shell syntax.
				process := exec.Command("sh", "-c", editor+` "$@"`, "sweet-editor", path)
				process.Stdin = cmd.InOrStdin()
				process.Stdout = cmd.OutOrStdout()
				process.Stderr = cmd.ErrOrStderr()
				if err := process.Run(); err != nil {
					return fmt.Errorf("editor failed; release note kept at %s: %w", path, err)
				}
			}
			cmd.Printf("Created %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read the Markdown body from standard input instead of opening EDITOR")
	cmd.Flags().StringVar(&changeType, "type", "", "Change type (opens a type picker when omitted)")
	return cmd
}

func missingChangePackage(dir string, current, stdin bool) error {
	message := fmt.Sprintf("%q is not a Sweet package (no .sweet.yml).", dir)
	if current {
		message = "the current directory is not a Sweet package (no .sweet.yml)."
	}
	root, err := core.RepositoryRoot()
	if err == nil {
		packages, discoveryErr := core.DiscoverPackages(root)
		if discoveryErr != nil {
			return fmt.Errorf("%s\n\nCould not list available packages: %w", message, discoveryErr)
		}
		if len(packages) > 0 {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			cwd, err = filepath.EvalSymlinks(cwd)
			if err != nil {
				return err
			}
			root, err = filepath.EvalSymlinks(root)
			if err != nil {
				return err
			}
			message += "\n\nChoose a package with one of these commands:\n"
			for _, pkg := range packages {
				path, err := filepath.Rel(cwd, filepath.Join(root, pkg.Path))
				if err != nil {
					return err
				}
				command := "sweet change"
				if stdin {
					command += " --stdin"
				}
				// Quote paths for the shell, including spaces and apostrophes.
				message += "\n  " + command + " -- '" + strings.ReplaceAll(filepath.ToSlash(path), "'", "'\\''") + "'"
			}
			return fmt.Errorf("%s", message)
		}
	}
	if current {
		return fmt.Errorf("%s\n\nRun 'sweet init package' here to create a package, or pass an existing package directory to 'sweet change'.", message)
	}
	return fmt.Errorf("%s\n\nPass an existing package directory, or initialize this directory with 'sweet init package <dir>'.", message)
}
