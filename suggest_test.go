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
	ps := &peopleStore{
		people: []person{
			{name: "Alice", email: "alice@example.com"},
			{name: "Bob", email: "bob@example.com"},
		},
	}
	lines := []string{"Co-authored-by: A"}
	sugs := computeSuggestions(lines, 0, len(lines[0]), ps)
	if len(sugs) == 0 {
		t.Fatal("expected co-author suggestions")
	}
	if sugs[0].text != "Alice <alice@example.com>" {
		t.Errorf("expected Alice, got %s", sugs[0].text)
	}
	if sugs[0].kind != "person" {
		t.Errorf("expected kind 'person', got %s", sugs[0].kind)
	}
}

func TestComputeSuggestionsReviewedBy(t *testing.T) {
	ps := &peopleStore{
		people: []person{
			{name: "Alice", email: "alice@example.com"},
		},
	}
	lines := []string{"", "Reviewed-by: A"}
	sugs := computeSuggestions(lines, 1, len(lines[1]), ps)
	if len(sugs) == 0 {
		t.Fatal("expected reviewed-by suggestions")
	}
	if sugs[0].text != "Alice <alice@example.com>" {
		t.Errorf("expected Alice, got %s", sugs[0].text)
	}
}

func TestComputeSuggestionsTrailerNames(t *testing.T) {
	lines := []string{"", "Co-a"}
	sugs := computeSuggestions(lines, 1, 4, nil)
	if len(sugs) == 0 {
		t.Fatal("expected trailer name suggestions")
	}
	found := false
	for _, s := range sugs {
		if s.text == "Co-authored-by: " {
			found = true
		}
		if s.kind != "trailer" {
			t.Errorf("expected kind 'trailer', got %s", s.kind)
		}
	}
	if !found {
		t.Error("expected 'Co-authored-by: ' in suggestions")
	}
}

func TestComputeSuggestionsTrailerNamesReviewed(t *testing.T) {
	lines := []string{"", "Re"}
	sugs := computeSuggestions(lines, 1, 2, nil)
	if len(sugs) == 0 {
		t.Fatal("expected trailer name suggestions for 'Re'")
	}
	foundReviewed := false
	foundReported := false
	for _, s := range sugs {
		if s.text == "Reviewed-by: " {
			foundReviewed = true
		}
		if s.text == "Reported-by: " {
			foundReported = true
		}
	}
	if !foundReviewed {
		t.Error("expected 'Reviewed-by: ' in suggestions")
	}
	if !foundReported {
		t.Error("expected 'Reported-by: ' in suggestions")
	}
}

func TestComputeSuggestionsCoAuthorNoMatch(t *testing.T) {
	ps := &peopleStore{
		people: []person{
			{name: "Alice", email: "alice@example.com"},
		},
	}
	lines := []string{"Co-authored-by: Z"}
	sugs := computeSuggestions(lines, 0, len(lines[0]), ps)
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

func TestHighlightCommitReviewedByTrailer(t *testing.T) {
	result := highlightCommitLine("Reviewed-by: Bob <bob@example.com>", 1)
	if result == "Reviewed-by: Bob <bob@example.com>" {
		t.Error("expected colored output for Reviewed-by trailer")
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

func TestPeopleStoreAddPerson(t *testing.T) {
	ps := &peopleStore{
		people: []person{
			{name: "Alice", email: "alice@example.com"},
		},
	}
	ps.addPerson(person{name: "Bob", email: "bob@example.com"})
	if len(ps.people) != 2 {
		t.Fatalf("expected 2 people, got %d", len(ps.people))
	}
	// Adding a duplicate should not increase the count
	ps.addPerson(person{name: "Alice", email: "alice@example.com"})
	if len(ps.people) != 2 {
		t.Fatalf("expected 2 people after duplicate add, got %d", len(ps.people))
	}
}

func TestPeopleStoreMatchingPeople(t *testing.T) {
	ps := &peopleStore{
		people: []person{
			{name: "Alice", email: "alice@example.com"},
			{name: "Bob", email: "bob@example.com"},
			{name: "Charlie", email: "charlie@example.com"},
		},
	}
	matches := ps.matchingPeople("A")
	if len(matches) != 1 || matches[0].name != "Alice" {
		t.Fatalf("expected only Alice, got %v", matches)
	}
	matches = ps.matchingPeople("ali")
	if len(matches) != 1 || matches[0].name != "Alice" {
		t.Fatalf("expected Alice (case-insensitive), got %v", matches)
	}
}

func TestExtractPeopleFromTrailerLines(t *testing.T) {
	lines := []string{
		"feat: add feature",
		"",
		"Co-authored-by: Alice <alice@example.com>",
		"Reviewed-by: Bob <bob@example.com>",
		"Just a regular line",
		"Signed-off-by: Charlie <charlie@example.com>",
	}
	people := extractPeopleFromTrailerLines(lines)
	if len(people) != 3 {
		t.Fatalf("expected 3 people, got %d: %v", len(people), people)
	}
	names := map[string]bool{}
	for _, p := range people {
		names[p.name] = true
	}
	if !names["Alice"] || !names["Bob"] || !names["Charlie"] {
		t.Errorf("missing expected people, got %v", names)
	}
}
