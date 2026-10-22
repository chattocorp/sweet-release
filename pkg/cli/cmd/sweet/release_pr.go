package main

import (
	"encoding/json"

	"github.com/chattocorp/sweet-release/pkg/core"
	"github.com/spf13/cobra"
)

func newReleasePRCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release-pr",
		Short: "Plan release pull requests for automation",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "plan",
		Short: "Print the configured release PRs as JSON without changing files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			prs, err := core.PlanReleasePRs(root)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(prs)
		},
	})
	return cmd
}
