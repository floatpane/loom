package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// --- Diff line types ---

type diffLineKind int

const (
	diffContext diffLineKind = iota
	diffAdd
	diffDelete
)

type diffLine struct {
	kind diffLineKind
	text string
}

type diffHunk struct {
	oldStart, oldLines int
	newStart, newLines int
	lines              []diffLine
}

type diffFileChange struct {
	oldPath string
	newPath string
	hunks   []diffHunk
}

// --- Diff styles (hex colors, dark theme) ---

type diffStyle struct {
	background string
	foreground string
	isBold     bool
}

type diffStyles struct {
	filename diffStyle
	divider  diffStyle
	context  diffStyle
	add      diffStyle
	del      diffStyle
	missing  diffStyle
}

const diffDefaultFg = "#c9d1d9"

func defaultDiffStyles() diffStyles {
	return diffStyles{
		filename: diffStyle{background: "#30363d", foreground: diffDefaultFg, isBold: true},
		divider:  diffStyle{background: "#30363d", foreground: "#8b949e", isBold: true},
		context:  diffStyle{background: "#161b22", foreground: diffDefaultFg},
		add:      diffStyle{background: "#303a30", foreground: diffDefaultFg},
		del:      diffStyle{background: "#3a3030", foreground: diffDefaultFg},
		missing:  diffStyle{background: "#21262d", foreground: "#8b949e"},
	}
}

// --- Syntax highlighting engine (adapted from matcha) ---

type tokenKind int

const (
	tokPlain       tokenKind = iota
	tokKeyword               // language keywords
	tokString                // string/char literals
	tokComment               // comments
	tokNumber                // numeric literals
	tokFunction              // function/method names
	tokType                  // type / capitalized identifiers
	tokPunctuation           // operators, brackets
	tokConstant              // ALL_CAPS constants, booleans, nil
)

func hlStyles() map[tokenKind]lipgloss.Style {
	return map[tokenKind]lipgloss.Style{
		tokKeyword:     lipgloss.NewStyle().Foreground(lipgloss.Color("#C678DD")).Bold(true),
		tokString:      lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")),
		tokComment:     lipgloss.NewStyle().Foreground(lipgloss.Color("#7F848E")).Italic(true),
		tokNumber:      lipgloss.NewStyle().Foreground(lipgloss.Color("#D19A66")),
		tokFunction:    lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")),
		tokType:        lipgloss.NewStyle().Foreground(lipgloss.Color("#56B6C2")),
		tokPunctuation: lipgloss.NewStyle().Foreground(lipgloss.Color("#ABB2BF")),
		tokConstant:    lipgloss.NewStyle().Foreground(lipgloss.Color("#D19A66")),
	}
}

type hlRule struct {
	re    *regexp.Regexp
	kind  tokenKind
	group int
}

func mustRule(pattern string, kind tokenKind) hlRule {
	return hlRule{re: regexp.MustCompile(pattern), kind: kind}
}

func mustGroupRule(pattern string, kind tokenKind) hlRule {
	return hlRule{re: regexp.MustCompile(pattern), kind: kind, group: 1}
}

func funcRule() hlRule {
	return mustGroupRule(`\b([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(`, tokFunction)
}

func goStringRule() hlRule {
	return mustRule("`[^`]*`"+`|"(?:\\.|[^"\\])*"`+`|'(?:\\.|[^'\\])*'`, tokString)
}

func jsStringRule() hlRule {
	return mustRule("`(?:\\.|[^`\\])*`"+`|"(?:\\.|[^"\\])*"`+`|'(?:\\.|[^'\\])*'`, tokString)
}

func pyStringRule() hlRule {
	return mustRule(`"""[\s\S]*?"""|'''[\s\S]*?'''`+`|"(?:\\.|[^"\\])*"`+`|'(?:\\.|[^'\\])*'`, tokString)
}

func languageRules(lang string) []hlRule {
	switch normalizeLang(lang) {
	case "go":
		return []hlRule{
			mustRule(`\/\/[^\n]*|\/\*[\s\S]*?\*\/`, tokComment),
			goStringRule(),
			mustRule(`\b(break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)\b`, tokKeyword),
			mustRule(`\b(true|false|nil|iota)\b`, tokConstant),
			mustRule(`\b(bool|byte|complex64|complex128|error|float32|float64|int|int8|int16|int32|int64|rune|string|uint|uint8|uint16|uint32|uint64|uintptr|any|comparable)\b`, tokType),
			mustRule(`\b[A-Z][A-Za-z0-9_]*\b`, tokType),
			mustRule(`\b[0-9][0-9_]*(\.[0-9_]+)?([eE][+-]?[0-9]+)?\b`, tokNumber),
			mustRule(`\b0[xX][0-9a-fA-F_]+\b`, tokNumber),
			funcRule(),
			mustRule(`[{}()\[\];,.<>=:+\-*/%&|^!?]`, tokPunctuation),
		}
	case "python":
		return []hlRule{
			mustRule(`#[^\n]*`, tokComment),
			pyStringRule(),
			mustRule(`\b(False|None|True|And|as|assert|async|await|break|class|continue|def|del|elif|else|except|finally|for|from|global|if|import|in|is|lambda|nonlocal|not|or|pass|raise|return|try|while|with|yield|match|case)\b`, tokKeyword),
			mustRule(`\b[A-Z][A-Za-z0-9_]*\b`, tokType),
			mustRule(`\b[0-9][0-9_]*(\.[0-9_]+)?([eE][+-]?[0-9]+)?\b`, tokNumber),
			mustRule(`\b0[xX][0-9a-fA-F_]+\b`, tokNumber),
			funcRule(),
			mustRule(`[{}()\[\];,:.<>=+\-*/%&|^!~@]`, tokPunctuation),
		}
	case "javascript", "typescript":
		return []hlRule{
			mustRule(`\/\/[^\n]*|\/\*[\s\S]*?\*\/`, tokComment),
			jsStringRule(),
			mustRule(`\b(break|case|catch|class|const|continue|debugger|default|delete|do|else|enum|export|extends|finally|for|function|if|import|in|instanceof|let|new|of|return|super|switch|this|throw|try|typeof|var|void|while|with|yield|async|await|static|as|from)\b`, tokKeyword),
			mustRule(`\b(true|false|null|undefined|NaN|Infinity)\b`, tokConstant),
			mustRule(`\b(boolean|number|string|any|unknown|never|void|object|symbol|bigint)\b`, tokType),
			mustRule(`\b[A-Z][A-Za-z0-9_]*\b`, tokType),
			mustRule(`\b[0-9][0-9_]*(\.[0-9_]+)?([eE][+-]?[0-9]+)?\b`, tokNumber),
			mustRule(`\b0[xX][0-9a-fA-F_]+\b`, tokNumber),
			funcRule(),
			mustRule(`[{}()\[\];,:.<>=+\-*/%&|^!~?]`, tokPunctuation),
		}
	case "rust":
		return []hlRule{
			mustRule(`\/\/[^\n]*|\/\*[\s\S]*?\*\/`, tokComment),
			mustRule(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`, tokString),
			mustRule(`\b(as|async|await|break|const|continue|crate|dyn|else|enum|extern|false|fn|for|if|impl|in|let|loop|match|mod|move|mut|pub|ref|return|self|Self|static|struct|super|trait|true|type|unsafe|use|where|while)\b`, tokKeyword),
			mustRule(`\b(bool|char|f32|f64|i8|i16|i32|i64|i128|isize|str|u8|u16|u32|u64|u128|usize|String|Option|Result|Vec)\b`, tokType),
			mustRule(`\b[A-Z][A-Za-z0-9_]*\b`, tokType),
			mustRule(`\b[0-9][0-9_]*(\.[0-9_]+)?([eE][+-]?[0-9]+)?\b`, tokNumber),
			mustRule(`\b0[xX][0-9a-fA-F_]+\b`, tokNumber),
			funcRule(),
			mustRule(`[{}()\[\];,:.<>=+\-*/%&|^!@?]`, tokPunctuation),
		}
	case "c", "cpp", "c++", "cc", "cxx", "h", "hpp":
		return []hlRule{
			mustRule(`\/\/[^\n]*|\/\*[\s\S]*?\*\/`, tokComment),
			mustRule(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`, tokString),
			mustRule(`\b(alignas|alignof|and|asm|auto|bool|break|case|catch|char|class|const|constexpr|continue|decltype|default|delete|do|double|else|enum|explicit|export|extern|false|float|for|friend|goto|if|inline|int|long|mutable|namespace|new|noexcept|nullptr|operator|or|private|protected|public|register|reinterpret_cast|return|short|signed|sizeof|static|static_assert|static_cast|struct|switch|template|this|throw|true|try|typedef|typename|union|unsigned|using|virtual|void|volatile|while)\b`, tokKeyword),
			mustRule(`\b(int8_t|int16_t|int32_t|int64_t|uint8_t|uint16_t|uint32_t|uint64_t|size_t|ssize_t|ptrdiff_t|wchar_t|char16_t|char32_t)\b`, tokType),
			mustRule(`\b[A-Z][A-Za-z0-9_]*\b`, tokType),
			mustRule(`\b[0-9][0-9_]*(\.[0-9_]+)?([eE][+-]?[0-9]+)?[fFuUlL]*\b`, tokNumber),
			mustRule(`\b0[xX][0-9a-fA-F_]+\b`, tokNumber),
			funcRule(),
			mustRule(`[{}()\[\];,:.<>=+\-*/%&|^!~?]`, tokPunctuation),
		}
	case "java", "kotlin", "kt", "scala", "groovy":
		return []hlRule{
			mustRule(`\/\/[^\n]*|\/\*[\s\S]*?\*\/`, tokComment),
			mustRule(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`, tokString),
			mustRule(`\b(abstract|assert|boolean|break|byte|case|catch|char|class|const|continue|default|do|double|else|enum|extends|final|finally|float|for|goto|if|implements|import|instanceof|int|interface|long|native|new|package|private|protected|public|return|short|static|strictfp|super|switch|synchronized|this|throw|throws|transient|try|void|volatile|while|var|val|fun|when|object|data|sealed|by|as)\b`, tokKeyword),
			mustRule(`\b(true|false|null)\b`, tokConstant),
			mustRule(`\b[A-Z][A-Za-z0-9_]*\b`, tokType),
			mustRule(`\b[0-9][0-9_]*(\.[0-9_]+)?([eE][+-]?[0-9]+)?[fFdDlL]?\b`, tokNumber),
			mustRule(`\b0[xX][0-9a-fA-F_]+\b`, tokNumber),
			funcRule(),
			mustRule(`[{}()\[\];,:.<>=+\-*/%&|^!~?]`, tokPunctuation),
		}
	case "ruby", "rb":
		return []hlRule{
			mustRule(`#[^\n]*`, tokComment),
			mustRule(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`, tokString),
			mustRule(`\b(BEGIN|END|alias|and|begin|break|case|class|def|defined\?|do|else|elsif|end|ensure|false|for|if|in|module|next|nil|not|or|redo|rescue|retry|return|self|super|then|true|undef|unless|until|when|while|yield)\b`, tokKeyword),
			mustRule(`\b[A-Z][A-Za-z0-9_]*\b`, tokType),
			mustRule(`\b[0-9][0-9_]*(\.[0-9_]+)?([eE][+-]?[0-9]+)?\b`, tokNumber),
			funcRule(),
			mustRule(`[{}()\[\];,:.<>=+\-*/%&|^!@?]`, tokPunctuation),
		}
	case "bash", "sh", "shell", "zsh":
		return []hlRule{
			mustRule(`#[^\n]*`, tokComment),
			mustRule(`"(?:\\.|[^"\\])*"|'(?:[^'\\])*'`, tokString),
			mustRule(`\b(if|then|else|elif|fi|for|do|done|while|until|case|esac|in|function|return|local|export|readonly|declare|typeset|unset|shift|break|continue|exit)\b`, tokKeyword),
			mustRule(`\b(true|false|null)\b`, tokConstant),
			mustRule(`\b[0-9]+\b`, tokNumber),
			mustGroupRule(`\b([a-zA-Z_][a-zA-Z0-9_-]*)\s*\(`, tokFunction),
			mustRule(`[$]\{?[A-Za-z_][A-Za-z0-9_]*\}?`, tokConstant),
			mustRule(`[{}()\[\];,:.<>=+\-*/%&|^!]`, tokPunctuation),
		}
	case "html", "xml", "svg":
		return []hlRule{
			mustRule(`<!--[\s\S]*?-->`, tokComment),
			mustRule(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`, tokString),
			mustRule(`<\/?[a-zA-Z][a-zA-Z0-9:-]*`, tokKeyword),
			mustRule(`\/?>`, tokPunctuation),
			mustGroupRule(`([a-zA-Z_:][a-zA-Z0-9_:.-]*)\s*=`, tokType),
			mustRule(`[{}()\[\];,:.<>=+\-*/%&|^!?]`, tokPunctuation),
		}
	case "css", "scss", "less":
		return []hlRule{
			mustRule(`\/\*[\s\S]*?\*\/`, tokComment),
			mustRule(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`, tokString),
			mustRule(`\b(important|inherit|initial|unset|auto|none|inline|block|flex|grid|absolute|relative|fixed|sticky|static|hidden|visible)\b`, tokConstant),
			mustRule(`#[0-9a-fA-F]{3,8}\b`, tokNumber),
			mustRule(`\b[0-9]+(\.[0-9]+)?(px|em|rem|vh|vw|%|s|ms|deg|fr)?\b`, tokNumber),
			mustRule(`[.#][a-zA-Z_][a-zA-Z0-9_-]*`, tokType),
			mustGroupRule(`([a-zA-Z-]+)\s*:`, tokFunction),
			mustRule(`[{}()\[\];,:.<>=+\-*/%&|!]`, tokPunctuation),
		}
	case "json":
		return []hlRule{
			mustGroupRule(`("(?:\\.|[^"\\])*")\s*:`, tokType),
			mustRule(`"(?:\\.|[^"\\])*"`, tokString),
			mustRule(`\b(true|false|null)\b`, tokConstant),
			mustRule(`-?\b[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?\b`, tokNumber),
			mustRule(`[{}\[\]:,]`, tokPunctuation),
		}
	case "yaml", "yml":
		return []hlRule{
			mustRule(`#[^\n]*`, tokComment),
			mustRule(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`, tokString),
			mustRule(`\b(true|false|null|yes|no|on|off)\b`, tokConstant),
			mustRule(`-?\b[0-9]+(\.[0-9]+)?\b`, tokNumber),
			mustGroupRule(`\b([a-zA-Z_][a-zA-Z0-9_.-]*)\s*:`, tokType),
			mustRule(`[:{}\[\],\-]`, tokPunctuation),
		}
	case "sql":
		return []hlRule{
			mustRule(`--[^\n]*|\/\*[\s\S]*?\*\/`, tokComment),
			mustRule(`'(?:\\.|[^'\\])*'`, tokString),
			mustRule(`\b(SELECT|FROM|WHERE|INSERT|INTO|UPDATE|DELETE|CREATE|TABLE|DROP|ALTER|ADD|AND|OR|NOT|NULL|PRIMARY|KEY|FOREIGN|REFERENCES|JOIN|LEFT|RIGHT|INNER|OUTER|ON|GROUP|BY|ORDER|HAVING|LIMIT|OFFSET|DISTINCT|AS|VALUES|SET|DEFAULT|CONSTRAINT|UNIQUE|INDEX|VIEW|BEGIN|COMMIT|ROLLBACK|CASE|WHEN|THEN|ELSE|END|IN|IS|LIKE|BETWEEN|EXISTS|UNION|ALL)\b`, tokKeyword),
			mustRule(`\b(INT|INTEGER|BIGINT|SMALLINT|VARCHAR|CHAR|TEXT|BOOLEAN|BOOL|DATE|TIME|TIMESTAMP|FLOAT|DOUBLE|DECIMAL|NUMERIC|SERIAL|UUID|JSON|JSONB|BLOB)\b`, tokType),
			mustRule(`\b[0-9]+(\.[0-9]+)?\b`, tokNumber),
			mustRule(`[{}()\[\];,.<>=+\-*/%]`, tokPunctuation),
		}
	case "markdown", "md":
		return []hlRule{
			mustRule(`<!--[\s\S]*?-->`, tokComment),
			mustRule(`#{1,6}\s.*$`, tokKeyword),
			mustRule("`[^`]*`", tokString),
			mustRule(`\*\*[^*]+\*\*|__[^_]+__`, tokType),
			mustRule(`\[[^\]]*\]\([^)]*\)`, tokFunction),
			mustRule(`^[>\-\*\+]\s`, tokPunctuation),
		}
	}
	return nil
}

func normalizeLang(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	switch l {
	case "py":
		return "python"
	case "js", "jsx":
		return "javascript"
	case "ts", "tsx":
		return "typescript"
	case "rs":
		return "rust"
	case "rb":
		return "ruby"
	case "sh", "zsh":
		return "bash"
	case "yml":
		return "yaml"
	case "c++", "cc", "cxx", "hpp":
		return "cpp"
	case "kt":
		return "kotlin"
	case "md":
		return "markdown"
	}
	return l
}

func highlightCode(code, lang string) string {
	rules := languageRules(lang)
	if rules == nil || strings.TrimSpace(code) == "" {
		return code
	}

	type span struct {
		start, end int
		ruleIdx    int
	}
	var spans []span
	for ri, r := range rules {
		for _, m := range r.re.FindAllStringSubmatchIndex(code, -1) {
			start, end := m[0], m[1]
			if r.group > 0 && r.group*2+1 < len(m) {
				if m[r.group*2] >= 0 {
					start, end = m[r.group*2], m[r.group*2+1]
				}
			}
			spans = append(spans, span{start, end, ri})
		}
	}

	bestRule := make([]int, len(code))
	for i := range bestRule {
		bestRule[i] = -1
	}
	for _, s := range spans {
		for i := s.start; i < s.end && i < len(code); i++ {
			if bestRule[i] == -1 || s.ruleIdx < bestRule[i] {
				bestRule[i] = s.ruleIdx
			}
		}
	}

	styles := hlStyles()
	var b strings.Builder
	i := 0
	for i < len(code) {
		ri := bestRule[i]
		j := i + 1
		for j < len(code) && bestRule[j] == ri {
			j++
		}
		segment := code[i:j]
		if ri < 0 {
			b.WriteString(segment)
		} else {
			b.WriteString(styles[rules[ri].kind].Render(segment))
		}
		i = j
	}
	return b.String()
}

func langFromPath(path string) string {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	return normalizeLang(ext)
}

// --- ANSI reset rewriting (from matcha) ---

var resetRe = regexp.MustCompile(`\x1b\[0*m`)

func rewriteResets(text string, s diffStyle) string {
	if !strings.Contains(text, "\x1b[") {
		return text
	}
	return resetRe.ReplaceAllString(text, outerSGR(s))
}

func outerSGR(s diffStyle) string {
	var parts []string
	if s.isBold {
		parts = append(parts, "1")
	} else {
		parts = append(parts, "22")
	}
	parts = append(parts, "23")
	if s.foreground != "" {
		r, g, b := hexToRGB(s.foreground)
		parts = append(parts, fmt.Sprintf("38;2;%d;%d;%d", r, g, b))
	}
	if s.background != "" {
		r, g, b := hexToRGB(s.background)
		parts = append(parts, fmt.Sprintf("48;2;%d;%d;%d", r, g, b))
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

func hexToRGB(hex string) (r, g, b int) {
	hex = strings.TrimPrefix(hex, "#")
	if _, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b); err != nil {
		return 0, 0, 0
	}
	return
}

// --- Diff rendering ---

func diffStyleFor(s diffStyle, width int) lipgloss.Style {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color(s.background)).
		Foreground(lipgloss.Color(s.foreground)).
		Width(width)
	if s.isBold {
		style = style.Bold(true)
	}
	return style
}

func diffFillLine(text string, width int) string {
	w := ansi.StringWidth(text)
	if w >= width {
		return ansi.Truncate(text, width, "…")
	}
	return text + strings.Repeat(" ", width-w)
}

func lineNumberDigits(files []diffFileChange) (before, after int) {
	for _, fc := range files {
		for _, h := range fc.hunks {
			before = max(before, len(strconv.Itoa(h.oldStart+h.oldLines)))
			after = max(after, len(strconv.Itoa(h.newStart+h.newLines)))
		}
	}
	if before < 3 {
		before = 3
	}
	if after < 3 {
		after = 3
	}
	return
}

func lineNumbersStr(before, after, beforeDigits, afterDigits int) string {
	beforeStr := strings.Repeat(" ", beforeDigits)
	if before > 0 {
		beforeStr = fmt.Sprintf("%*d", beforeDigits, before)
	}
	afterStr := strings.Repeat(" ", afterDigits)
	if after > 0 {
		afterStr = fmt.Sprintf("%*d", afterDigits, after)
	}
	return " " + beforeStr + "  " + afterStr + " "
}

func diffFullLine(s diffStyle, before, after, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth int, prefix, text string) string {
	nums := lineNumbersStr(before, after, beforeDigits, afterDigits)
	content := ansi.Truncate(prefix+text, contentWidth, "…")
	content = diffFillLine(content, contentWidth)
	line := nums + content
	return diffStyleFor(s, totalWidth).Render(line)
}

func renderDiffFileHeader(fc diffFileChange, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth int) string {
	label := fc.newPath
	if fc.oldPath != "" && fc.oldPath != fc.newPath {
		label = fc.oldPath + " → " + fc.newPath
	}
	return diffFullLine(defaultDiffStyles().filename, 0, 0, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth, "  ", label)
}

func renderDiffHunkDivider(hunk diffHunk, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth int) string {
	content := fmt.Sprintf("  @@ -%d,%d +%d,%d @@", hunk.oldStart, hunk.oldLines, hunk.newStart, hunk.newLines)
	return diffFullLine(defaultDiffStyles().divider, 0, 0, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth, "", content)
}

func cleanDiffLineText(text string) string {
	return strings.TrimRight(text, "\r")
}

func advanceLineNums(line diffLine, before, after int) (int, int) {
	switch line.kind {
	case diffContext:
		return before + 1, after + 1
	case diffAdd:
		return before, after + 1
	case diffDelete:
		return before + 1, after
	}
	return before, after
}

func renderDiffLine(line diffLine, beforeLine, afterLine int, lang string, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth int) string {
	text := cleanDiffLineText(line.text)
	st := defaultDiffStyles()
	switch line.kind {
	case diffContext:
		hl := rewriteResets(highlightCode(text, lang), st.context)
		return diffFullLine(st.context, beforeLine, afterLine, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth, "  ", hl)
	case diffAdd:
		hl := rewriteResets(highlightCode(text, lang), st.add)
		return diffFullLine(st.add, 0, afterLine, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth, "+ ", hl)
	case diffDelete:
		hl := rewriteResets(highlightCode(text, lang), st.del)
		return diffFullLine(st.del, beforeLine, 0, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth, "- ", hl)
	}
	return ""
}

// renderDiff renders parsed file changes as a solid unified diff block.
// If width <= 0, it defaults to 80.
func renderDiff(files []diffFileChange, width int) string {
	if len(files) == 0 {
		return ""
	}

	totalWidth := width
	if totalWidth <= 0 {
		totalWidth = 80
	}

	beforeDigits, afterDigits := lineNumberDigits(files)
	numWidth := (beforeDigits + 2) + (afterDigits + 2)
	contentWidth := totalWidth - numWidth
	if contentWidth < 20 {
		contentWidth = 20
	}

	var b strings.Builder
	for fi, fc := range files {
		b.WriteString(renderDiffFileHeader(fc, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth))
		b.WriteString("\n")

		lang := langFromPath(fc.newPath)
		if lang == "" {
			lang = langFromPath(fc.oldPath)
		}

		for _, hunk := range fc.hunks {
			b.WriteString(renderDiffHunkDivider(hunk, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth))
			b.WriteString("\n")

			beforeLine := hunk.oldStart
			afterLine := hunk.newStart

			for _, line := range hunk.lines {
				b.WriteString(renderDiffLine(line, beforeLine, afterLine, lang, beforeDigits, afterDigits, numWidth, contentWidth, totalWidth))
				b.WriteString("\n")
				beforeLine, afterLine = advanceLineNums(line, beforeLine, afterLine)
			}
		}

		if fi < len(files)-1 {
			b.WriteString("\n")
		}
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// --- Unified diff parser ---

var (
	hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
)

// parseUnifiedDiff parses raw unified diff text into structured file changes.
func parseUnifiedDiff(diff string) []diffFileChange {
	lines := strings.Split(diff, "\n")
	var files []diffFileChange
	var cur *diffFileChange
	var curHunk *diffHunk

	for _, raw := range lines {
		line := raw

		switch {
		case strings.HasPrefix(line, "diff --git"):
			if curHunk != nil && cur != nil {
				cur.hunks = append(cur.hunks, *curHunk)
				curHunk = nil
			}
			if cur != nil {
				files = append(files, *cur)
			}
			cur = &diffFileChange{}
		case strings.HasPrefix(line, "--- "):
			if cur != nil {
				cur.oldPath = strings.TrimPrefix(line, "--- ")
				cur.oldPath = strings.TrimPrefix(cur.oldPath, "a/")
			}
		case strings.HasPrefix(line, "+++ "):
			if cur != nil {
				cur.newPath = strings.TrimPrefix(line, "+++ ")
				cur.newPath = strings.TrimPrefix(cur.newPath, "b/")
			}
		case strings.HasPrefix(line, "rename from "):
			if cur != nil {
				cur.oldPath = strings.TrimPrefix(line, "rename from ")
			}
		case strings.HasPrefix(line, "rename to "):
			if cur != nil {
				cur.newPath = strings.TrimPrefix(line, "rename to ")
			}
		case hunkHeaderRe.MatchString(line):
			if curHunk != nil && cur != nil {
				cur.hunks = append(cur.hunks, *curHunk)
			}
			m := hunkHeaderRe.FindStringSubmatch(line)
			oldStart, _ := strconv.Atoi(m[1])
			oldLines := 1
			if m[2] != "" {
				oldLines, _ = strconv.Atoi(m[2])
			}
			newStart, _ := strconv.Atoi(m[3])
			newLines := 1
			if m[4] != "" {
				newLines, _ = strconv.Atoi(m[4])
			}
			curHunk = &diffHunk{
				oldStart: oldStart,
				oldLines: oldLines,
				newStart: newStart,
				newLines: newLines,
			}
		case strings.HasPrefix(line, "+"):
			if curHunk != nil {
				curHunk.lines = append(curHunk.lines, diffLine{kind: diffAdd, text: line[1:]})
			}
		case strings.HasPrefix(line, "-"):
			if curHunk != nil {
				curHunk.lines = append(curHunk.lines, diffLine{kind: diffDelete, text: line[1:]})
			}
		case strings.HasPrefix(line, " "):
			if curHunk != nil {
				curHunk.lines = append(curHunk.lines, diffLine{kind: diffContext, text: line[1:]})
			}
		default:
			// skip other lines (index, mode, "\ No newline", etc.)
		}
	}

	if curHunk != nil && cur != nil {
		cur.hunks = append(cur.hunks, *curHunk)
	}
	if cur != nil {
		files = append(files, *cur)
	}

	return files
}
