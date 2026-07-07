package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
)

// editor is a lightweight, virtualized text editor built on the bubbles
// viewport. Unlike the bubbles textarea (which renders ALL lines with
// lipgloss styling on every keystroke), the editor stores content as a
// slice of lines and only renders the visible window through the viewport.
// This makes it performant even with thousands of lines.
type editor struct {
	vp       viewport.Model
	lines    []string
	row      int
	col      int
	width    int
	height   int
	focused  bool
}

func newEditor() *editor {
	vp := viewport.New()
	vp.SoftWrap = false
	return &editor{
		vp:      vp,
		lines:   []string{""},
		focused: true,
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

// insertRune inserts a character at the cursor position.
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

// insertNewline splits the current line at the cursor.
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

// deleteBackward deletes the character before the cursor (backspace).
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

// deleteForward deletes the character at the cursor (delete key).
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

// cursorUp moves the cursor up one line.
func (e *editor) cursorUp() {
	if e.row > 0 {
		e.row--
		e.clampCol()
		e.ensureVisible()
	}
}

// cursorDown moves the cursor down one line.
func (e *editor) cursorDown() {
	if e.row < len(e.lines)-1 {
		e.row++
		e.clampCol()
		e.ensureVisible()
	}
}

// cursorLeft moves the cursor left one character.
func (e *editor) cursorLeft() {
	if e.col > 0 {
		e.col--
	} else if e.row > 0 {
		e.row--
		e.col = len([]rune(e.lines[e.row]))
		e.ensureVisible()
	}
}

// cursorRight moves the cursor right one character.
func (e *editor) cursorRight() {
	if e.col < len([]rune(e.lines[e.row])) {
		e.col++
	} else if e.row < len(e.lines)-1 {
		e.row++
		e.col = 0
		e.ensureVisible()
	}
}

// lineStart moves to the beginning of the line.
func (e *editor) lineStart() {
	e.col = 0
}

// lineEnd moves to the end of the line.
func (e *editor) lineEnd() {
	e.col = len([]rune(e.lines[e.row]))
}

// wordBackward moves the cursor to the start of the previous word.
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

// wordForward moves the cursor to the start of the next word.
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

// deleteWordBackward deletes the word before the cursor.
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

// handleKey processes a key press message and returns true if it was handled.
func (e *editor) handleKey(msg tea.KeyPressMsg) bool {
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
		e.deleteBackward()
	case "delete":
		e.deleteForward()
	case "ctrl+d":
		e.vp.HalfPageDown()
		e.row = e.vp.YOffset()
		e.clampCol()
	case "ctrl+k":
		// delete to end of line
		if e.row >= 0 && e.row < len(e.lines) {
			runes := []rune(e.lines[e.row])
			if e.col < len(runes) {
				e.lines[e.row] = string(runes[:e.col])
				e.syncToViewport()
			}
		}
	case "enter":
		e.insertNewline()
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
		e.deleteWordBackward()
	default:
		key := tea.Key(msg)
		if key.Text != "" {
			for _, r := range key.Text {
				if isPrintable(r) {
					e.insertRune(r)
				}
			}
			return true
		}
		return false
	}
	return true
}

func isPrintable(r rune) bool {
	return r >= 32 && r != 127
}

// view renders the editor, showing only visible lines through the viewport.
func (e *editor) view() string {
	content := e.vp.View()
	if !e.focused {
		return content
	}

	cursorLine := e.row - e.vp.YOffset()
	if cursorLine < 0 || cursorLine >= e.height {
		return content
	}

	lines := strings.Split(content, "\n")
	if cursorLine < len(lines) {
		line := lines[cursorLine]
		runes := []rune(line)
		cursorCol := e.col
		if cursorCol > len(runes) {
			cursorCol = len(runes)
		}
		before := ""
		after := ""
		if cursorCol > 0 {
			before = string(runes[:cursorCol])
		}
		if cursorCol < len(runes) {
			after = string(runes[cursorCol:])
		}
		lines[cursorLine] = before + "\x1b[7m \x1b[0m" + after
	}

	return strings.Join(lines, "\n")
}
