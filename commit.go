package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type commitModel struct {
	path      string
	editor    *editor
	diffVP    viewport.Model
	diffRaw   string
	diffReady bool
	infoLines []string
	infoRaw   string
	author    string
	date      time.Time
	coAuthors []coAuthor
	width     int
	height    int
	saved     bool
	err       error
	diffFocus bool
}

func newCommitModel(path string) *commitModel {
	ed := newEditor()
	ed.commitMode = true
	m := &commitModel{path: path, editor: ed}
	m.diffVP = viewport.New()
	m.diffVP.SoftWrap = false

	m.loadCommitMeta()
	m.coAuthors = loadCoAuthors()
	ed.coAuthors = m.coAuthors

	if data, err := os.ReadFile(path); err == nil {
		m.parseCommitFile(string(data))
	} else {
		ed.lines = []string{""}
	}

	return m
}

func (m *commitModel) loadCommitMeta() {
	cmd := exec.Command("git", "config", "user.name")
	if out, err := cmd.Output(); err == nil {
		m.author = strings.TrimSpace(string(out))
	}
	m.date = time.Now()
}

func (m *commitModel) parseCommitFile(content string) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	commentStart := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			commentStart = i
			break
		}
	}

	diffStart := len(lines)
	for i := commentStart; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "diff --git") {
			diffStart = i
			break
		}
	}

	msgLines := lines[:commentStart]
	for len(msgLines) > 0 && strings.TrimSpace(msgLines[len(msgLines)-1]) == "" {
		msgLines = msgLines[:len(msgLines)-1]
	}
	if len(msgLines) == 0 {
		msgLines = []string{""}
	}
	m.editor.lines = msgLines
	m.editor.row = 0
	m.editor.col = 0
	m.editor.syncToViewport()

	var infoLines []string
	var infoRawLines []string
	for i := commentStart; i < diffStart; i++ {
		line := lines[i]
		infoRawLines = append(infoRawLines, line)
		if strings.HasPrefix(line, "# ") {
			infoLines = append(infoLines, line[2:])
		} else if line == "#" {
			infoLines = append(infoLines, "")
		} else {
			infoLines = append(infoLines, line)
		}
	}
	m.infoLines = infoLines
	m.infoRaw = strings.Join(infoRawLines, "\n")

	if diffStart < len(lines) {
		m.diffRaw = strings.Join(lines[diffStart:], "\n")
		m.prepareDiffView()
	}
}

func (m *commitModel) prepareDiffView() {
	files := parseUnifiedDiff(m.diffRaw)
	if len(files) == 0 {
		m.diffVP.SetContentLines([]string{"(no diff)"})
		m.diffReady = true
		return
	}

	diffWidth := m.width
	if diffWidth <= 0 {
		diffWidth = 80
	}
	diffWidth -= 4
	if diffWidth < 20 {
		diffWidth = 20
	}

	rendered := renderDiff(files, diffWidth)
	m.diffVP.SetContentLines(strings.Split(rendered, "\n"))
	m.diffVP.SetWidth(diffWidth + 4)
	m.diffReady = true
}

func (m *commitModel) Init() tea.Cmd {
	return nil
}

func (m *commitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case tea.KeyPressMsg:
		if m.diffFocus {
			switch msg.String() {
			case "ctrl+s":
				return m, m.save()
			case "ctrl+c":
				return m, tea.Quit
			case "esc", "tab":
				m.diffFocus = false
				m.editor.focused = true
				return m, nil
			case "up", "k":
				m.diffVP.ScrollUp(1)
			case "down", "j":
				m.diffVP.ScrollDown(1)
			case "pgup":
				m.diffVP.PageUp()
			case "pgdown":
				m.diffVP.PageDown()
			case "ctrl+u":
				m.diffVP.HalfPageUp()
			case "ctrl+d":
				m.diffVP.HalfPageDown()
			case "g":
				m.diffVP.GotoTop()
			case "G":
				m.diffVP.GotoBottom()
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+s":
			return m, m.save()
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if len(m.editor.suggestions) > 0 {
				m.editor.suggestions = nil
				m.editor.selSug = -1
				return m, nil
			}
			return m, tea.Quit
		case "tab":
			if len(m.editor.suggestions) > 0 && m.editor.selSug >= 0 {
				m.editor.acceptSuggestion()
				return m, nil
			}
			if m.diffReady {
				m.diffFocus = true
				m.editor.focused = false
			}
			return m, nil
		}

		m.editor.handleKey(msg)
		m.editor.ensureVisible()
		return m, nil
	}
	return m, nil
}

func (m *commitModel) save() tea.Cmd {
	content := m.editor.value()
	if m.infoRaw != "" {
		content = content + "\n" + m.infoRaw + "\n"
	}
	if m.diffRaw != "" {
		content = content + m.diffRaw
	}
	if err := os.WriteFile(m.path, []byte(content), 0644); err != nil {
		m.err = err
		return nil
	}
	m.saved = true
	return tea.Quit
}

func (m *commitModel) layout() {
	headerHeight := 3 // header line + padding
	footerHeight := 3 // footer line + padding
	bodyHeight := m.height - headerHeight - footerHeight

	if m.diffReady {
		diffHeight := min(bodyHeight/2, 20)
		if diffHeight < 5 {
			diffHeight = 5
		}
		infoHeight := 0
		if len(m.infoLines) > 0 {
			infoHeight = len(m.infoLines) + 2
			if infoHeight > 10 {
				infoHeight = 10
			}
		}
		editorHeight := bodyHeight - diffHeight - infoHeight - 2
		if editorHeight < 3 {
			editorHeight = 3
		}
		m.editor.setWidth(m.width)
		m.editor.setHeight(editorHeight)
		m.diffVP.SetWidth(m.width)
		m.diffVP.SetHeight(diffHeight)
		m.prepareDiffView()
	} else {
		infoHeight := 0
		if len(m.infoLines) > 0 {
			infoHeight = len(m.infoLines) + 2
			if infoHeight > 10 {
				infoHeight = 10
			}
		}
		editorHeight := bodyHeight - infoHeight - 1
		if editorHeight < 3 {
			editorHeight = 3
		}
		m.editor.setWidth(m.width)
		m.editor.setHeight(editorHeight)
	}
}

func (m *commitModel) View() tea.View {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		Background(lipgloss.Color("236")).
		Padding(0, 1).
		Width(m.width).
		Render(fmt.Sprintf(" loom — commit message   %s", m.path))

	value := m.editor.value()
	lineCount := len(m.editor.lines)
	wordCount := len(strings.Fields(value))

	focusLabel := "message"
	if m.diffFocus {
		focusLabel = "diff"
	}

	statusText := fmt.Sprintf("ctrl+s save  •  esc cancel  •  %d lines  %d words", lineCount, wordCount)
	if m.diffReady {
		statusText = fmt.Sprintf("ctrl+s save  •  esc cancel  •  %d lines  %d words  •  tab: focus %s", lineCount, wordCount, focusLabel)
	}
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

	editorBody := m.editor.view()

	var sections []string
	sections = append(sections, header, editorBody)

	if len(m.infoLines) > 0 {
		sections = append(sections, m.renderInfoPanel())
	}

	if m.diffReady {
		divLabel := " diff "
		if m.diffFocus {
			divLabel = " diff (focused) "
		}
		divider := lipgloss.NewStyle().
			Foreground(lipgloss.Color("238")).
			Background(lipgloss.Color("236")).
			Width(m.width).
			Render(strings.Repeat("─", 4) + divLabel + strings.Repeat("─", max(0, m.width-4-len(divLabel))))

		diffContent := m.diffVP.View()
		diffBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 0).
			Render(diffContent)

		sections = append(sections, divider, diffBox)
	}

	sections = append(sections, footer)
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)


	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *commitModel) renderInfoPanel() string {
	var lines []string

	dateStr := m.date.Format("Jan 02 2006 15:04")
	authorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	lines = append(lines, fmt.Sprintf("%s  %s", authorStyle.Render(m.author), dateStyle.Render(dateStr)))

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	for _, line := range m.infoLines {
		if line == "" {
			lines = append(lines, "")
			continue
		}
		if strings.Contains(line, ">8") {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(line))
			continue
		}
		if strings.HasPrefix(line, "	") || strings.HasPrefix(line, "    ") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
				fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
				lines = append(lines, fmt.Sprintf("  %s: %s", statusStyle.Render(strings.TrimSpace(parts[0])), fileStyle.Render(strings.TrimSpace(parts[1]))))
				continue
			}
		}
		if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "	") {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).Render(line))
			continue
		}
		lines = append(lines, dimStyle.Render(line))
	}

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Width(m.width).
		Render(strings.Join(lines, "\n"))

	return panel
}
