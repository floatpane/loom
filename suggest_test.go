package main

import (
	"testing"
)

func TestComputeSuggestionsCommitTypes(t *testing.T) {
	lines := []string{"fe"}
	sugs := computeSuggestions(lines, 0, 2, nil)
	if len(sugs) == 0 {
		t.Fatal("expected suggestions for 'fe'")
	}
	found := false
	for _, s := range sugs {
		if s.text == "feat: " {
			found = true
		}
	}
	if !found {
		t.Error("expected 'feat: ' in suggestions")
	}
}

func TestComputeSuggestionsExactMatch(t *testing.T) {
	lines := []string{"fix"}
	sugs := computeSuggestions(lines, 0, 3, nil)
	// "fix" is an exact match, should not suggest itself
	for _, s := range sugs {
		if s.text == "fix: " {
			t.Error("should not suggest exact match 'fix: '")
		}
	}
}

func TestComputeSuggestionsCoAuthor(t *testing.T) {
	authors := []coAuthor{
		{name: "Alice", email: "alice@example.com"},
		{name: "Bob", email: "bob@example.com"},
	}
	lines := []string{"Co-authored-by: A"}
	sugs := computeSuggestions(lines, 0, len(lines[0]), authors)
	if len(sugs) == 0 {
		t.Fatal("expected co-author suggestions")
	}
	if sugs[0].text != "Alice <alice@example.com>" {
		t.Errorf("expected Alice, got %s", sugs[0].text)
	}
}

func TestComputeSuggestionsCoAuthorNoMatch(t *testing.T) {
	authors := []coAuthor{
		{name: "Alice", email: "alice@example.com"},
	}
	lines := []string{"Co-authored-by: Z"}
	sugs := computeSuggestions(lines, 0, len(lines[0]), authors)
	if len(sugs) != 0 {
		t.Errorf("expected no suggestions, got %d", len(sugs))
	}
}

func TestComputeSuggestionsCommonWords(t *testing.T) {
	lines := []string{"", "upd"}
	sugs := computeSuggestions(lines, 1, 3, nil)
	found := false
	for _, s := range sugs {
		if s.text == "update" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'update' in suggestions for 'upd'")
	}
}

func TestComputeSuggestionsEmptyWord(t *testing.T) {
	lines := []string{"", ""}
	sugs := computeSuggestions(lines, 1, 0, nil)
	if sugs != nil {
		t.Error("expected nil for empty word")
	}
}

func TestHighlightCommitSubjectConventional(t *testing.T) {
	result := highlightCommitLine("feat: add new feature", 0)
	// Should contain ANSI escapes (colored)
	if result == "feat: add new feature" {
		t.Error("expected colored output for conventional commit subject")
	}
}

func TestHighlightCommitSubjectWithScope(t *testing.T) {
	result := highlightCommitLine("fix(api): handle timeout", 0)
	if result == "fix(api): handle timeout" {
		t.Error("expected colored output for scoped commit subject")
	}
}

func TestHighlightCommitTrailer(t *testing.T) {
	result := highlightCommitLine("Co-authored-by: Alice <alice@example.com>", 1)
	if result == "Co-authored-by: Alice <alice@example.com>" {
		t.Error("expected colored output for trailer")
	}
}

func TestHighlightCommitBullet(t *testing.T) {
	result := highlightCommitLine("- item one", 2)
	if result == "- item one" {
		t.Error("expected colored output for bullet point")
	}
}

func TestHighlightCommitPlainLine(t *testing.T) {
	result := highlightCommitLine("just a regular line", 3)
	if result != "just a regular line" {
		t.Error("expected plain output for regular line")
	}
}

func TestLoadCoAuthors(t *testing.T) {
	// This may return nil if not in a git repo with co-authors,
	// but it shouldn't crash
	authors := loadCoAuthors()
	_ = authors
}
