# gh-arc

<div markdown="1" style="text-align: center; font-size: 1.2em;">

<b>🚧 Fork in progress, expect some dust 🚧</b>

[![github-tag](https://img.shields.io/github/v/tag/serpro69/gh-arc?style=for-the-badge&logo=semver&logoColor=white)](https://github.com/serpro69/gh-arc/tags)
[![github-license](https://img.shields.io/github/license/serpro69/gh-arc?style=for-the-badge&logo=unlicense&logoColor=white)](https://opensource.org/license/mit)
[![github-stars](https://img.shields.io/github/stars/serpro69/gh-arc?logo=github&logoColor=white&color=gold&style=for-the-badge)](https://github.com/serpro69/gh-arc)

</div>

`gh-arc` is a [github cli extension](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions#about-github-cli-extensions) that implements an opinionated [trunk-based development workflow](https://martinfowler.com/articles/branching-patterns.html#Trunk-basedDevelopment) with Github, built upon the great traditions of tools like [phacility/arcanist](https://github.com/phacility/arcanist/tree/master) and [phorge/arcanist](https://github.com/phorgeit/arcanist.git).

> [!TIP]
> Check out this [DORA Trunk-based development](https://dora.dev/capabilities/trunk-based-development/) page if, by any chance, this is the first time you're hearing about it and interested to learn more about about this methodology.

## Installation

Install the extension using the GitHub CLI:

```bash
gh extension install serpro69/gh-arc
```

## Usage

### Authentication

`gh-arc` requires GitHub CLI authentication with specific OAuth scopes to access user information and repository data. The required _additional_ scopes are:

- `user:email` - Access to user email addresses
- `read:user` - Read user profile data

#### Setting up authentication

**If you're already logged in to GitHub CLI**, refresh your token with the additional required scopes:

```bash
gh auth refresh --scopes "user:email,read:user"
```

**If you're not logged in**, authenticate with the additional required scopes:

```bash
gh auth login --scopes "user:email,read:user"
```

**Alternatively**, you can provide your own personal access token:

```bash
echo "your-token-here" | gh auth login --with-token
```

> [!NOTE]
> Personal access tokens must include the `user:email` and `read:user` scopes, in addition to the scopes needed by `gh` cli itself, to work with `gh-arc`.

To verify your authentication and scopes:

```bash
gh arc auth
```

### Commands Overview

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

- enable tab completion with `gh arc completion`
- ...or extend the extension and add new commands

### Shell Completion

Github CLI does not currently support shell completion for extensions (See [cli/cli#5309](https://github.com/cli/cli/issues/5309)), but even if it did, who wants to type `gh arc <TAB> <TAB> <TAB>` ...that's just too much work if you ask me 🤷

So we've come up with a "simple as a rock" (is that a thing? 🤔) solution to both of the above problems 💡

- Find out where github cli installs extensions on your machine. On Linux (and maybe on Mac), it's usually under `~/.local/share/gh/extensions`. 

- Create a simple `arc` shell script file with the following contents:

    ```bash
    #!/usr/bin/env bash
    # Simple wrapper to enable completions for 'gh-arc' extension

    gh arc "$@"
    ```

- Make it executable and place it on your `PATH`.

- Get the completion script for your shell from the [releases](https://github.com/serpro69/gh-arc/releases/latest) page. Don't forget to source it or whatever, you know what to do.

- Then try typing `arc <TAB>` ...you should see the command completion for the extension; and as an added bonus, you've shortened the overall `gh ...` command as well... 🤯 Who said ingenious can't be simple?

- Profit... ⏱️

## Configuration

`gh-arc` can be configured using either JSON or YAML configuration files. The extension looks for configuration files in the following locations (in order of precedence):

1. Current directory: `./.arc.json`, `./.arc.yaml`, or `./.arc.yml`
2. User config: `~/.config/gh-arc/.arc.json`, `~/.config/gh-arc/.arc.yaml`, or `~/.config/gh-arc/.arc.yml`
3. System-wide: `/etc/gh-arc/.arc.json`, `/etc/gh-arc/.arc.yaml`, or `/etc/gh-arc/.arc.yml`

Within each directory, JSON files take precedence over YAML files (`.arc.json` > `.arc.yaml` > `.arc.yml`).

### Configuration Format

Create a `.arc.json` or `.arc.yaml` file in your project root or user config directory:

**JSON format (`.arc.json`):**

```json
{
  "github": {
    "defaultBranch": "main",
    "defaultReviewers": [],
    "autoAssignReviewer": false
  },
  "diff": {
    "createAsDraft": true,
    "autoUpdatePR": true,
    "includeCommitMessages": true,
    "enableStacking": true,
    "defaultBase": "",
    "showStackingWarnings": true,
    "templatePath": "",
    "requireTestPlan": true,
    "linearEnabled": false,
    "linearDefaultProject": ""
  },
  "land": {
    "defaultMergeMethod": "squash",
    "deleteLocalBranch": true,
    "deleteRemoteBranch": true,
    "requireApproval": true,
    "requireCI": true
  },
  "output": {
    "verbose": false,
    "quiet": false,
    "json": false,
    "color": true
  }
}
```

**YAML format (`.arc.yaml` or `.arc.yml`):**

```yaml
github:
  defaultBranch: main
  defaultReviewers: []
  autoAssignReviewer: false

diff:
  createAsDraft: true
  autoUpdatePR: true
  includeCommitMessages: true
  enableStacking: true
  defaultBase: ""
  showStackingWarnings: true
  templatePath: ""
  requireTestPlan: true
  linearEnabled: false
  linearDefaultProject: ""

land:
  defaultMergeMethod: squash
  deleteLocalBranch: true
  deleteRemoteBranch: true
  requireApproval: true
  requireCI: true

output:
  verbose: false
  quiet: false
  json: false
  color: true
```

### Configuration Options

#### GitHub Settings

- **`github.defaultBranch`** (string, default: `"main"`): The default base branch for new PRs
- **`github.defaultReviewer`** (string, default: `""`): Default reviewer to assign to PRs
- **`github.autoAssignReviewer`** (bool, default: `false`): Automatically assign the default reviewer to new PRs

#### Diff (PR Creation) Settings

- **`diff.createAsDraft`** (bool, default: `true`): Create PRs as drafts by default
- **`diff.autoUpdatePR`** (bool, default: `true`): Automatically update existing PRs when running `gh arc diff`
- **`diff.includeCommitMessages`** (bool, default: `true`): Include commit messages in PR description
- **`diff.enableStacking`** (bool, default: `true`): Enable automatic stacked PR detection
- **`diff.defaultBase`** (string, default: `""`): Force a specific base branch (empty = auto-detect)
- **`diff.showStackingWarnings`** (bool, default: `true`): Show warnings when dependent PRs exist
- **`diff.templatePath`** (string, default: `""`): Path to custom PR template (empty = use built-in)
- **`diff.requireTestPlan`** (bool, default: `true`): Require test plan in PR template
- **`diff.linearEnabled`** (bool, default: `false`): Enable Linear issue integration
- **`diff.linearDefaultProject`** (string, default: `""`): Default Linear project for issue references

#### Land (Merge) Settings

- **`land.defaultMergeMethod`** (string, default: `"squash"`): Default merge method (`squash`, `merge`, or `rebase`)
- **`land.deleteLocalBranch`** (bool, default: `true`): Delete local branch after landing
- **`land.deleteRemoteBranch`** (bool, default: `true`): Delete remote branch after landing
- **`land.requireApproval`** (bool, default: `true`): Require PR approval before landing
- **`land.requireCI`** (bool, default: `true`): Require CI checks to pass before landing

#### Output Settings

- **`output.verbose`** (bool, default: `false`): Enable verbose output
- **`output.quiet`** (bool, default: `false`): Suppress non-essential output
- **`output.json`** (bool, default: `false`): Output results in JSON format
- **`output.color`** (bool, default: `true`): Enable colored output

### Environment Variables

All configuration options can be overridden using environment variables with the `GHARC_` prefix. Use underscores to separate nested keys:

```bash
# Override diff.createAsDraft
export GHARC_DIFF_CREATEASDRAFT=false

# Override github.defaultBranch
export GHARC_GITHUB_DEFAULTBRANCH=master

# Override land.defaultMergeMethod
export GHARC_LAND_DEFAULTMERGEMETHOD=merge
```

### Advanced Configuration

#### Test Runners

Configure custom test runners in your `.arc.json`:

```json
{
  "test": {
    "runners": [
      {
        "name": "go-test",
        "command": "go",
        "args": ["test", "./..."],
        "workingDir": "",
        "timeout": "5m"
      }
    ]
  }
}
```

#### Lint Runners

Configure custom linters:

```json
{
  "lint": {
    "runners": [
      {
        "name": "golangci-lint",
        "command": "golangci-lint",
        "args": ["run"],
        "workingDir": "",
        "autoFix": true
      }
    ],
    "megaLinter": {
      "enabled": "auto",
      "config": ".mega-linter.yml",
      "fixIssues": false
    }
  }
}
```

### Example: Minimal Configuration

For most projects, you only need to override a few settings:

**JSON (`.arc.json`):**

```json
{
  "diff": {
    "createAsDraft": false,
    "requireTestPlan": false
  }
}
```

**YAML (`.arc.yaml`):**

```yaml
diff:
  createAsDraft: false
  requireTestPlan: false
```

This configuration:
- Creates PRs as ready-for-review by default
- Makes the test plan field optional

All other settings will use their default values.
