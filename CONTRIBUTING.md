# Contributing to loom

Thank you for your interest in contributing to loom! This guide will help you get started.

## Getting Started

### Prerequisites

- [Go 1.26+](https://go.dev/dl/)
- A terminal emulator with modern capabilities (kitty, ghostty, alacritty, etc.)

### Setup

```bash
git clone https://github.com/floatpane/loom.git
cd loom
go mod tidy
```

### Build & Run

```bash
go build -o loom .    # builds to ./loom
./loom <file-path>    # run loom
```

### Testing

```bash
go test -v ./...      # run all tests with verbose output
```

### Linting

```bash
gofmt -l .            # check formatting
go vet ./...          # run go vet
```

## Making Changes

### Branch Naming

Create a branch from `master` using one of these prefixes:

- `feature/` — new functionality
- `fix/` — bug fixes
- `docs/` — documentation changes
- `refactor/` — code restructuring without behavior changes

### Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/). Format your commit messages as:

```
type(scope): short description
```

Common types: `feat`, `fix`, `docs`, `test`, `ci`, `chore`, `refactor`, `perf`.

Keep commit messages to 40 characters or fewer.

Examples:
```
feat(suggest): add co-reviewed-by trailer
fix(editor): preserve cursor position on accept
docs: update installation instructions
```

### Before Submitting a PR

1. Run `gofmt -l .` and fix any issues.
2. Run `go test ./...` and make sure all tests pass.
3. Run `go vet ./...` and fix any warnings.
4. Keep your PR focused — one logical change per PR.
5. Write a clear PR description explaining **what** changed and **why**.

## Reporting Bugs

Open an issue with:

- Steps to reproduce the issue
- Expected vs. actual behavior
- Your OS, terminal emulator, and loom version

## Requesting Features

Open an issue with a clear description of the problem you're trying to solve and your proposed solution.

## AI Policy

We welcome contributions that use AI-assisted tools as part of the development process. That said, contributors are fully responsible for any code they submit, regardless of how it was written.

**What we expect:**

- **Understand what you submit.** You should be able to explain every line of your PR.
- **Review AI output carefully.** AI tools can produce plausible-looking code that is subtly wrong or doesn't match the project's patterns. Treat AI suggestions the same way you'd treat a Stack Overflow snippet — verify before committing.
- **Don't submit AI-generated issues, reviews, or comments.** Discussions should be genuine human communication.
- **No AI-generated tests that don't actually test anything.** Tests must be meaningful and actually validate behavior.

**What we won't accept:**

- Bulk PRs of AI-generated refactors or "improvements" that weren't requested.
- Code that introduces hallucinated dependencies, APIs, or patterns that don't exist in the project.
- Contributions where the author clearly doesn't understand the changes they're proposing.

The goal is simple: AI is a tool. Use it well, take ownership of the output, and make sure your contribution actually improves the project.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold a welcoming and respectful environment for everyone.
