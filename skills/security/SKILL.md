---
name: security
description: 'Run repository security scans for vulnerabilities, dependency risk, secrets, and release gates. Triggers: "security", "run repository security scans for", "security skill".'
practices:
- supply-chain-integrity
- design-by-contract
- sre
hexagonal_role: driven-adapter
consumes:
- repo-context
produces:
- security-report.json
context_rel:
- kind: supplier-to
  with: validate
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  tier: product
  dependencies: []
output_contract: 'stdout: security scan report'
---
# Security Skill

> **Purpose:** Run repeatable security checks across code, scripts, hooks, and release gates, plus composable binary/internal-testing primitives and offline repo-surface redteam for authorized targets.

Use this skill when you need deterministic security validation before merge/release, recurring scheduled checks, binary black-box assurance, or offline prompt-surface redteam.

This skill has two complementary surfaces:

1. **Repository security gate** (`scripts/security-gate.sh`) — fast/full/nightly scanner gates for code, scripts, hooks, and release readiness.
2. **Composable security suite** (`scripts/security_suite.py`, `scripts/prompt_redteam.py`) — testable, reusable primitives for authorized binaries and repo-managed prompt surfaces, with policy gating and machine-consumable outputs.

## Quick Start

```bash
/security                      # quick security gate
/security --full               # full gate with test-inclusive toolchain checks
/security --release            # full gate for release readiness
/security --json               # machine-readable report output
```

## Guardrails (suite primitives)

- Use the binary/redteam primitives only on binaries you own or are explicitly authorized to assess.
- Do not use this workflow to bypass legal restrictions or extract third-party proprietary content without authorization.
- Prefer behavioral assurance and policy gating over ad-hoc one-off reverse-engineering.

## Execution Contract (repository gate)

### 1) Pre-PR (fast)

Run quick gate:

```bash
scripts/security-gate.sh --mode quick
```

Expected behavior:
- Fails on high/critical findings from available scanners.
- Writes artifacts under `$TMPDIR/agentops-security/<run-id>/`.

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

## Composable Security Suite

This surface separates concerns into primitives so security workflows stay testable and reusable.

### Primitive Model

1. `collect-static` — file metadata, runtime heuristics, linked libraries, embedded archive signatures.
2. `collect-dynamic` — sandboxed execution trace (processes, file changes, network endpoints).
3. `collect-contract` — machine-readable behavior contract from help-surface probing.
4. `compare-baseline` — current vs baseline contract drift (added/removed commands, runtime change).
5. `enforce-policy` — allowlist/denylist gates and severity-based verdict.
6. `collect-redteam` — offline repo-surface attack-pack scan for prompt-injection, tool-misuse, secret-exfiltration, and unsafe-shell regressions.
7. `run` — thin binary orchestrator that composes primitives and writes suite summary.

### Suite Quick Start

Single run (default dynamic command is `--help`):

```bash
python3 skills/security/scripts/security_suite.py run \
  --binary "$(command -v ao)" \
  --out-dir .tmp/security-suite/ao-current
```

Baseline regression gate:

```bash
python3 skills/security/scripts/security_suite.py run \
  --binary "$(command -v ao)" \
  --out-dir .tmp/security-suite/ao-current \
  --baseline-dir .tmp/security-suite/ao-baseline \
  --fail-on-removed
```

Policy gate:

```bash
python3 skills/security/scripts/security_suite.py run \
  --binary "$(command -v ao)" \
  --out-dir .tmp/security-suite/ao-current \
  --policy-file skills/security/references/policy-example.json \
  --fail-on-policy-fail
```

Repo-surface redteam:

```bash
python3 skills/security/scripts/prompt_redteam.py scan \
  --repo-root . \
  --pack-file skills/security/references/agentops-redteam-pack.json \
  --out-dir .tmp/security-suite-redteam
```

For OWASP Top 10 code-level review, see [references/owasp-checklist.md](references/owasp-checklist.md).

### Recommended Suite Workflow

1. Capture baseline on known-good release.
2. Run suite on candidate binary in CI.
3. Compare against baseline and enforce policy.
4. Block promotion on failing verdict.

### Suite Output Contract

All outputs are written under `--out-dir`:

- `static/static-analysis.json`
- `dynamic/dynamic-analysis.json`
- `contract/contract.json`
- `compare/baseline-diff.json` (when baseline supplied)
- `policy/policy-verdict.json` (when policy supplied)
- `suite-summary.json`
- `redteam/redteam-results.json` (when repo-surface redteam is run)

This output structure is intentionally machine-consumable for CI gates.

### Policy Model

Use `skills/security/references/policy-example.json` as a starting point. Policy gating produces a machine-readable `policy-verdict.json`.

Supported checks:

- `required_top_level_commands`
- `deny_command_patterns`
- `max_created_files`
- `forbid_file_path_patterns`
- `allow_network_endpoint_patterns`
- `deny_network_endpoint_patterns`
- `block_if_removed_commands`
- `min_command_count`

### Redteam Pack Model

Use [agentops-redteam-pack.json](references/agentops-redteam-pack.json) as the
starting point for offline repo-surface redteam checks.

Supported target fields:

- `globs`
- `require_groups`
- `forbidden_any`
- `applies_if_any`

Each case expresses a concrete adversarial prompt or operator-bypass attempt and
binds it to one or more repo-owned files. The first shipped pack covers
instruction precedence, context overexposure, destructive git misuse, security
gate bypass, and unsafe shell or secret-handling regressions.

### Technique Coverage

This suite is designed for broad binary classes, not just CLI metadata:

- static runtime/library fingerprinting
- sandboxed behavior observation
- command/contract capture
- drift classification
- policy enforcement and CI verdicting
- repo-surface redteam checks for prompt and operator-contract regressions

It is intentionally modular so you can add deeper primitives later (syscall tracing, SBOM attestation verification, fuzz harnesses) without rewriting the workflow.

## Triage Guidance

When the repository gate fails:
1. Open latest artifact in `$TMPDIR/agentops-security/` and identify scanner + file.
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

## Validation

Run the merged skill validator (asserts the suite scripts/references, gate, and redteam pack stay healthy):

```bash
bash skills/security/scripts/validate.sh
bash tests/scripts/test-security-suite-redteam.sh
```

Suite smoke test (recommended):

```bash
python3 skills/security/scripts/security_suite.py run \
  --binary "$(command -v ao)" \
  --out-dir .tmp/security-suite-smoke \
  --policy-file skills/security/references/policy-example.json
```

Repo-surface smoke test:

```bash
python3 skills/security/scripts/prompt_redteam.py scan \
  --repo-root . \
  --pack-file skills/security/references/agentops-redteam-pack.json \
  --out-dir .tmp/security-suite-redteam-smoke
```

## Notes

- Use this as the canonical security runbook instead of ad-hoc scanner commands.
- Keep workflow wiring aligned with this contract in:
  - `.github/workflows/validate.yml`
  - `.github/workflows/nightly.yml`
  - `.github/workflows/release.yml`
- For binary/internal black-box assurance plus offline repo-surface redteam, use the composable suite above (`security_suite.py` and `prompt_redteam.py`).
- For dependency vulnerability and license scanning, use:
  - deps — Audit dependency risks and updates: vulnerability scanning and license compliance (absorbed into this skill)

## Examples

### Scenario: Quick Security Gate Before Opening a PR

**User says:** `/security`

**What happens:**
1. The skill runs `scripts/security-gate.sh --mode quick`, which executes available scanners (semgrep, gosec, gitleaks) against the current working tree and flags high/critical findings.
2. Run `deps vuln` to scan for vulnerable dependencies (OWASP A06: Vulnerable and Outdated Components).
3. Scan artifacts are written to `$TMPDIR/agentops-security/<run-id>/` for review, and the gate reports a pass/blocked verdict.

**Result:** The gate passes with no high/critical findings, confirming the branch is safe to open a PR.

### Scenario: Full Security Gate for a Release

**User says:** `/security --release`

**What happens:**
1. The skill runs `scripts/security-gate.sh --mode full`, which performs a comprehensive scan including all scanner passes, test-inclusive toolchain checks, and stricter severity thresholds.
2. Artifacts are retained under `$TMPDIR/agentops-security/<run-id>/` for audit trail and incident response, and a structured report is generated.

**Result:** The full gate blocks the release on two medium-severity findings in `cli/internal/config.go`; the operator triages and fixes them before re-running the gate to get a clean pass.

### Scenario: Capture a Baseline and Gate a New Release (suite)

**User says:** `/security run --binary $(command -v ao) --out-dir .tmp/security-suite/ao-v2.4`

**What happens:**
1. The suite runs static analysis (file metadata, linked libraries, embedded archive signatures), dynamic tracing (sandboxed `--help` execution observing processes, file changes, network endpoints), and contract capture against the `ao` binary.
2. It writes `static/static-analysis.json`, `dynamic/dynamic-analysis.json`, `contract/contract.json`, and `suite-summary.json` under the output directory.

**Result:** A complete baseline snapshot is captured for `ao` v2.4, ready to be used as `--baseline-dir` for future release comparisons.

### Scenario: CI Regression Gate With Baseline and Policy (suite)

**User says:** `/security run --binary ./bin/ao-candidate --out-dir .tmp/ao-candidate --baseline-dir .tmp/security-suite/ao-v2.4 --policy-file skills/security/references/policy-example.json --fail-on-removed --fail-on-policy-fail`

**What happens:**
1. The suite runs all three collection primitives on the candidate binary, then compares the resulting contract against the v2.4 baseline to produce `compare/baseline-diff.json` with any added, removed, or changed commands.
2. It evaluates the policy file checks (required commands, denied patterns, network allowlists, file limits) and writes `policy/policy-verdict.json` with a pass/fail verdict.

**Result:** The suite exits non-zero if any commands were removed or a policy check failed, blocking the candidate from promotion in the CI pipeline.

### Scenario: Offline Redteam the Repo's Prompt and Skill Surfaces (suite)

**User says:** `/security collect-redteam --repo-root .`

**What happens:**
1. The redteam scanner loads the attack pack from [`agentops-redteam-pack.json`](references/agentops-redteam-pack.json) and evaluates repo-owned control surfaces against concrete attack cases.
2. It writes `redteam/redteam-results.json` and `redteam/redteam-results.md` under the chosen output directory, then exits non-zero if a fail-severity case is not resisted.

**Result:** The repo gets a deterministic redteam verdict for prompt-injection, tool misuse, context overexposure, secret-handling, and unsafe-shell regressions without needing hosted model scanning.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Gate reports "scanner not found" and skips checks | Required scanner (semgrep, gosec, or gitleaks) is not installed | Install the missing scanner: `brew install semgrep`, `go install github.com/securego/gosec/v2/cmd/gosec@latest`, or `brew install gitleaks`. |
| Gate passes locally but fails in CI | CI environment has additional scanners or stricter config | Compare `$TMPDIR/agentops-security/` artifacts from both environments; align scanner versions and config files across local and CI. |
| False positive blocking the gate | Scanner flags a non-issue as high/critical severity | Add a scanner-specific inline suppression comment (e.g., `# nosemgrep: rule-id`) or update the scanner config to exclude the pattern, then document the suppression reason. |
| Artifacts directory `$TMPDIR/agentops-security/` not created | Script lacks write permissions or `$TMPDIR` is not writable | Verify `$TMPDIR` is set and writable; the script auto-creates subdirectories on each run. |
| Nightly scan not detecting regressions | Nightly workflow is not configured or is pointing at stale branch | Verify `.github/workflows/nightly.yml` runs `scripts/security-gate.sh --mode full` against the correct branch (typically `main`). |
| Suite exits non-zero with no clear finding | `--fail-on-removed` or `--fail-on-policy-fail` triggered on a legitimate change | Review `compare/baseline-diff.json` and `policy/policy-verdict.json` to identify the specific delta, then update the baseline or policy file accordingly. |
| `dynamic/dynamic-analysis.json` is empty or minimal | Binary requires arguments beyond `--help`, or sandbox blocked execution | Supply a custom dynamic command if supported, or verify the binary runs in the sandboxed environment (check permissions, missing shared libraries). |
| `contract/contract.json` shows zero commands | The binary does not expose a `--help` surface or uses a non-standard help flag | Verify the binary supports `--help`; for binaries with unusual help interfaces, run `collect-contract` separately with the correct invocation. |
| Policy verdict fails on `deny_command_patterns` | A new subcommand matches a deny regex in the policy file | Either rename the subcommand or update `deny_command_patterns` in your policy JSON to exclude the legitimate pattern. |
| `baseline-diff.json` not generated | `--baseline-dir` was not provided or points to a missing directory | Ensure the baseline directory exists and contains a valid `contract/contract.json` from a prior run. |
| Redteam scan fails after a wording cleanup | The attack pack no longer matches the intended guardrail language in target files | Review `redteam/redteam-results.json`, confirm whether the control regressed or the regex is too brittle, then update the target file or the pack intentionally. |

## Reference Documents

- [references/security.feature](references/security.feature) — Executable spec: run scanners, fail on high/critical, gate release, retain audit artifacts (soc-qk4b)
- [references/security-suite.feature](references/security-suite.feature) — Executable spec: composable primitives (static/dynamic/contract) → security-report.json, authorization-bounded, supplier-to vibe (soc-qk4b)
- [references/owasp-checklist.md](references/owasp-checklist.md) — OWASP Top 10 code-level review checklist
- [references/agentops-redteam-pack.json](references/agentops-redteam-pack.json) — Offline repo-surface attack pack
- [references/policy-example.json](references/policy-example.json) — Starter policy for the suite policy gate
