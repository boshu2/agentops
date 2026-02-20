---
name: security-suite
description: 'Composable binary security suite for static analysis, dynamic tracing, contract capture, baseline drift, and policy gating. Triggers: "binary security", "reverse engineer binary", "black-box binary test", "behavioral trace", "baseline diff", "security suite".'
metadata:
  tier: execution
  dependencies: []
---

# Security Suite

> **Purpose:** Provide composable, repeatable security/internal-testing primitives for authorized binaries.

This skill separates concerns into primitives so security workflows stay testable and reusable.

## Guardrails

- Use only on binaries you own or are explicitly authorized to assess.
- Do not use this workflow to bypass legal restrictions or extract third-party proprietary content without authorization.
- Prefer behavioral assurance and policy gating over ad-hoc one-off reverse-engineering.

## Primitive Model

1. `collect-static` — file metadata, runtime heuristics, linked libraries, embedded archive signatures.
2. `collect-dynamic` — sandboxed execution trace (processes, file changes, network endpoints).
3. `collect-contract` — machine-readable behavior contract from help-surface probing.
4. `compare-baseline` — current vs baseline contract drift (added/removed commands, runtime change).
5. `enforce-policy` — allowlist/denylist gates and severity-based verdict.
6. `run` — thin orchestrator that composes primitives and writes suite summary.

## Quick Start

Single run (default dynamic command is `--help`):

```bash
python3 skills/security-suite/scripts/security_suite.py run \
  --binary "$(command -v ao)" \
  --out-dir .agents/security-suite/ao-current
```

Baseline regression gate:

```bash
python3 skills/security-suite/scripts/security_suite.py run \
  --binary "$(command -v ao)" \
  --out-dir .agents/security-suite/ao-current \
  --baseline-dir .agents/security-suite/ao-baseline \
  --fail-on-removed
```

Policy gate:

```bash
python3 skills/security-suite/scripts/security_suite.py run \
  --binary "$(command -v ao)" \
  --out-dir .agents/security-suite/ao-current \
  --policy-file skills/security-suite/references/policy-example.json \
  --fail-on-policy-fail
```

## Recommended Workflow

1. Capture baseline on known-good release.
2. Run suite on candidate binary in CI.
3. Compare against baseline and enforce policy.
4. Block promotion on failing verdict.

## Output Contract

All outputs are written under `--out-dir`:

- `static/static-analysis.json`
- `dynamic/dynamic-analysis.json`
- `contract/contract.json`
- `compare/baseline-diff.json` (when baseline supplied)
- `policy/policy-verdict.json` (when policy supplied)
- `suite-summary.json`

This output structure is intentionally machine-consumable for CI gates.

## Policy Model

Use `skills/security-suite/references/policy-example.json` as a starting point.

Supported checks:

- `required_top_level_commands`
- `deny_command_patterns`
- `max_created_files`
- `forbid_file_path_patterns`
- `allow_network_endpoint_patterns`
- `deny_network_endpoint_patterns`
- `block_if_removed_commands`
- `min_command_count`

## Technique Coverage

This suite is designed for broad binary classes, not just CLI metadata:

- static runtime/library fingerprinting
- sandboxed behavior observation
- command/contract capture
- drift classification
- policy enforcement and CI verdicting

It is intentionally modular so you can add deeper primitives later (syscall tracing, SBOM attestation verification, fuzz harnesses) without rewriting the workflow.

## Validation

Run:

```bash
bash skills/security-suite/scripts/validate.sh
```

Smoke test (recommended):

```bash
python3 skills/security-suite/scripts/security_suite.py run \
  --binary "$(command -v ao)" \
  --out-dir .tmp/security-suite-smoke \
  --policy-file skills/security-suite/references/policy-example.json
```
