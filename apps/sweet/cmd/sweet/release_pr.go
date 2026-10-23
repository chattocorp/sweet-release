package main

import (
	"encoding/json"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/spf13/cobra"
)

func newReleasePRCommand() *cobra.Command {
	var details bool
	cmd := &cobra.Command{
		Use:   "release-pr",
		Short: "Plan release pull requests for automation",
	}
	plan := &cobra.Command{
		Use:   "plan",
		Short: "Print the configured release PRs as JSON without changing files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			var prs any
			if details {
				prs, err = core.PlanReleasePRDetails(root)
			} else {
				prs, err = core.PlanReleasePRs(root)
			}
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(prs)
		},
	}
	plan.Flags().BoolVar(&details, "details", false, "Include planned releases and rendered PR descriptions")
	cmd.AddCommand(plan)
	return cmd
}
