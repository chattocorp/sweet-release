package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func promptDescription(cmd *cobra.Command) (string, error) {
	input, inputOK := cmd.InOrStdin().(*os.File)
	output, outputOK := cmd.ErrOrStderr().(*os.File)
	if !inputOK || !outputOK || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return "", fmt.Errorf("enter a description in a terminal, or use --type <type> --stdin to supply the Markdown body")
	}
	var description string
	form := huh.NewForm(huh.NewGroup(huh.NewInput().
		Title("Describe your change").
		Description("One line. Leave empty to open $EDITOR.").
		Validate(func(value string) error {
			if strings.ContainsAny(value, "\r\n") {
				return fmt.Errorf("enter a one-line description")
			}
			return nil
		}).Value(&description))).WithInput(input).WithOutput(output)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", fmt.Errorf("change creation cancelled")
		}
		return "", fmt.Errorf("read description: %w", err)
	}
	return strings.TrimSpace(description), nil
}
