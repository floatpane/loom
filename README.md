<div align="center">

<img src="assets/logo.png" alt="Logo">

# loom

A beautiful Git commit and rebase editor for the terminal.

[![Go Version](https://img.shields.io/github/go-mod/go-version/floatpane/loom)](https://golang.org)
[![GitHub release](https://img.shields.io/github/v/release/floatpane/loom)](https://github.com/floatpane/loom/releases)
[![License](https://img.shields.io/github/license/floatpane/loom)](LICENSE)

</div>

loom is a lightweight TUI that replaces `GIT_EDITOR` for writing commit
messages and interactive rebasing. It provides syntax-highlighted commit
messages, conventional-commit autocomplete, co-author suggestions, a
scrollable diff view, a floating suggestion popup styled after VSCode
and nvim-cmp, gitmoji support, undo/redo, and much more.

## Features

### Commit Editor
- **Commit message editor** — syntax highlighting for conventional commits,
  trailers (`Co-authored-by`, `Reviewed-by`, `Signed-off-by`, etc.), and
  bullet points
- **Merge & tag support** — automatically detects `MERGE_MSG` and
  `TAG_EDITMSG` files and adjusts the interface accordingly
- **Floating autocomplete** — VSCode/nvim-style suggestion popup at the
  cursor position, powered by the [bubble-overlay](https://github.com/floatpane/bubble-overlay)
  library
- **Conventional commit types** — `feat`, `fix`, `docs`, `refactor`, and
  more, with descriptions, ranked by frequency in your history
- **Scope suggestions** — type `feat(` to see scopes from your commit
  history, with branch-derived scope prioritized
- **Gitmoji support** — type `:sparkles`, `:bug`, etc. to insert emoji
  prefixes (60+ emoji)
- **Breaking change marker** — type `feat!` to get `!: ` suggestion
- **Trailer autocomplete** — suggests 30+ trailer names and person values
  from git history and a persistent local store
- **Issue number suggestions** — `Fixes: #` suggests issue numbers from
  your branch name
- **Branch-aware suggestions** — derives commit type, scope, and issue
  number from the current branch name (e.g. `feat/api-123` →
  `feat(api): ... #123`)
- **Fuzzy matching** — word suggestions use subsequence matching for
  flexible autocomplete
- **Recent commit words** — word suggestions include words from your
  recent 50 commits, filtered by relevance
- **Persistent people** — co-authors and reviewers are saved to
  `~/.loom/people.json` and recalled across sessions
- **Diff view** — scrollable, syntax-highlighted diff with line numbers
  for 35+ languages
- **Undo/redo** — full undo/redo stack (ctrl+z / ctrl+y)
- **Auto-continue bullet lists** — pressing Enter after a bullet line
  creates a new bullet; pressing Enter on an empty bullet removes it
- **Line operations** — duplicate line (ctrl+d), move line up/down
  (alt+↑/↓), transpose characters (ctrl+t), goto line (ctrl+g)
- **Search** — search within the message (ctrl+n for next match)
- **Co-author quick-add** — ctrl+o adds `Co-authored-by:` for the
  current git user
- **Signoff support** — configurable auto-signoff via `~/.loom/config.json`
- **Draft autosave** — unsaved messages are autosaved to
  `~/.loom/draft.txt` and cleared on successful save
- **Exit confirmation** — warns about unsaved changes before quitting
- **Help overlay** — ctrl+h shows a full keybinding reference
- **Fullscreen mode** — ctrl+f toggles fullscreen for editor or diff
- **Configuration** — `~/.loom/config.json` for signoff preference
  and co-author list

### Rebase Editor
- **Full action support** — pick, reword, edit, squash, fixup, drop,
  exec, break, label, reset, merge, update-ref
- **Short action names** — supports `p`, `r`, `e`, `s`, `f`, `d` in
  todo files and normalizes them
- **Action cycling** — press `c` to cycle through actions
- **Squash-all-up** — press `S` to squash all commits from the cursor
  up to the first pick
- **Inline diff expansion** — press tab/space to expand a diff for any
  commit
- **Search/filter** — press `/` to search commit messages, hashes, and
  authors; `n` for next match
- **Commit metadata** — shows author and date for each commit
- **Commit count** — header shows total number of commits
- **Help overlay** — ctrl+h shows a full keybinding reference

### Diff View
- **35+ languages** — syntax highlighting for Go, Python, JavaScript,
  TypeScript, Rust, C/C++, Java, Kotlin, Scala, Groovy, Ruby, Bash,
  HTML/XML, CSS/SCSS, JSON, YAML, SQL, Markdown, PHP, Swift, Dart,
  Lua, Elixir, Haskell, Clojure, Perl, TOML, Dockerfile, Makefile,
  Protobuf, GraphQL, Nim, Zig, V, Nix, Terraform/HCL, Julia, R, Vue,
  Svelte, and more
- **File status indicators** — new files (✚), deleted files (✖),
  renamed files (→), binary files (◆), mode changes (⊞)
- **Word-level diff** — additions and deletions highlighted with
  language-aware syntax coloring
- **Binary file detection** — shows "Binary file — no diff available"

## Installation

### Homebrew

```bash
brew tap floatpane/loom
brew install loom
```

### Snap

```bash
sudo snap install loom
```

### Build from source

```bash
go install github.com/floatpane/loom@latest
```

### Download binary

Download the latest binary from the
[releases page](https://github.com/floatpane/loom/releases).

## Usage

Set loom as your Git editor:

```bash
git config --global core.editor "loom"
git config --global sequence.editor "loom"
```

Or use it ad-hoc:

```bash
GIT_EDITOR=loom git commit
GIT_EDITOR=loom git rebase -i HEAD~5
```

loom automatically detects whether the file is a `git-rebase-todo`, a
commit message, a merge message (`MERGE_MSG`), or a tag annotation
(`TAG_EDITMSG`) and launches the appropriate interface.

### Subcommands

```bash
loom version       # show version info
loom help          # show detailed help with all keybindings
loom config        # show current configuration
loom rebase        # rebase current branch onto upstream
loom rebase pr 123 # rebase PR #123
```

### Rebase command

loom also includes a convenience rebase command that fetches and rebases
the current branch onto its upstream default branch:

```bash
loom rebase
```

This fetches the latest changes from the current branch's upstream remote
and rebases onto `upstream/<default-branch>` (or the current remote's
default branch if no upstream is configured).

To rebase a specific PR's branch without leaving your current branch:

```bash
loom rebase pr 123
```

This checks out PR #123's branch, rebases it onto its upstream, then
returns you to the branch you were on.


## Contributing

Pull requests are welcome. Please follow the
[PR template](.github/pull_request_template.md) format.

## License

[MIT](LICENSE)
