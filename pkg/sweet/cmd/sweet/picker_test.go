package main

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chattocorp/sweet-release/pkg/sweet/core"
)

func updatePicker(m pickerModel, msg tea.Msg) pickerModel {
	model, _ := m.Update(msg)
	return model.(pickerModel)
}

func TestBumpPickerEligibilityAndVersions(t *testing.T) {
	statuses := []core.PackageStatus{
		{Package: core.Package{Path: "api", Version: "1.2.3"}, Changes: []core.Change{{Type: "fix"}}, Bump: "patch", NextVersion: "1.2.4"},
		{Package: core.Package{Path: "empty", Version: "1.0.0"}, Bump: "none", NextVersion: "1.0.0"},
		{Package: core.Package{Path: "chore", Version: "1.0.0"}, Changes: []core.Change{{Type: "chore"}}, Bump: "none", NextVersion: "1.0.0"},
		{Package: core.Package{Path: "invalid"}, Changes: []core.Change{{Type: "fix"}}, Problems: []core.Problem{{Message: "invalid version"}}},
	}
	m := bumpPicker(statuses)
	if !reflect.DeepEqual(m.options, []string{"api"}) || !strings.Contains(m.View(), "1.2.3 → 1.2.4") {
		t.Fatalf("picker: %+v\n%s", m, m.View())
	}
}

func TestPickerMultipleSelectionAcrossFilters(t *testing.T) {
	m := newPickerModel("package", []string{"pkg/cli", "pkg/core", "pkg/web"})
	m.multiple = true
	m.selected = map[string]bool{}
	space := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")}
	m = updatePicker(m, space)
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updatePicker(m, space)
	m = updatePicker(m, space) // Toggle core back off.
	if m.input.Value() != "" || !strings.Contains(m.View(), "1 selected") {
		t.Fatalf("space did not toggle selection: %s", m.View())
	}
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("web")})
	m = updatePicker(m, space)
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zzz")})
	m = updatePicker(m, space) // No matches: keep the existing selection.
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.accepted || !reflect.DeepEqual(m.selectedOptions(), []string{"pkg/cli", "pkg/web"}) {
		t.Fatalf("selection = %v, accepted = %v", m.selectedOptions(), m.accepted)
	}
}

func TestPickerMultipleEnterAndCancel(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyEnter, tea.KeyEsc, tea.KeyCtrlC} {
		m := newPickerModel("package", []string{"api", "web"})
		m.multiple = true
		m.selected = map[string]bool{}
		m = updatePicker(m, tea.KeyMsg{Type: tea.KeyDown})
		m = updatePicker(m, tea.KeyMsg{Type: key})
		if key == tea.KeyEnter {
			if !m.accepted || !reflect.DeepEqual(m.selectedOptions(), []string{"web"}) {
				t.Fatalf("highlighted selection = %v", m.selectedOptions())
			}
		} else if m.accepted || !m.cancelled {
			t.Fatal("cancel accepted a release")
		}
	}
}

func TestPickerFuzzySelection(t *testing.T) {
	m := newPickerModel("package", []string{"pkg/cli", "pkg/core", "pkg/a space"})
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pcr")})
	if !reflect.DeepEqual(m.matches, []string{"pkg/core"}) {
		t.Fatalf("matches = %v", m.matches)
	}
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.accepted || m.matches[m.cursor] != "pkg/core" {
		t.Fatalf("selection = %+v", m)
	}
}

func TestPickerNavigationAndNoMatches(t *testing.T) {
	m := newPickerModel("type", []string{"fix", "feature", "chore"})
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.matches[m.cursor] != "feature" {
		t.Fatal("down did not select feature")
	}
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Fatal("up did not select fix")
	}
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zzz")})
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.accepted || !strings.Contains(m.View(), "No matching options") {
		t.Fatal("accepted an empty result")
	}
	m = updatePicker(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if !reflect.DeepEqual(m.matches, m.options) {
		t.Fatalf("cleared filter = %v", m.matches)
	}
}

func TestPickerCancelAndResize(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyEsc, tea.KeyCtrlC} {
		m := newPickerModel("type", []string{"fix"})
		m = updatePicker(m, tea.WindowSizeMsg{Width: 20, Height: 8})
		if m.height != 2 {
			t.Fatalf("height = %d", m.height)
		}
		m = updatePicker(m, tea.KeyMsg{Type: key})
		if m.accepted || !m.cancelled || m.View() != "" {
			t.Fatal("cancellation did not clear picker")
		}
	}
}
