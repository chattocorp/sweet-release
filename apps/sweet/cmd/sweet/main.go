package main

import (
	"fmt"
	"io"
	"os"

	"github.com/chattocorp/sweet-release/apps/sweet"
	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/spf13/cobra"
)

// version can be set at build time with -ldflags '-X main.version=...'.
var version = sweet.Version

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cmd := newRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(stderr, "sweet: %v\n", err)
		return 2
	}
	return 0
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "sweet",
		Short:         "Release packages from Markdown change files",
		Long:          "Release packages from Markdown change files.\n\nRun 'sweet change' to write a release note.\nRun 'sweet init repo' to create repository configuration at the Git root.\nRun 'sweet bump' to prepare a package release.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "doctor" {
				return nil
			}
			root, err := core.RepositoryRoot()
			if err != nil {
				return nil
			}
			return core.CheckTagPrefixes(root)
		},
	}
	cmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newBumpCommand())
	cmd.AddCommand(newChangelogCommand())
	cmd.AddCommand(newGitCommand())
	cmd.AddCommand(newChangeCommand())
	cmd.AddCommand(&cobra.Command{
		Use: "doctor", Short: "Check package configuration and release tag ownership", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			problems, err := core.DiagnosePackages(root)
			if err != nil {
				return err
			}
			for _, problem := range problems {
				cmd.Printf("Warning: %s\n", problem)
			}
			if len(problems) == 0 {
				cmd.Println("No package problems found.")
			}
			return nil
		},
	})
	return cmd
}
