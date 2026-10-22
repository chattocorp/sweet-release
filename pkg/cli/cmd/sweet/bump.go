package main

import (
	"fmt"
	"path/filepath"

	"github.com/chattocorp/sweet-release/pkg/core"
	"github.com/spf13/cobra"
)

func newBumpCommand() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "bump [package]",
		Short: "Update versions and changelogs, then consume release notes",
		Long:  "Prepare a release for the current package or a given package directory.\nUse --all to prepare all packages. This changes local files without committing or publishing.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return fmt.Errorf("use either a package directory or --all")
			}
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			var paths []string
			if !all {
				dir := "."
				if len(args) > 0 {
					dir = args[0]
				}
				dir, err = filepath.Abs(dir)
				if err != nil {
					return err
				}
				dir, err = filepath.EvalSymlinks(dir)
				if err != nil {
					return err
				}
				root, err = filepath.EvalSymlinks(root)
				if err != nil {
					return err
				}
				path, err := filepath.Rel(root, dir)
				if err != nil {
					return err
				}
				paths = []string{path}
			}
			releases, err := core.Bump(root, paths)
			if err != nil {
				return err
			}
			if len(releases) == 0 {
				cmd.Println("No packages need a release.")
			}
			for _, release := range releases {
				cmd.Printf("Bumped %s: %s -> %s (%d notes consumed)\n", release.Path, release.Version, release.NextVersion, len(release.Changes))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Prepare releases for all packages in the repository")
	return cmd
}
