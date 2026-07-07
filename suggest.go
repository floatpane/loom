package main

import (
	"os/exec"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

// coAuthor represents a co-author extracted from git history.
type coAuthor struct {
	name  string
	email string
}

// commitType represents a conventional commit type for autocomplete.
type commitType struct {
	prefix   string
	desc     string
}

var conventionalCommitTypes = []commitType{
	{"feat", "new feature"},
	{"fix", "bug fix"},
	{"docs", "documentation"},
	{"style", "formatting"},
	{"refactor", "code restructuring"},
	{"perf", "performance"},
	{"test", "adding tests"},
	{"chore", "build/tooling"},
	{"ci", "CI changes"},
	{"build", "build system"},
	{"revert", "revert commit"},
}

var commonCommitWords = []string{
	"add", "remove", "update", "delete", "fix", "improve",
	"refactor", "rename", "move", "clean", "simplify",
	"optimize", "handle", "support", "allow", "prevent",
	"ensure", "validate", "check", "return", "parse",
	"render", "display", "show", "hide", "toggle",
	"enable", "disable", "reset", "clear", "init",
}

// loadCoAuthors parses Co-authored-by trailers from recent git history.
func loadCoAuthors() []coAuthor {
	cmd := exec.Command("git", "log", "--format=%(trailers:key=co-authored-by:valueonly)", "-100")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var authors []coAuthor
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "Name <email>"
		if !strings.HasSuffix(line, ">") {
			continue
		}
		idx := strings.LastIndex(line, "<")
		if idx < 1 {
			continue
		}
		name := strings.TrimSpace(line[:idx])
		email := strings.TrimSpace(line[idx+1 : len(line)-1])
		if name == "" || email == "" {
			continue
		}
		key := name + " <" + email + ">"
		if seen[key] {
			continue
		}
		seen[key] = true
		authors = append(authors, coAuthor{name: name, email: email})
	}
	return authors
}

// suggestion represents a single autocomplete suggestion.
type suggestion struct {
	text     string // full text to insert
	display  string // text shown in the popup
	kind     string // "type", "word", "coauthor"
}

// computeSuggestions returns suggestions based on the current word being typed
// at the given position in the text. Returns nil if no suggestions.
func computeSuggestions(lines []string, row, col int, coAuthors []coAuthor) []suggestion {
	if row < 0 || row >= len(lines) {
		return nil
	}
	line := lines[row]

	// Find the start of the current word
	wordStart := col
	for wordStart > 0 && !isWordBoundary(rune(line[wordStart-1])) {
		wordStart--
	}
	currentWord := line[wordStart:col]
	if currentWord == "" {
		return nil
	}
	lower := strings.ToLower(currentWord)

	// Check if we're on the first line and at the start — suggest commit types
	if row == 0 && wordStart == 0 {
		var suggestions []suggestion
		for _, ct := range conventionalCommitTypes {
			if strings.HasPrefix(ct.prefix, lower) && ct.prefix != lower {
				suggestions = append(suggestions, suggestion{
					text:    ct.prefix + ": ",
					display: ct.prefix + " — " + ct.desc,
					kind:    "type",
				})
			}
		}
		if len(suggestions) > 0 {
			return suggestions
		}
	}

	// Check if we're typing a Co-authored-by trailer
	if strings.HasPrefix(strings.ToLower(line), "co-authored-by:") {
		// Get the part after the colon
		afterColon := strings.TrimSpace(line[strings.Index(strings.ToLower(line), ":")+1:])
		if afterColon == "" || strings.Contains(afterColon, ">") {
			return nil
		}
		var suggestions []suggestion
		for _, ca := range coAuthors {
			formatted := ca.name + " <" + ca.email + ">"
			if strings.HasPrefix(strings.ToLower(ca.name), strings.ToLower(afterColon)) ||
				strings.HasPrefix(strings.ToLower(formatted), strings.ToLower(afterColon)) {
				suggestions = append(suggestions, suggestion{
					text:    formatted,
					display: formatted,
					kind:    "coauthor",
				})
			}
		}
		sort.Slice(suggestions, func(i, j int) bool {
			return suggestions[i].display < suggestions[j].display
		})
		return suggestions
	}

	// Generic word suggestions from common commit words
	var suggestions []suggestion
	for _, w := range commonCommitWords {
		if strings.HasPrefix(w, lower) && w != lower {
			suggestions = append(suggestions, suggestion{
				text:    w,
				display: w,
				kind:    "word",
			})
		}
	}

	if len(suggestions) == 0 {
		return nil
	}
	// Limit to 8 suggestions
	if len(suggestions) > 8 {
		suggestions = suggestions[:8]
	}
	return suggestions
}

// findWordStart returns the column where the current word starts.
func findWordStart(line string, col int) int {
	i := col
	for i > 0 && !isWordBoundary(rune(line[i-1])) {
		i--
	}
	return i
}

func isWordBoundary(r rune) bool {
	return r == ' ' || r == '\t' || r == ':' || r == '(' || r == ')' || r == '[' || r == ']' || r == ',' || r == ';' || r == '\n' || r == '\r'
}

// --- Commit message coloring ---

var (
	commitTypeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	commitScopeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	commitColonStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	commitDescStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("254"))
	commitBulletStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	commitTrailerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	commitEmailStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	commitDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// highlightCommitLine colors a single commit message line based on its content.
func highlightCommitLine(line string, lineNum int) string {
	if lineNum == 0 {
		return highlightCommitSubject(line)
	}
	lower := strings.ToLower(line)
	if strings.Contains(lower, ":") {
		for _, trailer := range []string{"co-authored-by", "signed-off-by", "fixes", "closes", "refs", "reviewed-by", "tested-by", "reported-by"} {
			if strings.HasPrefix(lower, trailer+":") {
				return highlightTrailerLine(line, trailer)
			}
		}
	}
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		indent := line[:len(line)-len(trimmed)]
		return commitDimStyle.Render(indent) + commitBulletStyle.Render(trimmed[:2]) + commitDescStyle.Render(trimmed[2:])
	}
	return line
}

func highlightCommitSubject(line string) string {
	colonIdx := strings.Index(line, ":")
	if colonIdx <= 0 {
		return line
	}
	typePart := line[:colonIdx]
	rest := line[colonIdx:]
	parenIdx := strings.Index(typePart, "(")
	if parenIdx >= 0 {
		typeName := typePart[:parenIdx]
		scopePart := typePart[parenIdx:]
		return commitTypeStyle.Render(typeName) +
			commitScopeStyle.Render(scopePart) +
			commitColonStyle.Render(":") +
			commitDescStyle.Render(rest[1:])
	}
	return commitTypeStyle.Render(typePart) +
		commitColonStyle.Render(":") +
		commitDescStyle.Render(rest[1:])
}

func highlightTrailerLine(line, trailer string) string {
	colonIdx := strings.Index(line, ":")
	if colonIdx < 0 {
		return line
	}
	prefix := line[:colonIdx+1]
	rest := strings.TrimSpace(line[colonIdx+1:])
	emailIdx := strings.LastIndex(rest, "<")
	if emailIdx >= 0 && strings.HasSuffix(rest, ">") {
		name := strings.TrimSpace(rest[:emailIdx])
		email := rest[emailIdx:]
		return commitTrailerStyle.Render(prefix+" ") +
			commitDescStyle.Render(name+" ") +
			commitEmailStyle.Render(email)
	}
	return commitTrailerStyle.Render(prefix+" ") + commitDescStyle.Render(rest)
}
