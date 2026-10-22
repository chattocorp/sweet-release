package main

import (
	"os"

	"github.com/chattocorp/sweet-release/pkg/core"
	"github.com/spf13/cobra"
)

func newReleaseNotesCommand() *cobra.Command {
	var version string
	cmd := &cobra.Command{
		Use:   "release-notes <changelog>",
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
