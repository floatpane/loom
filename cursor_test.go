package main

import (
	"strings"
	"testing"
)

func TestInsertCursorPreservesColor(t *testing.T) {
	// Simulate what lipgloss outputs: \x1b[1;38;5;42mfeat\x1b[m\x1b[1;38;5;241m:\x1b[m\x1b[38;5;254m add stuff\x1b[m
	highlighted := "\x1b[1;38;5;42mfeat\x1b[m\x1b[1;38;5;241m:\x1b[m\x1b[38;5;254m add stuff\x1b[m"

	// Insert cursor at col 4 (the ':')
	result := insertCursorInColored(highlighted, 4)

	// After the cursor's \x1b[27m, the SGR state should be re-emitted.
	// The ':' is in a separate styled segment after a full reset, so
	// currentSGR should be empty at that point (reset was consumed).
	// The ' add stuff' part has its own \x1b[38;5;254m prefix, so it
	// should still be present in the output.
	if !strings.Contains(result, "\x1b[38;5;254m") {
		t.Errorf("color for 'add stuff' was lost after cursor insertion\nresult: %q", result)
	}

	// The 'feat' color should also be preserved
	if !strings.Contains(result, "\x1b[1;38;5;42m") {
		t.Errorf("color for 'feat' was lost\nresult: %q", result)
	}
}

func TestInsertCursorAtEnd(t *testing.T) {
	highlighted := "\x1b[1;38;5;42mfeat\x1b[m"
	result := insertCursorInColored(highlighted, 4)
	// Should have reverse-video space at end
	if !strings.Contains(result, "\x1b[7m") {
		t.Errorf("expected reverse-video at end\nresult: %q", result)
	}
}

func TestInsertCursorReEmitsSGR(t *testing.T) {
	// When cursor is in the middle of a styled segment (no reset between),
	// the SGR should be re-emitted after \x1b[27m.
	// Simulate: \x1b[38;5;254mhello world\x1b[m
	// Cursor at col 5 (after "hello", on " ")
	highlighted := "\x1b[38;5;254mhello world\x1b[m"
	result := insertCursorInColored(highlighted, 5)

	// After \x1b[27m, the \x1b[38;5;254m should be re-emitted
	idx := strings.Index(result, "\x1b[27m")
	if idx < 0 {
		t.Fatal("expected \\x1b[27m in result")
	}
	afterCursor := result[idx:]
	if !strings.Contains(afterCursor, "\x1b[38;5;254m") {
		t.Errorf("expected SGR re-emitted after cursor\nafter cursor: %q", afterCursor)
	}
}
