package main

import (
	"github.com/chattocorp/sweet-release/packages/core"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a repository or package",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "repo",
		Short: "Create .sweet.config.yml at the Git repository root",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			path, err := core.InitRepository(root)
			if err != nil {
				return err
			}
			cmd.Printf("Created %s\n", path)
			return nil
		},
	}, &cobra.Command{
		Use:   "package [dir]",
		Short: "Create .sweet.yml in the current or given directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			path, err := core.InitPackage(dir)
			if err != nil {
				return err
			}
			cmd.Printf("Created %s\n", path)
			return nil
		},
	})
	return cmd
}
