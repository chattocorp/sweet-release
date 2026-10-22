package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chattocorp/sweet-release/pkg/core"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// selectChangePackage returns an empty path when interactive selection is unavailable.
func selectChangePackage(cmd *cobra.Command) (string, error) {
	terminal, ok := cmd.ErrOrStderr().(*os.File)
	if !ok || !term.IsTerminal(int(terminal.Fd())) {
		return "", nil
	}
	fzf, err := exec.LookPath("fzf")
	if err != nil {
		return "", nil
	}
	root, err := core.RepositoryRoot()
	if err != nil {
		return "", nil
	}
	packages, err := core.DiscoverPackages(root)
	if err != nil {
		return "", err
	}
	if len(packages) == 0 {
		return "", nil
	}
	return pickPackage(fzf, root, packages, terminal)
}

func pickPackage(fzf, root string, packages []core.Package, terminal io.Writer) (string, error) {
	paths := make([]string, len(packages))
	for i, pkg := range packages {
		paths[i] = pkg.Path
	}
	selected, err := pickOption(fzf, "package", paths, terminal)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, selected), nil
}

func selectChangeType(cmd *cobra.Command, types []string) (string, error) {
	terminal, ok := cmd.ErrOrStderr().(*os.File)
	fzf, err := exec.LookPath("fzf")
	if !ok || !term.IsTerminal(int(terminal.Fd())) || err != nil {
		return "", fmt.Errorf("choose a change type with --type <type>; available types: %s (interactive selection requires fzf and a terminal)", strings.Join(types, ", "))
	}
	return pickOption(fzf, "type", types, terminal)
}

func pickOption(fzf, kind string, options []string, terminal io.Writer) (string, error) {
	process := exec.Command(fzf, "--read0", "--print0", "--no-multi", "--height=40%", "--layout=reverse", "--prompt="+kind+"> ", "--header=Choose a "+kind+" for your release note")
	// fzf reads candidate paths from this pipe and keyboard input from the terminal.
	// Release-note stdin is left untouched until a package has been chosen.
	process.Stdin = strings.NewReader(strings.Join(options, "\x00") + "\x00")
	process.Stderr = terminal
	// Global filter, multi-select, or output options must not change this protocol.
	process.Env = append(os.Environ(), "FZF_DEFAULT_OPTS=", "FZF_DEFAULT_OPTS_FILE=")
	output, err := process.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 130 || exit.ExitCode() == 1) {
			return "", fmt.Errorf("%s selection cancelled", kind)
		}
		return "", fmt.Errorf("select %s with fzf: %w", kind, err)
	}
	selected := strings.TrimSuffix(string(output), "\x00")
	for _, option := range options {
		if selected == option {
			return option, nil
		}
	}
	return "", fmt.Errorf("fzf did not return an available %s", kind)
}
