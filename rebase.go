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
	"github.com/floatpane/bubble-overlay"
)

type rebaseItem struct {
	action string
	hash   string
	msg    string
	author string
	date   time.Time
	// For exec/break/label/reset/merge commands, hash may be empty and
	// msg holds the command or label text.
}

type rebaseModel struct {
	path       string
	items      []rebaseItem
	cursor     int
	width      int
	height     int
	saved      bool
	err        error
	expanded   int // -1 = no item expanded, otherwise index into items
	diff       string
	diffErr    error
	diffVP     viewport.Model
	diffReady  bool
	helpVisible bool
	searchQuery string
	searchActive bool
}

func newRebaseModel(path string) *rebaseModel {
	m := &rebaseModel{path: path, expanded: -1, helpVisible: false}
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
		if len(fields) < 1 {
			continue
		}
		action := fields[0]
		switch action {
		case "p", "pick", "r", "reword", "e", "edit", "s", "squash",
			"f", "fixup", "d", "drop", "x", "exec", "b", "break",
			"l", "label", "t", "reset", "m", "merge", "u", "update-ref":
			if action == "x" || action == "exec" {
				// exec command — everything after the action is the command
				msg := strings.TrimSpace(line[len(action):])
				m.items = append(m.items, rebaseItem{action: "exec", msg: msg})
			} else if action == "b" || action == "break" {
				m.items = append(m.items, rebaseItem{action: "break"})
			} else if action == "l" || action == "label" {
				msg := strings.TrimSpace(line[len(action):])
				m.items = append(m.items, rebaseItem{action: "label", msg: msg})
			} else if action == "t" || action == "reset" {
				msg := strings.TrimSpace(line[len(action):])
				m.items = append(m.items, rebaseItem{action: "reset", msg: msg})
			} else if action == "m" || action == "merge" {
				// merge [-C <commit>] <commit> [#<msg>]
				rest := strings.TrimSpace(line[len(action):])
				m.items = append(m.items, rebaseItem{action: "merge", msg: rest})
			} else if action == "u" || action == "update-ref" {
				m.items = append(m.items, rebaseItem{action: "update-ref"})
			} else if len(fields) >= 3 {
				// Normal pick/reword/edit/squash/fixup/drop with hash and msg
				hash := fields[1]
				msg := strings.Join(fields[2:], " ")
				// Normalize short action names
				switch action {
				case "p":
					action = "pick"
				case "r":
					action = "reword"
				case "e":
					action = "edit"
				case "s":
					action = "squash"
				case "f":
					action = "fixup"
				case "d":
					action = "drop"
				}
				m.items = append(m.items, rebaseItem{action: action, hash: hash, msg: msg})
			}
		default:
			// Unknown action, skip
		}
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
		if it.hash != "" {
			hashes = append(hashes, it.hash)
		}
	}
	if len(hashes) == 0 {
		return
	}

	args := append([]string{"log", "--format=%H%x1f%an%x1f%aI%x1f%s", "--no-patch"}, hashes...)
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	// Build a map keyed by both full and short (first 7) hashes
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

		item := rebaseItem{author: author, date: date, msg: subject}
		meta[hash] = item
		// Also index by short hash (first 7 chars)
		if len(hash) >= 7 {
			meta[hash[:7]] = item
		}
	}

	for i := range m.items {
		if m.items[i].hash == "" {
			continue
		}
		// Try exact match first, then prefix match
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
		} else {
			// Try matching by prefix: find a key that starts with the short hash
			shortHash := m.items[i].hash
			for fullHash, info := range meta {
				if strings.HasPrefix(fullHash, shortHash) {
					if m.items[i].author == "" {
						m.items[i].author = info.author
					}
					if m.items[i].date.IsZero() {
						m.items[i].date = info.date
					}
					if m.items[i].msg == "" || m.items[i].msg == m.items[i].hash {
						m.items[i].msg = info.msg
					}
					break
				}
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
	content, err := m.writeToString()
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, []byte(content), 0644)
}

func (m *rebaseModel) writeToString() (string, error) {
	var b strings.Builder
	for _, it := range m.items {
		switch it.action {
		case "exec":
			fmt.Fprintf(&b, "exec %s\n", it.msg)
		case "break":
			fmt.Fprintf(&b, "break\n")
		case "label":
			fmt.Fprintf(&b, "label %s\n", it.msg)
		case "reset":
			fmt.Fprintf(&b, "reset %s\n", it.msg)
		case "merge":
			fmt.Fprintf(&b, "merge %s\n", it.msg)
		case "update-ref":
			fmt.Fprintf(&b, "update-ref\n")
		default:
			if it.hash != "" {
				fmt.Fprintf(&b, "%s %s %s\n", it.action, it.hash, it.msg)
			} else {
				fmt.Fprintf(&b, "%s\n", it.action)
			}
		}
	}
	return b.String(), nil
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
		case "ctrl+h", "f1":
			m.helpVisible = !m.helpVisible
			return m, nil
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
			if m.cursor < len(m.items) && m.items[m.cursor].hash != "" {
				m.expanded = m.cursor
				m.loadDiff(m.items[m.cursor].hash)
			}
		case "esc":
			if m.helpVisible {
				m.helpVisible = false
				return m, nil
			}
			if m.searchActive {
				m.searchActive = false
				m.searchQuery = ""
				return m, nil
			}
			return m, tea.Quit
		// Action keybindings
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
		// Action cycling (space already used for expand, use 'c')
		case "c":
			if m.cursor < len(m.items) && m.items[m.cursor].hash != "" {
				m.items[m.cursor].action = cycleAction(m.items[m.cursor].action)
			}
		// Squash all up — squash everything from cursor to the first pick above
		case "S":
			m.squashAllUp()
		// Search
		case "/":
			m.searchActive = true
			m.searchQuery = ""
			return m, nil
		case "n":
			if m.searchQuery != "" {
				m.searchNext()
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
	case "exec":
		return base.Foreground(lipgloss.Color("141")).Italic(true)
	case "break":
		return base.Foreground(lipgloss.Color("203"))
	case "label":
		return base.Foreground(lipgloss.Color("99"))
	case "reset":
		return base.Foreground(lipgloss.Color("203"))
	case "merge":
		return base.Foreground(lipgloss.Color("117"))
	case "update-ref":
		return base.Foreground(lipgloss.Color("245"))
	default:
		return base.Foreground(lipgloss.Color("245"))
	}
}

// cycleAction cycles through pick → reword → edit → squash → fixup → drop → pick
func cycleAction(action string) string {
	switch action {
	case "pick":
		return "reword"
	case "reword":
		return "edit"
	case "edit":
		return "squash"
	case "squash":
		return "fixup"
	case "fixup":
		return "drop"
	case "drop":
		return "pick"
	default:
		return "pick"
	}
}

// squashAllUp sets all items from the first pick down to the cursor
// (exclusive of the first pick) to squash, collapsing all commits into
// the topmost pick.
func (m *rebaseModel) squashAllUp() {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return
	}
	// Find the first pick in the list
	start := -1
	for i := 0; i <= m.cursor; i++ {
		if m.items[i].action == "pick" {
			start = i
			break
		}
	}
	if start < 0 {
		return
	}
	// Squash everything after the first pick up to and including cursor
	for i := start + 1; i <= m.cursor && i < len(m.items); i++ {
		if m.items[i].hash != "" {
			m.items[i].action = "squash"
		}
	}
}

// searchNext moves cursor to the next item matching the search query.
func (m *rebaseModel) searchNext() {
	if m.searchQuery == "" {
		return
	}
	query := strings.ToLower(m.searchQuery)
	for i := m.cursor + 1; i < len(m.items); i++ {
		if strings.Contains(strings.ToLower(m.items[i].msg), query) ||
			strings.Contains(strings.ToLower(m.items[i].hash), query) ||
			strings.Contains(strings.ToLower(m.items[i].author), query) {
			m.cursor = i
			return
		}
	}
	// Wrap around
	for i := 0; i <= m.cursor; i++ {
		if strings.Contains(strings.ToLower(m.items[i].msg), query) ||
			strings.Contains(strings.ToLower(m.items[i].hash), query) ||
			strings.Contains(strings.ToLower(m.items[i].author), query) {
			m.cursor = i
			return
		}
	}
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("Jan 02 2006")
}

// wrapText word-wraps text to the given width, returning a slice of lines.
// Uses simple space-based wrapping (no hyphenation).
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	var current strings.Builder
	currentLen := 0
	for _, w := range words {
		wLen := len(w)
		if currentLen > 0 && currentLen+1+wLen > width {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(w)
			currentLen = wLen
		} else {
			if currentLen > 0 {
				current.WriteString(" ")
				currentLen++
			}
			current.WriteString(w)
			currentLen += wLen
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

// renderFooterBar renders a one-line or multi-line footer bar that is
// always pinned to the bottom of the screen. When expanded is true,
// the full help text is shown with word wrapping; otherwise a compact
// single line is shown.
func renderFooterBar(compactText, expandedText string, width int, expanded bool) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("236")).
		Padding(0, 1).
		Width(width)

	if expanded {
		wrapped := wrapText(expandedText, width-2)
		return style.Render(strings.Join(wrapped, "\n"))
	}
	return style.Render(compactText)
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
		Render(fmt.Sprintf("loom — interactive rebase  (%d commits)", len(m.items)))

	var rows []string
	for i, it := range m.items {
		selected := i == m.cursor
		action := actionStyle(it.action).Render(it.action)

		hashStr := ""
		if it.hash != "" {
			hashStr = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(it.hash)
		}

		var msg string
		displayMsg := it.msg
		if it.hash == "" && it.action == "break" {
			displayMsg = "(stop here)"
		}
		if displayMsg == "" && it.action == "break" {
			displayMsg = "(stop here)"
		}
		if m.expanded == i {
			msg = lipgloss.NewStyle().Bold(true).Render(displayMsg)
		} else {
			msg = lipgloss.NewStyle().Render(displayMsg)
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

		row = fmt.Sprintf("%s%s %s %s %s%s", gutter, marker, action, hashStr, msg, meta)

		if selected {
			gutter = "▶ "
			row = fmt.Sprintf("%s%s %s %s %s%s", gutter, marker, action, hashStr, msg, meta)
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

	commitCount := len(m.items)
	compactHelp := fmt.Sprintf("↑/k ↓/j move  p/r/e/s/f/d action  c cycle  S squash-all  tab diff  / search  ctrl+h help  enter save  q quit  •  %d commits", commitCount)
	if m.expanded >= 0 {
		compactHelp = "↑/k ↓/j scroll  pgup/pgdn page  g/G top/bottom  tab/space collapse  esc back  enter save  ctrl+h help"
	}
	if m.searchActive {
		compactHelp = "search: " + m.searchQuery + "  •  esc cancel  •  n next match  •  ctrl+h help"
	}

	footer := renderFooterBar(compactHelp, "", m.width, false)

	// Pin footer at bottom: compute content height, pad, then append footer
	titleSection := lipgloss.JoinVertical(lipgloss.Left, title, "", list)
	contentHeight := strings.Count(titleSection, "\n") + 1
	footerHeight := strings.Count(footer, "\n") + 1
	padLines := m.height - contentHeight - footerHeight
	if padLines < 0 {
		padLines = 0
	}

	allSections := append([]string{titleSection}, make([]string, padLines)...)
	allSections = append(allSections, footer)
	content := lipgloss.JoinVertical(lipgloss.Left, allSections...)

	// Render help as a centered floating overlay
	if m.helpVisible {
		content = overlay.Center(content, m.renderHelpOverlay(), m.width, m.height)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *rebaseModel) renderHelpOverlay() string {
	helpText := `loom rebase — keybindings

  ↑/k ↓/j      move cursor          p/r/e/s/f/d  set action
  shift+↑/K    reorder up           c            cycle action
  shift+↓/J    reorder down         S            squash-all-up
  tab/space    expand/collapse diff /            search
  n            next search match    esc          cancel search
  enter        save & quit          q/ctrl+c     quit without saving
  ctrl+h/f1    toggle this help`
	help := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Background(lipgloss.Color("234")).
		Padding(1, 2).
		Width(60).
		Render(helpText)
	return help
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
