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

## Development Processes

!!! THIS SECTION IS **VERY IMPORTANT** AND **MUST BE FOLLOWED DURING ALL DEVELOPMENT WORK** !!!

### Task Analysis and Implementation Plan

**Goal: Before starting the implementation, make sure you understand the requirements and implementation plan.**

#### Ideas and Prototypes

_Use this for ideas that are not fully thought out and do not have a fully-formed PRD/design/specification/implementation-plan._

**For example:** I've got an idea I want to talk through with you before I proceed with the implementation.

**Your task:** Help me turn it into a fully formed design and spec, and eventually an implementation plan.

- Check out the current state of the project in our working directory to understand where we're starting off.
- Check the contributing guidelines and documentation @./CONTRIBUTING.md , @./docs/contributing/ARCHITECTURE.md , and @./docs/contributing/TESTING.md
- Then ask me questions, one at a time, to help refine the idea. 
- Ideally, the questions would be multiple choice, but open-ended questions are OK, too. Don't forget: only one question per message!
- Once you believe you understand what we're trying to achieve, stop and describe the whole design to me, in sections of 200-300 words at a time, asking after each section whether it looks right so far.
- Then document in .md files the entire design and write a comprehensive implementation plan in @./docs/wip/[feature-title]/{design,implementation}.md . Feel free to break out the design/implementation documents into multi-part files, if necessary.
- When writing documentation:
    - Assume the developer who is going to implement the feature is an experienced and highly-skilled Go-developer, but has zero context for our codebase, and knows almost nothing about or problem domain. Basically - a first-time contributor with a lot of programming experience.
    - Document everything the developer may need to know: which files to touch for each task, code structure to be aware of, testing approaches, any potential docs they might need to check. Give them the whole plan as bite-sized tasks.
    - Make sure the plan is unambiguous, as well as detailed and comprehensive so the developer can adhere to DRY, YAGNI, TDD, atomic/self-contained commits principles when following this plan.
- But of course, **DO NOT:**
    - **DO NOT add complete code examples**. The documentation should be a guideline that gives the developer all the information they may need when writing the actual code, not copy-paste code chunks.
    - **DO NOT add commit message templates** to tasks, that the developer should use when commiting the changes.
    - **DO NOT add other small, generic details that do not bring value** and/or are not specifically relevant to this particular feature. For example, adding something like "to run tests, execute: `go test ./...`" to a task does not bring value. Remember, the developer is experienced and skilled.

#### Existing task-master tasks

_For tasks that already exist in task-master._

**For example:** Let's work on task 6 next.

**Your task:** Make sure the task is well-documented and you understand the requirements and how to implement it. Then implement the task.

- Get the task from task-master
- Does it have linked documentation for the design and implementation plan?
    - **YES:**
        - Read the design and implementation documentation and understand what needs to be done and how.
        - Check the contributing guidelines and documentation @./CONTRIBUTING.md , @./docs/contributing/ARCHITECTURE.md , and @./docs/contributing/TESTING.md
        - Then proceed with implementing the task.
    - **NO:**
        - Follow the [Ideas and Prototypes](#ideas-and-prototypes) section.
        - Instead of creating a new task as the last step, update the existing task with necessary information.

### Working with Dependencies

- Always try to use latest versions for dependencies. To find dependencnes, run:

```bash
go list -m -versions <module>
```

- Before trying alternative methods, always try to use context7 MCP to lookup documentation for dependencies, libraries, SDKs, APIs and other external frameworks and tools. 
    - **IMPORTANT! Always make sure that documentation version is the same as declared dependency version itself.**
    - Only revert to web-search or other alternative methods if you can't find documentation in context7.

### Testing & Quality Assurance

- Always try to add tests for any new functionality, and make sure to cover all cases and code branches, according to requirements.
- Always try to add tests for any bug-fixes, if the discovered bug is not already covered by tests. If the bug was already covered by tests, fix the existing tests.
- Always run all tests after you are done with a given implementation

Use the following guidelines when working with tests:

- Comprehensive testing with testing package and testify
- Table-driven tests and test generation
- Benchmark tests and performance regression detection
- Integration testing with test containers
- Mock generation with mockery and gomock
- Property-based testing with gopter
- End-to-end testing strategies
- Code coverage analysis and reporting

### Documentation

- After completing a new feature, always see if you need to update the Architecture documentation @./docs/contributing/ARCHITECTURE.md and Test documentation @./docs/contributing/TESTING.md for other developers, so anyone could easily pick up the work and understand the project and the feature that was added.
- If the code change included prior decision-making out of several alternatives, document an ADR in @./docs/adr for any non-trivial/-obvious decisions that should be preserved.

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
