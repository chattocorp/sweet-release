package main

import (
	"os"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/spf13/cobra"
)

func newChangelogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "Read package changelogs",
	}
	cmd.AddCommand(newChangelogExtractCommand())
	return cmd
}

func newChangelogExtractCommand() *cobra.Command {
	var version string
	cmd := &cobra.Command{
		Use:   "extract <file>",
		Short: "Read one version's notes from an existing changelog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			notes, err := core.ChangelogRelease(string(data), version)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write([]byte(notes))
			return err
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Release version heading to read")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}
