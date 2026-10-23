package main

import (
	"encoding/json"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/spf13/cobra"
)

func newTagsCommand() *cobra.Command {
	var base, head string
	cmd := &cobra.Command{Use: "tags", Short: "Plan version tags for release automation"}
	plan := &cobra.Command{
		Use:   "plan --base <commit> --head <commit>",
		Short: "Print version tags for a pushed commit range as JSON without changing Git",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			tags, err := core.PlanTags(root, base, head)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(tags)
		},
	}
	plan.Flags().StringVar(&base, "base", "", "Commit before the push (or the all-zero SHA for branch creation)")
	plan.Flags().StringVar(&head, "head", "", "Last commit in the push")
	_ = plan.MarkFlagRequired("base")
	_ = plan.MarkFlagRequired("head")
	cmd.AddCommand(plan)
	return cmd
}
