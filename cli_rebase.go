package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// runRebaseCmd handles `loom rebase` and `loom rebase pr <number>`.
func runRebaseCmd(args []string) error {
	if len(args) == 0 {
		if err := rebaseUpstream(); err != nil {
			return err
		}
		return pushCurrentBranch()
	}

	switch args[0] {
	case "pr":
		if len(args) < 2 {
			return fmt.Errorf("usage: loom rebase pr <number>")
		}
		number, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid PR number %q: %w", args[1], err)
		}
		return rebasePR(number)
	default:
		return fmt.Errorf("unknown rebase subcommand %q (expected: pr <number>)", args[0])
	}
}

// gitOutput runs a git command and returns trimmed stdout, or an error.
func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(exit.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// gitRun runs a git command, inheriting stdin/stdout/stderr so interactive
// editors and rebase sequences work correctly.
func gitRun(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// pushCurrentBranch force-pushes (with lease) the current branch to its
// upstream. Since a rebase rewrites history, a force-push is required.
// --force-with-lease is used to avoid overwriting remote changes that we
// don't know about locally.
func pushCurrentBranch() error {
	fmt.Fprintln(os.Stderr, "Pushing...")
	if err := gitRun("push", "--force-with-lease"); err != nil {
		return fmt.Errorf("push: %w", err)
	}
	return nil
}

// currentBranch returns the name of the current branch.
func currentBranch() (string, error) {
	out, err := gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	if out == "HEAD" {
		return "", fmt.Errorf("HEAD is detached (no current branch)")
	}
	return out, nil
}

// remoteExists reports whether a git remote with the given name exists.
func remoteExists(name string) bool {
	out, err := gitOutput("remote")
	if err != nil {
		return false
	}
	for _, r := range strings.Fields(out) {
		if r == name {
			return true
		}
	}
	return false
}

// defaultBranchForRemote returns the default branch of the given remote
// (e.g. "main", "master") by querying the remote's HEAD ref.
func defaultBranchForRemote(remote string) (string, error) {
	out, err := gitOutput("symbolic-ref", "refs/remotes/"+remote+"/HEAD")
	if err != nil {
		return "", fmt.Errorf("determine default branch for %s: %w", remote, err)
	}
	// Output: refs/remotes/<remote>/<branch>
	branch := out
	if idx := strings.LastIndex(out, "/"); idx >= 0 {
		branch = out[idx+1:]
	}
	return branch, nil
}

// rebaseUpstream determines the upstream ref to rebase onto and runs the
// rebase. It fetches the target remote first.
//
// Logic:
//  1. Find the current branch's configured upstream (e.g. origin/main).
//  2. If upstream exists, fetch that remote and rebase onto upstream ref.
//  3. If no upstream, fall back to the current remote's default branch.
func rebaseUpstream() error {
	branch, err := currentBranch()
	if err != nil {
		return err
	}

	// Try configured upstream for the current branch.
	upstream, _ := gitOutput("rev-parse", "--abbrev-ref", branch+"@{u}")

	if upstream != "" {
		// upstream is like "origin/main"
		parts := strings.SplitN(upstream, "/", 2)
		remote := parts[0]
		if len(parts) < 2 {
			return fmt.Errorf("unexpected upstream format: %s", upstream)
		}

		if !remoteExists(remote) {
			return fmt.Errorf("upstream remote %q does not exist", remote)
		}

		fmt.Fprintf(os.Stderr, "Fetching %s...\n", remote)
		if err := gitRun("fetch", remote); err != nil {
			return fmt.Errorf("fetch %s: %w", remote, err)
		}

		fmt.Fprintf(os.Stderr, "Rebasing %s onto %s...\n", branch, upstream)
		return gitRun("rebase", upstream)
	}

	// No upstream configured. Fall back to current remote's default branch.
	remote := "origin"
	if !remoteExists(remote) {
		// Pick the first available remote.
		out, err := gitOutput("remote")
		if err != nil || out == "" {
			return fmt.Errorf("no remotes configured")
		}
		remote = strings.Fields(out)[0]
	}

	defBranch, err := defaultBranchForRemote(remote)
	if err != nil {
		return err
	}

	target := remote + "/" + defBranch
	fmt.Fprintf(os.Stderr, "Fetching %s...\n", remote)
	if err := gitRun("fetch", remote); err != nil {
		return fmt.Errorf("fetch %s: %w", remote, err)
	}

	fmt.Fprintf(os.Stderr, "Rebasing %s onto %s...\n", branch, target)
	return gitRun("rebase", target)
}

// rebasePR checks out the PR branch via `gh`, rebases it onto the upstream
// default branch, then returns to the original branch.
func rebasePR(number int) error {
	origBranch, err := currentBranch()
	if err != nil {
		return err
	}

	// Find the PR's head branch name.
	out, err := gitOutput("gh", "pr", "view", strconv.Itoa(number), "--json", "headRefName")
	if err != nil {
		return fmt.Errorf("gh pr view %d: %w", number, err)
	}
	branch := parseJSONField(out, "headRefName")
	if branch == "" {
		return fmt.Errorf("could not determine PR branch name from gh output")
	}

	// Determine where to return if something goes wrong.
	defer func() {
		if cur, err := currentBranch(); err == nil && cur != origBranch {
			fmt.Fprintf(os.Stderr, "Returning to %s...\n", origBranch)
			_ = gitRun("checkout", origBranch)
		}
	}()

	fmt.Fprintf(os.Stderr, "Checking out PR #%d (%s)...\n", number, branch)
	if err := gitRun("checkout", branch); err != nil {
		return fmt.Errorf("checkout %s: %w", branch, err)
	}

	fmt.Fprintln(os.Stderr, "Rebasing onto upstream...")
	if err := rebaseUpstream(); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "Pushing...")
	if err := gitRun("push", "--force-with-lease"); err != nil {
		return fmt.Errorf("push: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Returning to %s...\n", origBranch)
	if err := gitRun("checkout", origBranch); err != nil {
		return fmt.Errorf("return to %s: %w", origBranch, err)
	}

	return nil
}

// parseJSONField extracts a string field value from minimal JSON like
// {"headRefName":"feat/x"}. It avoids pulling in encoding/json for a single
// field and tolerates extra fields.
func parseJSONField(json, field string) string {
	key := `"` + field + `":"`
	idx := strings.Index(json, key)
	if idx < 0 {
		return ""
	}
	start := idx + len(key)
	end := strings.Index(json[start:], `"`)
	if end < 0 {
		return ""
	}
	return json[start : start+end]
}
