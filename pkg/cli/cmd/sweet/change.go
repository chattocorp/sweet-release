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
	return newChangeCommandWithPrompt(promptDescription)
}

func newChangeCommandWithPrompt(prompt func(*cobra.Command) (string, error)) *cobra.Command {
	var stdin bool
	var changeType string
	cmd := &cobra.Command{
		Use:   "change [package]",
		Short: "Create a release note in a package directory",
		Long:  "Create a release note in a package directory (the current directory by default).\nEnter a one-line description, leave it empty to open $EDITOR, or use --stdin to read Markdown.",
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
			policies, err := core.LoadChangeTypes(dir)
			if err != nil {
				return err
			}
			types := make([]string, len(policies))
			for i, policy := range policies {
				types[i] = policy.Name
			}
			selectedType := changeType
			if selectedType == "" {
				selectedType, err = selectChangeType(cmd, policies)
				if err != nil {
					return err
				}
			}
			if !slices.Contains(types, selectedType) {
				return fmt.Errorf("unknown change type %q; available types: %s", selectedType, strings.Join(types, ", "))
			}
			var content io.Reader = strings.NewReader("")
			useEditor := false
			editor := os.Getenv("EDITOR")
			if stdin {
				content = cmd.InOrStdin()
			} else {
				description, err := prompt(cmd)
				if err != nil {
					return err
				}
				description = strings.TrimSpace(description)
				useEditor = description == ""
				if useEditor {
					if strings.TrimSpace(editor) == "" {
						return fmt.Errorf("empty description: set EDITOR to open an editor, or enter a description")
					}
				} else {
					content = strings.NewReader(description + "\n")
				}
			}
			path, err := core.CreateChange(dir, selectedType, content)
			if err != nil {
				return err
			}
			if useEditor {
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
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read the Markdown body from standard input instead of prompting")
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
		return fmt.Errorf("%s\n\nTo set up Sweet in a new project:\n\n  sweet init repo       Create shared configuration at the Git repository root.\n  sweet init package    Make the current directory a release package.\n\nThen run 'sweet' again to write a release note.\nIf repository configuration already exists, skip 'sweet init repo'.\nFor an existing package elsewhere, run 'sweet change <dir>'.", message)
	}
	return fmt.Errorf("%s\n\nPass an existing package directory, or initialize this directory with 'sweet init package <dir>'.", message)
}
