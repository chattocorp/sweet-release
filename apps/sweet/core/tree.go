package core

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// fileReader reads repository files by slash-separated relative path.
type fileReader interface {
	ReadFile(name string) ([]byte, error)
}

// snapshot is a set of repository files: sorted repository-relative paths and
// a reader for their contents. configPath describes the repository
// configuration file in errors.
type snapshot struct {
	files      fileReader
	paths      []string
	configPath string
}

// isManifest and isNote identify the files a release plan reads.
func isManifest(name string) bool { return path.Base(name) == ".sweet.yml" }
func isNote(name string) bool {
	return path.Base(path.Dir(name)) == ".sweet" && path.Ext(name) == ".md"
}

// workingSnapshot reads tracked and untracked files from the working tree,
// respecting Git ignore rules.
func workingSnapshot(root string) (snapshot, error) {
	paths, err := repositoryFiles(root)
	if err != nil {
		return snapshot{}, err
	}
	return snapshot{files: os.DirFS(root).(fs.ReadFileFS), paths: paths, configPath: filepath.Join(root, ".sweet.config.yml")}, nil
}

// maxTreeBlob bounds the size of a file read from a tree.
const maxTreeBlob = 1 << 20

// treeSnapshot reads files from a Git tree-ish (commit, tag, or tree object)
// in the repository of the current directory, or GIT_DIR. It reads blobs only:
// no working tree, index, filters, hooks, or lazy fetches. All plan files are
// read up front through one `git cat-file --batch` process; Git failures are
// errors, while unreadable files become per-file errors like in a checkout.
func treeSnapshot(rev string) (snapshot, error) {
	if rev == "" || strings.HasPrefix(rev, "-") {
		return snapshot{}, fmt.Errorf("invalid revision %q", rev)
	}
	tree, err := treeGit("rev-parse", "--verify", rev+"^{tree}")
	if err != nil {
		return snapshot{}, fmt.Errorf("resolve revision %q to a tree: %w", rev, err)
	}
	listing, err := treeGit("ls-tree", "-r", "-z", "-l", "--full-tree", strings.TrimSpace(string(tree)))
	if err != nil {
		return snapshot{}, fmt.Errorf("list files in %s: %w", rev, err)
	}
	t := &gitTree{entries: map[string]treeEntry{}, dirs: map[string]bool{"": true}}
	var paths []string
	for _, record := range strings.Split(string(listing), "\x00") {
		meta, name, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 4 {
			continue
		}
		entry := treeEntry{mode: fields[0], oid: fields[2]}
		if fields[1] == "commit" {
			t.dirs[name] = true // A submodule checks out as an empty directory.
		} else if entry.size, err = strconv.ParseInt(fields[3], 10, 64); err != nil {
			return snapshot{}, fmt.Errorf("list files in %s: object %s for %s is missing", rev, entry.oid, name)
		} else {
			t.entries[name] = entry
		}
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			t.dirs[dir] = true
		}
		paths = append(paths, name)
	}
	sort.Strings(paths)

	if err := t.start(); err != nil {
		return snapshot{}, err
	}
	defer t.stop()
	files := treeFiles{}
	for _, name := range append([]string{".sweet.config.yml"}, paths...) {
		if _, done := files[name]; done || name != ".sweet.config.yml" && !isManifest(name) && !isNote(name) {
			continue
		}
		data, err := t.resolve(name)
		var gitErr *treeGitError
		if errors.As(err, &gitErr) {
			return snapshot{}, fmt.Errorf("read %s from %s: %w", name, rev, err)
		}
		files[name] = treeFile{data, err}
	}
	if err := t.stop(); err != nil {
		return snapshot{}, fmt.Errorf("read files from %s: %w", rev, err)
	}
	return snapshot{files: files, paths: paths, configPath: rev + ":.sweet.config.yml"}, nil
}

// treeFiles holds the plan files read from a tree. Other paths do not exist.
type treeFiles map[string]treeFile

type treeFile struct {
	data []byte
	err  error
}

func (f treeFiles) ReadFile(name string) ([]byte, error) {
	file, ok := f[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return file.data, file.err
}

type treeEntry struct {
	mode, oid string
	size      int64
}

// gitTree resolves paths in a listed tree, reading blobs from a batch process.
type gitTree struct {
	entries map[string]treeEntry
	dirs    map[string]bool
	cmd     *exec.Cmd
	stderr  bytes.Buffer
	in      io.WriteCloser
	out     *bufio.Reader
}

// treeGitError marks Git failures, as opposed to unreadable files.
type treeGitError struct{ err error }

func (e *treeGitError) Error() string { return e.err.Error() }
func (e *treeGitError) Unwrap() error { return e.err }

func (t *gitTree) start() error {
	t.cmd = treeCommand("cat-file", "--batch")
	t.cmd.Stderr = &t.stderr
	in, err := t.cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := t.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("start git cat-file: %w", err)
	}
	t.in, t.out = in, bufio.NewReader(out)
	return nil
}

// stop ends the batch process once; later calls return nil.
func (t *gitTree) stop() error {
	if t.cmd == nil {
		return nil
	}
	t.in.Close()
	err := t.cmd.Wait()
	t.cmd = nil
	return gitFailure(err, t.stderr.Bytes())
}

func (t *gitTree) blob(entry treeEntry) ([]byte, error) {
	fail := func(err error) ([]byte, error) {
		if stopErr := t.stop(); stopErr != nil {
			err = stopErr
		}
		return nil, &treeGitError{err}
	}
	if _, err := fmt.Fprintln(t.in, entry.oid); err != nil {
		return fail(err)
	}
	header, err := t.out.ReadString('\n')
	if err != nil {
		return fail(err)
	}
	fields := strings.Fields(header)
	if len(fields) != 3 || fields[1] != "blob" || fields[2] != strconv.FormatInt(entry.size, 10) {
		return fail(fmt.Errorf("git cat-file: unexpected object %q", strings.TrimSpace(header)))
	}
	data := make([]byte, entry.size+1) // Content and a trailing newline.
	if _, err := io.ReadFull(t.out, data); err != nil {
		return fail(err)
	}
	return data[:entry.size], nil
}

// resolve reads a file, following symbolic links one path component at a
// time like a checkout would. Links that leave the tree do not exist.
func (t *gitTree) resolve(name string) ([]byte, error) {
	// Errors match those of os.DirFS in a checkout.
	fail := func(err error) ([]byte, error) { return nil, &fs.PathError{Op: "open", Path: name, Err: err} }
	pending := strings.Split(name, "/")
	current, hops := "", 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if current == "" {
				return fail(fs.ErrNotExist)
			}
			current = parentDir(current)
			continue
		}
		next := path.Join(current, part)
		entry, ok := t.entries[next]
		switch {
		case ok && entry.mode == "120000":
			if hops++; hops > 40 {
				return fail(errors.New("too many levels of symbolic links"))
			}
			if entry.size > maxTreeBlob {
				return fail(errors.New("symbolic link target too long"))
			}
			target, err := t.blob(entry)
			if err != nil {
				return nil, err
			}
			if path.IsAbs(string(target)) {
				return fail(fs.ErrNotExist)
			}
			pending = append(strings.Split(string(target), "/"), pending...)
		case ok && len(pending) > 0:
			return fail(errors.New("not a directory"))
		case ok:
			if entry.size > maxTreeBlob {
				return fail(fmt.Errorf("file is larger than %d bytes", maxTreeBlob))
			}
			return t.blob(entry)
		case t.dirs[next]:
			current = next
		default:
			return fail(fs.ErrNotExist)
		}
	}
	return nil, &fs.PathError{Op: "read", Path: name, Err: errors.New("is a directory")}
}

func parentDir(dir string) string {
	if parent := path.Dir(dir); parent != "." {
		return parent
	}
	return ""
}

// treeCommand runs Git without lazy fetches of missing objects.
func treeCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1")
	return cmd
}

func treeGit(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := treeCommand(args...)
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	return output, gitFailure(err, stderr.Bytes())
}

// gitFailure adds Git's error output to a failed command's error.
func gitFailure(err error, stderr []byte) error {
	if message := strings.TrimSpace(string(stderr)); err != nil && message != "" {
		return fmt.Errorf("%w: %s", err, message)
	}
	return err
}

// readOptional reads a file, reporting whether it exists.
func readOptional(files fileReader, name string) ([]byte, bool, error) {
	data, err := files.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}
