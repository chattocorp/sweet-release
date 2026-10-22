package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chattocorp/sweet-release/pkg/core"
)

func TestChangeSuggestsPackages(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if output, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	for _, name := range []string{"cli", "core"} {
		dir := filepath.Join(root, "pkg", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := core.InitPackage(dir); err != nil {
			t.Fatal(err)
		}
	}
	// Package guidance comes before editor setup and consumes no piped input.
	t.Setenv("EDITOR", "")
	for _, nested := range []bool{false, true} {
		if nested {
			t.Chdir(filepath.Join(root, "pkg"))
		}
		cmd := newRootCommand()
		cmd.SetArgs([]string{"change", "--stdin"})
		input := strings.NewReader("Keep this input")
		cmd.SetIn(input)
		err := cmd.Execute()
		prefix := "pkg/"
		if nested {
			prefix = ""
		}
		if err == nil {
			t.Fatal("expected package guidance")
		}
		for _, want := range []string{"current directory is not a Sweet package", "sweet change --stdin -- '" + prefix + "cli'", "sweet change --stdin -- '" + prefix + "core'"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("missing %q in %s", want, err)
			}
		}
		if input.Len() != len("Keep this input") {
			t.Fatal("consumed release note input")
		}
	}
}

func TestChangeMissingPackageGuidance(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("EDITOR", "")
	for _, args := range [][]string{nil, {"change"}, {"change", "missing"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code == 0 {
			t.Fatal("accepted missing package")
		}
		if !strings.Contains(stderr.String(), "not a Sweet package") || !strings.Contains(stderr.String(), "sweet init package") {
			t.Fatalf("missing guidance: %s", &stderr)
		}
		if len(args) < 2 && !strings.Contains(stderr.String(), "sweet init repo") {
			t.Fatalf("missing repository setup guidance: %s", &stderr)
		}
	}
}

func TestChangeStdin(t *testing.T) {
	for _, mode := range []string{"default", "current", "given"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			if _, err := core.InitPackage(dir); err != nil {
				t.Fatal(err)
			}
			args := []string{"change", "--stdin", "--type", "fix"}
			if mode == "default" {
				args = args[1:]
			}
			if mode == "given" {
				args = append(args, dir)
			} else {
				t.Chdir(dir)
			}
			t.Setenv("EDITOR", "exit 99")
			cmd := newRootCommand()
			cmd.SetArgs(args)
			const body = "A release note.\n\nWith another paragraph.\n"
			cmd.SetIn(strings.NewReader(body))
			var output bytes.Buffer
			cmd.SetOut(&output)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			files, err := filepath.Glob(filepath.Join(dir, ".sweet", "*.md"))
			if err != nil || len(files) != 1 {
				t.Fatalf("notes = %v, error = %v", files, err)
			}
			data, err := os.ReadFile(files[0])
			if err != nil || string(data) != "---\ntype: fix\n---\n\n"+body {
				t.Fatalf("content = %q, error = %v", data, err)
			}
		})
	}
}

func TestChangeEditor(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "package with spaces $(false)")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := core.InitPackage(dir); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "editor.sh")
	if err := os.WriteFile(script, []byte("test \"$1\" = '--wait' || exit 2\nprintf 'Edited note.\\n' >> \"$2\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", "sh '"+script+"' --wait")
	var stdout, stderr bytes.Buffer
	if code := runChangeWithDescription([]string{dir, "--type", "fix"}, &stdout, &stderr, ""); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, &stderr)
	}
	files, _ := filepath.Glob(filepath.Join(dir, ".sweet", "*.md"))
	if len(files) != 1 {
		t.Fatalf("notes = %v", files)
	}
	data, err := os.ReadFile(files[0])
	if err != nil || string(data) != "---\ntype: fix\n---\n\nEdited note.\n" {
		t.Fatalf("content = %q, error = %v", data, err)
	}

	t.Setenv("EDITOR", "exit 7")
	if code := runChangeWithDescription([]string{dir, "--type", "fix"}, &stdout, &stderr, ""); code == 0 || !strings.Contains(stderr.String(), "release note kept at") {
		t.Fatalf("editor failure: %d, %s", code, &stderr)
	}
	files, _ = filepath.Glob(filepath.Join(dir, ".sweet", "*.md"))
	if len(files) != 2 {
		t.Fatalf("failed editor note was not preserved: %v", files)
	}
}

func TestChangeWithoutEditor(t *testing.T) {
	dir := t.TempDir()
	if _, err := core.InitPackage(dir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", "")
	var stdout, stderr bytes.Buffer
	if code := runChangeWithDescription([]string{dir, "--type", "fix"}, &stdout, &stderr, " "); code == 0 || !strings.Contains(stderr.String(), "set EDITOR") {
		t.Fatalf("exit = %d, stderr = %s", code, &stderr)
	}
	files, _ := os.ReadDir(filepath.Join(dir, ".sweet"))
	if len(files) != 0 {
		t.Fatal("created a note without an editor")
	}
}
