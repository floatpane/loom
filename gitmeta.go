package main

import (
	"os"
	"os/exec"
	"sort"
	"strings"
)

// --- Git metadata helpers for commit suggestions and display ---

// currentBranchName returns the current git branch name.
func currentBranchName() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// branchNameToSuggestions parses a branch name and derives suggested
// conventional commit type and scope. For example:
//
//	feat/api-pagination   → type=feat, scope=api-pagination
//	fix/123-login-bug     → type=fix, scope=123-login-bug
//	123-login-bug         → scope=123 (issue number)
//	jira-456-refactor     → issue=456
//	feature/add-oauth     → type=feat (feature→feat)
//	hotfix/urgent-crash   → type=fix (hotfix→fix)
//	chore/deps            → type=chore, scope=deps
//	docs/readme-update    → type=docs, scope=readme-update
func branchNameToSuggestions(branch string) (commitType string, scope string, issueNum string) {
	if branch == "" || branch == "HEAD" {
		return
	}

	// Extract issue number from full branch name (before stripping)
	issueNum = extractIssueNumber(branch)

	// Strip known remote prefixes only (origin/, upstream/, etc.)
	for _, prefix := range []string{"origin/", "upstream/", "fork/"} {
		if strings.HasPrefix(branch, prefix) {
			branch = branch[len(prefix):]
			break
		}
	}

	// Try to find a known commit type prefix
	rest := branch
	for _, ct := range conventionalCommitTypes {
		aliases := commitTypeAliases(ct.prefix)
		all := append([]string{ct.prefix}, aliases...)
		for _, alias := range all {
			if strings.HasPrefix(rest, alias+"/") {
				commitType = ct.prefix
				rest = rest[len(alias)+1:]
				scope = rest
				break
			}
			if strings.HasPrefix(rest, alias+"-") {
				commitType = ct.prefix
				rest = rest[len(alias)+1:]
				scope = rest
				break
			}
		}
		if commitType != "" {
			break
		}
	}

	// If scope is empty, don't use full branch as scope
	if scope == "" {
		if commitType == "" {
			// No type prefix was found
			scope = ""
		}
	}

	return
}

func commitTypeAliases(prefix string) []string {
	switch prefix {
	case "feat":
		return []string{"feature"}
	case "fix":
		return []string{"hotfix", "bugfix", "bug"}
	case "chore":
		return []string{"misc", "deps"}
	case "docs":
		return []string{"doc"}
	case "refactor":
		return []string{"ref"}
	default:
		return nil
	}
}

// extractIssueNumber finds a GitHub/GitLab issue number in a string.
func extractIssueNumber(s string) string {
	// Patterns: #123, issue-123, 123-..., ...-123, JIRA-456, PROJ-789
	parts := strings.Split(s, "/")
	s = parts[len(parts)-1]

	// #123 or #123-...
	if strings.HasPrefix(s, "#") {
		rest := s[1:]
		// Extract leading digits
		num := ""
		for _, c := range rest {
			if c >= '0' && c <= '9' {
				num += string(c)
			} else {
				break
			}
		}
		if num != "" {
			return num
		}
	}

	// 123-... or ...-123
	for _, part := range strings.Split(s, "-") {
		if isAllDigits(part) && len(part) >= 1 {
			return part
		}
	}

	return ""
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// loadRecentCommitWords scans recent commit messages and returns unique
// words that can be used for generic word suggestions.
func loadRecentCommitWords(limit int) []string {
	if limit <= 0 {
		limit = 200
	}
	out, err := exec.Command("git", "log", "--format=%s%n%b", "-50").Output()
	if err != nil {
		return nil
	}
	words := make(map[string]bool)
	for _, word := range strings.Fields(string(out)) {
		word = strings.Trim(strings.ToLower(word), ":,.!?()[]{}\"'")
		if len(word) < 3 {
			continue
		}
		if isCommitStopWord(word) {
			continue
		}
		words[word] = true
	}
	result := make([]string, 0, len(words))
	for w := range words {
		result = append(result, w)
	}
	sort.Strings(result)
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func isCommitStopWord(w string) bool {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"into": true, "is": true, "was": true, "are": true, "were": true,
		"be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "this": true, "that": true, "it": true, "its": true,
		"as": true, "if": true, "then": true, "else": true, "when": true,
		"up": true, "down": true, "out": true, "off": true, "over": true,
		"under": true, "again": true, "not": true, "no": true, "yes": true,
		"all": true, "any": true, "some": true, "can": true, "will": true,
		"just": true, "about": true, "after": true, "before": true,
	}
	return stopWords[w]
}

// loadScopesFromHistory extracts conventional commit scopes from recent history.
func loadScopesFromHistory() []string {
	out, err := exec.Command("git", "log", "--format=%s", "-100").Output()
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var scopes []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Extract scope from "type(scope): ..."
		idx := strings.Index(line, "(")
		if idx <= 0 {
			continue
		}
		closeIdx := strings.Index(line[idx:], ")")
		if closeIdx < 0 {
			continue
		}
		scope := strings.TrimSpace(line[idx+1 : idx+closeIdx])
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return scopes
}

// loadDiffStat returns a short summary of staged changes.
func loadDiffStat() string {
	out, err := exec.Command("git", "diff", "--cached", "--stat").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// loadStagedFiles returns the list of staged files with their status.
func loadStagedFiles() []stagedFile {
	out, err := exec.Command("git", "diff", "--cached", "--name-status").Output()
	if err != nil {
		return nil
	}
	var files []stagedFile
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		files = append(files, stagedFile{status: parts[0], path: parts[1]})
	}
	return files
}

type stagedFile struct {
	status string
	path   string
}

// loadCommitTemplate reads the configured commit template, if any.
func loadCommitTemplate() string {
	out, err := exec.Command("git", "config", "--get", "commit.template").Output()
	if err != nil {
		return ""
	}
	templatePath := strings.TrimSpace(string(out))
	if templatePath == "" {
		return ""
	}
	// Expand ~ if needed
	if strings.HasPrefix(templatePath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			templatePath = home + templatePath[1:]
		}
	}
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return ""
	}
	return string(data)
}

// loadCommitTypeFrequencies returns how often each conventional commit type
// appears in recent history, for ranking suggestions.
func loadCommitTypeFrequencies() map[string]int {
	out, err := exec.Command("git", "log", "--format=%s", "-200").Output()
	if err != nil {
		return nil
	}
	freqs := make(map[string]int)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		ct := extractConventionalType(line)
		if ct != "" {
			freqs[ct]++
		}
	}
	return freqs
}
