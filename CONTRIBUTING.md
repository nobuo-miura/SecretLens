# Contributing to SecretLens

Thank you for your interest in contributing! Contributions of all kinds are welcome: bug reports, detection rules, documentation, and code.

## Getting Started

```bash
git clone https://github.com/nobuo-miura/SecretLens.git
cd SecretLens
make build   # builds bin/secretlens
go test ./...
```

Requirements:

- Go (version pinned in `go.mod`)
- `golangci-lint` for linting (`make lint` if available, or `golangci-lint run`)

## Development Workflow

1. Fork the repository and create a branch from `main`.
2. Make your changes. Please:
   - Run `gofmt` (or rely on your editor's Go tooling).
   - Run `golangci-lint run` and fix any new warnings.
   - Add or update tests for behavior changes.
3. Run the full test suite: `go test ./...`
4. Open a pull request against `main` with a clear description of the change and its motivation.

## Adding Detection Rules

Built-in rules live in [rules/](rules/). When adding a rule:

- Include realistic **positive** and **negative** test fixtures (use clearly fake secrets, e.g. `AKIAIOSFODNN7EXAMPLE`-style placeholders — never real credentials).
- Keep regexes anchored and specific enough to avoid noisy false positives.
- Set an appropriate severity.

## Reporting Bugs / Requesting Features

Use the issue templates. For **security vulnerabilities**, follow [SECURITY.md](SECURITY.md) instead of opening a public issue.

## Commit Messages

Write commit messages in English, in imperative mood (e.g. `Add Slack webhook rule`, `Fix entropy threshold off-by-one`).

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE.md).
