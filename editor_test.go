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
	if e.col != 4 {
		t.Errorf("expected col=4, got %d", e.col)
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
	if e.row != 0 {
		t.Errorf("expected row=0, got %d", e.row)
	}
	if e.col != 5 {
		t.Errorf("expected col=5, got %d", e.col)
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
	if e.col != 2 {
		t.Errorf("expected col=2 (clamped), got %d", e.col)
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
	if e.row != 0 {
		t.Errorf("expected row=0, got %d", e.row)
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
	e.wordBackward()
	if e.col != 6 {
		t.Errorf("expected col=6, got %d", e.col)
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
	e.wordForward()
	if e.col != 11 {
		t.Errorf("expected col=11, got %d", e.col)
	}
}

func TestEditorDeleteWordBackward(t *testing.T) {
	e := newEditor()
	e.setContent("hello world")
	e.col = 11
	e.deleteWordBackward()
	if e.value() != "hello " {
		t.Errorf("expected 'hello ', got %q", e.value())
	}
	if e.col != 6 {
		t.Errorf("expected col=6, got %d", e.col)
	}
}

func TestEditorSetContent(t *testing.T) {
	e := newEditor()
	e.setContent("line1\nline2\nline3")
	if len(e.lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(e.lines))
	}
	if e.value() != "line1\nline2\nline3" {
		t.Errorf("expected 'line1\\nline2\\nline3', got %q", e.value())
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
		lines = append(lines, "this is a line of text that is reasonably long")
	}
	e.setContent(strings.Join(lines, "\n"))
	if len(e.lines) != 5000 {
		t.Errorf("expected 5000 lines, got %d", len(e.lines))
	}
	e.row = 2500
	e.col = 10
	e.clampCol()
	if e.col != 10 {
		t.Errorf("expected col=10, got %d", e.col)
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
