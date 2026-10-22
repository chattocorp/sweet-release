package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/chattocorp/sweet-release/pkg/core"
	"github.com/spf13/cobra"
)

func newReleasePlanCommand() *cobra.Command {
	var from, to, prefix string
	cmd := &cobra.Command{
		Use:   "release-plan <package>",
		Short: "List committed package releases as JSON without publishing",
		Long:  "Read version increases and changelog notes from a first-parent Git range.\nThe range excludes --from and includes --to; the working tree is ignored.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			root, err = filepath.EvalSymlinks(root)
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			cwd, err = filepath.EvalSymlinks(cwd)
			if err != nil {
				return err
			}
			path := args[0]
			if !filepath.IsAbs(path) {
				path = filepath.Join(cwd, path)
			}
			// Historical package directories need not exist in the working tree.
			if resolved, err := filepath.EvalSymlinks(path); err == nil {
				path = resolved
			}
			path, err = filepath.Rel(root, path)
			if err != nil {
				return err
			}
			releases, err := core.ReleasePlan(root, path, from, to, prefix)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(releases)
		},
	}
	cmd.Flags().StringVar(&from, "from", "HEAD^", "Exclusive base commit")
	cmd.Flags().StringVar(&to, "to", "HEAD", "Inclusive end commit")
	cmd.Flags().StringVar(&prefix, "tag-prefix", "v", "Prefix for release tags")
	return cmd
}
