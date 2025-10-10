# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`gh-arc` is a GitHub CLI extension implementing an opinionated trunk-based development workflow, inspired by tools like Arcanist. It wraps GitHub (and potentially Linear) to provide a simplified command-line API for code review and revision control operations.

The extension is written in Go and uses the `github.com/cli/go-gh/v2` library to interact with GitHub's API.

## Development Commands

### Building
```bash
go build -o gh-arc
```

### Running
```bash
./gh-arc
# Or install as a gh extension and run:
gh arc <command>
```

### Testing
```bash
go test ./...
```

### Formatting
```bash
go fmt ./...
```

### Linting
```bash
go vet ./...
```

### Install Extension Locally
```bash
gh extension install .
```

## Architecture

### Current State
The project is in early development. Currently only contains:
- `main.go`: Entry point with basic GitHub API authentication check
- Compiled binary `gh-arc` (gitignored)
- Go module configuration

### Planned Features
Based on README.md, the extension will implement commands for:
- `gh arc diff`: Send code to GitHub for review
- `gh arc list`: Show pending revisions
- `gh arc cover`: Find reviewers for changes
- `gh arc patch`: Apply revision changes to working copy
- `gh arc export`: Download patches from GitHub
- `gh arc amend`: Update commit messages after review
- `gh arc land`: Push changes
- `gh arc branch`: View enhanced branch information
- `gh arc lint`: Check code syntax and style
- `gh arc unit`: Run unit tests for changes
- `gh arc gist`: Create and view GitHub gists
- `gh arc shell-complete`: Tab completion setup

### GitHub CLI Extension Requirements
- Must be named with `gh-` prefix
- Can be written in any language, compiled to a binary
- Uses `go-gh` library for GitHub API interactions
- Released using `cli/gh-extension-precompile` action for multi-platform builds

## Key Dependencies

- `github.com/cli/go-gh/v2`: Official Go library for GitHub CLI extensions
- Go 1.23.4+

## References

- [GitHub CLI Extensions Documentation](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions)
- [go-gh examples](https://github.com/cli/go-gh/blob/trunk/example_gh_test.go)
- [Trunk-based Development](https://martinfowler.com/articles/branching-patterns.html#Trunk-basedDevelopment)

## Task Master AI Instructions
**Import Task Master's development workflow commands and guidelines, treat as if import is in the main CLAUDE.md file.**
@./.taskmaster/CLAUDE.md
