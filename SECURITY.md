# Security Policy

## Supported Versions

Only the latest release of SecretLens receives security updates.

## Reporting a Vulnerability

Please **do not** open a public issue for security vulnerabilities.

Instead, report them privately via [GitHub Security Advisories](https://github.com/nobuo-miura/SecretLens/security/advisories/new).

You can expect an initial response within 7 days. Once the issue is confirmed and fixed, we will publish an advisory and credit the reporter (unless you prefer to remain anonymous).

## Scope

SecretLens is a secret *detection* tool. False negatives (secrets it fails to detect) are quality issues, not vulnerabilities — please report those as regular issues. Security reports should cover things like:

- SecretLens itself leaking scanned secrets (e.g. to logs, telemetry, or network)
- Code execution or path traversal triggered by scanned content
- Vulnerabilities in the verification (`--verify`) network calls
