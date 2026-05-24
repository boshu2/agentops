# Hook-Noise Audit (3.0 reconciliation)

> **Bead:** `soc-6zihw` (W3 of the 3.0 reconciliation), resolved through `soc-e2ju0` S1–S3. **Criterion:** [docs/3.0.md](../3.0.md) — "hooks may help, but they must not inject random noise"; the 3.0-ready acceptance is *"every hook is a bounded adapter or a gate, none a noise-injector."* This audit originally classified all 53 hook scripts against that criterion; the noise-injectors have since been deleted, leaving **46 hooks, all gates or bounded adapters — none a noise-injector**. The acceptance box is met.
>
> **Status (go-hookless epic `soc-e2ju0`) — RESOLVED, all 7 noise-injectors deleted:** the operator chose to **delete** the noise-injectors outright, not quiet them (they are value-negative: A/B Δ=0). S1 (`soc-s1i3b`) deleted 5 — `research-loop-detector`, `context-monitor`, `write-time-quality`, `edit-knowledge-surface`, `go-vet-post-edit` — leaving 48 hooks. S2 (`soc-khev6`) deleted `standards-injector` and retired its dedicated `standards-injector-completeness` CI gate, leaving 47 hooks. S3 (`soc-vpmzg`) deleted `commit-review-gate` — the value-negative noise-injector that injected the staged diff + a "SELF-REVIEW" prompt on every `git commit` (never blocked, always exited 0) — leaving **46 hooks**. The "+7 conditional injectors" the audit originally grouped with the noise-injectors were reclassified as kept-gates (they speak only on a real violation), so S3 deleted only `commit-review-gate`. **No noise-injector remains; the docs/3.0.md "none a noise-injector" box is now met (`soc-cul67`).** The separate default-install-zero-hooks lift is S4 (ADR-0002), still open and not a 3.0-close blocker.

## Method

Each hook in `hooks/*.sh` is classified by **what it emits to the agent**, not how often it fires:

- **GATE** — blocks/denies an operation (`permissionDecision: deny` or non-zero exit). Bounded, intentional. Keep.
- **BOUNDED-ADAPTER** — does a bounded side-effect (regen, log, snapshot, cleanup, fail-open bootstrap) and stays silent unless it has a real result. Keep.
- **NOISE-INJECTOR** — pushes `additionalContext`/advisory stdout into the agent's prompt window **on every matching event regardless of relevance**. This is the 2.x failure mode docs/3.0.md calls out (the A/B showed Δ=0 from injected context). Cut or quiet.

The discriminator: *does the hook speak only when it blocks or detects a real problem (gate/adapter), or does it speak unconditionally on the event (noise)?* A hook that fires often but only speaks on a real violation is **not** noise.

## Summary

**Original audit (53 hooks, before deletion):**

| Category | Count | Original disposition |
|---|---|---|
| GATE | 14 | keep |
| BOUNDED-ADAPTER | 25 | keep |
| NOISE-INJECTOR | 14 | cut or quiet |

**Current state (46 hooks, after `soc-e2ju0` S1–S3 deleted the 7 true noise-injectors):**

| Category | Count | Disposition |
|---|---|---|
| GATE | 20 | keep — 13 original gates + the 7 conditional injectors reclassified as gates (speak only on a real violation) |
| BOUNDED-ADAPTER | 26 | keep |
| NOISE-INJECTOR | 0 | **all 7 deleted** |

The shift: the 14 entries originally bucketed as "noise-injectors" split into **7 true unconditional noise-injectors (deleted)** and **7 conditional injectors that only speak on a real violation (kept, reclassified as gates)**. `go-vet-post-edit` left the gate column too — it was one of the 7 deleted hooks, not a surviving blocking-path gate. **All 46 surviving hooks meet the 3.0 criterion: every one is a bounded adapter or a gate, none a noise-injector.**

## Noise-injectors (all 7 deleted)

The 7 true noise-injectors — those that emitted `additionalContext`/advisory stdout on **every** matching event regardless of relevance — were deleted outright across `soc-e2ju0` S1–S3. None survive. Listed below for the historical record, each with the stage that removed it.

| Hook | Event | Emitted additionalContext | Verdict |
|---|---|---|---|
| `research-loop-detector.sh` | PostToolUse:Read/Grep/Glob/Web* | escalating "you have made N read-only calls" from 8 reads | **DELETED in S1 (`soc-s1i3b`)** |
| `context-monitor.sh` | PostToolUse | context warnings at 35%/25% remaining (fired 3-5×/session) | **DELETED in S1 (`soc-s1i3b`)** |
| `write-time-quality.sh` | PostToolUse:Edit/Write | advisory quality warnings on routine writes | **DELETED in S1 (`soc-s1i3b`)** |
| `edit-knowledge-surface.sh` | PreToolUse:Edit | always — greps `.agents/learnings/` + "Relevant learnings" on every edit | **DELETED in S1 (`soc-s1i3b`)** |
| `go-vet-post-edit.sh` | PostToolUse (Go) | density warnings on every Go edit | **DELETED in S1 (`soc-s1i3b`)** |
| `standards-injector.sh` | PreToolUse:Edit/Write (6 file types) | always — JIT language standards on every edit | **DELETED in S2 (`soc-khev6`)** — replaced by the standards skill, read on demand; its `standards-injector-completeness` CI gate retired with it |
| `commit-review-gate.sh` | PreToolUse:Bash (`git commit`) | always — staged diff + "SELF-REVIEW" every commit (misnamed: never blocked, line 4 "Non-blocking, always exit 0") | **DELETED in S3 (`soc-vpmzg`)** — value-negative, no replacement |

The original audit grouped "+7 lower-frequency conditional injectors" with these. On closer inspection they only speak on a real violation, so they were **reclassified as kept-gates, not deleted** (see below).

**Reclassified as kept-gates (conditional, gated on a real violation — NOT noise):** `session-pr-counter.sh` (fires once at the 5-PR threshold), `check-test-pair-on-commit.sh`, `check-sibling-citation-on-commit.sh`, `update-principles-check.sh`, `codex-parity-warn.sh` (only on actual skills/ parity drift), `config-change-monitor.sh`, `context-guard.sh` (only on CRITICAL context). These speak only when a real condition holds, so they are bounded gates, not noise.

## Gates (20, keep)

The 13 original gates — `dangerous-git-guard`, `edit-scope-guard`, `go-test-precommit`, `go-complexity-precommit`, `holdout-isolation-gate`, `lead-only-worker-git-guard`, `git-worker-guard`, `pre-mortem-gate`, `stop-team-guard`, `task-validation-gate`, `skill-lint-gate`, `ao-agents-check`, `constraint-compiler` — plus the 7 reclassified conditional gates: `session-pr-counter`, `check-test-pair-on-commit`, `check-sibling-citation-on-commit`, `update-principles-check`, `codex-parity-warn`, `config-change-monitor`, `context-guard`. All block or speak only on a real violation and inject nothing on the happy path. (`go-vet-post-edit` was previously listed here for its blocking path — it was one of the 7 deleted hooks and no longer exists.)

## Bounded adapters (26, keep)

Silent or diagnostic side-effects: `session-start`, `ao-inject`, `ao-extract`, `ao-forge`, `ao-flywheel-close`, `ao-maturity-scan`, `ao-ratchet-status`, `ao-session-outcome`, `ao-task-sync`, `ao-feedback-loop`, `citation-tracker`, `compile-session-defrag`, `edit-audit`, `eval-verdict-compiler`, `factory-router`, `finding-compiler`, `pending-cleaner`, `postedit-codex-refresh`, `precompact-snapshot`, `quality-signals`, `ratchet-advance`, `session-end-maintenance`, `stop-auto-handoff`, `subagent-stop`, `worktree-setup`, `worktree-cleanup`.

## Resolution + follow-on

This audit was the analysis deliverable; the behavior change has since landed. Rather than flip the noise-injectors to opt-in-by-default, the operator deleted the 7 true noise-injectors outright (`soc-e2ju0` S1–S3) — they were value-negative (A/B Δ=0), so quieting them would have kept dead weight. With them gone, all 46 surviving hooks are gates or bounded adapters, and the docs/3.0.md "none a noise-injector" acceptance box is **met** (`soc-cul67`). The remaining default-install-zero-hooks lift is `soc-e2ju0` S4 (ADR-0002) — separate, still open, and not a 3.0-close blocker.
