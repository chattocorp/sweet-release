package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chattocorp/sweet-release/pkg/core"
)

func TestChangeTypeRequired(t *testing.T) {
	dir := t.TempDir()
	if _, err := core.InitPackage(dir); err != nil {
		t.Fatal(err)
	}
	for _, extra := range [][]string{nil, {"--type", "unknown"}} {
		cmd := newRootCommand()
		cmd.SetArgs(append([]string{"change", dir, "--stdin"}, extra...))
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)
		input := strings.NewReader("untouched")
		cmd.SetIn(input)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "available types: fix, feature, chore, refactor, performance") {
			t.Fatalf("missing type guidance: %v", err)
		}
		if input.Len() != len("untouched") {
			t.Fatal("consumed input before validating type")
		}
		files, _ := os.ReadDir(filepath.Join(dir, ".sweet"))
		if len(files) != 0 {
			t.Fatal("created a note without a valid type")
		}
	}
}
