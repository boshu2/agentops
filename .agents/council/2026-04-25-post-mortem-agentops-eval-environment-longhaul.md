---
id: post-mortem-2026-04-25-agentops-eval-environment-longhaul
type: post-mortem
date: 2026-04-25
source: ".agents/plans/2026-04-24-agentops-evaluation-environment.md"
---

# Post-Mortem: AgentOps Eval Environment Longhaul

**Epic:** agentops-dv5
**Branch:** codex/eval-env-discovery
**Duration Reviewed:** overnight evolve/RPI run, through cycle 54
**Commit Range:** origin/codex/eval-env-discovery..d749108b
**Changed Surface:** 202 files, including eval CLI, schemas, 54 public suites, 54 baselines, hooks, docs, Codex artifacts, and validation scripts.

## Council Verdict: WARN

| Judge | Verdict | Key Finding |
|---|---|---|
| Plan-Compliance | PASS | The original objective is materially satisfied: deterministic eval contracts, runner, compare/baseline/coverage/scorecard commands, public canaries, runtime adapters, and gate integration all exist and pass. |
| Tech-Debt | WARN | Product gates are green, but closeout cannot be considered clean until canonical-root dirtiness and closure-integrity replay failures are resolved. |
| Learnings | WARN | The run proved the eval flywheel works, but also showed that long autonomous loops need a final disposition/closure audit before selecting new work. |

## Applied Prior Findings

- `.agents/findings/f-2026-04-14-002.md`: applicable. Closure replay can fail when bead evidence references ephemeral or parser-invisible proof. This directly matches the `agentops-dv5.3`/`.4`/`.5` audit results.
- `.agents/findings/f-2026-04-14-001.md`: applicable. The final `status.go` hardening was paired with `TestLoadFlywheelBrief` validation and the status eval suite.
- `.agents/findings/f-2026-04-10-001.md`: applicable. JSON output and baseline promotion paths were validated through eval canaries and the JSON flag consistency/pre-push surfaces.

## Validation Summary

- `go test ./cmd/ao ./internal/eval`: PASS.
- `scripts/eval-agentops.sh --fast`: PASS, 54 suites / 273 cases / 209 critical, failures=0, warnings=0.
- Headless runtime skills: PASS, 7/7.
- Codex RPI contract: PASS.
- Codex lifecycle guards: PASS.
- Codex semantic parity: PASS.
- Smart pre-push gate: BLOCKED on repository disposition only. Code, docs, CLI docs, hooks/docs parity, CI policy parity, shellcheck, no-symlinks, contract compatibility, and eval canaries passed before final summary.

## Four-Surface Closure

| Surface | Verdict | Notes |
|---|---|---|
| Code | PASS | Focused Go tests and eval engine tests passed; no whitespace errors. |
| Documentation | PASS | Doc-release, mkdocs strict build, CLI docs parity, hooks/docs parity, and CI policy parity passed inside pre-push. |
| Examples | PASS | CLI command surface matrix validates generated command/subcommand help coverage. |
| Proof | WARN | Public eval and targeted runtime proof pass; release-style proof remains blocked by canonical-root dirty metadata outside this linked branch. |

## Deep Audit Sweep

| Area | Result | Evidence |
|---|---|---|
| Eval suite inventory | PASS | 54 `evals/agentops-core/*.json` suites and 54 promoted baselines. |
| Baseline regression | PASS | Every suite compared cleanly with aggregate_delta=0. |
| Runtime isolation | PASS | Headless runtime skill smoke and Codex lifecycle guards passed. |
| Closure integrity | FAIL -> follow-up | `bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto agentops-dv5` found 3 failures. |
| Metadata verification | WARN | Naive sweep reported illustrative plan placeholders plus unresolved Beads CLI reference links. |
| Behavioral validation | SKIP | No `.agents/holdout/` or `.agents/specs/` artifacts were present in this worktree. Public holdout-isolation eval passed. |
| Worktree disposition | FAIL -> follow-up | Canonical root `main` is dirty in tracked `.agents` files. |

## Closure-Integrity Findings

- `agentops-dv5.1`, `.2`, and `.6` passed by grace-window evidence.
- `agentops-dv5.3` failed because the audit extracted `skills/scenario/SKILL.md` from the description, but the delivered proof is in `cli/cmd/ao/scenario_add.go`, `cli/cmd/ao/scenario_test.go`, `cli/cmd/ao/scenario_validate.go`, `evals/agentops-core/*`, and `cli/docs/COMMANDS.md` from the close reason.
- `agentops-dv5.4` and `.5` failed parser extraction despite detailed close reasons and passing validation evidence.
- Follow-up bead: `agentops-aeg`.

## What Worked

- The eval system now gives a concrete better/worse signal through promoted baselines and aggregate deltas.
- Public canaries cover CLI, hooks, skills, RPI, runtime, retrieval, scenario, security, docs, and process surfaces.
- Evolve cycles reliably found thin spots and converted them into targeted suites without regressing the aggregate gate.
- The final status hardening caught a subtle goroutine/global-hook race and closed it with a focused test plus status eval proof.

## What Needs Cleanup

- Resolve canonical root dirty metadata before any push or release gate: `agentops-6xe`.
- Repair closure-integrity replay for the recovered eval epic: `agentops-aeg`.
- Add a direct security-toolchain governance suite for `scripts/toolchain-validate.sh`: `agentops-f6g`.
- Audit copied Beads CLI reference links in shared and Codex skills: `agentops-i6j`.
- When credentials and runtime CLIs are intentionally available, run live Claude/Codex eval adapters as a separate nightly/release proof step.

## Test Pyramid Assessment

| Level | Actual Coverage | Gap |
|---|---|---|
| L0 Contract | Eval suite/run schemas, contract compatibility, next-work parity. | None blocking. |
| L1 Unit | `cli/internal/eval` and `cli/cmd/ao` focused tests. | None blocking. |
| L2 Integration | `scripts/eval-agentops.sh --fast`, headless runtime, hook/doc/script gates. | None blocking. |
| L3 Component | Public product canary set across AgentOps surfaces. | Live runtime and Windows PowerShell execution remain environment-dependent. |
| BF4 Negative | Baseline regression, holdout isolation, policy fail, missing runtime and parser failure paths. | Closure-integrity replay gap needs cleanup. |
| BF9 Security | Security-suite behavioral gates and release-security gates. | Direct toolchain-validate governance should be added. |

## Prediction Accuracy

The pre-mortem WARN was directionally accurate. It predicted risk around runtime isolation, baseline promotion, validation separation, and proof drift. Runtime isolation and baseline drift were addressed with deterministic contracts and canaries; proof drift remains visible in closure-integrity and worktree disposition rather than product behavior.

## Knowledge Lifecycle

- Citations recorded for three applicable findings.
- One new local learning captured: `.agents/learnings/2026-04-25-eval-longhaul-closeout-disposition.md`.
- Four follow-up beads filed and one next-work batch appended.
- No stale learning retirement performed during this closeout pass.

## Flywheel: Next Cycle

Highest priority follow-up:

> **Repair closure-integrity evidence for eval epic children** (bug, high)
> Product validation passes, but the closure audit cannot replay evidence for three recovered epic children.

Ready to run:

```bash
$rpi "Repair closure-integrity evidence for eval epic children"
```

Or, if you want to clear the push blocker first:

```bash
$rpi "Resolve canonical root knowledge metadata dirtiness"
```
