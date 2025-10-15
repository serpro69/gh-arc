# Contributing to gh-arc

Thank you for your interest in contributing to `gh-arc`! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Code Style and Standards](#code-style-and-standards)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Additional Resources](#additional-resources)

## Getting Started

`gh-arc` is a GitHub CLI extension written in Go that implements a trunk-based development workflow. Before contributing, you should:

1. **Understand the domain**: Familiarize yourself with trunk-based development and code review workflows
2. **Read the documentation**: Review the [README.md](README.md) to understand what `gh-arc` does
3. **Explore the codebase**: Look at the project structure and existing code

### Prerequisites

- **Go 1.23.4+**: The project requires Go 1.23.4 or later
- **GitHub CLI (`gh`)**: Install from https://cli.github.com/
- **Git**: Version control system
- **Basic understanding of**:
  - Go programming language
  - GitHub API and pull request workflows
  - Git operations (branches, commits, diffs, rebasing)
  - CLI tool development

## Development Setup

### 1. Fork and Clone

```bash
# Fork the repository on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/gh-arc.git
cd gh-arc
```

### 2. Install Dependencies

```bash
# Download Go dependencies
go mod download

# Verify everything compiles
go build -o gh-arc
```

### 3. Install as Local Extension

```bash
# Install the extension locally for testing
gh extension install .
```

### 4. Verify Setup

```bash
# Check that the extension is installed
gh extension list

# Run the extension
gh arc version

# Check authentication
gh arc auth
```

### 5. Set Up GitHub Authentication

```bash
# If not already authenticated, login with required scopes
gh auth login --scopes "user:email,read:user"

# Or refresh existing authentication
gh auth refresh --scopes "user:email,read:user"
```

## Project Structure

```
gh-arc/
├── cmd/                    # Cobra command definitions
│   ├── root.go            # Root command and global flags
│   ├── auth.go            # Authentication verification
│   ├── diff.go            # PR creation/update command
│   ├── list.go            # PR listing command
│   ├── version.go         # Version information
│   └── *_test.go          # Command tests
│
├── internal/              # Internal packages (not importable by other projects)
│   ├── cache/            # HTTP response caching with TTL
│   ├── codeowners/       # CODEOWNERS file parsing
│   ├── config/           # Configuration management (Viper)
│   ├── diff/             # Diff output formatting
│   ├── filter/           # PR filtering logic
│   ├── format/           # PR table formatting
│   ├── git/              # Git operations (go-git)
│   ├── github/           # GitHub API client (go-gh)
│   │   ├── client.go     # Base client with auth
│   │   ├── pullrequest.go # PR operations
│   │   ├── ratelimit.go  # Rate limit handling
│   │   ├── retry.go      # Retry logic with backoff
│   │   ├── cache.go      # Response caching
│   │   └── errors.go     # Error types
│   ├── lint/             # Linting support
│   ├── logger/           # Structured logging (zerolog)
│   ├── template/         # PR template parsing
│   └── version/          # Version management
│
├── main.go               # Entry point
├── go.mod                # Go module definition
├── go.sum                # Dependency checksums
├── README.md             # User documentation
├── CLAUDE.md             # AI assistant instructions
└── CONTRIBUTING.md       # This file
```

### Key Files to Know

- **`cmd/root.go`**: Defines the root command, global flags, and initialization logic
- **`internal/github/client.go`**: GitHub API client wrapper with rate limiting and caching
- **`internal/git/git.go`**: Git repository operations using go-git
- **`internal/config/config.go`**: Configuration loading and management with Viper
- **`internal/logger/logger.go`**: Centralized logging configuration

## Development Workflow

### TDD (Test-Driven Development)

We follow TDD principles:

1. **Write a failing test** that describes the desired behavior
2. **Write minimal code** to make the test pass
3. **Refactor** the code while keeping tests green
4. **Repeat** for the next feature

### DRY (Don't Repeat Yourself)

- Extract common functionality into helper functions or packages
- Reuse existing utilities before creating new ones
- Look for patterns and abstract them appropriately

### YAGNI (You Aren't Gonna Need It)

- Implement only what's needed for current requirements
- Don't add speculative features or over-engineer solutions
- Keep code simple and focused

### Frequent Commits

- Commit often with clear, descriptive messages
- Each commit should represent a logical unit of work
- Use conventional commit format: `type(scope): description`
  - Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`
  - Example: `feat(diff): add PR stacking support`

### Typical Development Flow

```bash
# 1. Create a feature branch from master
git checkout -b feature/my-feature

# 2. Write tests first (TDD)
# Create or modify *_test.go files

# 3. Run tests (they should fail)
go test ./...

# 4. Implement the feature
# Edit source files

# 5. Run tests until they pass
go test ./...

# 6. Run all tests and checks
go test ./...
go vet ./...
go fmt ./...

# 7. Commit your changes
git add .
git commit -m "feat(scope): add feature description"

# 8. Push and create a PR
git push origin feature/my-feature
gh pr create
```

## Code Style and Standards

### Go Code Style

- Follow **official Go style guidelines**: https://go.dev/doc/effective_go
- Use **`gofmt`** for formatting: `go fmt ./...`
- Use **`go vet`** for static analysis: `go vet ./...`
- Write **godoc comments** for all exported functions, types, and packages

### Naming Conventions

- **Packages**: lowercase, single word (e.g., `github`, `config`)
- **Files**: lowercase with underscores (e.g., `pull_request.go`)
- **Types**: PascalCase (e.g., `GitClient`, `PROptions`)
- **Functions/Methods**: camelCase or PascalCase depending on visibility
- **Constants**: PascalCase or UPPER_CASE for exported/private
- **Test files**: `*_test.go` (e.g., `client_test.go`)

### Error Handling

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to fetch PR: %w", err)
}

// Good: Define custom error types for specific cases
var ErrNotFound = errors.New("resource not found")

// Good: Check error types
if errors.Is(err, ErrNotFound) {
    // Handle not found
}
```

### Logging

```go
// Use the centralized logger from internal/logger
logger := logger.Get()

// Different log levels
logger.Debug().Msg("detailed debugging information")
logger.Info().Msg("general information")
logger.Warn().Msg("warning message")
logger.Error().Err(err).Msg("error occurred")

// Add context with fields
logger.Info().
    Str("repo", "owner/repo").
    Int("pr_number", 123).
    Msg("processing PR")
```

### Configuration

```go
// Access configuration through viper
createAsDraft := viper.GetBool("diff.createAsDraft")
defaultBranch := viper.GetString("github.defaultBranch")

// Provide defaults
branch := viper.GetString("github.defaultBranch")
if branch == "" {
    branch = "main"
}
```

## Testing

### Test Organization

- Place tests in `*_test.go` files alongside the code they test
- Use table-driven tests for multiple similar test cases
- Write tests for both happy paths and error cases
- Aim for high test coverage (>80% for critical packages)

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests for a specific package
go test ./internal/github

# Run a specific test
go test -run TestClientNew ./internal/github

# Run tests with verbose output
go test -v ./...
```

### Writing Tests

See [docs/contributing/TESTING.md](docs/contributing/TESTING.md) for detailed testing guidelines and examples.

## Pull Request Process

### Before Submitting

- [ ] **Write tests** for your changes
- [ ] **Run all tests**: `go test ./...`
- [ ] **Run go vet**: `go vet ./...`
- [ ] **Format code**: `go fmt ./...`
- [ ] **Update documentation** if you changed functionality
- [ ] **Test manually** with `gh arc` commands
- [ ] **Commit frequently** with clear messages

### PR Guidelines

1. **Title**: Use conventional commit format
   - Example: `feat(diff): add support for Linear integration`

2. **Description**: Include:
   - What changed and why
   - How to test the changes
   - Links to related issues
   - Screenshots (if UI-related)

3. **Size**: Keep PRs focused and reasonably sized
   - Prefer smaller, incremental PRs over large rewrites
   - Split large features into multiple PRs if possible

4. **Tests**: All PRs must include tests
   - New features require new tests
   - Bug fixes require tests that would have caught the bug

5. **Documentation**: Update docs if needed
   - README.md for user-facing changes
   - Code comments for complex logic
   - CONTRIBUTING.md for development process changes

### PR Review Process

1. **Submit PR**: Create a pull request with a clear description
2. **CI Checks**: Ensure all automated checks pass
3. **Code Review**: Maintainers will review your code
4. **Address Feedback**: Make requested changes
5. **Approval**: Once approved, a maintainer will merge your PR

### After Merging

- Delete your feature branch
- Update your fork's master branch
- Celebrate! 🎉

## Additional Resources

### Detailed Guides

- [Architecture Guide](docs/contributing/ARCHITECTURE.md) - Detailed explanation of the codebase architecture
- [Testing Guide](docs/contributing/TESTING.md) - Comprehensive testing patterns and examples
- [Workflows Guide](docs/contributing/WORKFLOWS.md) - Common development tasks and workflows
- [Troubleshooting Guide](docs/contributing/TROUBLESHOOTING.md) - Debugging and problem-solving

### External Resources

- [Go Documentation](https://go.dev/doc/)
- [Effective Go](https://go.dev/doc/effective_go)
- [GitHub CLI Extensions](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions)
- [go-gh Library](https://github.com/cli/go-gh)
- [go-git Library](https://github.com/go-git/go-git)
- [Cobra CLI Framework](https://github.com/spf13/cobra)
- [Trunk-Based Development](https://trunkbaseddevelopment.com/)

### Getting Help

- **Issues**: Open an issue for bugs or feature requests
- **Discussions**: Use GitHub Discussions for questions
- **Code Review**: Ask questions in PR comments

## Code of Conduct

- Be respectful and constructive
- Welcome newcomers and help them learn
- Focus on what is best for the community
- Show empathy towards other contributors

Thank you for contributing to `gh-arc`!
