# SecretLens

> An open-source secret scanning CLI for Go that scans **Git history, CI logs, environment files, and Docker layers**, then reports prioritized alerts.

[![CI](https://github.com/nobuo-miura/SecretLens/actions/workflows/ci.yml/badge.svg)](https://github.com/nobuo-miura/SecretLens/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.26-blue)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/nobuo-miura/SecretLens)](https://goreportcard.com/report/github.com/nobuo-miura/SecretLens)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE.md)

---

## Installation

### go install

```bash
go install github.com/nobuo-miura/SecretLens/cmd/secretlens@latest
```

### Build from source

```bash
git clone https://github.com/nobuo-miura/SecretLens.git
cd SecretLens
make build        # creates bin/secretlens
make install      # installs to $GOPATH/bin/secretlens
```

---

## Quick Start

```bash
# Scan Git history and environment files (default)
secretlens scan .

# Explicitly scan Git history and environment files
secretlens scan --all .

# Scan CI logs from GitHub Actions
secretlens scan --source=cilog --repo=owner/repo

# Scan Docker image layers
secretlens scan --source=docker --image=myapp:latest

# Write SARIF output
secretlens scan --format=sarif --out=results.sarif .

# Generate an HTML report
secretlens scan --format=html --out=report.html .

# Return exit code 1 when findings meet the severity threshold
secretlens scan --fail-on=HIGH .
```

---

## Command Reference

### `secretlens scan`

| Flag | Description | Default |
|------|-------------|---------|
| `--all` | Scan Git history and environment files | false |
| `--source` | Scan source: `git` `envfile` `cilog` `docker` | git+envfile |
| `--format` | Output format: `text` `json` `sarif` `html` `github-pr` | text |
| `--out` | Output file path. Writes to stdout when omitted | - |
| `--fail-on` | Exit with code 1 at or above severity: `CRITICAL` `HIGH` `MEDIUM` `LOW` | - |
| `--rules-dir` | Directory containing YAML rules | `rules/` next to the executable |
| `--baseline` | Baseline file path | `.secretlens.baseline.json` |
| `--repo` | GitHub repository in `owner/repo` format, used by `cilog` | - |
| `--image` | Docker image name, used by `docker` | - |
| `--pr` | Pull request number for posting a PR comment | - |
| `--sha` | Commit SHA for creating a Check Run | - |
| `--github-token` | GitHub API token. `GITHUB_TOKEN` is also supported | - |
| `--slack-webhook` | Slack webhook URL. `SLACK_WEBHOOK_URL` is also supported | - |
| `--verify` | Run live API verification for detected secrets (opt-in) | false |

### `secretlens org`

Scan all repositories in a GitHub Organization concurrently.

```bash
secretlens org --org=my-company --format=html --out=audit.html
```

| Flag | Description | Default |
|------|-------------|---------|
| `--org` | GitHub Organization name (required) | - |
| `--token` | GitHub API token. `GITHUB_TOKEN` is also supported | - |
| `--concurrency` | Number of concurrent scans | 4 |
| `--format` | Output format: `text` `json` `html` | text |
| `--out` | Output file path | - |

### `secretlens baseline`

```bash
secretlens baseline list            # list registered fingerprints
secretlens baseline update          # show baseline update guidance
```

### `secretlens rules list`

```bash
secretlens rules list               # list enabled rules with ID, severity, and name
```

---

## Scoring and Severity

Each finding receives a score, which is then mapped to a severity level.

| Condition | Score |
|-----------|-------|
| Base rule: CRITICAL | +60 |
| Base rule: HIGH | +40 |
| Base rule: MEDIUM | +20 |
| Live verification passed | +50 |
| Entropy > 4.5 | +20 |
| Sensitive file name such as `.env` or `credentials` | +15 |
| Test code | -30 |
| Comment line | -20 |

| Score | Severity |
|-------|----------|
| 60 or higher | CRITICAL |
| 40-59 | HIGH |
| 20-39 | MEDIUM |
| Below 20 | LOW |

---

## Custom Rules

Add YAML files under `rules/` to extend the rule set.

```yaml
# rules/my-company.yaml
rules:
  - id: myco-internal-token
    name: MyCompany Internal Token
    severity: CRITICAL
    pattern: 'MYCO_[A-Za-z0-9]{32}'
    entropy_min: 4.0
    context_exclude:
      - "**/*_test.go"
      - "**/testdata/**"
    tags:
      - myco
      - internal
```

### Rule Schema

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique rule ID (required) |
| `name` | string | Display name (required) |
| `severity` | string | `CRITICAL` `HIGH` `MEDIUM` `LOW` (required) |
| `pattern` | string | Regular expression using Go regexp syntax (required) |
| `entropy_min` | float | Minimum entropy threshold (optional) |
| `context_exclude` | []string | Exclusion glob patterns (optional) |
| `tags` | []string | Tags (optional) |

---

## Baseline Management

Add known false positives to a baseline file so future scans can ignore them.

```bash
# Inspect fingerprints
secretlens scan --format=json . | jq -r '.[].fingerprint'

# Add fingerprints manually to .secretlens.baseline.json
{
  "fingerprints": {
    "abc123...": true
  }
}
```

---

## GitHub Actions Integration

```yaml
# .github/workflows/secretlens.yml
name: Secret Scan

on: [push, pull_request]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # fetch full Git history

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Install SecretLens
        run: go install github.com/nobuo-miura/SecretLens/cmd/secretlens@latest

      - name: Scan
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          secretlens scan \
            --all \
            --format=sarif \
            --out=results.sarif \
            --fail-on=HIGH \
            .

      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: results.sarif

      - name: PR Comment
        if: github.event_name == 'pull_request'
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          secretlens scan \
            --all \
            --format=github-pr \
            --repo=${{ github.repository }} \
            --pr=${{ github.event.pull_request.number }} \
            --sha=${{ github.sha }} \
            .
```

---

## Built-in Rules

| File | Coverage |
|------|----------|
| `rules/aws.yaml` | AWS Access Key ID / Secret Access Key |
| `rules/gcp.yaml` | GCP Service Account Key / API Key |
| `rules/azure.yaml` | Azure Storage Account Key / Connection String |
| `rules/jwt.yaml` | JWT Token |
| `rules/generic.yaml` | API Key / Password / Token / Private Key / Connection String |

---

## Development

```bash
make test           # run tests
make check          # run vet, tests, and lint
make run-scan       # run a sample scan
make report-html    # generate and open an HTML report
make release        # cross-compile for major platforms
make help           # show available commands
```

### Directory Layout

```text
secretlens/
|-- cmd/secretlens/          # CLI entrypoint (cobra)
|-- internal/
|   |-- scanner/
|   |   |-- git/             # streaming parser for git log --all -p
|   |   |-- cilog/           # GitHub Actions / GitLab CI log APIs
|   |   |-- envfile/         # scan .env / .tfvars / *.yaml files
|   |   `-- docker/          # scan Docker image layers
|   |-- detector/
|   |   |-- regex/           # YAML rule loading and regex matching
|   |   |-- entropy/         # Shannon entropy calculation
|   |   |-- context/         # exclude test code and comments
|   |   `-- verifier/        # live API verification for AWS / GCP / GitHub
|   |-- finding/             # Finding type and scoring
|   |-- reporter/
|   |   |-- sarif/           # SARIF v2.1.0 output
|   |   |-- github/          # PR comments and Check Run API
|   |   |-- slack/           # Slack webhook notifications (Block Kit)
|   |   `-- html/            # interactive HTML reports
|   |-- baseline/            # .secretlens.baseline.json management
|   `-- org/                 # GitHub Organization audit mode
|-- rules/                   # built-in YAML rules
`-- testdata/                # sample files for tests
```

---

## License

MIT License
