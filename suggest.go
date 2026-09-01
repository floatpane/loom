package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

// person represents a name + email pair used in commit trailers
// (Co-authored-by, Reviewed-by, Signed-off-by, etc.).
type person struct {
	name  string
	email string
}

func (p person) formatted() string {
	return p.name + " <" + p.email + ">"
}

func (p person) key() string {
	return p.formatted()
}

// commitType represents a conventional commit type for autocomplete.
type commitType struct {
	prefix string
	desc   string
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
	"migrate", "deprecate", "restore", "revert", "replace",
	"extract", "inline", "merge", "split", "separate",
	"combine", "rename", "wrap", "unwrap", "guard",
	"document", "configure", "install", "build", "release",
	"test", "mock", "stub", "skip", "flaky",
	"cache", "buffer", "queue", "flush", "drain",
	"load", "store", "fetch", "push", "pull",
	"connect", "disconnect", "bind", "unbind", "listen",
	"resolve", "reject", "retry", "timeout", "cancel",
	"abort", "commit", "rollback", "rebase", "cherry-pick",
	"upgrade", "downgrade", "bump", "pin", "unpin",
	"format", "lint", "typecheck", "coverage", "benchmark",
	"parallel", "concurrent", "async", "sync", "atomic",
	"panic", "error", "warn", "log", "trace",
	"memory", "cpu", "disk", "network", "latency",
	"crash", "leak", "deadlock", "race", "overflow",
	"sanitize", "escape", "unescape", "encode", "decode",
	"encrypt", "decrypt", "hash", "verify", "authenticate",
	"authorize", "login", "logout", "session", "token",
	"schema", "migration", "column", "index", "constraint",
	"route", "endpoint", "handler", "middleware", "request",
	"response", "header", "cookie", "status", "redirect",
	"component", "props", "state", "hook", "effect",
	"context", "provider", "consumer", "selector", "reducer",
}

// trailerDef describes a commit trailer that loom knows about.
type trailerDef struct {
	// canonical is the properly-cased trailer key as it should appear
	// in the commit message (e.g. "Co-authored-by").
	canonical string
	// personValue indicates the trailer's value is a "Name <email>" pair.
	personValue bool
}

// knownTrailers lists all trailers that loom recognizes for autocomplete
// and syntax highlighting. The order is the priority order for suggestion
// display.
var knownTrailers = []trailerDef{
	{"Co-authored-by", true},
	{"Reviewed-by", true},
	{"Co-reviewed-by", true},
	{"Signed-off-by", true},
	{"Acked-by", true},
	{"Tested-by", true},
	{"Reported-by", true},
	{"Suggested-by", true},
	{"Requested-by", true},
	{"Helped-by", true},
	{"Mentored-by", true},
	{"Written-by", true},
	{"Documented-by", true},
	{"Based-on-patch-by", true},
	{"Original-author", true},
	{"Co-developed-by", true},
	{"Fixes", false},
	{"Closes", false},
	{"Resolves", false},
	{"Refs", false},
	{"Reverts", false},
	{"Part-of", false},
	{"Link", false},
	{"Bug", false},
	{"Issue", false},
	{"PR", false},
	{"See-also", false},
	{"Supersedes", false},
	{"Cc", false},
	{"Signed-off", false},
	{"Change-id", false},
	{"Reviewed-on", false},
	{"Tested-on", false},
	{"Release", false},
	{"Stability", false},
	{"Honor", false},
	{"Thanks-to", true},
	{"Notification", false},
	{"Commit-message", false},
}

// personValueTrailers returns the set of trailer canonical names that
// take a person value, for fast lookup.
func personValueTrailers() map[string]bool {
	m := make(map[string]bool)
	for _, t := range knownTrailers {
		if t.personValue {
			m[strings.ToLower(t.canonical)] = true
		}
	}
	return m
}

// isKnownCommitType reports whether the given string is a recognized
// conventional commit type (feat, fix, etc.).
func isKnownCommitType(t string) bool {
	for _, ct := range conventionalCommitTypes {
		if ct.prefix == t {
			return true
		}
	}
	return false
}

// extractConventionalType returns the type part of a conventional commit
// subject (e.g. "feat" from "feat(api): add thing"), or "" if not conventional.
func extractConventionalType(subject string) string {
	idx := strings.Index(subject, ":")
	if idx <= 0 {
		return ""
	}
	typePart := subject[:idx]
	parenIdx := strings.Index(typePart, "(")
	if parenIdx >= 0 {
		return typePart[:parenIdx]
	}
	// Strip trailing "!" (breaking change marker)
	typePart = strings.TrimSuffix(typePart, "!")
	return typePart
}

// visibleWidth counts visible characters in a string, ignoring ANSI escape
// sequences. Used for displaying subject length in the status bar.
func visibleWidth(s string) int {
	count := 0
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) {
					if s[j] >= 0x40 && s[j] <= 0x7e {
						j++
						break
					}
					j++
				}
			} else if j < len(s) {
				j++
			}
			i = j
			continue
		}
		count++
		i++
	}
	return count
}

// trailerCanonicalNames returns all known trailer canonical names.
func trailerCanonicalNames() []string {
	names := make([]string, len(knownTrailers))
	for i, t := range knownTrailers {
		names[i] = t.canonical
	}
	return names
}

// peopleStore manages a collection of people (name + email) used for
// commit trailer autocomplete. It merges people discovered from git
// history with people persisted to a local file, and supports saving
// new people for future sessions.
type peopleStore struct {
	people   []person
	filePath string
	loaded   bool
}

// newPeopleStore creates a store that loads from git history and
// persists to ~/.loom/people.json.
func newPeopleStore() *peopleStore {
	ps := &peopleStore{
		filePath: peopleFilePath(),
	}
	ps.load()
	return ps
}

// peopleFilePath returns the path to the persistent people file.
func peopleFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".loom", "people.json")
}

// load populates the store from the persistent file and git history.
func (ps *peopleStore) load() {
	seen := make(map[string]bool)

	// 1. Load from persistent file
	if ps.filePath != "" {
		if data, err := os.ReadFile(ps.filePath); err == nil {
			var entries []personEntry
			if err := json.Unmarshal(data, &entries); err == nil {
				for _, e := range entries {
					if e.Name == "" || e.Email == "" {
						continue
					}
					p := person{name: e.Name, email: e.Email}
					k := p.key()
					if !seen[k] {
						seen[k] = true
						ps.people = append(ps.people, p)
					}
				}
			}
		}
	}

	// 2. Load from git history (Co-authored-by trailers)
	gitPeople := loadPeopleFromGit()
	for _, p := range gitPeople {
		k := p.key()
		if !seen[k] {
			seen[k] = true
			ps.people = append(ps.people, p)
		}
	}

	// 3. Load from git history (Signed-off-by, Reviewed-by, etc.)
	for _, trailer := range []string{"signed-off-by", "reviewed-by", "acked-by", "tested-by", "reported-by", "suggested-by"} {
		extra := loadPeopleFromGitTrailer(trailer)
		for _, p := range extra {
			k := p.key()
			if !seen[k] {
				seen[k] = true
				ps.people = append(ps.people, p)
			}
		}
	}

	// Sort alphabetically by name
	sort.Slice(ps.people, func(i, j int) bool {
		return ps.people[i].name < ps.people[j].name
	})

	ps.loaded = true
}

// personEntry is the JSON representation for the persistent file.
type personEntry struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// save writes the current people list to the persistent file.
func (ps *peopleStore) save() {
	if ps.filePath == "" || len(ps.people) == 0 {
		return
	}
	entries := make([]personEntry, len(ps.people))
	for i, p := range ps.people {
		entries[i] = personEntry{Name: p.name, Email: p.email}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(ps.filePath), 0755); err != nil {
		return
	}
	_ = os.WriteFile(ps.filePath, data, 0644)
}

// addPerson adds a person to the store if not already present.
func (ps *peopleStore) addPerson(p person) {
	if p.name == "" || p.email == "" {
		return
	}
	for _, existing := range ps.people {
		if existing.key() == p.key() {
			return
		}
	}
	ps.people = append(ps.people, p)
	sort.Slice(ps.people, func(i, j int) bool {
		return ps.people[i].name < ps.people[j].name
	})
}

// matchingPeople returns people whose name or formatted string starts
// with the given prefix (case-insensitive).
func (ps *peopleStore) matchingPeople(prefix string) []person {
	if len(ps.people) == 0 {
		return nil
	}
	lower := strings.ToLower(prefix)
	var matches []person
	for _, p := range ps.people {
		if strings.HasPrefix(strings.ToLower(p.name), lower) ||
			strings.HasPrefix(strings.ToLower(p.formatted()), lower) {
			matches = append(matches, p)
		}
	}
	return matches
}

// loadPeopleFromGit extracts Co-authored-by trailers from recent git history.
func loadPeopleFromGit() []person {
	return loadPeopleFromGitTrailer("co-authored-by")
}

// loadPeopleFromGitTrailer extracts person values from a specific trailer
// key in recent git history.
func loadPeopleFromGitTrailer(trailer string) []person {
	cmd := exec.Command("git", "log", "--format=%(trailers:key="+trailer+":valueonly)", "-100")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var people []person
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
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
		people = append(people, person{name: name, email: email})
	}
	return people
}

// loadCoAuthors is kept for backward compatibility with tests.
func loadCoAuthors() []coAuthor {
	ps := newPeopleStore()
	result := make([]coAuthor, len(ps.people))
	for i, p := range ps.people {
		result[i] = coAuthor(p)
	}
	return result
}

// coAuthor is kept for backward compatibility with tests.
type coAuthor struct {
	name  string
	email string
}

// suggestion represents a single autocomplete suggestion.
type suggestion struct {
	text    string // full text to insert
	display string // text shown in the popup
	kind    string // "type", "word", "trailer", "person", "scope", "gitmoji", "issue"
	score   int    // ranking score (higher = more relevant)
}

// suggestionCtx provides git context to the suggestion engine, enabling
// branch-based type/scope suggestions, recent-commit-words, etc.
type suggestionCtx struct {
	branch       string
	branchType   string // conventional type derived from branch name
	branchScope  string // scope derived from branch name
	branchIssue  string // issue number from branch name
	scopes       []string // scopes seen in recent history
	recentWords  []string // words from recent commits
	typeFreqs    map[string]int // conventional type frequency in history
	gitmojiWords []string
}

var globalSugCtx *suggestionCtx

// loadSuggestionCtx populates global suggestion context from git.
func loadSuggestionCtx() *suggestionCtx {
	ctx := &suggestionCtx{}
	branch := currentBranchName()
	ctx.branch = branch
	ctx.branchType, ctx.branchScope, ctx.branchIssue = branchNameToSuggestions(branch)
	ctx.scopes = loadScopesFromHistory()
	ctx.recentWords = loadRecentCommitWords(150)
	ctx.typeFreqs = loadCommitTypeFrequencies()
	globalSugCtx = ctx
	return ctx
}

// computeSuggestions returns suggestions based on the current word being typed
// at the given position in the text. Returns nil if no suggestions.
func computeSuggestions(lines []string, row, col int, ps *peopleStore) []suggestion {
	if row < 0 || row >= len(lines) {
		return nil
	}
	line := lines[row]
	lineLower := strings.ToLower(line)

	// Clamp col to line length
	lineRunes := []rune(line)
	if col > len(lineRunes) {
		col = len(lineRunes)
	}

	// --- Gitmoji suggestions: triggered by ":" at start of line 0 ---
	if row == 0 && col > 0 && line[0] == ':' {
		// Check that everything before col is part of the gitmoji query
		query := strings.ToLower(line[1:col])
		isValid := true
		for _, c := range query {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
				isValid = false
				break
			}
		}
		if isValid {
			return gitmojiSuggestions(query)
		}
	}

	// --- Breaking change marker: "type!" or "type(scope)!" ---
	if row == 0 && col > 0 && col <= len(line) && line[col-1] == '!' {
		// Check if what's before ! is a valid conventional type
		before := line[:col-1]
		if isKnownCommitType(before) || (strings.Contains(before, "(") && isKnownCommitType(before[:strings.Index(before, "(")])) {
			return []suggestion{
				{
					text:    "!: ",
					display: "! — breaking change",
					kind:    "type",
					score:   50,
				},
			}
		}
	}

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

	ctx := globalSugCtx
	if ctx == nil {
		ctx = &suggestionCtx{}
	}

	// --- Gitmoji suggestions: triggered by ":" at start of line 0 ---
	if row == 0 && wordStart == 0 && strings.HasPrefix(currentWord, ":") {
		query := strings.ToLower(currentWord[1:])
		return gitmojiSuggestions(query)
	}

	// --- Conventional commit type suggestions (line 0, start of line) ---
	if row == 0 && wordStart == 0 {
		var suggestions []suggestion
		for _, ct := range conventionalCommitTypes {
			if strings.HasPrefix(ct.prefix, lower) && ct.prefix != lower {
				score := 10
				if ctx.typeFreqs != nil {
					score += ctx.typeFreqs[ct.prefix]
				}
				if ctx.branchType == ct.prefix {
					score += 100
				}
				suggestions = append(suggestions, suggestion{
					text:    ct.prefix + ": ",
					display: ct.prefix + " — " + ct.desc,
					kind:    "type",
					score:   score,
				})
			}
		}
		// Also suggest exact-match type with scope from branch
		if ctx.branchType != "" && ctx.branchType == lower && ctx.branchScope != "" {
			suggestions = append(suggestions, suggestion{
				text:    ctx.branchType + "(" + ctx.branchScope + "): ",
				display: ctx.branchType + "(" + ctx.branchScope + ") — from branch",
				kind:    "type",
				score:   200,
			})
		}
		if len(suggestions) > 0 {
			return rankAndLimit(suggestions, 8)
		}
	}

	// --- Scope suggestions: "type(" → suggest scopes from history ---
	if row == 0 {
		// Check if we're inside a scope: "type(sco"
		parenOpen := strings.Index(line, "(")
		if parenOpen >= 0 && parenOpen < col {
			// Check there's no closing paren yet
			closeParen := strings.Index(line[parenOpen:], ")")
			if closeParen < 0 || parenOpen+closeParen >= col {
				afterParen := line[parenOpen+1 : col]
				// Only suggest if the type part is a known commit type
				typePart := line[:parenOpen]
				if isKnownCommitType(typePart) {
					return scopeSuggestions(afterParen, ctx)
				}
			}
		}
	}

	// --- Breaking change marker: "type!" or "type(scope)!" ---
	if row == 0 && wordStart == 0 && currentWord == "!" {
		return []suggestion{
			{
				text:    "!: ",
				display: "! — breaking change",
				kind:    "type",
				score:   50,
			},
		}
	}

	// --- Trailer value suggestions (e.g. "Co-authored-by: Al") ---
	for _, t := range knownTrailers {
		prefix := strings.ToLower(t.canonical) + ":"
		if strings.HasPrefix(lineLower, prefix) {
			afterColon := strings.TrimSpace(line[len(prefix):])
			if t.personValue {
				// Person-value trailer: suggest people
				if afterColon == "" || strings.Contains(afterColon, ">") {
					return nil
				}
				return personSuggestions(ps, afterColon)
			}
			// Non-person trailer: suggest issue numbers for Fixes/Closes/Resolves/Refs
			if t.canonical == "Fixes" || t.canonical == "Closes" || t.canonical == "Resolves" || t.canonical == "Refs" || t.canonical == "Issue" || t.canonical == "Bug" || t.canonical == "PR" {
				return issueRefSuggestions(afterColon, ctx)
			}
			return nil
		}
	}

	// --- Trailer name suggestions (start of non-first line) ---
	if row > 0 && wordStart == 0 && col > 0 {
		var suggestions []suggestion
		for _, t := range knownTrailers {
			canonLower := strings.ToLower(t.canonical)
			if strings.HasPrefix(canonLower, lower) && canonLower != lower {
				suggestions = append(suggestions, suggestion{
					text:    t.canonical + ": ",
					display: t.canonical,
					kind:    "trailer",
					score:   10,
				})
			}
		}
		if len(suggestions) > 0 {
			return rankAndLimit(suggestions, 8)
		}
		// Exact match — suggest with value hint
		for _, t := range knownTrailers {
			if strings.ToLower(t.canonical) == lower {
				suggestions = append(suggestions, suggestion{
					text:    t.canonical + ": ",
					display: t.canonical,
					kind:    "trailer",
					score:   5,
				})
			}
		}
		if len(suggestions) > 0 {
			return suggestions
		}
	}

	// --- Generic word suggestions: combine common words + recent commit words ---
	var suggestions []suggestion
	seen := make(map[string]bool)
	for _, w := range commonCommitWords {
		if (strings.HasPrefix(w, lower) || fuzzyMatch(lower, w)) && w != lower && !seen[w] {
			seen[w] = true
			score := 10
			if strings.HasPrefix(w, lower) {
				score += 20
			}
			suggestions = append(suggestions, suggestion{
				text:    w,
				display: w,
				kind:    "word",
				score:   score,
			})
		}
	}
	// Add recent words from git history
	for _, w := range ctx.recentWords {
		if (strings.HasPrefix(w, lower) || fuzzyMatch(lower, w)) && w != lower && !seen[w] {
			seen[w] = true
			score := 5
			if strings.HasPrefix(w, lower) {
				score += 15
			}
			suggestions = append(suggestions, suggestion{
				text:    w,
				display: w + " ~",
				kind:    "word",
				score:   score,
			})
		}
	}

	if len(suggestions) == 0 {
		return nil
	}
	return rankAndLimit(suggestions, 8)
}

// rankAndLimit sorts suggestions by score (descending) and truncates.
func rankAndLimit(suggestions []suggestion, limit int) []suggestion {
	// Simple insertion sort by score descending
	for i := 1; i < len(suggestions); i++ {
		for j := i; j > 0 && suggestions[j].score > suggestions[j-1].score; j-- {
			suggestions[j], suggestions[j-1] = suggestions[j-1], suggestions[j]
		}
	}
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}
	return suggestions
}

// fuzzyMatch checks if all characters of `query` appear in order in `target`.
func fuzzyMatch(query, target string) bool {
	if len(query) < 3 {
		return false
	}
	qi := 0
	for ti := 0; ti < len(target) && qi < len(query); ti++ {
		if query[qi] == target[ti] {
			qi++
		}
	}
	return qi == len(query)
}

// gitmojiSuggestions returns emoji suggestions for a ":" prefix query.
func gitmojiSuggestions(query string) []suggestion {
	var suggestions []suggestion
	for _, g := range gitmojiList {
		if query == "" || strings.Contains(g.code, query) {
			suggestions = append(suggestions, suggestion{
				text:    g.emoji + " ",
				display: g.emoji + " :" + g.code + " — " + g.desc,
				kind:    "gitmoji",
				score:   10,
			})
		}
	}
	if len(suggestions) == 0 {
		return nil
	}
	return rankAndLimit(suggestions, 8)
}

// scopeSuggestions returns scope suggestions from history matching the prefix.
func scopeSuggestions(prefix string, ctx *suggestionCtx) []suggestion {
	if ctx == nil || len(ctx.scopes) == 0 {
		return nil
	}
	lower := strings.ToLower(prefix)
	var suggestions []suggestion
	// Branch-derived scope always gets highest priority
	if ctx.branchScope != "" && strings.HasPrefix(strings.ToLower(ctx.branchScope), lower) {
		suggestions = append(suggestions, suggestion{
			text:    ctx.branchScope + "): ",
			display: ctx.branchScope + " — from branch",
			kind:    "scope",
			score:   200,
		})
	}
	for _, s := range ctx.scopes {
		if strings.HasPrefix(strings.ToLower(s), lower) && s != ctx.branchScope {
			suggestions = append(suggestions, suggestion{
				text:    s + "): ",
				display: s,
				kind:    "scope",
				score:   50,
			})
		}
	}
	if len(suggestions) == 0 {
		return nil
	}
	return rankAndLimit(suggestions, 8)
}

// issueRefSuggestions suggests issue/PR references.
func issueRefSuggestions(prefix string, ctx *suggestionCtx) []suggestion {
	var suggestions []suggestion

	// Suggest "#" prefix if empty
	if prefix == "" {
		suggestions = append(suggestions, suggestion{
			text:    "#",
			display: "# — issue number",
			kind:    "issue",
			score:   10,
		})
	}

	// Suggest branch-derived issue number
	if ctx != nil && ctx.branchIssue != "" {
		if strings.HasPrefix(ctx.branchIssue, strings.TrimPrefix(prefix, "#")) {
			suggestions = append(suggestions, suggestion{
				text:    "#" + ctx.branchIssue,
				display: "#" + ctx.branchIssue + " — from branch",
				kind:    "issue",
				score:   100,
			})
		}
	}

	if len(suggestions) == 0 {
		return nil
	}
	return rankAndLimit(suggestions, 8)
}

// personSuggestions builds suggestion entries from the people store
// matching the given prefix.
func personSuggestions(ps *peopleStore, prefix string) []suggestion {
	if ps == nil {
		return nil
	}
	matches := ps.matchingPeople(prefix)
	if len(matches) == 0 {
		return nil
	}
	suggestions := make([]suggestion, len(matches))
	for i, p := range matches {
		suggestions[i] = suggestion{
			text:    p.formatted(),
			display: p.formatted(),
			kind:    "person",
		}
	}
	return suggestions
}

// extractPeopleFromTrailerLines scans the given lines for person-value
// trailers and returns all people found. This is used to save people
// from the commit message before saving.
func extractPeopleFromTrailerLines(lines []string) []person {
	personTrailers := personValueTrailers()
	var people []person
	seen := make(map[string]bool)
	for _, line := range lines {
		lineLower := strings.ToLower(line)
		colonIdx := strings.Index(lineLower, ":")
		if colonIdx < 0 {
			continue
		}
		trailerKey := strings.TrimSpace(lineLower[:colonIdx])
		if !personTrailers[trailerKey] {
			continue
		}
		value := strings.TrimSpace(line[colonIdx+1:])
		if !strings.HasSuffix(value, ">") {
			continue
		}
		emailIdx := strings.LastIndex(value, "<")
		if emailIdx < 1 {
			continue
		}
		name := strings.TrimSpace(value[:emailIdx])
		email := strings.TrimSpace(value[emailIdx+1 : len(value)-1])
		if name == "" || email == "" {
			continue
		}
		p := person{name: name, email: email}
		k := p.key()
		if !seen[k] {
			seen[k] = true
			people = append(people, p)
		}
	}
	return people
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
		for _, trailer := range trailerCanonicalNames() {
			if strings.HasPrefix(lower, strings.ToLower(trailer)+":") {
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
