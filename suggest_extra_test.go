package main

import (
	"testing"
)

func TestFuzzyMatchExact(t *testing.T) {
	if !fuzzyMatch("abc", "abc") {
		t.Error("expected exact match to fuzzy match")
	}
}

func TestFuzzyMatchSubsequence(t *testing.T) {
	if !fuzzyMatch("bdt", "abundant") {
		t.Error("expected bdt to fuzzy match abundant")
	}
}

func TestFuzzyMatchNoMatch(t *testing.T) {
	if fuzzyMatch("xyz", "abc") {
		t.Error("expected xyz to not fuzzy match abc")
	}
}

func TestFuzzyMatchTooShort(t *testing.T) {
	if fuzzyMatch("ab", "abc") {
		t.Error("expected too-short query to not fuzzy match")
	}
}

func TestGitmojiSuggestions(t *testing.T) {
	sugs := gitmojiSuggestions("sparkles")
	if len(sugs) == 0 {
		t.Fatal("expected gitmoji suggestions for 'sparkles'")
	}
	if sugs[0].text != "✨ " {
		t.Errorf("expected '✨ ', got %q", sugs[0].text)
	}
	if sugs[0].kind != "gitmoji" {
		t.Errorf("expected kind 'gitmoji', got %s", sugs[0].kind)
	}
}

func TestGitmojiSuggestionsBug(t *testing.T) {
	sugs := gitmojiSuggestions("bug")
	if len(sugs) == 0 {
		t.Fatal("expected gitmoji suggestions for 'bug'")
	}
	if sugs[0].text != "🐛 " {
		t.Errorf("expected '🐛 ', got %q", sugs[0].text)
	}
}

func TestGitmojiSuggestionsEmpty(t *testing.T) {
	sugs := gitmojiSuggestions("")
	if len(sugs) == 0 {
		t.Fatal("expected all gitmoji for empty query")
	}
}

func TestGitmojiSuggestionsNoMatch(t *testing.T) {
	sugs := gitmojiSuggestions("zzzznotreal")
	if sugs != nil {
		t.Errorf("expected nil for no match, got %d", len(sugs))
	}
}

func TestScopeSuggestions(t *testing.T) {
	ctx := &suggestionCtx{
		scopes:      []string{"api", "auth", "ui"},
		branchScope: "api",
	}
	sugs := scopeSuggestions("a", ctx)
	if len(sugs) == 0 {
		t.Fatal("expected scope suggestions for 'a'")
	}
	// Branch scope should rank first
	if sugs[0].text != "api): " {
		t.Errorf("expected 'api): ' first, got %q", sugs[0].text)
	}
}

func TestScopeSuggestionsBranchFirst(t *testing.T) {
	ctx := &suggestionCtx{
		scopes:      []string{"auth", "api"},
		branchScope: "api",
	}
	sugs := scopeSuggestions("a", ctx)
	if sugs[0].text != "api): " {
		t.Errorf("expected branch scope 'api' first, got %q", sugs[0].text)
	}
}

func TestIssueRefSuggestionsBranch(t *testing.T) {
	ctx := &suggestionCtx{branchIssue: "42"}
	sugs := issueRefSuggestions("", ctx)
	found := false
	for _, s := range sugs {
		if s.text == "#42" {
			found = true
		}
	}
	if !found {
		t.Error("expected #42 from branch in suggestions")
	}
}

func TestIssueRefSuggestionsHashPrefix(t *testing.T) {
	ctx := &suggestionCtx{branchIssue: "42"}
	sugs := issueRefSuggestions("", ctx)
	found := false
	for _, s := range sugs {
		if s.text == "#" {
			found = true
		}
	}
	if !found {
		t.Error("expected '#' prefix suggestion")
	}
}

func TestComputeSuggestionsGitmoji(t *testing.T) {
	lines := []string{":spa"}
	sugs := computeSuggestions(lines, 0, 4, nil)
	if len(sugs) == 0 {
		t.Fatal("expected gitmoji suggestions for ':spa'")
	}
	if sugs[0].kind != "gitmoji" {
		t.Errorf("expected kind 'gitmoji', got %s", sugs[0].kind)
	}
}

func TestComputeSuggestionsBreakingChange(t *testing.T) {
	lines := []string{"feat!"}
	sugs := computeSuggestions(lines, 0, 5, nil)
	if len(sugs) == 0 {
		t.Fatal("expected suggestions for 'feat!'")
	}
	found := false
	for _, s := range sugs {
		if s.text == "!: " {
			found = true
		}
	}
	if !found {
		t.Error("expected breaking change suggestion")
	}
}

func TestComputeSuggestionsScopeFromHistory(t *testing.T) {
	ctx := &suggestionCtx{scopes: []string{"api", "auth"}}
	oldCtx := globalSugCtx
	globalSugCtx = ctx
	defer func() { globalSugCtx = oldCtx }()

	lines := []string{"feat(a"}
	sugs := computeSuggestions(lines, 0, 6, nil)
	if len(sugs) == 0 {
		t.Fatal("expected scope suggestions for 'feat(a'")
	}
	if sugs[0].kind != "scope" {
		t.Errorf("expected kind 'scope', got %s", sugs[0].kind)
	}
}

func TestComputeSuggestionsBranchTypePriority(t *testing.T) {
	ctx := &suggestionCtx{branchType: "fix", typeFreqs: map[string]int{"feat": 10}}
	oldCtx := globalSugCtx
	globalSugCtx = ctx
	defer func() { globalSugCtx = oldCtx }()

	lines := []string{"f"}
	sugs := computeSuggestions(lines, 0, 1, nil)
	if len(sugs) == 0 {
		t.Fatal("expected suggestions for 'f'")
	}
	// "fix" should rank higher due to branch type
	if sugs[0].text != "fix: " {
		t.Errorf("expected 'fix: ' first (branch priority), got %q", sugs[0].text)
	}
}

func TestComputeSuggestionsMoreTrailers(t *testing.T) {
	// Test that new trailers are suggested
	lines := []string{"", "co-dev"}
	sugs := computeSuggestions(lines, 1, 6, nil)
	if len(sugs) == 0 {
		t.Fatal("expected trailer suggestions for 'co-dev'")
	}
	found := false
	for _, s := range sugs {
		if s.text == "Co-developed-by: " {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Co-developed-by: ' in suggestions")
	}
}

func TestRankAndLimit(t *testing.T) {
	sugs := []suggestion{
		{text: "low", score: 1},
		{text: "high", score: 100},
		{text: "mid", score: 50},
	}
	result := rankAndLimit(sugs, 2)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if result[0].text != "high" {
		t.Errorf("expected 'high' first, got %q", result[0].text)
	}
	if result[1].text != "mid" {
		t.Errorf("expected 'mid' second, got %q", result[1].text)
	}
}

func TestMoreKnownTrailers(t *testing.T) {
	names := trailerCanonicalNames()
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	expected := []string{
		"Co-authored-by", "Reviewed-by", "Signed-off-by",
		"Co-developed-by", "Helped-by", "Mentored-by",
		"Resolves", "Reverts", "Link", "See-also",
		"Thanks-to", "Supersedes",
	}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("expected trailer %q in knownTrailers", e)
		}
	}
}
