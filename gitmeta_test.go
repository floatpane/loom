package main

import (
	"testing"
)

func TestBranchNameToSuggestionsFeat(t *testing.T) {
	ct, scope, _ := branchNameToSuggestions("feat/api-pagination")
	if ct != "feat" {
		t.Errorf("expected type=feat, got %s", ct)
	}
	if scope != "api-pagination" {
		t.Errorf("expected scope=api-pagination, got %s", scope)
	}
}

func TestBranchNameToSuggestionsFeatureAlias(t *testing.T) {
	ct, _, _ := branchNameToSuggestions("feature/add-oauth")
	if ct != "feat" {
		t.Errorf("expected type=feat for 'feature/', got %s", ct)
	}
}

func TestBranchNameToSuggestionsHotfixAlias(t *testing.T) {
	ct, _, _ := branchNameToSuggestions("hotfix/urgent-crash")
	if ct != "fix" {
		t.Errorf("expected type=fix for 'hotfix/', got %s", ct)
	}
}

func TestBranchNameToSuggestionsIssueNumber(t *testing.T) {
	_, _, issue := branchNameToSuggestions("fix/123-login-bug")
	if issue != "123" {
		t.Errorf("expected issue=123, got %s", issue)
	}
}

func TestBranchNameToSuggestionsHashPrefix(t *testing.T) {
	_, _, issue := branchNameToSuggestions("feature/#456-add-thing")
	if issue != "456" {
		t.Errorf("expected issue=456, got %s", issue)
	}
}

func TestBranchNameToSuggestionsRemotePrefix(t *testing.T) {
	ct, scope, _ := branchNameToSuggestions("origin/feat/test")
	if ct != "feat" {
		t.Errorf("expected type=feat, got %s", ct)
	}
	if scope != "test" {
		t.Errorf("expected scope=test, got %s", scope)
	}
}

func TestBranchNameToSuggestionsNoPrefix(t *testing.T) {
	ct, _, issue := branchNameToSuggestions("123-just-a-branch")
	if ct != "" {
		t.Errorf("expected empty type, got %s", ct)
	}
	_ = issue // issue may or may not be set depending on parsing
}

func TestBranchNameToSuggestionsEmpty(t *testing.T) {
	ct, scope, issue := branchNameToSuggestions("")
	if ct != "" || scope != "" || issue != "" {
		t.Errorf("expected all empty for empty branch")
	}
}

func TestBranchNameToSuggestionsHEAD(t *testing.T) {
	ct, _, _ := branchNameToSuggestions("HEAD")
	if ct != "" {
		t.Errorf("expected empty type for HEAD")
	}
}

func TestExtractIssueNumberDirect(t *testing.T) {
	if n := extractIssueNumber("#42"); n != "42" {
		t.Errorf("expected 42, got %s", n)
	}
}

func TestExtractIssueNumberInBranch(t *testing.T) {
	if n := extractIssueNumber("feature/add-thing-789"); n != "789" {
		t.Errorf("expected 789, got %s", n)
	}
}

func TestExtractIssueNumberNone(t *testing.T) {
	if n := extractIssueNumber("no-numbers-here"); n != "" {
		t.Errorf("expected empty, got %s", n)
	}
}

func TestIsAllDigits(t *testing.T) {
	if !isAllDigits("123") {
		t.Error("expected true for '123'")
	}
	if isAllDigits("12a") {
		t.Error("expected false for '12a'")
	}
	if isAllDigits("") {
		t.Error("expected false for empty string")
	}
}

func TestCommitTypeAliases(t *testing.T) {
	if len(commitTypeAliases("feat")) == 0 {
		t.Error("expected aliases for feat")
	}
	if len(commitTypeAliases("fix")) == 0 {
		t.Error("expected aliases for fix")
	}
	if len(commitTypeAliases("unknown")) != 0 {
		t.Error("expected no aliases for unknown type")
	}
}
