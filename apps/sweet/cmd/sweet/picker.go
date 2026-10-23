package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chattocorp/sweet-release/apps/sweet/core"
	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func pickerAvailable(cmd *cobra.Command) bool {
	terminal, ok := cmd.ErrOrStderr().(*os.File)
	return ok && term.IsTerminal(int(terminal.Fd()))
}

// selectChangePackage returns an empty path when interactive selection is unavailable.
func selectChangePackage(cmd *cobra.Command) (string, error) {
	if !pickerAvailable(cmd) {
		return "", nil
	}
	root, err := core.RepositoryRoot()
	if err != nil {
		return "", nil
	}
	packages, err := core.DiscoverPackages(root)
	if err != nil {
		return "", err
	}
	if len(packages) == 0 {
		return "", nil
	}
	paths := make([]string, len(packages))
	for i, pkg := range packages {
		paths[i] = pkg.Path
	}
	selected, err := pickOption(cmd, "package", paths, nil)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, selected), nil
}

func selectChangeType(cmd *cobra.Command, policies []core.ChangeType) (string, error) {
	types := make([]string, len(policies))
	annotations := make(map[string]string, len(policies))
	for i, policy := range policies {
		types[i] = policy.Name
		if policy.Bump != "none" {
			annotations[policy.Name] = policy.Bump
		}
	}
	if !pickerAvailable(cmd) {
		return "", fmt.Errorf("choose a change type with --type <type>; available types: %s (interactive selection requires a terminal)", strings.Join(types, ", "))
	}
	return pickOption(cmd, "type", types, annotations)
}

func pickOption(cmd *cobra.Command, kind string, options []string, annotations map[string]string) (string, error) {
	initial := newPickerModel(kind, options)
	initial.annotations = annotations
	picker, err := runPicker(cmd, initial)
	if err != nil {
		return "", err
	}
	return picker.matches[picker.cursor], nil
}

func runPicker(cmd *cobra.Command, initial pickerModel) (pickerModel, error) {
	kind := initial.kind
	var input io.Reader = cmd.InOrStdin()
	file, ok := input.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		// Keep piped release-note input untouched while the user picks options.
		device := "/dev/tty"
		if runtime.GOOS == "windows" {
			device = "CONIN$"
		}
		terminal, err := os.Open(device)
		if err != nil {
			return initial, fmt.Errorf("open terminal for %s selection: %w", kind, err)
		}
		defer terminal.Close()
		input = terminal
	}
	model, err := tea.NewProgram(initial, tea.WithInput(input), tea.WithOutput(cmd.ErrOrStderr())).Run()
	if err != nil {
		return initial, fmt.Errorf("select %s: %w", kind, err)
	}
	picker := model.(pickerModel)
	if !picker.accepted {
		return picker, fmt.Errorf("%s selection cancelled", kind)
	}
	return picker, nil
}

type pickerModel struct {
	kind        string
	input       textinput.Model
	options     []string
	annotations map[string]string
	matches     []string
	cursor      int
	height      int
	width       int
	accepted    bool
	cancelled   bool
	multiple    bool
	selected    map[string]bool
}

func (m pickerModel) selectedOptions() []string {
	var options []string
	for _, option := range m.options {
		if m.selected[option] {
			options = append(options, option)
		}
	}
	return options
}

func newPickerModel(kind string, options []string) pickerModel {
	input := textinput.New()
	input.Prompt = "⌕ "
	input.Placeholder = "Type to filter"
	input.Focus()
	return pickerModel{kind: kind, input: input, options: options, matches: options, height: 10, width: 80}
}

func (m pickerModel) Init() tea.Cmd { return textinput.Blink }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(10, msg.Width-2)
		m.height = max(1, min(10, msg.Height-6))
		m.input.Width = max(1, m.width-2)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if len(m.matches) > 0 || (m.multiple && len(m.selectedOptions()) > 0) {
				if m.multiple && len(m.selectedOptions()) == 0 {
					m.selected[m.matches[m.cursor]] = true
				}
				m.accepted = true
				return m, tea.Quit
			}
			return m, nil
		case " ":
			if m.multiple {
				if len(m.matches) > 0 {
					option := m.matches[m.cursor]
					m.selected[option] = !m.selected[option]
				}
				return m, nil
			}
		case "up", "ctrl+p":
			m.cursor = max(0, m.cursor-1)
			return m, nil
		case "down", "ctrl+n":
			m.cursor = min(max(0, len(m.matches)-1), m.cursor+1)
			return m, nil
		}
	}
	before := m.input.Value()
	var command tea.Cmd
	m.input, command = m.input.Update(msg)
	if m.input.Value() != before {
		m.cursor = 0
		m.matches = m.options
		if query := m.input.Value(); query != "" {
			m.matches = nil
			for _, match := range fuzzy.Find(query, m.options) {
				m.matches = append(m.matches, m.options[match.Index])
			}
		}
	}
	return m, command
}

func (m pickerModel) View() string {
	if m.accepted || m.cancelled {
		return ""
	}
	var view strings.Builder
	title := "Choose a " + m.kind
	if m.multiple {
		title = "Choose packages to release"
	}
	view.WriteString(lipgloss.NewStyle().Bold(true).Render(title) + "\n")
	view.WriteString(m.input.View() + "\n\n")
	start := max(0, m.cursor-m.height+1)
	for i := start; i < min(len(m.matches), start+m.height); i++ {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			prefix = "❯ "
			style = style.Bold(true).Foreground(lipgloss.Color("6"))
		}
		if m.multiple {
			if m.selected[m.matches[i]] {
				prefix += "[x] "
			} else {
				prefix += "[ ] "
			}
		}
		row := style.Render(prefix + m.matches[i])
		if annotation := m.annotations[m.matches[i]]; annotation != "" {
			muted := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "242"})
			row += muted.Render(" [" + annotation + "]")
		}
		view.WriteString(lipgloss.NewStyle().MaxWidth(m.width).Render(row) + "\n")
	}
	if len(m.matches) == 0 {
		view.WriteString("No matching options\n")
	}
	if m.multiple {
		fmt.Fprintf(&view, "\n%d/%d · %d selected · ↑/↓ move · Space toggle · Enter confirm · Esc cancel", len(m.matches), len(m.options), len(m.selectedOptions()))
	} else {
		fmt.Fprintf(&view, "\n%d/%d · ↑/↓ move · Enter select · Esc cancel", len(m.matches), len(m.options))
	}
	return view.String()
}
