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
	var paths []string
	submodules := map[string]bool{}
	for _, record := range strings.Split(string(listing), "\x00") {
		meta, name, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 4 {
			continue
		}
		if fields[1] == "commit" {
			submodules[name] = true
		} else if _, err := strconv.ParseInt(fields[3], 10, 64); err != nil {
			return snapshot{}, fmt.Errorf("list files in %s: object %s for %s is missing", rev, fields[2], name)
		}
		paths = append(paths, name)
	}
	sort.Strings(paths)

	var names []string
	seen := map[string]bool{}
	for _, name := range append([]string{".sweet.config.yml"}, paths...) {
		if !seen[name] && (name == ".sweet.config.yml" || isManifest(name) || isNote(name)) {
			seen[name] = true
			names = append(names, name)
		}
	}
	files, err := readTree(strings.TrimSpace(string(tree)), names, submodules)
	if err != nil {
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

// readTree reads files from a tree with one `git cat-file --batch
// --follow-symlinks` process, which resolves symbolic links within the tree
// like a checkout would. Links that leave the tree do not exist.
func readTree(tree string, names []string, submodules map[string]bool) (treeFiles, error) {
	files := treeFiles{}
	var requests []string
	for _, name := range names {
		if strings.Contains(name, "\n") { // Batch requests are lines.
			files[name] = treeFile{err: &fs.PathError{Op: "open", Path: name, Err: errors.New("unsupported file name")}}
		} else {
			requests = append(requests, name)
		}
	}
	var stderr bytes.Buffer
	cmd := treeCommand("cat-file", "--batch", "--follow-symlinks")
	cmd.Stderr = &stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start git cat-file: %w", err)
	}
	go func() {
		defer in.Close()
		for _, name := range requests {
			if _, err := fmt.Fprintf(in, "%s:%s\n", tree, name); err != nil {
				return
			}
		}
	}()
	reader := bufio.NewReader(out)
	for _, name := range requests {
		var file treeFile
		if file, err = readTreeFile(reader, name, submodules[name]); err != nil {
			cmd.Process.Kill()
			break
		}
		files[name] = file
	}
	if waitErr := cmd.Wait(); err == nil {
		err = waitErr
	}
	if err != nil {
		return nil, gitFailure(err, stderr.Bytes())
	}
	return files, nil
}

// readTreeFile reads one batch response. File errors match those of os.DirFS
// in a checkout; a returned error is a Git failure.
func readTreeFile(reader *bufio.Reader, name string, submodule bool) (treeFile, error) {
	fail := func(op string, err error) (treeFile, error) {
		return treeFile{err: &fs.PathError{Op: op, Path: name, Err: err}}, nil
	}
	header, err := reader.ReadString('\n')
	if err != nil {
		return treeFile{}, err
	}
	header = strings.TrimSuffix(header, "\n")
	if strings.HasSuffix(header, " missing") {
		if submodule { // A submodule checks out as an empty directory.
			return fail("read", errors.New("is a directory"))
		}
		return fail("open", fs.ErrNotExist)
	}
	// "<oid> <type> <size>" for objects, "<type> <size>" for unresolved links.
	fields := strings.Fields(header)
	if len(fields) < 2 || len(fields) > 3 {
		return treeFile{}, fmt.Errorf("git cat-file: unexpected response %q", header)
	}
	typ := fields[len(fields)-2]
	size, err := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	if err != nil || size < 0 {
		return treeFile{}, fmt.Errorf("git cat-file: unexpected response %q", header)
	}
	if typ == "blob" {
		data := make([]byte, size+1) // Content and a trailing newline.
		if _, err := io.ReadFull(reader, data); err != nil {
			return treeFile{}, err
		}
		return treeFile{data: data[:size]}, nil
	}
	// Other responses only matter by type: skip their content unbuffered.
	if _, err := io.CopyN(io.Discard, reader, size+1); err != nil {
		return treeFile{}, err
	}
	switch typ {
	case "tree":
		return fail("read", errors.New("is a directory"))
	case "loop":
		return fail("open", errors.New("too many levels of symbolic links"))
	case "symlink", "dangling":
		return fail("open", fs.ErrNotExist)
	}
	return treeFile{}, fmt.Errorf("git cat-file: unexpected response %q", header)
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
