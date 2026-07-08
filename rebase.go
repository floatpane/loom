package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type rebaseItem struct {
	action string
	hash   string
	msg    string
	author string
	date   time.Time
}

type rebaseModel struct {
	path      string
	items     []rebaseItem
	cursor    int
	width     int
	height    int
	saved     bool
	err       error
	expanded  int // -1 = no item expanded, otherwise index into items
	diff      string
	diffErr   error
	diffVP    viewport.Model
	diffReady bool
}

func newRebaseModel(path string) *rebaseModel {
	m := &rebaseModel{path: path, expanded: -1}
	m.diffVP = viewport.New()
	m.diffVP.SoftWrap = false
	m.load()
	return m
}

func (m *rebaseModel) load() {
	f, err := os.Open(m.path)
	if err != nil {
		m.err = err
		return
	}
	defer func() { _ = f.Close() }()

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

	m.loadCommitMetadata()
}

func (m *rebaseModel) loadCommitMetadata() {
	if len(m.items) == 0 {
		return
	}

	var hashes []string
	for _, it := range m.items {
		hashes = append(hashes, it.hash)
	}

	args := append([]string{"log", "--format=%H%x1f%an%x1f%aI%x1f%s", "--no-patch"}, hashes...)
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	meta := make(map[string]rebaseItem)
	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		hash := parts[0]
		author := parts[1]
		dateStr := parts[2]
		subject := parts[3]

		var date time.Time
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			date = t
		}

		meta[hash] = rebaseItem{author: author, date: date, msg: subject}
	}

	for i := range m.items {
		if info, ok := meta[m.items[i].hash]; ok {
			if m.items[i].author == "" {
				m.items[i].author = info.author
			}
			if m.items[i].date.IsZero() {
				m.items[i].date = info.date
			}
			if m.items[i].msg == "" || m.items[i].msg == m.items[i].hash {
				m.items[i].msg = info.msg
			}
		}
	}
}

func (m *rebaseModel) loadDiff(hash string) {
	cmd := exec.Command("git", "show", "--no-color", "--patch", "--format=", hash)
	output, err := cmd.Output()
	if err != nil {
		m.diffErr = fmt.Errorf("git show failed: %v", err)
		m.diff = ""
		m.diffReady = false
		return
	}
	m.diff = string(output)
	m.diffErr = nil
	m.prepareDiffView()
}

// prepareDiffView parses the raw diff and pre-renders all lines into the
// viewport. This runs once when the diff is expanded so that View() only
// needs to slice the visible window — no re-parsing or re-highlighting.
func (m *rebaseModel) prepareDiffView() {
	files := parseUnifiedDiff(m.diff)
	if len(files) == 0 {
		m.diffVP.SetContentLines([]string{"(no changes)"})
		m.diffReady = true
		return
	}

	diffWidth := m.width
	if diffWidth <= 0 {
		diffWidth = 80
	}
	// Account for the border + left margin we add in renderExpandedDiff
	diffWidth -= 4
	if diffWidth < 20 {
		diffWidth = 20
	}

	rendered := renderDiff(files, diffWidth)
	lines := strings.Split(rendered, "\n")
	m.diffVP.SetContentLines(lines)

	// Set viewport dimensions to the available height
	diffHeight := m.height
	if diffHeight > 0 {
		// title(1) + blank(1) + commit rows + blank(1) + help(2 lines) ≈ subtract overhead
		// We'll give the diff up to half the screen
		diffHeight = max(5, m.height/2)
	}
	m.diffVP.SetWidth(diffWidth + 4)
	m.diffVP.SetHeight(diffHeight)
	m.diffReady = true
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
		if m.expanded >= 0 && m.diffReady {
			m.prepareDiffView()
		}
		return m, nil

	case tea.KeyPressMsg:
		// When diff is expanded, scroll keys go to the diff viewport
		if m.expanded >= 0 {
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
			case "tab", " ":
				m.expanded = -1
				m.diff = ""
				m.diffReady = false
			case "esc":
				m.expanded = -1
				m.diff = ""
				m.diffReady = false
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
		case "tab", " ":
			m.expanded = m.cursor
			m.loadDiff(m.items[m.cursor].hash)
		case "esc":
			return m, tea.Quit
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

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("Jan 02 2006")
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

		var msg string
		if m.expanded == i {
			msg = lipgloss.NewStyle().Bold(true).Render(it.msg)
		} else {
			msg = lipgloss.NewStyle().Render(it.msg)
		}

		authorStr := ""
		if it.author != "" {
			authorStr = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(it.author)
		}
		dateStr := ""
		if !it.date.IsZero() {
			dateStr = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(formatDate(it.date))
		}

		gutter := "  "
		marker := ""
		if m.expanded == i {
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Render("▾")
		} else {
			marker = " "
		}

		var row string
		metaParts := []string{}
		if authorStr != "" {
			metaParts = append(metaParts, authorStr)
		}
		if dateStr != "" {
			metaParts = append(metaParts, dateStr)
		}
		meta := ""
		if len(metaParts) > 0 {
			meta = "  " + strings.Join(metaParts, "  ")
		}

		row = fmt.Sprintf("%s%s %s %s %s%s", gutter, marker, action, hash, msg, meta)

		if selected {
			gutter = "▶ "
			row = fmt.Sprintf("%s%s %s %s %s%s", gutter, marker, action, hash, msg, meta)
			row = lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color("236")).
				Render(row)
		}

		rows = append(rows, row)

		if m.expanded == i {
			rows = append(rows, m.renderExpandedDiff())
		}
	}

	list := strings.Join(rows, "\n")

	helpText := "↑/k ↓/j move  •  shift+↑/K shift+↓/J reorder  •  p pick  r reword  e edit  s squash  f fixup  d drop  •  tab/space expand diff  •  enter save  q/esc cancel"
	if m.expanded >= 0 {
		helpText = "↑/k ↓/j scroll  •  pgup/pgdn page  •  g/G top/bottom  •  tab/space collapse  •  esc back  •  enter save"
	}

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(1, 1).
		Render(helpText)

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", list, "", help)

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *rebaseModel) renderExpandedDiff() string {
	if m.diffErr != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Padding(0, 2).
			Render(fmt.Sprintf("  %v", m.diffErr))
	}
	if !m.diffReady {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 2).
			Render("  loading…")
	}

	content := m.diffVP.View()

	boxStyle := lipgloss.NewStyle().
		MarginLeft(2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 0)

	return boxStyle.Render(content)
}
