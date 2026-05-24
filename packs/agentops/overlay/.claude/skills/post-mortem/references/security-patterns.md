# Security Patterns for Post-Mortem

Security checks to run during post-mortem Phase 3.

---

## Tool Detection

Before running security tools, check availability:

```bash
command -v gitleaks &>/dev/null && HAS_GITLEAKS=true || HAS_GITLEAKS=false
command -v semgrep &>/dev/null && HAS_SEMGREP=true || HAS_SEMGREP=false
command -v govulncheck &>/dev/null && HAS_GOVULN=true || HAS_GOVULN=false
command -v pip-audit &>/dev/null && HAS_PIPAUDIT=true || HAS_PIPAUDIT=false
```

---

## Static Analysis Tools

### Gitleaks (Secret Detection)

```bash
# Full scan
gitleaks detect --source . --verbose --report-format json --report-path reports/gitleaks.json

# Changed files only
git diff --name-only HEAD~10 | xargs gitleaks detect --source
```

**Severity:**
- API keys, passwords: CRITICAL
- Generic secrets: HIGH
- Potential false positives: MEDIUM

### Semgrep (SAST)

```bash
# OWASP patterns
semgrep --config "p/owasp-top-ten" --json -o reports/semgrep.json .

# Python-specific
semgrep --config "p/python" .

# Go-specific
semgrep --config "p/golang" .
```

### Language-Specific Vulnerability Scanners

**Python:**
```bash
pip-audit -r requirements.txt --format json -o reports/pip-audit.json
safety check -r requirements.txt --json > reports/safety.json
```

**Go:**
```bash
govulncheck -json ./... > reports/govulncheck.json
```

**JavaScript:**
```bash
npm audit --json > reports/npm-audit.json
```

---

## Pattern-Based Detection

When tools aren't available, use grep patterns:

### SEC-P01: SQL Injection

```bash
# Python
grep -rn "execute.*%s\|execute.*format\|execute.*f\"\|cursor.execute.*+" --include="*.py" .

# Go
grep -rn "fmt.Sprintf.*SELECT\|fmt.Sprintf.*INSERT\|Query.*+" --include="*.go" .
```

**Fix:** Use parameterized queries.

### SEC-P02: Command Injection

```bash
# Python
grep -rn "os.system\|subprocess.*shell=True\|eval(\|exec(" --include="*.py" .

# Go
grep -rn "exec.Command.*Shell\|os.Exec" --include="*.go" .
```

**Fix:** Use safe APIs, never shell=True with user input.

### SEC-P03: Hardcoded Secrets

```bash
# All languages
grep -rn "password.*=.*['\"].*['\"]" --include="*.py" --include="*.go" --include="*.js" .
grep -rn "api_key.*=.*['\"]" --include="*.py" --include="*.go" --include="*.js" .
grep -rn "secret.*=.*['\"].*[a-zA-Z0-9]" --include="*.py" --include="*.go" --include="*.js" .
```

**Fix:** Use environment variables or secrets management.

### SEC-P04: Insecure Deserialization

```bash
# Python
grep -rn "pickle.load\|yaml.load.*Loader" --include="*.py" .

# Go
grep -rn "json.Unmarshal.*interface{}" --include="*.go" .
```

**Fix:** Use safe loaders, validate before deserializing.

### SEC-P05: XSS Patterns

```bash
# JavaScript/TypeScript
grep -rn "innerHTML.*=\|dangerouslySetInnerHTML\|v-html" --include="*.js" --include="*.ts" --include="*.vue" .

# Python (templates)
grep -rn "mark_safe\|{{.*\|raw}}\|autoescape false" --include="*.html" --include="*.jinja" .
```

**Fix:** Use safe templating, escape output.

### SEC-P06: Path Traversal

```bash
# All languages
grep -rn "open.*%s\|open.*format\|os.path.join.*input" --include="*.py" .
grep -rn "filepath.Join.*user" --include="*.go" .
```

**Fix:** Validate and sanitize file paths.

### SEC-P07: Insecure TLS

```bash
# Python
grep -rn "verify=False\|CERT_NONE" --include="*.py" .

# Go
grep -rn "InsecureSkipVerify.*true" --include="*.go" .
```

**Fix:** Never disable certificate verification in production.

### SEC-P08: Weak Cryptography

```bash
# Python
grep -rn "MD5\|SHA1\|DES\|RC4" --include="*.py" .

# Go
grep -rn "md5.\|sha1.\|des.\|rc4." --include="*.go" .
```

**Fix:** Use strong algorithms (SHA256+, AES).

---

## OWASP Top 10 Checklist

| OWASP ID | Category | Check | Pattern |
|----------|----------|-------|---------|
| A01 | Broken Access Control | Auth checks on routes | Missing `@login_required` |
| A02 | Cryptographic Failures | Weak crypto, plaintext | SEC-P03, SEC-P08 |
| A03 | Injection | SQL, command, XSS | SEC-P01, SEC-P02, SEC-P05 |
| A04 | Insecure Design | Business logic flaws | Manual review |
| A05 | Security Misconfiguration | Debug mode, defaults | `DEBUG=True`, default passwords |
| A06 | Vulnerable Components | Old dependencies | pip-audit, npm audit |
| A07 | Auth Failures | Weak auth, session | Timing attacks, session fixation |
| A08 | Data Integrity | Deserialization, CI/CD | SEC-P04 |
| A09 | Logging Failures | Missing logs, PII in logs | grep for sensitive in logs |
| A10 | SSRF | Server requests | `requests.get(user_input)` |

---

## Expert Agent Delegation

For CRITICAL findings, spawn security expert:

```python
Task(
    subagent_type="security-expert",
    prompt=f"""Deep security review for post-mortem.

Findings to analyze:
{findings}

Changed files:
{changed_files}

Please:
1. Verify each finding is real (not false positive)
2. Assess exploitability
3. Recommend specific fixes
4. Identify any additional vulnerabilities
"""
)
```

---

## Report Format

```markdown
## Security Scan Results

**Date:** YYYY-MM-DD
**Epic:** <epic-id>
**Changed Files:** N files

### Summary

| Category | CRITICAL | HIGH | MEDIUM | LOW |
|----------|----------|------|--------|-----|
| Secrets | 0 | 0 | 0 | 0 |
| Vulnerabilities | 0 | 0 | 0 | 0 |
| Patterns | 0 | 1 | 2 | 0 |

### Findings

#### SEC-001 [HIGH] Potential SQL Injection
- **File:** services/db.py:42
- **Pattern:** `cursor.execute(f"SELECT * FROM {table}")`
- **Fix:** Use parameterized query: `cursor.execute("SELECT * FROM ?", (table,))`
- **Issue Created:** <issue-id>

### Tools Used
- gitleaks: v8.18.0
- semgrep: v1.0.0
- grep patterns: SEC-P01 through SEC-P08

### Recommendations
1. Fix HIGH findings before merge
2. Consider adding pre-commit hook for gitleaks
3. Schedule full security audit (quarterly)
```

---

## Integration with Post-Mortem

In Phase 3 of post-mortem:

```python
def run_security_scan(changed_files, epic_id):
    findings = []

    # 1. Run available tools
    if HAS_GITLEAKS:
        findings += run_gitleaks(changed_files)

    if HAS_SEMGREP:
        findings += run_semgrep(changed_files)

    # 2. Run grep patterns
    findings += run_grep_patterns(changed_files)

    # 3. Check OWASP Top 10
    findings += check_owasp(changed_files)

    # 4. Delegate CRITICAL to expert
    critical = [f for f in findings if f.severity == 'CRITICAL']
    if critical:
        expert_review = spawn_security_expert(critical, changed_files)
        findings += expert_review.additional_findings

    # 5. Create issues for HIGH+
    for finding in findings:
        if finding.severity in ['CRITICAL', 'HIGH']:
            bd_create(
                title=f"Security: {finding.title}",
                type="bug",
                priority="P1" if finding.severity == 'CRITICAL' else "P2",
                description=finding.details
            )

    return SecurityReport(findings)
```
