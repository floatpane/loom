<div align="center">

# loom

A beautiful Git commit and rebase editor for the terminal.

[![Go Version](https://img.shields.io/github/go-mod/go-version/floatpane/loom)](https://golang.org)
[![GitHub release](https://img.shields.io/github/v/release/floatpane/loom)](https://github.com/floatpane/loom/releases)
[![License](https://img.shields.io/github/license/floatpane/loom)](LICENSE)

</div>

loom is a lightweight TUI that replaces `GIT_EDITOR` for writing commit
messages and interactive rebasing. It provides syntax-highlighted commit
messages, conventional-commit autocomplete, co-author suggestions, a
scrollable diff view, and a floating suggestion popup styled after VSCode
and nvim-cmp.

## Features

- **Commit message editor** — syntax highlighting for conventional commits,
  trailers (`Co-authored-by`, `Reviewed-by`, `Signed-off-by`, etc.), and
  bullet points
- **Interactive rebase editor** — change actions (pick, reword, edit, squash,
  fixup, drop), reorder commits, and expand inline diffs
- **Floating autocomplete** — VSCode/nvim-style suggestion popup at the
  cursor position, powered by the [bubble-overlay](https://github.com/floatpane/bubble-overlay)
  library
- **Conventional commit types** — `feat`, `fix`, `docs`, `refactor`, and
  more, with descriptions
- **Trailer autocomplete** — suggests trailer names (`Co-authored-by:`,
  `Reviewed-by:`, …) and person values from git history and a persistent
  local store
- **Persistent people** — co-authors and reviewers are saved to
  `~/.loom/people.json` and recalled across sessions
- **Diff view** — scrollable, syntax-highlighted diff with line numbers for
  15+ languages

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

loom automatically detects whether the file is a `git-rebase-todo` or a
commit message and launches the appropriate interface.


## Contributing

Pull requests are welcome. Please follow the
[PR template](.github/pull_request_template.md) format.

## License

[MIT](LICENSE)
