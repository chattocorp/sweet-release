package core

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// snapshot is a set of repository files: sorted repository-relative paths and
// a filesystem to read them from. configPath describes the repository
// configuration file in errors.
type snapshot struct {
	fsys       fs.FS
	paths      []string
	configPath string
}

// workingSnapshot reads tracked and untracked files from the working tree,
// respecting Git ignore rules.
func workingSnapshot(root string) (snapshot, error) {
	paths, err := repositoryFiles(root)
	if err != nil {
		return snapshot{}, err
	}
	return snapshot{fsys: os.DirFS(root), paths: paths, configPath: filepath.Join(root, ".sweet.config.yml")}, nil
}

// treeSnapshot reads files from a Git tree-ish (commit, tag, or tree object)
// in the repository of the current directory, or GIT_DIR. It reads blobs only:
// no working tree, index, filters, or hooks.
func treeSnapshot(rev string) (snapshot, error) {
	if rev == "" || strings.HasPrefix(rev, "-") {
		return snapshot{}, fmt.Errorf("invalid revision %q", rev)
	}
	tree, err := exec.Command("git", "rev-parse", "--verify", "--quiet", rev+"^{tree}").Output()
	if err != nil {
		return snapshot{}, fmt.Errorf("resolve revision %q to a tree: %w", rev, err)
	}
	oid := strings.TrimSpace(string(tree))
	output, err := exec.Command("git", "ls-tree", "-r", "-z", "--full-tree", oid).Output()
	if err != nil {
		return snapshot{}, fmt.Errorf("list files in %s: %w", rev, err)
	}
	fsys := treeFS{}
	var paths []string
	for _, entry := range strings.Split(string(output), "\x00") {
		meta, name, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[1] != "blob" {
			continue // Submodules have no files in this tree.
		}
		fsys[name] = treeEntry{mode: fields[0], oid: fields[2]}
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return snapshot{fsys: fsys, paths: paths, configPath: rev + ":.sweet.config.yml"}, nil
}

type treeEntry struct{ mode, oid string }

// treeFS maps repository-relative paths to blobs. Symbolic links resolve within
// the tree, like they would in a checkout; links leaving it do not exist.
type treeFS map[string]treeEntry

func (t treeFS) ReadFile(name string) ([]byte, error) {
	resolved := name
	for range 40 {
		entry, ok := t[resolved]
		if !ok {
			break
		}
		data, err := exec.Command("git", "cat-file", "blob", entry.oid).Output()
		if err != nil {
			return nil, &fs.PathError{Op: "read", Path: name, Err: err}
		}
		if entry.mode != "120000" {
			return data, nil
		}
		target := string(data)
		if path.IsAbs(target) {
			break
		}
		resolved = path.Join(path.Dir(resolved), target)
		if !fs.ValidPath(resolved) {
			break
		}
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (t treeFS) Open(name string) (fs.File, error) {
	data, err := t.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return &memFile{Reader: bytes.NewReader(data), name: path.Base(name), size: int64(len(data))}, nil
}

type memFile struct {
	*bytes.Reader
	name string
	size int64
}

func (f *memFile) Stat() (fs.FileInfo, error) { return f, nil }
func (f *memFile) Close() error               { return nil }
func (f *memFile) Name() string               { return f.name }
func (f *memFile) Size() int64                { return f.size }
func (f *memFile) Mode() fs.FileMode          { return 0o444 }
func (f *memFile) ModTime() time.Time         { return time.Time{} }
func (f *memFile) IsDir() bool                { return false }
func (f *memFile) Sys() any                   { return nil }

// readOptional reads a file, reporting whether it exists.
func readOptional(fsys fs.FS, name string) ([]byte, bool, error) {
	data, err := fs.ReadFile(fsys, name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}
