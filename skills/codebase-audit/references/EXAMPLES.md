# Example Audit Reports

## Security Audit (Excerpt)

```
# Security Audit Report: Example Project

## Summary
- Total findings: 4
- Critical: 1 | High: 2 | Medium: 1 | Low: 0

## Critical Findings

### SQL Injection in Search Endpoint
- Location: src/search.rs:42
- Severity: Critical
- Issue: Raw string concatenation into SQL query.
- Root Cause: Missing parameterized queries.
- Fix: Use prepared statements with bound params.

## High Priority Findings

### Secrets in Repo
- Location: config/dev.toml:12
- Severity: High
- Issue: API key committed in plaintext.
- Root Cause: Missing secret management and .gitignore.
- Fix: Move to env vars and rotate key.
```

## CLI Audit (Excerpt)

```
# CLI Audit Report: Example Project

## Summary
- Total findings: 3
- Critical: 0 | High: 1 | Medium: 1 | Low: 1

## High Priority Findings

### Non-zero exit codes on success
- Location: src/cli/main.rs:118
- Severity: High
- Issue: Command exits with code 1 after successful run.
- Root Cause: Shared error path for success and failure.
- Fix: Return 0 on success; reserve 1+ for errors.
```
