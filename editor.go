package main

import (
	"image/color"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/floatpane/bubble-overlay"
)

// editor is a lightweight, virtualized text editor built on the bubbles
// viewport. It stores content as plain text (no ANSI codes) and applies
// highlighting only during rendering, so the user never edits escape codes.
type editor struct {
	vp          viewport.Model
	lines       []string
	row         int
	col         int
	width       int
	height      int
	focused     bool
	commitMode  bool
	suggestions []suggestion
	selSug      int
	people      *peopleStore

	// undo/redo stacks
	undoStack [][]string
	redoStack [][]string
	undoRow   int
	undoCol   int

	// search state
	searchQuery  string
	searchActive bool

	// display options
	showLineNums bool
}

func newEditor() *editor {
	vp := viewport.New()
	vp.SoftWrap = false
	return &editor{
		vp:          vp,
		lines:       []string{""},
		focused:     true,
		selSug:      -1,
		showLineNums: false,
	}
}

func (e *editor) setContent(content string) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if content == "" {
		e.lines = []string{""}
	} else {
		e.lines = strings.Split(content, "\n")
	}
	e.row = 0
	e.col = 0
	e.syncToViewport()
}

func (e *editor) value() string {
	return strings.Join(e.lines, "\n")
}

func (e *editor) setWidth(w int) {
	e.width = w
	e.vp.SetWidth(w)
}

func (e *editor) setHeight(h int) {
	e.height = h
	e.vp.SetHeight(h)
}

func (e *editor) syncToViewport() {
	e.vp.SetContentLines(e.lines)
}

func (e *editor) ensureVisible() {
	e.vp.EnsureVisible(e.row, e.col, e.col+1)
}

// --- Editing operations ---

func (e *editor) insertRune(r rune) {
	if e.row < 0 || e.row >= len(e.lines) {
		return
	}
	line := e.lines[e.row]
	runes := []rune(line)
	if e.col > len(runes) {
		e.col = len(runes)
	}
	runes = append(runes[:e.col], append([]rune{r}, runes[e.col:]...)...)
	e.lines[e.row] = string(runes)
	e.col++
	e.syncToViewport()
}

func (e *editor) insertNewline() {
	if e.row < 0 || e.row >= len(e.lines) {
		return
	}
	line := e.lines[e.row]
	runes := []rune(line)
	if e.col > len(runes) {
		e.col = len(runes)
	}
	before := string(runes[:e.col])
	after := string(runes[e.col:])
	e.lines[e.row] = before
	e.lines = append(e.lines[:e.row+1], append([]string{after}, e.lines[e.row+1:]...)...)
	e.row++
	e.col = 0
	e.syncToViewport()
}

func (e *editor) deleteBackward() {
	if e.col > 0 {
		line := e.lines[e.row]
		runes := []rune(line)
		runes = append(runes[:e.col-1], runes[e.col:]...)
		e.lines[e.row] = string(runes)
		e.col--
		e.syncToViewport()
	} else if e.row > 0 {
		prevLine := e.lines[e.row-1]
		curLine := e.lines[e.row]
		e.col = len([]rune(prevLine))
		e.lines[e.row-1] = prevLine + curLine
		e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
		e.row--
		e.syncToViewport()
	}
}

func (e *editor) deleteForward() {
	if e.row < 0 || e.row >= len(e.lines) {
		return
	}
	line := e.lines[e.row]
	runes := []rune(line)
	if e.col < len(runes) {
		runes = append(runes[:e.col], runes[e.col+1:]...)
		e.lines[e.row] = string(runes)
		e.syncToViewport()
	} else if e.row < len(e.lines)-1 {
		curLine := e.lines[e.row]
		nextLine := e.lines[e.row+1]
		e.lines[e.row] = curLine + nextLine
		e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
		e.syncToViewport()
	}
}

func (e *editor) cursorUp() {
	if e.row > 0 {
		e.row--
		e.clampCol()
		e.ensureVisible()
	}
}

func (e *editor) cursorDown() {
	if e.row < len(e.lines)-1 {
		e.row++
		e.clampCol()
		e.ensureVisible()
	}
}

func (e *editor) cursorLeft() {
	if e.col > 0 {
		e.col--
	} else if e.row > 0 {
		e.row--
		e.col = len([]rune(e.lines[e.row]))
		e.ensureVisible()
	}
}

func (e *editor) cursorRight() {
	if e.col < len([]rune(e.lines[e.row])) {
		e.col++
	} else if e.row < len(e.lines)-1 {
		e.row++
		e.col = 0
		e.ensureVisible()
	}
}

func (e *editor) lineStart() { e.col = 0 }

func (e *editor) lineEnd() { e.col = len([]rune(e.lines[e.row])) }

func (e *editor) wordBackward() {
	if e.row == 0 && e.col == 0 {
		return
	}
	if e.col == 0 {
		e.row--
		e.col = len([]rune(e.lines[e.row]))
		e.ensureVisible()
		return
	}
	runes := []rune(e.lines[e.row])
	i := e.col - 1
	for i > 0 && isWhitespace(runes[i]) {
		i--
	}
	for i > 0 && !isWhitespace(runes[i-1]) {
		i--
	}
	e.col = i
}

func (e *editor) wordForward() {
	if e.row == len(e.lines)-1 && e.col == len([]rune(e.lines[e.row])) {
		return
	}
	runes := []rune(e.lines[e.row])
	i := e.col
	for i < len(runes) && isWhitespace(runes[i]) {
		i++
	}
	for i < len(runes) && !isWhitespace(runes[i]) {
		i++
	}
	if i >= len(runes) && e.row < len(e.lines)-1 {
		e.row++
		e.col = 0
		e.ensureVisible()
	} else {
		e.col = i
	}
}

func (e *editor) deleteWordBackward() {
	if e.col == 0 {
		e.deleteBackward()
		return
	}
	runes := []rune(e.lines[e.row])
	i := e.col - 1
	for i > 0 && isWhitespace(runes[i]) {
		i--
	}
	for i > 0 && !isWhitespace(runes[i-1]) {
		i--
	}
	e.lines[e.row] = string(runes[:i]) + string(runes[e.col:])
	e.col = i
	e.syncToViewport()
}

// --- Undo / Redo ---

func (e *editor) snapshot() {
	// Deep copy lines
	cp := make([]string, len(e.lines))
	copy(cp, e.lines)
	e.undoStack = append(e.undoStack, cp)
	e.undoRow = e.row
	e.undoCol = e.col
	// Limit stack size
	if len(e.undoStack) > 100 {
		e.undoStack = e.undoStack[1:]
	}
	// Clear redo stack on new edit
	e.redoStack = nil
}

func (e *editor) undo() {
	if len(e.undoStack) == 0 {
		return
	}
	// Push current state to redo
	cp := make([]string, len(e.lines))
	copy(cp, e.lines)
	e.redoStack = append(e.redoStack, cp)

	prev := e.undoStack[len(e.undoStack)-1]
	e.undoStack = e.undoStack[:len(e.undoStack)-1]
	e.lines = make([]string, len(prev))
	copy(e.lines, prev)
	if e.undoRow < len(e.lines) {
		e.row = e.undoRow
	} else {
		e.row = len(e.lines) - 1
	}
	e.col = e.undoCol
	e.clampCol()
	e.syncToViewport()
}

func (e *editor) redo() {
	if len(e.redoStack) == 0 {
		return
	}
	cp := make([]string, len(e.lines))
	copy(cp, e.lines)
	e.undoStack = append(e.undoStack, cp)

	next := e.redoStack[len(e.redoStack)-1]
	e.redoStack = e.redoStack[:len(e.redoStack)-1]
	e.lines = make([]string, len(next))
	copy(e.lines, next)
	if e.row >= len(e.lines) {
		e.row = len(e.lines) - 1
	}
	e.clampCol()
	e.syncToViewport()
}

// --- Auto-continue bullet lists ---

func (e *editor) autoContinueBullet() {
	if e.row <= 0 || e.row >= len(e.lines) {
		return
	}
	prevLine := e.lines[e.row-1]
	trimmed := strings.TrimLeft(prevLine, " \t")
	if !strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "* ") {
		return
	}
	// If previous line is just the bullet with no content, remove it
	content := strings.TrimSpace(trimmed[2:])
	if content == "" {
		// Remove the empty bullet line
		e.lines = append(e.lines[:e.row-1], e.lines[e.row:]...)
		e.row--
		e.col = 0
		e.syncToViewport()
		return
	}
	// Insert a new bullet line with the same indentation
	indent := prevLine[:len(prevLine)-len(trimmed)]
	bullet := string(trimmed[0]) + " "
	newLine := indent + bullet
	// Insert at current row position (which is a blank line just added)
	e.lines[e.row] = newLine
	e.col = len([]rune(newLine))
	e.syncToViewport()
}

// --- Transpose characters ---

func (e *editor) transposeChars() {
	if e.row < 0 || e.row >= len(e.lines) {
		return
	}
	runes := []rune(e.lines[e.row])
	if len(runes) < 2 {
		return
	}
	if e.col == 0 {
		// Transpose last two chars of line
		if len(runes) >= 2 {
			runes[len(runes)-2], runes[len(runes)-1] = runes[len(runes)-1], runes[len(runes)-2]
		}
	} else if e.col >= len(runes) {
		runes[len(runes)-2], runes[len(runes)-1] = runes[len(runes)-1], runes[len(runes)-2]
		e.col = len(runes)
	} else {
		runes[e.col-1], runes[e.col] = runes[e.col], runes[e.col-1]
		e.col++
	}
	e.lines[e.row] = string(runes)
	e.syncToViewport()
}

// --- Go to line ---

func (e *editor) gotoLine(n int) {
	if n < 0 {
		n = 0
	}
	if n >= len(e.lines) {
		n = len(e.lines) - 1
	}
	e.row = n
	e.col = 0
	e.ensureVisible()
}

// --- Duplicate line ---

func (e *editor) duplicateLine() {
	if e.row < 0 || e.row >= len(e.lines) {
		return
	}
	dup := e.lines[e.row]
	e.lines = append(e.lines[:e.row+1], append([]string{dup}, e.lines[e.row+1:]...)...)
	e.row++
	e.syncToViewport()
}

// --- Move line up/down ---

func (e *editor) moveLineUp() {
	if e.row <= 0 {
		return
	}
	e.lines[e.row-1], e.lines[e.row] = e.lines[e.row], e.lines[e.row-1]
	e.row--
	e.syncToViewport()
}

func (e *editor) moveLineDown() {
	if e.row >= len(e.lines)-1 {
		return
	}
	e.lines[e.row], e.lines[e.row+1] = e.lines[e.row+1], e.lines[e.row]
	e.row++
	e.syncToViewport()
}

// --- Search ---

func (e *editor) searchNext() {
	if e.searchQuery == "" {
		return
	}
	query := strings.ToLower(e.searchQuery)
	// Search from current position forward
	for i := e.row; i < len(e.lines); i++ {
		line := strings.ToLower(e.lines[i])
		startCol := 0
		if i == e.row {
			startCol = e.col + 1
		}
		idx := strings.Index(line[startCol:], query)
		if idx >= 0 {
			e.row = i
			e.col = startCol + idx
			e.ensureVisible()
			return
		}
	}
	// Wrap around
	for i := 0; i <= e.row; i++ {
		line := strings.ToLower(e.lines[i])
		endCol := len(line)
		if i == e.row {
			endCol = e.col
		}
		idx := strings.Index(line[:endCol], query)
		if idx >= 0 {
			e.row = i
			e.col = idx
			e.ensureVisible()
			return
		}
	}
}

// --- Select all (just moves cursor to start, for clearing) ---

func (e *editor) selectAllClear() {
	e.lines = []string{""}
	e.row = 0
	e.col = 0
	e.syncToViewport()
}

func (e *editor) clampCol() {
	if e.row < 0 || e.row >= len(e.lines) {
		return
	}
	lineLen := len([]rune(e.lines[e.row]))
	if e.col > lineLen {
		e.col = lineLen
	}
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// --- Key handling ---

func (e *editor) handleKey(msg tea.KeyPressMsg) bool {
	if len(e.suggestions) > 0 {
		switch msg.String() {
		case "tab":
			e.acceptSuggestion()
			return true
		case "down":
			e.selSug = (e.selSug + 1) % len(e.suggestions)
			return true
		case "up":
			e.selSug = (e.selSug - 1 + len(e.suggestions)) % len(e.suggestions)
			return true
		case "esc":
			e.suggestions = nil
			e.selSug = -1
			return true
		}
	}

	s := msg.String()
	switch s {
	case "up":
		e.cursorUp()
	case "down":
		e.cursorDown()
	case "left":
		e.cursorLeft()
	case "right":
		e.cursorRight()
	case "home", "ctrl+a":
		e.lineStart()
	case "end", "ctrl+e":
		e.lineEnd()
	case "ctrl+left", "alt+b":
		e.wordBackward()
	case "ctrl+right", "alt+f":
		e.wordForward()
	case "backspace", "ctrl+h":
		e.snapshot()
		e.deleteBackward()
	case "delete":
		e.snapshot()
		e.deleteForward()
	case "ctrl+d":
		e.duplicateLine()
	case "ctrl+k":
		if e.row >= 0 && e.row < len(e.lines) {
			runes := []rune(e.lines[e.row])
			if e.col < len(runes) {
				e.snapshot()
				e.lines[e.row] = string(runes[:e.col])
				e.syncToViewport()
			}
		}
	case "enter":
		e.snapshot()
		e.insertNewline()
		// Auto-continue bullet lists
		e.autoContinueBullet()
	case "ctrl+z":
		e.undo()
	case "ctrl+y", "ctrl+shift+z":
		e.redo()
	case "ctrl+t":
		e.transposeChars()
	case "alt+up":
		e.moveLineUp()
	case "alt+down":
		e.moveLineDown()
	case "ctrl+g":
		// Goto line — toggled externally, no-op here
	case "ctrl+l":
		// Select all / clear
		e.selectAllClear()
	case "ctrl+n":
		// Search next
		e.searchNext()
	case "pgup":
		e.vp.PageUp()
		e.row = e.vp.YOffset()
		e.clampCol()
	case "pgdown":
		e.vp.PageDown()
		e.row = e.vp.YOffset()
		e.clampCol()
	case "ctrl+u":
		e.vp.HalfPageUp()
		e.row = e.vp.YOffset()
		e.clampCol()
	case "ctrl+w":
		e.snapshot()
		e.deleteWordBackward()
	default:
		key := tea.Key(msg)
		if key.Text != "" {
			for _, r := range key.Text {
				if isPrintable(r) {
					e.insertRune(r)
				}
			}
			if e.commitMode {
				e.updateSuggestions()
			}
			return true
		}
		return false
	}

	if e.commitMode {
		e.updateSuggestions()
	}
	return true
}

func isPrintable(r rune) bool {
	return r >= 32 && r != 127
}

// --- Suggestions ---

func (e *editor) updateSuggestions() {
	if !e.commitMode {
		e.suggestions = nil
		e.selSug = -1
		return
	}
	e.suggestions = computeSuggestions(e.lines, e.row, e.col, e.people)
	if len(e.suggestions) > 0 {
		e.selSug = 0
	} else {
		e.selSug = -1
	}
}

func (e *editor) acceptSuggestion() {
	if len(e.suggestions) == 0 || e.selSug < 0 || e.selSug >= len(e.suggestions) {
		return
	}
	s := e.suggestions[e.selSug]
	line := e.lines[e.row]
	wordStart := findWordStart(line, e.col)
	runes := []rune(line)
	newLine := string(runes[:wordStart]) + s.text + string(runes[e.col:])
	e.lines[e.row] = newLine
	e.col = wordStart + len([]rune(s.text))
	e.syncToViewport()
	e.suggestions = nil
	e.selSug = -1

	// If this was a person suggestion, persist the person for future sessions
	if s.kind == "person" && e.people != nil {
		p := parsePerson(s.text)
		if p != nil {
			e.people.addPerson(*p)
			e.people.save()
		}
	}
}

// parsePerson parses a "Name <email>" string into a person struct.
func parsePerson(s string) *person {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, ">") {
		return nil
	}
	idx := strings.LastIndex(s, "<")
	if idx < 1 {
		return nil
	}
	name := strings.TrimSpace(s[:idx])
	email := strings.TrimSpace(s[idx+1 : len(s)-1])
	if name == "" || email == "" {
		return nil
	}
	return &person{name: name, email: email}
}

// --- Rendering ---

// view renders the editor with syntax highlighting on visible lines only.
// The cursor is inserted by highlighting the ENTIRE plain line first, then
// walking through the highlighted output (skipping ANSI escape codes) to
// find the cursor's visible column and inserting reverse-video there.
// This preserves all colors on both sides of the cursor.
func (e *editor) view() string {
	content := e.vp.View()
	lines := strings.Split(content, "\n")
	yOffset := e.vp.YOffset()

	for i := range lines {
		absLine := yOffset + i
		if absLine < 0 || absLine >= len(e.lines) {
			continue
		}
		raw := e.lines[absLine]

		if e.commitMode {
			highlighted := highlightCommitLine(raw, absLine)
			if e.focused && absLine == e.row {
				lines[i] = insertCursorInColored(highlighted, e.col)
			} else {
				lines[i] = highlighted
			}
		} else if e.focused && absLine == e.row {
			lines[i] = insertCursorInColored(raw, e.col)
		}
	}

	result := strings.Join(lines, "\n")

	// Overlay the suggestion popup as a floating box at the cursor position
	if len(e.suggestions) > 0 && e.focused {
		cursorScreenLine := e.row - yOffset
		if cursorScreenLine >= 0 && cursorScreenLine < len(lines) {
			popupBlock := e.renderSuggestionPopup()
			result = overlay.BlockFloat(result, popupBlock, cursorScreenLine, e.col, e.width, e.height)
		}
	}

	return result
}

// insertCursorInColored walks through a string that may contain ANSI escape
// codes, counting visible characters. At visible column `col`, it wraps the
// character in reverse-video. After the cursor, it re-emits the SGR state
// that was active before the cursor, so colors are preserved on both sides.
func insertCursorInColored(s string, col int) string {
	var b strings.Builder
	i := 0
	visible := 0
	// Track the raw SGR sequences that form the current style, so we can
	// re-emit them after the cursor's \x1b[27m.
	var currentSGR strings.Builder

	for i < len(s) {
		// Skip ANSI escape sequences
		if s[i] == '\x1b' {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				// CSI sequence: \x1b[ params intermediates final
				// Skip the '[' introducer, then scan for final byte (0x40-0x7E)
				j++
				for j < len(s) {
					if s[j] >= 0x40 && s[j] <= 0x7e {
						j++
						break
					}
					j++
				}
			} else if j < len(s) {
				// Non-CSI escape: \x1b + single char
				j++
			}
			seq := s[i:j]
			b.WriteString(seq)
			// Track SGR sequences (CSI ... m)
			if len(seq) >= 3 && seq[0] == '\x1b' && seq[1] == '[' && seq[len(seq)-1] == 'm' {
				body := seq[2 : len(seq)-1]
				if body == "" || body == "0" {
					currentSGR.Reset()
				} else {
					currentSGR.WriteString(seq)
				}
			}
			i = j
			continue
		}

		if visible == col {
			// Found cursor position: emit reverse-video for this char,
			// then re-emit the current SGR state so colors continue.
			r, size := utf8.DecodeRuneInString(s[i:])
			b.WriteString("\x1b[7m")
			b.WriteRune(r)
			b.WriteString("\x1b[27m")
			// Re-apply the style that was active before the cursor
			if currentSGR.Len() > 0 {
				b.WriteString(currentSGR.String())
			}
			i += size
			// Copy the rest
			b.WriteString(s[i:])
			return b.String()
		}

		_, size := utf8.DecodeRuneInString(s[i:])
		b.WriteString(s[i : i+size])
		i += size
		visible++
	}

	// Cursor is at end of line (past last visible char)
	b.WriteString("\x1b[7m \x1b[27m")
	return b.String()
}

// renderSuggestionPopup builds the popup lines as styled strings,
// styled like VSCode / nvim-cmp: a bordered box where the selected item
// has a highlighted background, unselected items are plain, and each
// item shows a right-aligned kind label.
func (e *editor) renderSuggestionPopup() []string {
	// Determine max display width for alignment
	maxDisplay := 0
	for _, s := range e.suggestions {
		w := lipgloss.Width(s.display)
		if w > maxDisplay {
			maxDisplay = w
		}
	}

	// Kind label + color (shown right-aligned in each row)
	kindLabel := map[string]string{
		"type":    "Type",
		"word":    "Text",
		"person":  "Person",
		"trailer": "Trailer",
		"scope":   "Scope",
		"gitmoji": "Emoji",
		"issue":   "Issue",
	}
	kindColor := map[string]color.Color{
		"type":    lipgloss.Color("42"),
		"word":    lipgloss.Color("39"),
		"person":  lipgloss.Color("99"),
		"trailer": lipgloss.Color("141"),
		"scope":   lipgloss.Color("213"),
		"gitmoji": lipgloss.Color("214"),
		"issue":   lipgloss.Color("203"),
	}

	selectedBg := lipgloss.Color("62")
	itemWidth := maxDisplay + 8 // display + gap + kind label (max ~6 chars)

	items := make([]string, len(e.suggestions))
	for i, s := range e.suggestions {
		label := kindLabel[s.kind]
		if label == "" {
			label = "Text"
		}
		kc := kindColor[s.kind]
		if kc == nil {
			kc = lipgloss.Color("39")
		}

		// Pad display to maxDisplay for alignment
		paddedDisplay := s.display + strings.Repeat(" ", maxDisplay-lipgloss.Width(s.display))

		if i == e.selSug {
			// Selected: white text on blue background, kind label in light blue
			textStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Background(selectedBg).
				Bold(true)
			kindStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("117")).
				Background(selectedBg)
			gap := strings.Repeat(" ", itemWidth-maxDisplay-lipgloss.Width(label))
			items[i] = textStyle.Render(paddedDisplay) + textStyle.Render(gap) + kindStyle.Render(label)
		} else {
			// Unselected: gray text, kind label in its color
			textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
			kindStyle := lipgloss.NewStyle().Foreground(kc)
			gap := strings.Repeat(" ", itemWidth-maxDisplay-lipgloss.Width(label))
			items[i] = textStyle.Render(paddedDisplay) + textStyle.Render(gap) + kindStyle.Render(label)
		}
	}

	popup := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Background(lipgloss.Color("234")).
		Padding(0, 1).
		Render(strings.Join(items, "\n"))

	return strings.Split(popup, "\n")
}
