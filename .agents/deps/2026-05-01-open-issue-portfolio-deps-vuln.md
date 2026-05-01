# Dependency Vulnerability Check: Open Issue Portfolio Validation

- **Date:** 2026-05-01
- **Scope:** advisory lifecycle check for `HEAD~2..HEAD`
- **Command:** `cd cli && go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- **Result:** PASS

`govulncheck` reported no vulnerabilities.
