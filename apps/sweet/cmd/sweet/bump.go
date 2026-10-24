package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/spf13/cobra"
)

func newBumpCommand() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "bump [package...]",
		Short: "Update versions and changelogs, then consume release notes",
		Long:  "Prepare releases for the given package directories, or choose pending releases interactively.\nRepositories with one package select it automatically.\nUse --all to prepare all packages. This changes local files without committing or publishing.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return fmt.Errorf("use either a package directory or --all")
			}
			root, err := core.RepositoryRoot()
			if err != nil {
				return err
			}
			var paths []string
			if !all && len(args) == 0 {
				paths, err = selectBumpPackages(cmd, root)
				if err != nil {
					return err
				}
				if len(paths) == 0 {
					cmd.Println("No packages need a release.")
					return nil
				}
			}
			for _, dir := range args {
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
				paths = append(paths, path)
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

func bumpPicker(statuses []core.PackageStatus) pickerModel {
	model := newPickerModel("package", nil)
	model.multiple = true
	model.selected = map[string]bool{}
	model.annotations = map[string]string{}
	for _, status := range statuses {
		if len(status.Problems) > 0 || len(status.Changes) == 0 || status.Bump == "none" || status.NextVersion == "" {
			continue
		}
		model.options = append(model.options, status.Path)
		model.annotations[status.Path] = fmt.Sprintf("%s → %s · %s", status.Version, status.NextVersion, status.Bump)
	}
	model.matches = model.options
	return model
}

func selectBumpPackages(cmd *cobra.Command, root string) ([]string, error) {
	statuses, err := core.RepositoryStatus(root)
	if err != nil {
		return nil, err
	}
	model := bumpPicker(statuses)
	// Report invalid packages even though they cannot be selected for release.
	for _, status := range statuses {
		for _, problem := range status.Problems {
			fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", problem.Error())
		}
	}
	if len(model.options) == 0 {
		return nil, nil
	}
	if len(statuses) == 1 {
		return model.options, nil
	}
	if !pickerAvailable(cmd) {
		return nil, fmt.Errorf("interactive selection requires a terminal; use sweet bump <package...> or sweet bump --all; pending packages: %s", strings.Join(model.options, ", "))
	}
	model, err = runPicker(cmd, model)
	if err != nil {
		return nil, err
	}
	return model.selectedOptions(), nil
}
