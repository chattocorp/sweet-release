package main

import (
	"fmt"
	"github.com/chattocorp/sweet-release/pkg/core"
	"github.com/spf13/cobra"
	"text/tabwriter"
)

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show package paths and current versions in the repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			packages, err := core.DiscoverPackages(root)
			if err != nil {
				return err
			}
			if len(packages) == 0 {
				cmd.Println("No packages found.")
				return nil
			}
			out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(out, "PACKAGE\tVERSION")
			for _, pkg := range packages {
				fmt.Fprintf(out, "%s\t%s\n", pkg.Path, pkg.Version)
			}
			return out.Flush()
		},
	}
}
