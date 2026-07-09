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
		fmt.Fprintf(os.Stderr, "loom %s (commit %s, built %s)\n", version, commit, date)
		fmt.Fprintln(os.Stderr, "usage: loom <file-path>")
		fmt.Fprintln(os.Stderr, "       loom rebase")
		fmt.Fprintln(os.Stderr, "       loom rebase pr <number>")
		os.Exit(2)
	}

	// Top-level subcommands.
	if os.Args[1] == "rebase" {
		if err := runRebaseCmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "loom rebase: %v\n", err)
			os.Exit(1)
		}
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
