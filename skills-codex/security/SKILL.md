---
name: security
description: "Run repository security scans for vulnerabilities, dependency risk, secrets, and release gates. Triggers: security review, release security, scan this repository."
---
# Security Skill

> **Purpose:** Run repeatable security checks across code, scripts, release gates, authorized binaries, and repo-managed prompt surfaces, then return severity-first evidence.

Use this skill for deterministic pre-merge/release validation, scheduled checks, authorized binary assurance, dependency risk, secrets, or offline prompt-surface redteam.

## Critical Constraints

- Scan only repositories, binaries, and prompt surfaces the operator owns or is explicitly authorized to assess. **Why:** a security review does not grant access to third-party systems or proprietary material.
- Keep collection read-only by default; do not exfiltrate secrets, execute destructive payloads, or mutate policy/baselines to manufacture green. **Why:** the assessment must not become the incident or erase its evidence.
- Treat missing/error scanners as a coverage gap, never a clean finding; use `--require-tools` when complete tool coverage is required. **Why:** absent evidence is not evidence of absence.
- Use the current Codex agent and local shell; do not start another runtime or orchestration substrate unless explicitly requested. **Why:** repository scanning is a bounded operation, not permission to fan out.
- `WARN|FAIL|REFUTED -> AUTO-REDO`: consult the pawl, reproduce and classify the finding, remediate only when authorized, then rerun the same gate. **Why:** a negative verdict is loop evidence, not a human andon by itself.
- `BREAKER -> HOLD -> ONE-HELPER`; `HELPER-UNSTUCK -> AUTO-REDO`. Hold promotion and use one bounded local-shell helper to inspect the scanner, artifact, or authorization boundary. **Why:** one recovery pass can distinguish tooling failure from a real security stop.
- `HELPER-ESCALATE -> HUMAN`; `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`. **Why:** target authorization, risk acceptance, suppression/baseline judgment, or exhausted recovery requires accountable human ownership.

## Security Surfaces

1. **Repository gate:** `scripts/security-gate.sh` composes available scanners for quick/full/release checks.
2. **Composable suite:** `scripts/security_suite.py` provides static, dynamic, contract, baseline, and policy primitives for authorized binaries.
3. **Offline redteam:** `scripts/prompt_redteam.py` checks repo-owned prompt and tool-control surfaces against the attack pack.

Read [the suite runbook](references/security-suite-runbook.md) before binary, policy, baseline, or redteam work.

## Execution Workflow

### 1) Quick gate

Run:

```bash
scripts/security-gate.sh --mode quick
```

**Checkpoint:** preserve the exit code and verify the reported `security-gate-summary.json` exists and parses before triage.

### 2) Full or release gate

Run:

```bash
scripts/security-gate.sh --mode full
```

Add `--require-tools` when skipped scanners would invalidate the assurance claim. **Checkpoint:** do not authorize promotion until the full artifact passes the output validator and the process exits zero.

### 3) Scheduled gate

Scheduled automation runs the full gate against the intended branch and retains its artifact directory. A failing scheduled run creates actionable tracked work; AgentOps itself does not supply the scheduler.

### 4) Triage and re-run

1. Lead with critical/high findings, blocked release conditions, and the exact evidence that triggered them.
2. Open the latest artifact and identify scanner, severity, file, and coverage gaps.
3. Reproduce the finding with the narrowest safe command.
4. For authorized remediation, fix critical/high findings; otherwise report them with owner and next action.
5. Re-run the same gate. Do not downgrade, suppress, or update a baseline merely to pass.

## Output Specification

**Artifact directory:** repository gates write `${SECURITY_GATE_OUTPUT_DIR:-${TMPDIR:-/tmp}/agentops-security}/<run-id>/`; composable-suite and redteam runs use their explicit `--out-dir`.
**Filename convention:** repository gates require `security-gate-summary.json` (and raw `summary.json`); suite runs require `suite-summary.json`; redteam runs require `redteam/redteam-results.json`.
**Serialization/schema format:** `security-gate-summary.json` is JSON with nonempty `mode`, `run_id`, `output_dir`, and `gate_status`, numeric `missing_tool_count`, boolean `require_tools`, and object `toolchain`.
**Validator command:** with `OUT=<security-gate-run-dir>`, run `jq -e '(.mode|type)=="string" and (.mode|length)>0 and (.run_id|type)=="string" and (.run_id|length)>0 and (.output_dir|type)=="string" and (.output_dir|length)>0 and .gate_status=="PASS" and (.missing_tool_count|type)=="number" and (.require_tools|type)=="boolean" and (.toolchain|type)=="object"' "$OUT/security-gate-summary.json" >/dev/null`.
**Downstream handoff:** give validation/release a findings-first packet containing artifact path, command/exit code, mode, gate status, missing-tool coverage, ranked findings, authorization boundary, owner, and next action; negative verdicts re-enter through the pawl.

## Quality Checklist

- [ ] Target and authorization boundary are explicit; collection stayed within them.
- [ ] Scanner availability and skipped/error coverage are visible in the report.
- [ ] Findings include severity, location, reproducible evidence, and remediation/owner.
- [ ] Raw evidence is separated from the final security judgment and contains no newly exposed secrets.
- [ ] Required gate and output validator both pass before promotion is declared safe.
- [ ] Suppressions, policy changes, baselines, and risk acceptance require explicit judgment.
- [ ] WARN/FAIL/REFUTED consulted the pawl before any human andon.

## Validation

Run the Codex-facing skill and redteam validators:

```bash
bash skills-codex/security/scripts/validate.sh
bash tests/scripts/test-security-suite-redteam.sh
```

For a bounded suite smoke test, use an owned binary and a temporary output directory as shown in [the suite runbook](references/security-suite-runbook.md).

## Examples

- `$security` — run the quick repository gate, validate its summary, and report coverage/findings first.
- `$security --release` — run the full gate, preserve artifacts, and block promotion until the verdict is green.
- `$security run --binary "$(command -v ao)" --out-dir .tmp/security-suite/ao-current` — capture an authorized binary baseline via the composable suite.
- `$security collect-redteam --repo-root .` — run the offline attack pack over repo-owned control surfaces.

## Troubleshooting

| Problem | Response |
|---------|----------|
| Scanner missing/error | Record the coverage gap; install it or rerun with `--require-tools` when required |
| Local/CI mismatch | Compare scanner versions, config, mode, and both artifact directories |
| Suspected false positive | Reproduce narrowly; document any authorized suppression and its owner |
| Suite/baseline failure | Inspect the named compare/policy artifact; never refresh baseline reflexively |
| Redteam failure after wording change | Decide whether the control regressed or the attack-pack matcher needs intentional revision |

## Local Resources

### references/

- [references/security-suite-runbook.md](references/security-suite-runbook.md) — binary/policy/baseline/redteam commands and artifacts

### scripts/

- `scripts/security-gate.sh`
- `scripts/security_suite.py`
- `scripts/prompt_redteam.py`
- `scripts/validate.sh`
