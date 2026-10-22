package main

import (
	"encoding/json"
	"fmt"
	"github.com/chattocorp/sweet-release/pkg/core"
	"github.com/spf13/cobra"
	"strings"
	"text/tabwriter"
)

func newStatusCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show package versions, pending changes, and expected releases",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			packages, err := core.RepositoryStatus(root)
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(packages)
			}
			if len(packages) == 0 {
				cmd.Println("No packages found.")
				return nil
			}
			out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(out, "PACKAGE\tVERSION\tBUMP\tNEXT")
			for _, pkg := range packages {
				bump, next := pkg.Bump, pkg.NextVersion
				if len(pkg.Changes) == 0 {
					bump, next = "", ""
				}
				if len(pkg.Problems) > 0 {
					bump, next = "invalid", ""
				}
				fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", pkg.Path, pkg.Version, bump, next)
			}
			if err := out.Flush(); err != nil {
				return err
			}
			var problems []core.Problem
			for _, pkg := range packages {
				problems = append(problems, pkg.Problems...)
			}
			if len(problems) > 0 {
				cmd.Println("\nProblems:")
				for _, problem := range problems {
					cmd.Printf("- %s: %s\n", problem.Path, strings.ReplaceAll(problem.Message, "\n", "\n  "))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the package inventory as JSON")
	return cmd
}
