---
name: security
description: 'Continuous repository security scanning and release gating. Triggers: "security scan", "security audit", "pre-release security", "run scanners", "check vulnerabilities".'
metadata:
  tier: product
  dependencies: []
---

# Security Skill

> **Purpose:** Run repeatable security checks across code, scripts, hooks, and release gates.

Use this skill when you need deterministic security validation before merge/release, or recurring scheduled checks.

## Quick Start

```bash
/security                      # quick security gate
/security --full               # full gate with test-inclusive toolchain checks
/security --release            # full gate for release readiness
/security --json               # machine-readable report output
```

## Execution Contract

### 1) Pre-PR (fast)

Run quick gate:

```bash
scripts/security-gate.sh --mode quick
```

Expected behavior:
- Fails on high/critical findings from available scanners.
- Writes artifacts under `.agents/security/<run-id>/`.

### 2) Pre-Release (strict)

Run full gate:

```bash
scripts/security-gate.sh --mode full
```

Expected behavior:
- Full scanner pass before release workflow can continue.
- Artifacts retained for audit and incident response.

### 3) Nightly (continuous)

Nightly workflow should run:

```bash
scripts/security-gate.sh --mode full
```

Expected behavior:
- Detects drift/regressions outside active PR windows.
- Failing run creates actionable signal in workflow summary/issues.

## Triage Guidance

When gate fails:
1. Open latest artifact in `.agents/security/` and identify scanner + file.
2. Classify severity (critical/high/medium).
3. Fix immediately for critical/high or create tracked follow-up issue with owner.
4. Re-run `scripts/security-gate.sh` until gate passes.

## Reporting Template

```markdown
Security gate run: <run-id>
Mode: <quick|full>
Result: <pass|blocked>
Top findings:
- <scanner> <severity> <file> <summary>
Actions:
- <fix or issue id>
```

## Notes

- Use this as the canonical security runbook instead of ad-hoc scanner commands.
- Keep workflow wiring aligned with this contract in:
  - `.github/workflows/validate.yml`
  - `.github/workflows/nightly.yml`
  - `.github/workflows/release.yml`
- For binary/internal black-box assurance (static + dynamic + baseline + policy), use:
  - `skills/security-suite/SKILL.md`
  - `scripts/security-suite.sh`
