package main

import (
	"fmt"
	"os"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/spf13/cobra"
)

func newGitHubCommand() *cobra.Command {
	var title, notesFile string
	var draft, latest, replacePublished bool
	cmd := &cobra.Command{Use: "github", Short: "Publish releases on GitHub"}
	release := &cobra.Command{
		Use:   "release <tag> [assets...]",
		Short: "Create, upload to, and publish a GitHub release for a tag",
		Long: "Create a draft GitHub release for an existing Sweet tag, or reuse it, with notes from the\n" +
			"package changelog. Upload assets, replacing files with the same name, then publish it.\n" +
			"Prerelease versions are marked as prereleases. The release becomes Latest only for the\n" +
			"highest stable version of the root package, or of the only package in the repository.\n" +
			"Reruns are safe. A published release is left unchanged unless --replace-published is given.\n" +
			"Requires the gh CLI, authenticated with GH_TOKEN.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			opts := core.GitHubReleaseOptions{Tag: args[0], Assets: args[1:], Title: title, Draft: draft, ReplacePublished: replacePublished}
			if notesFile != "" {
				data, err := os.ReadFile(notesFile)
				if err != nil {
					return err
				}
				notes := string(data)
				opts.Notes = &notes
			}
			if cmd.Flags().Changed("latest") {
				opts.Latest = &latest
			}
			result, err := core.PublishGitHubRelease(root, opts)
			if result.State == "published" && !replacePublished && err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Left published release %s unchanged (use --replace-published to replace its assets)\n", result.Tag)
				return nil
			}
			if result.State != "" {
				action := map[string]string{"missing": "Created draft", "draft": "Reused draft", "published": "Reused published"}[result.State]
				fmt.Fprintf(cmd.OutOrStdout(), "%s release %s\n", action, result.Tag)
			}
			if err != nil {
				return err
			}
			if len(opts.Assets) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Uploaded %d assets\n", len(opts.Assets))
			}
			if result.Published {
				fmt.Fprintf(cmd.OutOrStdout(), "Published %s (prerelease: %t, latest: %t)\n", result.Tag, result.Prerelease, result.Latest)
			}
			return nil
		},
	}
	release.Flags().BoolVar(&draft, "draft", false, "Create and upload only; leave a new release unpublished")
	release.Flags().StringVar(&title, "title", "", "Title for a new release (default: package name and version, or the tag)")
	release.Flags().StringVar(&notesFile, "notes-file", "", "Read notes for a new release from a file instead of the changelog")
	release.Flags().BoolVar(&latest, "latest", false, "Mark as Latest (--latest) or never (--latest=false) instead of choosing automatically")
	release.Flags().BoolVar(&replacePublished, "replace-published", false, "Also upload to an already published release, replacing same-name assets, and reapply prerelease and Latest")
	cmd.AddCommand(release)
	return cmd
}
