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

> [!WARNING]
> This project was created with the help of Claude-Code and I'm using it as a "pet project" to try out different CC workflows. That said, I try (yeah, I know, "do or do not...", but you know, our desires don't always conicide with our possibiltiies) not to compromise on quality and testing. I also use this extension daily when working with github code reviews. That said, if you have any problems with code that is written by AI - you've been warned.

## Installation

Install the latest release using the GitHub CLI:

```bash
gh extension install serpro69/gh-arc
```

### Installation from Source

```bash
git clone https://github.com/serpro69/gh-arc.git
cd gh-arc && go build . && gh extension install .
```

> [!TIP]
> Clone the repo to a permanent location, gh cli will alias the binary when you run `gh extension install .` command.

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

Once you've [configured lint and unit test integration](#lint-and-unit-tests), you can also:

- check your code for syntax and style errors with `gh arc lint`
- run all configured test runners with `gh arc unit`

This extension also integrates with other tools:

- create and view github gists with `gh arc gist`

It has some advanced features as well, you can:

- enable tab completion with `gh arc completion`
- ...or extend the extension and add new commands

### Lint and Unit Tests

Configure the tools your project uses under [`lint.runners`](#lint-runners) and
[`test.runners`](#test-runners), then run:

```bash
gh arc lint             # Lint files changed in commits on this branch
gh arc lint --all       # Let each runner select its full target set
gh arc lint --fix       # Append fixArgs for runners with autoFix enabled
gh arc unit             # Run all configured tests
gh arc unit --json      # Emit execution metadata as one JSON object
```

`lint` compares `HEAD` with the merge-base of the default branch. It excludes
deleted files; staged, unstaged, and untracked changes are not part of that file
selection. Tools read the current working-tree contents of selected files. Use
`--all` to check uncommitted work as part of the runner's full target set. There
is no file-extension filtering: configure each runner's `workingDir` or use a
wrapper when different tools need different file types.

`lint` requires a Git repository and uses local refs without fetching. It tries
`origin/<defaultBranch>`, then the local branch if that remote ref is missing.
Set `github.defaultBranch` explicitly in detached CI checkouts or repositories
without a cached `origin/HEAD` or a common local default branch. Otherwise the
autodetection fallback can select the first local branch, including the feature
branch itself. A missing base ref is an error suggesting `git fetch origin`;
unrelated or shallow local history with no common ancestor falls back to all
files. Fetch the needed history first when changed-file scoping matters.

`unit` does not scope tests to changed files and also works outside Git. Within a
repository, both commands resolve empty or relative runner working directories
from the repository root, including when invoked from a subdirectory. Outside
Git, `unit` uses the invocation directory. Absolute working directories are kept.

Runners execute sequentially, including after another runner fails. Normal mode
streams tool output and prints a summary. Exit status is `0` when every executed
runner passes or there is nothing to run, and `1` for runner or setup failures.
No configured runners prints guidance and succeeds. Lint also succeeds without
running tools when there are no eligible changed files.

`--quiet` suppresses guidance, banners, successful summaries, and tool stdout.
Tool stderr and gh-arc's failure summaries remain visible. Findings or test
reports that a tool prints to stdout are suppressed even on failure; rerun
without `--quiet` to see them. `--json` takes precedence over quiet mode and
still emits the result object.

With `--json`, runner output, banners, guidance, and summaries are suppressed.
Execution results contain metadata, not the tools' lint findings or test reports:

```json
{
  "command": "unit",
  "success": true,
  "runners": [
    { "name": "go-test", "status": "passed", "exit_code": 0, "duration_ms": 320 }
  ]
}
```

An empty configuration produces `"runners":[]`. A lint skip also includes
`"skipped":"no changed files"` or
`"skipped":"no changed files in runner working directories"`. Runner statuses
are `passed`, `failed`, or `error`; startup failures and timeouts include a
per-runner `error` message. A zero `exit_code` on an `error` result does not mean
success: no normal exit status may be available. Use `status` and `success`.
JSON mode discards the underlying tools' output even if they emit JSON; configure
those tools to write report files when you need their findings.

Setup, invalid configuration, and argument errors also return one JSON object,
with exit status `1` and empty stderr:

```json
{"command":"unit","success":false,"runners":[],"error":"invalid timeout ..."}
```

Lint/unit reject invalid configuration before running tools. `output.json: true`
also enables this envelope when configuration can be loaded; pass `--json`
explicitly for malformed or unreadable config files. `--json=false` restores
ordinary error output. Help remains text. If a stdout write returns an error,
the command reports it on stderr instead of appending another JSON object.
Termination by a signal, such as a closed output pipe, can prevent diagnostics.
Errors before a valid command is identified (for example,
`gh arc --unknown unit --json`) retain ordinary top-level CLI diagnostics.

These commands do not call GitHub or require GitHub authentication themselves;
configured tools can still use the network. MegaLinter/Docker execution,
parallel runners, unit coverage flags, and `--dry-run` are not implemented.

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

1. Repository root: `./.arc.json`, `./.arc.yaml`, or `./.arc.yml` (resolved from the git repository root, so it works from any subdirectory)
2. User config: `~/.config/gh-arc/.arc.json`, `~/.config/gh-arc/.arc.yaml`, or `~/.config/gh-arc/.arc.yml`
3. System-wide: `/etc/gh-arc/.arc.json`, `/etc/gh-arc/.arc.yaml`, or `/etc/gh-arc/.arc.yml`

Within each directory, JSON files take precedence over YAML files (`.arc.json` > `.arc.yaml` > `.arc.yml`).

The first existing file is used; configuration files are not merged. A malformed
or unreadable file is an error, not a reason to try a lower-priority file. Outside
a Git repository, the first search location is the invocation directory.

### Configuration Format

A [JSON Schema](docs/arc.schema.json) is available for editor autocompletion and validation. For `.arc.json`, add the `$schema` key as shown below. For YAML, configure your editor's YAML language server to associate the schema with `.arc.yaml`/`.arc.yml` files.

Create a `.arc.json` or `.arc.yaml` file in your project root or user config directory:

**JSON format (`.arc.json`):**

```json
{
  "$schema": "https://raw.githubusercontent.com/serpro69/gh-arc/main/docs/arc.schema.json",
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
    "requireApproval": "strict",
    "requireCI": "required"
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
  requireApproval: strict
  requireCI: required

output:
  verbose: false
  quiet: false
  json: false
  color: true
```

### Configuration Options

#### GitHub Settings

- **`github.defaultBranch`** (string, default: `""`): Repository default branch override; empty allows autodetection
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

- **`land.defaultMergeMethod`** (string, default: `"squash"`): Default merge method (`squash` or `rebase`)
- **`land.deleteLocalBranch`** (bool, default: `true`): Delete local branch after landing
- **`land.requireApproval`** (string, default: `"strict"`): Approval check mode — `"strict"` (block, `--force` to bypass), `"prompt"` (interactive confirmation), `"none"` (skip)
- **`land.requireCI`** (string, default: `"required"`): CI check mode — `"required"` (only branch-protection required checks), `"all"` (every check must pass), `"none"` (skip)

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
export GHARC_LAND_DEFAULTMERGEMETHOD=rebase
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

`gh arc unit` runs every entry in order. `name` labels its results, `command` is
the executable, and `args` is an array of literal arguments. The command is not
interpreted by a shell, so shell expansion, pipes, and redirection require an
explicit wrapper script. Tools must already be installed and available on `PATH`
(or specified by executable path).

`workingDir` defaults to the project root (invocation directory outside Git).
`timeout` accepts Go durations such as `30s` or `5m`; omitted, zero, and negative
durations currently disable the timeout. Use a positive duration to bound a run.
An invalid duration is an error. A timeout stops the direct runner process;
wrapper scripts are responsible for any child-process cleanup.

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
        "autoFix": true,
        "fixArgs": ["--fix"],
        "timeout": "5m"
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

Lint runners use the same `name`, `command`, `args`, `workingDir`, and `timeout`
fields as test runners. The arguments are assembled in this order:
`args`, then `fixArgs` when both `--fix` and `autoFix: true` are set, then selected
file paths. `autoFix` defaults to `false`; `fixArgs` defaults to an empty array.
Merely setting `autoFix: true` does not add tool-specific fix flags.

Changed paths are relative to each runner's working directory. Files outside it
are omitted, and a runner with no remaining files is not executed. With `--all`,
no file paths are appended: configure the runner's arguments to select its full
target set. For package-oriented tools such as the Go example above, use
`gh arc lint --all` when mixing explicit filenames would not be supported by the
tool. For a file-oriented linter, use an empty `args` array or options without a
fixed target; adding `.` to `args` can make it scan everything even in changed-file
mode.

The `megaLinter` settings are reserved for future integration. Enabling them
currently prints a notice when no explicit lint runners are configured; it does
not launch Docker or perform linting.

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
