package core

import "errors"

// CheckRelease validates package configuration, tag ownership, pending notes,
// and release preparation without writing files.
func CheckRelease(root string) error {
	statuses, err := RepositoryStatus(root)
	if err != nil {
		return err
	}
	var packages []Package
	var problems []error
	for _, status := range statuses {
		packages = append(packages, status.Package)
		for _, problem := range status.Problems {
			problems = append(problems, problem)
		}
	}
	for _, problem := range TagPrefixProblems(packages) {
		problems = append(problems, problem)
	}
	if err := errors.Join(problems...); err != nil {
		return err
	}
	_, err = prepareReleases(root, nil)
	return err
}
