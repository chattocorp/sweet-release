package main

import (
	"encoding/json"
	"fmt"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/spf13/cobra"
)

func newGitCommand() *cobra.Command {
	var base, head string
	var dryRun, jsonOutput bool
	cmd := &cobra.Command{Use: "git", Short: "Manage releases in Git"}
	tag := &cobra.Command{
		Use:   "tag",
		Short: "Create missing local tags for committed releases",
		Long:  "Create annotated local tags for current package versions at their first-parent introduction commits. Requires full Git history. Fetch remote tags first to include them in conflict checks. Tags are never pushed or overwritten.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			var tags []core.ReleaseTag
			if base != "" {
				tags, err = core.PlanTags(root, base, head)
			} else {
				tags, err = core.PlanCurrentTags(root)
			}
			if err != nil {
				return err
			}
			if !dryRun {
				if err := core.CreateReleaseTags(root, tags); err != nil {
					return err
				}
			}
			if jsonOutput {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(tags)
			}
			count := 0
			for _, tag := range tags {
				if tag.Exists {
					continue
				}
				action := "Created"
				if dryRun {
					action = "Would create"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s at %s\n", action, tag.Name, tag.Commit)
				count++
			}
			if count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No new release tags.")
			}
			return nil
		},
	}
	tag.Flags().BoolVar(&dryRun, "dry-run", false, "Show tags without creating them")
	tag.Flags().BoolVar(&jsonOutput, "json", false, "Print the tag plan as JSON (exists describes tags present before creation)")
	tag.Flags().StringVar(&base, "base", "", "Tag version changes after this commit instead (all-zero SHA selects only head)")
	tag.Flags().StringVar(&head, "head", "", "Last commit in an explicit range")
	tag.MarkFlagsRequiredTogether("base", "head")
	cmd.AddCommand(tag)
	return cmd
}
