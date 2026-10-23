package main

import (
	"fmt"
	"io"
	"os"

	"github.com/chattocorp/sweet-release/apps/sweet"
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
	}
	cmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newBumpCommand())
	cmd.AddCommand(newChangelogCommand())
	cmd.AddCommand(newGitCommand())
	cmd.AddCommand(newChangeCommand())
	return cmd
}
