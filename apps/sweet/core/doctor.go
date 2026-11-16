package core

// CheckRelease validates package configuration, tag ownership, pending notes,
// and release preparation without writing files.
func CheckRelease(root string) error {
	_, err := prepareReleases(root, nil)
	return err
}
