package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/spf13/cobra"
)

func newChangelogCommand() *cobra.Command {
	var version string
	cmd := &cobra.Command{
		Use:   "changelog [package]",
		Short: "Show a package's released changes",
		Long:  "Show the latest release from a package's changelog, or select a release with --version.\nPackage paths are relative to the current directory. With no argument, use the current\npackage or choose one interactively. Output contains the release body without its heading.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			} else if _, err := os.Stat(filepath.Join(dir, ".sweet.yml")); os.IsNotExist(err) {
				selected, err := selectChangePackage(cmd)
				if err != nil {
					return err
				}
				if selected == "" {
					return fmt.Errorf("the current directory is not a Sweet package; use sweet changelog <package> (interactive selection requires a terminal)")
				}
				dir = selected
			}
			notes, err := core.PackageChangelog(dir, version)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write([]byte(notes))
			return err
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Release version to show (latest changelog entry by default)")
	return cmd
}
