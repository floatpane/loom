package main

import (
	"testing"
)

func TestEditorUndoRedo(t *testing.T) {
	e := newEditor()
	e.setContent("hello")
	e.snapshot()
	e.col = 5
	e.insertRune('!')
	if e.value() != "hello!" {
		t.Errorf("expected 'hello!', got %q", e.value())
	}
	e.undo()
	if e.value() != "hello" {
		t.Errorf("expected 'hello' after undo, got %q", e.value())
	}
	e.redo()
	if e.value() != "hello!" {
		t.Errorf("expected 'hello!' after redo, got %q", e.value())
	}
}

func TestEditorAutoContinueBullet(t *testing.T) {
	e := newEditor()
	e.setContent("- item one\n")
	e.row = 1
	e.col = 0
	e.autoContinueBullet()
	if e.lines[1] != "- " {
		t.Errorf("expected '- ' on line 1, got %q", e.lines[1])
	}
	if e.col != 2 {
		t.Errorf("expected col=2, got %d", e.col)
	}
}

func TestEditorAutoContinueBulletEmptyRemoves(t *testing.T) {
	e := newEditor()
	e.setContent("- item one\n- \n")
	e.row = 2
	e.col = 0
	e.autoContinueBullet()
	// The empty bullet line should be removed
	if len(e.lines) != 2 {
		t.Errorf("expected 2 lines (empty bullet removed), got %d", len(e.lines))
	}
}

func TestEditorTransposeCharsMiddle(t *testing.T) {
	e := newEditor()
	e.setContent("ab")
	e.col = 1
	e.transposeChars()
	if e.lines[0] != "ba" {
		t.Errorf("expected 'ba', got %q", e.lines[0])
	}
	if e.col != 2 {
		t.Errorf("expected col=2, got %d", e.col)
	}
}

func TestEditorTransposeCharsEnd(t *testing.T) {
	e := newEditor()
	e.setContent("ab")
	e.col = 2
	e.transposeChars()
	if e.lines[0] != "ba" {
		t.Errorf("expected 'ba', got %q", e.lines[0])
	}
}

func TestEditorDuplicateLine(t *testing.T) {
	e := newEditor()
	e.setContent("hello\nworld")
	e.row = 0
	e.duplicateLine()
	if len(e.lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(e.lines))
	}
	if e.lines[0] != "hello" || e.lines[1] != "hello" {
		t.Errorf("expected 'hello\\nhello\\nworld', got %v", e.lines)
	}
	if e.row != 1 {
		t.Errorf("expected row=1, got %d", e.row)
	}
}

func TestEditorMoveLineUp(t *testing.T) {
	e := newEditor()
	e.setContent("a\nb\nc")
	e.row = 1
	e.moveLineUp()
	if e.lines[0] != "b" || e.lines[1] != "a" {
		t.Errorf("expected b,a,c got %v", e.lines)
	}
	if e.row != 0 {
		t.Errorf("expected row=0, got %d", e.row)
	}
}

func TestEditorMoveLineDown(t *testing.T) {
	e := newEditor()
	e.setContent("a\nb\nc")
	e.row = 1
	e.moveLineDown()
	if e.lines[1] != "c" || e.lines[2] != "b" {
		t.Errorf("expected a,c,b got %v", e.lines)
	}
	if e.row != 2 {
		t.Errorf("expected row=2, got %d", e.row)
	}
}

func TestEditorMoveLineUpAtTop(t *testing.T) {
	e := newEditor()
	e.setContent("a\nb")
	e.row = 0
	e.moveLineUp()
	if e.lines[0] != "a" || e.lines[1] != "b" {
		t.Errorf("should not move at top, got %v", e.lines)
	}
}

func TestEditorSearchNext(t *testing.T) {
	e := newEditor()
	e.setContent("foo bar foo baz")
	e.searchQuery = "foo"
	e.col = 0
	e.searchNext()
	// First "foo" is at col 0, should find next at col 8
	if e.col != 8 {
		t.Errorf("expected col=8, got %d", e.col)
	}
}

func TestEditorSearchWrap(t *testing.T) {
	e := newEditor()
	e.setContent("foo bar foo")
	e.searchQuery = "foo"
	e.col = 8
	e.searchNext()
	// Should wrap to col 0
	if e.col != 0 {
		t.Errorf("expected col=0 (wrap), got %d", e.col)
	}
}

func TestEditorSearchMultiline(t *testing.T) {
	e := newEditor()
	e.setContent("hello\nworld\nhello")
	e.searchQuery = "hello"
	e.row = 0
	e.col = 0
	e.searchNext()
	if e.row != 2 {
		t.Errorf("expected row=2, got %d", e.row)
	}
}

func TestEditorGotoLine(t *testing.T) {
	e := newEditor()
	e.setContent("a\nb\nc\nd")
	e.gotoLine(2)
	if e.row != 2 {
		t.Errorf("expected row=2, got %d", e.row)
	}
}

func TestEditorGotoLineClamp(t *testing.T) {
	e := newEditor()
	e.setContent("a\nb")
	e.gotoLine(100)
	if e.row != 1 {
		t.Errorf("expected row=1 (clamped), got %d", e.row)
	}
}

func TestEditorSelectAllClear(t *testing.T) {
	e := newEditor()
	e.setContent("hello\nworld")
	e.selectAllClear()
	if len(e.lines) != 1 || e.lines[0] != "" {
		t.Errorf("expected single empty line, got %v", e.lines)
	}
}

func TestEditorUndoStackLimit(t *testing.T) {
	e := newEditor()
	e.setContent("start")
	for i := 0; i < 150; i++ {
		e.snapshot()
	}
	// Stack should be limited to 100
	if len(e.undoStack) > 100 {
		t.Errorf("expected undo stack <= 100, got %d", len(e.undoStack))
	}
}

func TestEditorSnapshotClearsRedo(t *testing.T) {
	e := newEditor()
	e.setContent("hello")
	e.snapshot()
	e.insertRune('x')
	e.snapshot()
	e.undo()
	if len(e.redoStack) != 1 {
		t.Fatalf("expected 1 redo, got %d", len(e.redoStack))
	}
	e.snapshot()
	if len(e.redoStack) != 0 {
		t.Errorf("expected redo cleared, got %d", len(e.redoStack))
	}
}

func TestEditorShowLineNums(t *testing.T) {
	e := newEditor()
	if e.showLineNums {
		t.Error("expected showLineNums to default to false")
	}
	e.showLineNums = true
	if !e.showLineNums {
		t.Error("expected showLineNums to be settable")
	}
}
