package main

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type commitModel struct {
	path   string
	editor *editor
	width  int
	height int
	saved  bool
	err    error
}

func newCommitModel(path string) *commitModel {
	ed := newEditor()

	if data, err := os.ReadFile(path); err == nil {
		ed.setContent(string(data))
	} else {
		ed.lines = []string{""}
	}

	return &commitModel{path: path, editor: ed}
}

func (m *commitModel) Init() tea.Cmd {
	return nil
}

func (m *commitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 3
		footerHeight := 3
		m.editor.setWidth(msg.Width)
		m.editor.setHeight(max(1, msg.Height-headerHeight-footerHeight))
		m.editor.syncToViewport()
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+s":
			if err := os.WriteFile(m.path, []byte(m.editor.value()), 0644); err != nil {
				m.err = err
				return m, nil
			}
			m.saved = true
			return m, tea.Quit
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m, tea.Quit
		}

		m.editor.handleKey(msg)
		m.editor.ensureVisible()
		return m, nil
	}
	return m, nil
}

func (m *commitModel) View() tea.View {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		Background(lipgloss.Color("236")).
		Padding(0, 1).
		Width(m.width).
		Render(fmt.Sprintf(" loom — commit message   %s", m.path))

	statusText := "ctrl+s save  •  esc cancel"
	if m.saved {
		statusText = "saved!"
	} else if m.err != nil {
		statusText = fmt.Sprintf("error: %v", m.err)
	}

	lineCount := len(m.editor.lines)
	wordCount := 0
	for _, line := range m.editor.lines {
		wordCount += len(strings.Fields(line))
	}
	statusText = fmt.Sprintf("ctrl+s save  •  esc cancel  •  %d lines  %d words", lineCount, wordCount)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("236")).
		Padding(0, 1).
		Width(m.width).
		Render(statusText)

	body := m.editor.view()

	content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
