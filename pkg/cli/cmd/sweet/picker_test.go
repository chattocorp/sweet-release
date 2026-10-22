package main

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func updatePicker(m pickerModel, msg tea.Msg) pickerModel {
	model, _ := m.Update(msg)
	return model.(pickerModel)
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
