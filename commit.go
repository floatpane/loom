package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/floatpane/bubble-overlay"
)

type commitModel struct {
	path        string
	editor      *editor
	diffVP      viewport.Model
	diffRaw     string
	diffReady   bool
	diffFromGit bool
	infoLines   []string
	infoRaw     string
	author      string
	date        time.Time
	people      *peopleStore
	width       int
	height      int
	saved       bool
	err         error
	diffFocus   bool
	fullscreen  bool

	// new fields
	branch       string
	stagedFiles  []stagedFile
	diffStat     string
	helpVisible  bool
	exitConfirm  bool
	dirty        bool // unsaved changes
	mode         string // "commit", "merge", "tag"
	cfg          *loomConfig
}

func newCommitModel(path string) *commitModel {
	ed := newEditor()
	ed.commitMode = true
	m := &commitModel{path: path, editor: ed, mode: "commit"}
	m.diffVP = viewport.New()
	m.diffVP.SoftWrap = false

	m.loadCommitMeta()
	m.people = newPeopleStore()
	ed.people = m.people
	m.cfg = loadLoomConfig()

	// Load git context for suggestions
	loadSuggestionCtx()
	m.branch = currentBranchName()
	m.stagedFiles = loadStagedFiles()
	m.diffStat = loadDiffStat()

	// Detect mode from path
	m.mode = detectEditorMode(path)

	if data, err := os.ReadFile(path); err == nil {
		m.parseCommitFile(string(data))
	} else {
		// Try loading commit template
		tmpl := loadCommitTemplate()
		if tmpl != "" {
			m.parseCommitFile(tmpl)
		} else {
			ed.lines = []string{""}
		}
	}

	// If config has signoff, add Signed-off-by trailer template
	if m.cfg.Signoff && m.author != "" {
		m.ensureSignoff()
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

// detectEditorMode determines what kind of git message file this is.
func detectEditorMode(path string) string {
	_ = filepath.Base(path)
	switch {
	case strings.Contains(path, "MERGE_MSG"):
		return "merge"
	case strings.Contains(path, "TAG_EDITMSG") || strings.Contains(path, "tag"):
		return "tag"
	case strings.Contains(path, "COMMIT_EDITMSG"):
		return "commit"
	case strings.Contains(path, "git-rebase-todo"):
		return "rebase"
	default:
		return "commit"
	}
}

// ensureSignoff adds a Signed-off-by trailer if not already present.
func (m *commitModel) ensureSignoff() {
	email, _ := exec.Command("git", "config", "user.email").Output()
	emailStr := strings.TrimSpace(string(email))
	if m.author == "" || emailStr == "" {
		return
	}
	signoff := "Signed-off-by: " + m.author + " <" + emailStr + ">"
	for _, line := range m.editor.lines {
		if strings.TrimSpace(line) == signoff {
			return
		}
	}
	// Ensure blank line before trailers
	if len(m.editor.lines) > 0 && m.editor.lines[len(m.editor.lines)-1] != "" {
		m.editor.lines = append(m.editor.lines, "")
	}
	m.editor.lines = append(m.editor.lines, signoff)
	m.editor.syncToViewport()
}

// addCoAuthor adds a Co-authored-by trailer for the current git user.
func (m *commitModel) addCoAuthor() {
	email, _ := exec.Command("git", "config", "user.email").Output()
	emailStr := strings.TrimSpace(string(email))
	if m.author == "" || emailStr == "" {
		return
	}
	coAuthor := "Co-authored-by: " + m.author + " <" + emailStr + ">"
	for _, line := range m.editor.lines {
		if strings.TrimSpace(line) == coAuthor {
			return
		}
	}
	// Ensure blank line before trailers
	if len(m.editor.lines) > 0 && m.editor.lines[len(m.editor.lines)-1] != "" {
		m.editor.lines = append(m.editor.lines, "")
	}
	m.editor.lines = append(m.editor.lines, coAuthor)
	m.editor.syncToViewport()
	m.dirty = true
}

// autosaveDraft saves the current message to ~/.loom/draft.txt.
func (m *commitModel) autosaveDraft() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	draftPath := filepath.Join(home, ".loom", "draft.txt")
	content := m.editor.value()
	_ = os.MkdirAll(filepath.Dir(draftPath), 0755)
	_ = os.WriteFile(draftPath, []byte(content), 0644)
}

// loadDraft loads a previously saved draft if one exists.
func loadDraft() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	draftPath := filepath.Join(home, ".loom", "draft.txt")
	data, err := os.ReadFile(draftPath)
	if err != nil {
		return ""
	}
	return string(data)
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
	} else {
		m.loadDiffFromGit()
	}
}

func (m *commitModel) loadDiffFromGit() {
	cmd := exec.Command("git", "diff", "--cached", "--no-color")
	output, err := cmd.Output()
	if err != nil {
		return
	}
	diff := strings.TrimSpace(string(output))
	if diff == "" {
		return
	}
	m.diffRaw = diff
	m.diffFromGit = true
	m.prepareDiffView()
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
				if m.fullscreen {
					m.layout()
				}
				return m, nil
			case "ctrl+f":
				m.fullscreen = !m.fullscreen
				m.layout()
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
			if m.dirty && !m.exitConfirm {
				m.exitConfirm = true
				return m, nil
			}
			return m, tea.Quit
		case "esc":
			if len(m.editor.suggestions) > 0 {
				m.editor.suggestions = nil
				m.editor.selSug = -1
				return m, nil
			}
			if m.helpVisible {
				m.helpVisible = false
				return m, nil
			}
			if m.exitConfirm {
				m.exitConfirm = false
				return m, nil
			}
			if m.dirty {
				m.exitConfirm = true
				return m, nil
			}
			return m, tea.Quit
		case "ctrl+f":
			m.fullscreen = !m.fullscreen
			m.layout()
			return m, nil
		case "ctrl+h", "f1":
			m.helpVisible = !m.helpVisible
			return m, nil
		case "ctrl+o":
			m.addCoAuthor()
			return m, nil
		case "tab":
			if len(m.editor.suggestions) > 0 && m.editor.selSug >= 0 {
				m.editor.acceptSuggestion()
				return m, nil
			}
			if m.diffReady {
				m.diffFocus = true
				m.editor.focused = false
				if m.fullscreen {
					m.layout()
				}
			}
			return m, nil
		}

		m.editor.handleKey(msg)
		m.editor.ensureVisible()
		m.dirty = true
		m.autosaveDraft()
		return m, nil
	}
	return m, nil
}

func (m *commitModel) save() tea.Cmd {
	// Extract and persist any people found in trailer lines
	if m.people != nil {
		people := extractPeopleFromTrailerLines(m.editor.lines)
		for _, p := range people {
			m.people.addPerson(p)
		}
		m.people.save()
	}

	content := m.editor.value()
	if m.infoRaw != "" {
		content = content + "\n" + m.infoRaw + "\n"
	}
	if m.diffRaw != "" && !m.diffFromGit {
		content = content + m.diffRaw
	}
	if err := os.WriteFile(m.path, []byte(content), 0644); err != nil {
		m.err = err
		return nil
	}
	m.saved = true
	m.dirty = false
	m.exitConfirm = false

	// Clear draft
	home, err := os.UserHomeDir()
	if err == nil {
		_ = os.Remove(filepath.Join(home, ".loom", "draft.txt"))
	}

	return tea.Quit
}

func (m *commitModel) layout() {
	headerHeight := 1
	footerHeight := 1
	bodyHeight := m.height - headerHeight - footerHeight

	if m.fullscreen {
		if m.diffFocus && m.diffReady {
			m.diffVP.SetWidth(m.width)
			m.diffVP.SetHeight(bodyHeight)
		} else {
			m.editor.setWidth(m.width)
			m.editor.setHeight(bodyHeight)
		}
		return
	}

	if m.diffReady {
		// Give the diff the lion's share of the screen — editor stays small
		infoHeight := 0
		if len(m.infoLines) > 0 {
			infoHeight = len(m.infoLines) + 2
			if infoHeight > 8 {
				infoHeight = 8
			}
		}
		editorHeight := 6
		if editorHeight < 4 {
			editorHeight = 4
		}
		diffHeight := bodyHeight - editorHeight - infoHeight - 3
		if diffHeight < 8 {
			diffHeight = 8
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
		editorHeight := bodyHeight - infoHeight
		if editorHeight < 3 {
			editorHeight = 3
		}
		m.editor.setWidth(m.width)
		m.editor.setHeight(editorHeight)
	}
}

func (m *commitModel) View() tea.View {
	modeLabel := "commit message"
	switch m.mode {
	case "merge":
		modeLabel = "merge message"
	case "tag":
		modeLabel = "tag annotation"
	}

	headerText := fmt.Sprintf(" loom — %s   %s", modeLabel, m.path)
	if m.branch != "" && m.branch != "HEAD" {
		headerText += fmt.Sprintf("   ⎇ %s", m.branch)
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		Background(lipgloss.Color("236")).
		Padding(0, 1).
		Width(m.width).
		Render(headerText)

	value := m.editor.value()
	lineCount := len(m.editor.lines)
	wordCount := len(strings.Fields(value))
	subjectLen := 0
	if lineCount > 0 {
		subjectLen = visibleWidth(m.editor.lines[0])
	}

	focusLabel := "message"
	if m.diffFocus {
		focusLabel = "diff"
	}

	fsLabel := "fullscreen"
	if m.fullscreen {
		fsLabel = "unfullscreen"
	}

	var statusText string

	if m.fullscreen {
		statusText = fmt.Sprintf("ctrl+s save  •  esc cancel  •  ctrl+f %s  •  ctrl+h help", fsLabel)
		if m.diffReady {
			statusText += fmt.Sprintf("  •  tab: focus %s", focusLabel)
		}
	} else {
		statusText = fmt.Sprintf("ctrl+s save  •  esc cancel  •  ctrl+f %s  •  ctrl+h help  •  ctrl+o co-author  •  %dL %dW subj:%d", fsLabel, lineCount, wordCount, subjectLen)
		if m.diffReady {
			statusText = fmt.Sprintf("ctrl+s save  •  esc cancel  •  ctrl+f %s  •  ctrl+h help  •  %dL %dW subj:%d  •  tab: focus %s", fsLabel, lineCount, wordCount, subjectLen, focusLabel)
		}
	}

	if m.saved {
		statusText = "saved!"
	} else if m.err != nil {
		statusText = fmt.Sprintf("error: %v", m.err)
	} else if m.exitConfirm {
		statusText = "unsaved changes — press esc/ctrl+c again to discard, ctrl+s to save"
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("236")).
		Padding(0, 1).
		Width(m.width).
		Render(statusText)

	var contentSections []string
	contentSections = append(contentSections, header)

	if m.fullscreen {
		if m.diffFocus && m.diffReady {
			contentSections = append(contentSections, m.diffVP.View())
		} else {
			contentSections = append(contentSections, m.editor.view())
		}
	} else {
		contentSections = append(contentSections, m.editor.view())

		if len(m.infoLines) > 0 {
			contentSections = append(contentSections, m.renderInfoPanel())
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

			contentSections = append(contentSections, divider, diffBox)
		}
	}

	contentAbove := lipgloss.JoinVertical(lipgloss.Left, contentSections...)
	contentHeight := strings.Count(contentAbove, "\n") + 1
	footerHeight := strings.Count(footer, "\n") + 1
	padLines := m.height - contentHeight - footerHeight
	if padLines < 0 {
		padLines = 0
	}

	allSections := append(contentSections, make([]string, padLines)...)
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

func (m *commitModel) renderHelpOverlay() string {
	helpText := `loom — keybindings

  ctrl+s     save & quit          ctrl+z     undo
  esc        cancel / close       ctrl+y     redo
  ctrl+c     force quit           ctrl+t     transpose chars
  ctrl+f     toggle fullscreen    ctrl+d     duplicate line
  ctrl+h/f1  toggle this help     alt+↑/↓    move line up/down
  ctrl+o     add co-author        ctrl+w     delete word
  tab        accept / focus diff  ctrl+n     search next
  ↑↓←→       cursor               ctrl+g     goto line
  ctrl+←/→   jump by word         ctrl+l     clear all

  Autocomplete: type → then tab to accept
  Conventional: type prefix (feat, fix, …) → tab
  Scopes: type "feat(" → scope suggestions
  Gitmoji: type ":sparkles" → emoji insert
  Trailers: on body lines, type prefix → tab
  Co-authors: "Co-authored-by: " → name suggestions`
	help := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Background(lipgloss.Color("234")).
		Padding(1, 2).
		Width(60).
		Render(helpText)
	return help
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
