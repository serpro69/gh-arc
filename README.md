# gh-arc

`gh-arc` is a [github cli extension](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions#about-github-cli-extensions) that implements an opinionated [trunk-based development workflow](https://martinfowler.com/articles/branching-patterns.html#Trunk-basedDevelopment) with Github, built upon the great traditions of tools like [phacility/arcanist](https://github.com/phacility/arcanist/tree/master) and [phorge/arcanist](https://github.com/phorgeit/arcanist.git).

> Check out this [DORA Trunk-based development](https://dora.dev/capabilities/trunk-based-development/) page if, by any chance, this is the first time you're hearing about it and interested to learn more about about this methodology.

## Installation

Install the extension using the GitHub CLI:

```bash
gh extension install serpro69/gh-arc
```

## Authentication

`gh-arc` requires GitHub CLI authentication with specific OAuth scopes to access user information and repository data. The required scopes are:

- `user:email` - Access to user email addresses
- `read:user` - Read user profile data

### Setting up authentication

**If you're already logged in to GitHub CLI**, refresh your token with the required scopes:

```bash
gh auth refresh --scopes "user:email,read:user"
```

**If you're not logged in**, authenticate with the required scopes:

```bash
gh auth login --scopes "user:email,read:user"
```

**Alternatively**, you can provide your own personal access token:

```bash
echo "your-token-here" | gh auth login --with-token
```

> **Note**: Personal access tokens must include the `user:email` and `read:user` scopes to work with `gh-arc`.

To verify your authentication and scopes:

```bash
gh arc auth
```

## Overview

`gh-arc` is a "wrapper" that sits on top of other tools: Github (naturally, being a github-cli extension) and Linear (if you use it for issue management instead of Github), but also linters, formatters, unit test frameworks and others. It provides a simple command-line API to manage code review and some related revision control operations.

For a detailed list of all available commands, run:

```bash
gh arc help
```

For detailed information about a specific command, run:

```bash
gh arc help <command>
```

In a gist, `gh arc` allows you to do things like:

- get detailed help about available commands with `gh arc help`
- create a new, short-lived, feature branch from an up-to-date `origin/HEAD` with `gh arc work`
- send your code to Github for review with `gh arc diff`
- show pending revision information with `gh arc list`
- find likely reviewers for a change with `gh arc cover`
- apply changes in a revision to the working copy with `gh arc patch`
- download a patch from Github with `gh arc export`
- update Git commit messages after review with `gh arc amend`
- push changes with `gh arc land`
- view enhanced information about Git branches with `gh arc branch`

Once you've [configured lint and unit test integration](TODO), you can also:

- check your code for syntax and style errors with `gh arc lint`
- run unit tests that cover your changes with `gh arc unit`

This extension also integrates with other tools:

- create and view github gists with `gh arc gist`

It has some advanced features as well, you can:

- activate tab completion with `gh arc shell-complete`
- ...or extend the extension and add new commands
