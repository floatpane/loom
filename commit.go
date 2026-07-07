package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
)

type commitModel struct {
	path     string
	textarea textarea.Model
	width    int
	height   int
	saved    bool
	err      error
}

func newCommitModel(path string) *commitModel {
	ta := textarea.New()
	ta.ShowLineNumbers = true

	if data, err := os.ReadFile(path); err == nil {
		ta.SetValue(string(data))
	} else {
		ta.Placeholder = "Enter your commit message..."
	}
	ta.Focus()

	return &commitModel{path: path, textarea: ta}
}

func (m *commitModel) Init() tea.Cmd {
	return m.textarea.Focus()
}

func (m *commitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 3
		footerHeight := 3
		m.textarea.SetWidth(msg.Width)
		m.textarea.SetHeight(max(1, msg.Height-headerHeight-footerHeight))
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+s":
			if err := os.WriteFile(m.path, []byte(m.textarea.Value()), 0644); err != nil {
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
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
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

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("236")).
		Padding(0, 1).
		Width(m.width).
		Render(statusText)

	body := m.textarea.View()

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
