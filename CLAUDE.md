# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`gh-arc` is a GitHub CLI extension implementing an opinionated trunk-based development workflow. It is a port of a famous CLI tool [phorgeit/arcanist](https://github.com/phorgeit/arcanist) that was originally developed as as command-line tool for interacting with Phorge, with the main difference that `gh-arc` targets Github as a code-hosting and code-review platform. It wraps GitHub (and potentially Linear) to provide a simplified command-line API for code review and revision control operations. The main motivation is to enable developers to work within the environments (command line interface) they are familiar with during the entire development workflow, without switching contexts or opening external tools, browsers, and so on for code-review processes.

> Phorge (pronounced like the word forge) is a suite of web applications which make it easier to build software, particularly when working with teams. Phorge is a fork of Phabricator, which in turn is largely based on Facebook's internal tools.
>
> The major components of Phorge are:
>
> - Differential - a code review tool (similar to Github Pull Requests)
> - Diffusion    - a repository browser (similar to Github repositories)
> - Maniphest    - a bug tracker (similar to Github Issues)
> - Phriction    - a wiki (similar to Github Wiki)
> - Paste        - standalone code-pastes (similar to Github Gists)

The `gh-arc` extension is written in Go and uses the `github.com/cli/go-gh/v2` library to interact with GitHub's API.

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

See @./docs/contributing/ARCHITECTURE.md for an architecture overview.

### Planned Features

Based on @./README.md, the extension will implement commands for:

- `gh arc work`: Create a new, short-lived, feature branch from an up-to-date origin/HEAD
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
- [Arcanist User Guide](https://we.phorge.it/book/phorge/article/arcanist/). Also as raw documentation files:
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_commit_ranges.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_coverage.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_diff.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_extending_lint.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_lint.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_lint_script_and_regex.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_lint_unit.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_mac_os_x.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_new_project.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_quick_start.diviner
    - https://raw.githubusercontent.com/phorgeit/phorge/refs/heads/master/src/docs/user/userguide/arcanist_windows.diviner
- [Arcanist CLI Repo](https://github.com/phorgeit/arcanist)
- [Trunk-based Development](https://martinfowler.com/articles/branching-patterns.html#Trunk-basedDevelopment)

## Task Master AI Instructions

**IMPORTANT!!! Import Task Master's development workflow commands and guidelines, treat as if import is in the main CLAUDE.md file.**

@./.taskmaster/CLAUDE.md
