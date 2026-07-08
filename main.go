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
		os.Exit(2)
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
