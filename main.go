package main

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	// Top-level subcommands.
	switch os.Args[1] {
	case "rebase":
		if err := runRebaseCmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "loom rebase: %v\n", err)
			os.Exit(1)
		}
		return
	case "version", "--version", "-v":
		fmt.Printf("loom %s (commit %s, built %s)\n", version, commit, date)
		return
	case "help", "--help", "-h":
		printHelp()
		return
	case "config":
		runConfigCmd(os.Args[2:])
		return
	}

	path := os.Args[1]

	var m tea.Model
	if strings.Contains(path, "git-rebase-todo") {
		m = newRebaseModel(path)
	} else {
		m = newCommitModel(path)
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stdout), tea.WithInput(os.Stdin))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "loom: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "loom %s (commit %s, built %s)\n", version, commit, date)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "A beautiful Git commit and rebase editor for the terminal.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "USAGE")
	fmt.Fprintln(os.Stderr, "  loom <file-path>        Edit a commit/merge/tag message")
	fmt.Fprintln(os.Stderr, "  loom rebase             Rebase current branch onto upstream")
	fmt.Fprintln(os.Stderr, "  loom rebase pr <n>      Rebase PR #n onto upstream")
	fmt.Fprintln(os.Stderr, "  loom version            Show version info")
	fmt.Fprintln(os.Stderr, "  loom help               Show detailed help")
	fmt.Fprintln(os.Stderr, "  loom config             Show/edit configuration")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "SETUP")
	fmt.Fprintln(os.Stderr, "  git config --global core.editor \"loom\"")
	fmt.Fprintln(os.Stderr, "  git config --global sequence.editor \"loom\"")
}

func printHelp() {
	fmt.Printf("loom %s\n\n", version)
	fmt.Println("A beautiful Git commit and rebase editor for the terminal.")
	fmt.Println()
	fmt.Println("loom replaces GIT_EDITOR and GIT_SEQUENCE_EDITOR, providing:")
	fmt.Println()
	fmt.Println("COMMIT EDITOR")
	fmt.Println("  • Syntax highlighting for conventional commits")
	fmt.Println("  • Floating autocomplete (types, scopes, trailers, people)")
	fmt.Println("  • Gitmoji emoji suggestions (type :sparkles, :bug, etc.)")
	fmt.Println("  • Co-author suggestions from git history")
	fmt.Println("  • Commit message linting (length, mood, formatting)")
	fmt.Println("  • Scrollable, syntax-highlighted diff view (35+ languages)")
	fmt.Println("  • Branch-aware suggestions (type/scope from branch name)")
	fmt.Println("  • Undo/redo, bullet auto-continue, line duplication")
	fmt.Println("  • Draft autosave to ~/.loom/draft.txt")
	fmt.Println("  • Exit confirmation for unsaved changes")
	fmt.Println("  • Merge message and tag annotation support")
	fmt.Println()
	fmt.Println("REBASE EDITOR")
	fmt.Println("  • Full action support: pick, reword, edit, squash, fixup, drop")
	fmt.Println("  • exec, break, label, reset, merge, update-ref commands")
	fmt.Println("  • Inline diff expansion per commit")
	fmt.Println("  • Action cycling, squash-all-up, search/filter")
	fmt.Println("  • Commit metadata (author, date)")
	fmt.Println()
	fmt.Println("KEYBINDINGS (commit editor)")
	fmt.Println("  ctrl+s     save & quit          ctrl+z     undo")
	fmt.Println("  esc        cancel               ctrl+y     redo")
	fmt.Println("  ctrl+f     toggle fullscreen    ctrl+h     help overlay")
	fmt.Println("  ctrl+o     add co-author        ctrl+d     duplicate line")
	fmt.Println("  tab        accept suggestion / focus diff")
	fmt.Println()
	fmt.Println("KEYBINDINGS (rebase editor)")
	fmt.Println("  p/r/e/s/f/d  set action         c          cycle action")
	fmt.Println("  S            squash-all-up       /          search")
	fmt.Println("  tab/space    expand diff         enter      save")
	fmt.Println()
	fmt.Println("CONFIGURATION")
	fmt.Println("  ~/.loom/config.json   Lint thresholds, signoff, co-authors")
	fmt.Println("  ~/.loom/people.json   Persistent co-author list")
	fmt.Println("  ~/.loom/draft.txt     Autosaved commit draft")
	fmt.Println()
	fmt.Println("INSTALLATION")
	fmt.Println("  git config --global core.editor \"loom\"")
	fmt.Println("  git config --global sequence.editor \"loom\"")
}

func runConfigCmd(args []string) {
	cfg := loadLoomConfig()
	if len(args) == 0 {
		// Show current config
		fmt.Println("# loom configuration (~/.loom/config.json)")
		fmt.Printf("signoff:       %v\n", cfg.Signoff)
		if len(cfg.CoAuthor) > 0 {
			fmt.Println("coAuthors:")
			for _, ca := range cfg.CoAuthor {
				fmt.Printf("  - %s\n", ca)
			}
		}
		return
	}
	// Could implement config editing here in the future
	fmt.Println("Config editing not yet implemented. Edit ~/.loom/config.json manually.")
}

