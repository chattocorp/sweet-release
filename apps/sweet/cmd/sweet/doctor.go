package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check repository consistency and package name uniqueness",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			problems, err := core.CheckRepository(root)
			if err != nil {
				return err
			}
			if asJSON {
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(problems); err != nil {
					return err
				}
			} else if len(problems) == 0 {
				cmd.Println("No problems found.")
			} else {
				cmd.Println("Problems:")
				for _, problem := range problems {
					cmd.Printf("- %s: %s\n", problem.Path, strings.ReplaceAll(problem.Message, "\n", "\n  "))
				}
			}
			if len(problems) > 0 {
				return fmt.Errorf("repository check found %d problem(s)", len(problems))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print problems as a JSON array")
	return cmd
}
