package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/spf13/cobra"
)

func runChangeWithDescription(args []string, stdout, stderr io.Writer, description string) int {
	cmd := newChangeCommandWithPrompt(func(*cobra.Command) (string, error) { return description, nil })
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func TestChangeDescription(t *testing.T) {
	dir := t.TempDir()
	if _, err := core.InitPackage(dir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", "exit 99")
	var stdout, stderr bytes.Buffer
	if code := runChangeWithDescription([]string{dir, "--type", "fix"}, &stdout, &stderr, "  Fix **package discovery**.  "); code != 0 {
		t.Fatalf("exit = %d: %s", code, &stderr)
	}
	files, _ := filepath.Glob(filepath.Join(dir, ".sweet", "*.md"))
	if len(files) != 1 {
		t.Fatalf("notes = %v", files)
	}
	data, err := os.ReadFile(files[0])
	if err != nil || string(data) != "---\ntype: fix\n---\n\nFix **package discovery**.\n" {
		t.Fatalf("note = %q, error = %v", data, err)
	}
}

func TestDescriptionCancellation(t *testing.T) {
	dir := t.TempDir()
	if _, err := core.InitPackage(dir); err != nil {
		t.Fatal(err)
	}
	cmd := newChangeCommandWithPrompt(func(*cobra.Command) (string, error) { return "", fmt.Errorf("change creation cancelled") })
	cmd.SetArgs([]string{dir, "--type", "fix"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected cancellation")
	}
	files, _ := os.ReadDir(filepath.Join(dir, ".sweet"))
	if len(files) != 0 {
		t.Fatal("cancelled prompt created a note")
	}
}

func TestDescriptionRequiresTerminal(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetIn(strings.NewReader(""))
	cmd.SetErr(io.Discard)
	if _, err := promptDescription(cmd); err == nil || !strings.Contains(err.Error(), "--stdin") {
		t.Fatalf("missing guidance: %v", err)
	}
}
