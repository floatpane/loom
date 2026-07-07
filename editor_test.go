package main

import (
	"strings"
	"testing"
)

func TestEditorInsertRune(t *testing.T) {
	e := newEditor()
	e.insertRune('h')
	e.insertRune('i')
	if e.value() != "hi" {
		t.Errorf("expected 'hi', got %q", e.value())
	}
	if e.col != 2 {
		t.Errorf("expected col=2, got %d", e.col)
	}
}

func TestEditorInsertNewline(t *testing.T) {
	e := newEditor()
	e.insertRune('a')
	e.insertRune('b')
	e.insertNewline()
	e.insertRune('c')
	if e.value() != "ab\nc" {
		t.Errorf("expected 'ab\\nc', got %q", e.value())
	}
	if e.row != 1 {
		t.Errorf("expected row=1, got %d", e.row)
	}
}

func TestEditorDeleteBackward(t *testing.T) {
	e := newEditor()
	e.setContent("hello")
	e.col = 5
	e.deleteBackward()
	if e.value() != "hell" {
		t.Errorf("expected 'hell', got %q", e.value())
	}
}

func TestEditorDeleteBackwardMerge(t *testing.T) {
	e := newEditor()
	e.setContent("hello\nworld")
	e.row = 1
	e.col = 0
	e.deleteBackward()
	if e.value() != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", e.value())
	}
}

func TestEditorCursorMovement(t *testing.T) {
	e := newEditor()
	e.setContent("hello\nworld")
	e.cursorRight()
	e.cursorRight()
	if e.col != 2 {
		t.Errorf("expected col=2, got %d", e.col)
	}
	e.cursorDown()
	if e.row != 1 {
		t.Errorf("expected row=1, got %d", e.row)
	}
	e.cursorUp()
	if e.row != 0 {
		t.Errorf("expected row=0, got %d", e.row)
	}
	e.cursorLeft()
	e.cursorLeft()
	e.cursorLeft()
	if e.col != 0 {
		t.Errorf("expected col=0, got %d", e.col)
	}
}

func TestEditorWordBackward(t *testing.T) {
	e := newEditor()
	e.setContent("hello world foo")
	e.col = 15
	e.wordBackward()
	if e.col != 12 {
		t.Errorf("expected col=12, got %d", e.col)
	}
}

func TestEditorWordForward(t *testing.T) {
	e := newEditor()
	e.setContent("hello world foo")
	e.col = 0
	e.wordForward()
	if e.col != 5 {
		t.Errorf("expected col=5, got %d", e.col)
	}
}

func TestEditorSetContent(t *testing.T) {
	e := newEditor()
	e.setContent("line1\nline2\nline3")
	if len(e.lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(e.lines))
	}
}

func TestEditorSetContentEmpty(t *testing.T) {
	e := newEditor()
	e.setContent("")
	if len(e.lines) != 1 {
		t.Errorf("expected 1 line for empty, got %d", len(e.lines))
	}
}

func TestEditorLargeContent(t *testing.T) {
	e := newEditor()
	var lines []string
	for i := 0; i < 5000; i++ {
		lines = append(lines, "this is a line of text")
	}
	e.setContent(strings.Join(lines, "\n"))
	if len(e.lines) != 5000 {
		t.Errorf("expected 5000 lines, got %d", len(e.lines))
	}
}

func TestEditorLineStartEnd(t *testing.T) {
	e := newEditor()
	e.setContent("hello")
	e.col = 3
	e.lineEnd()
	if e.col != 5 {
		t.Errorf("expected col=5, got %d", e.col)
	}
	e.lineStart()
	if e.col != 0 {
		t.Errorf("expected col=0, got %d", e.col)
	}
}

func TestEditorAcceptSuggestion(t *testing.T) {
	e := newEditor()
	e.commitMode = true
	e.lines = []string{"fe"}
	e.col = 2
	e.updateSuggestions()
	if len(e.suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
	for i, s := range e.suggestions {
		if s.text == "feat: " {
			e.selSug = i
			break
		}
	}
	e.acceptSuggestion()
	if e.lines[0] != "feat: " {
		t.Errorf("expected 'feat: ', got %q", e.lines[0])
	}
	if e.col != 6 {
		t.Errorf("expected col=6, got %d", e.col)
	}
}
