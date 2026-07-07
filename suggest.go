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
	{"Fixes", false},
	{"Closes", false},
	{"Refs", false},
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
	os.MkdirAll(filepath.Dir(ps.filePath), 0755)
	os.WriteFile(ps.filePath, data, 0644)
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
		result[i] = coAuthor{name: p.name, email: p.email}
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
	kind    string // "type", "word", "trailer", "person"
}

// computeSuggestions returns suggestions based on the current word being typed
// at the given position in the text. Returns nil if no suggestions.
func computeSuggestions(lines []string, row, col int, ps *peopleStore) []suggestion {
	if row < 0 || row >= len(lines) {
		return nil
	}
	line := lines[row]
	lineLower := strings.ToLower(line)

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

	// Check if we're typing a trailer value (e.g. "Co-authored-by: Al")
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
			// Non-person trailer: no suggestions for now
			return nil
		}
	}

	// Check if we're at the start of a non-first line — suggest trailer names
	if row > 0 && wordStart == 0 && col > 0 {
		var suggestions []suggestion
		for _, t := range knownTrailers {
			canonLower := strings.ToLower(t.canonical)
			if strings.HasPrefix(canonLower, lower) && canonLower != lower {
				suggestions = append(suggestions, suggestion{
					text:    t.canonical + ": ",
					display: t.canonical,
					kind:    "trailer",
				})
			}
		}
		if len(suggestions) > 0 {
			return suggestions
		}
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
	if len(suggestions) > 8 {
		suggestions = suggestions[:8]
	}
	return suggestions
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
