package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type rebaseItem struct {
	action string
	hash   string
	msg    string
}

type rebaseModel struct {
	path    string
	items   []rebaseItem
	cursor  int
	width   int
	height  int
	saved   bool
	err     error
}

func newRebaseModel(path string) *rebaseModel {
	m := &rebaseModel{path: path}
	m.load()
	return m
}

func (m *rebaseModel) load() {
	f, err := os.Open(m.path)
	if err != nil {
		m.err = err
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		action := fields[0]
		hash := fields[1]
		msg := strings.Join(fields[2:], " ")
		m.items = append(m.items, rebaseItem{action: action, hash: hash, msg: msg})
	}
	if err := scanner.Err(); err != nil {
		m.err = err
	}
}

func (m *rebaseModel) write() error {
	var b strings.Builder
	for _, it := range m.items {
		fmt.Fprintf(&b, "%s %s %s\n", it.action, it.hash, it.msg)
	}
	return os.WriteFile(m.path, []byte(b.String()), 0644)
}

func (m *rebaseModel) Init() tea.Cmd {
	return nil
}

func (m *rebaseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if err := m.write(); err != nil {
				m.err = err
				return m, nil
			}
			m.saved = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "shift+up", "K":
			if m.cursor > 0 {
				m.items[m.cursor-1], m.items[m.cursor] = m.items[m.cursor], m.items[m.cursor-1]
				m.cursor--
			}
		case "shift+down", "J":
			if m.cursor < len(m.items)-1 {
				m.items[m.cursor+1], m.items[m.cursor] = m.items[m.cursor], m.items[m.cursor+1]
				m.cursor++
			}
		case "p":
			if m.cursor < len(m.items) {
				m.items[m.cursor].action = "pick"
			}
		case "r":
			if m.cursor < len(m.items) {
				m.items[m.cursor].action = "reword"
			}
		case "e":
			if m.cursor < len(m.items) {
				m.items[m.cursor].action = "edit"
			}
		case "s":
			if m.cursor < len(m.items) {
				m.items[m.cursor].action = "squash"
			}
		case "f":
			if m.cursor < len(m.items) {
				m.items[m.cursor].action = "fixup"
			}
		case "d":
			if m.cursor < len(m.items) {
				m.items[m.cursor].action = "drop"
			}
		}
	}
	return m, nil
}

func actionStyle(action string) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	switch action {
	case "pick":
		return base.Foreground(lipgloss.Color("42"))
	case "reword":
		return base.Foreground(lipgloss.Color("39"))
	case "edit":
		return base.Foreground(lipgloss.Color("213"))
	case "squash":
		return base.Foreground(lipgloss.Color("214"))
	case "fixup":
		return base.Foreground(lipgloss.Color("208"))
	case "drop":
		return base.Foreground(lipgloss.Color("196"))
	default:
		return base.Foreground(lipgloss.Color("245"))
	}
}

func (m *rebaseModel) View() tea.View {
	if m.err != nil {
		content := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(
			fmt.Sprintf("error: %v", m.err))
		return tea.NewView(content)
	}
	if len(m.items) == 0 {
		return tea.NewView(lipgloss.NewStyle().Faint(true).Render("No commits to rebase."))
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		Padding(0, 1).
		Render("loom — interactive rebase")

	var rows []string
	for i, it := range m.items {
		selected := i == m.cursor
		action := actionStyle(it.action).Render(it.action)
		hash := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(it.hash)
		msg := lipgloss.NewStyle().Render(it.msg)

		gutter := "  "
		row := fmt.Sprintf("%s %s %s %s", gutter, action, hash, msg)

		if selected {
			gutter = "▶ "
			row = fmt.Sprintf("%s %s %s %s", gutter, action, hash, msg)
			row = lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color("236")).
				Render(row)
		}

		rows = append(rows, row)
	}

	list := strings.Join(rows, "\n")

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(1, 1).
		Render("↑/k ↓/j move  •  shift+↑/K shift+↓/J reorder  •  p pick  r reword  e edit  s squash  f fixup  d drop  •  enter save  q cancel")

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", list, "", help)

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
