# Hook-Noise Audit (3.0 reconciliation)

> **Bead:** `soc-6zihw` (W3 of the 3.0 reconciliation). **Criterion:** [docs/3.0.md](../3.0.md) — "hooks may help, but they must not inject random noise"; the 3.0-ready acceptance is *"every hook is a bounded adapter or a gate, none a noise-injector."* This audit classifies all 53 hook scripts against that criterion. Edits to hook behavior are tracked separately (see Follow-on).
>
> **Status (go-hookless epic `soc-e2ju0`):** operator chose to **delete** the noise-injectors outright, not quiet them (they are value-negative: A/B Δ=0). S1 (`soc-s1i3b`) deleted 5 — `research-loop-detector`, `context-monitor`, `write-time-quality`, `edit-knowledge-surface`, `go-vet-post-edit` — leaving 48 hooks. Remaining noise tier (`standards-injector`, `commit-review-gate`, + conditional injectors) deletes in S2–S3; default install goes to zero hooks in S4.

## Method

Each hook in `hooks/*.sh` is classified by **what it emits to the agent**, not how often it fires:

- **GATE** — blocks/denies an operation (`permissionDecision: deny` or non-zero exit). Bounded, intentional. Keep.
- **BOUNDED-ADAPTER** — does a bounded side-effect (regen, log, snapshot, cleanup, fail-open bootstrap) and stays silent unless it has a real result. Keep.
- **NOISE-INJECTOR** — pushes `additionalContext`/advisory stdout into the agent's prompt window **on every matching event regardless of relevance**. This is the 2.x failure mode docs/3.0.md calls out (the A/B showed Δ=0 from injected context). Cut or quiet.

The discriminator: *does the hook speak only when it blocks or detects a real problem (gate/adapter), or does it speak unconditionally on the event (noise)?* A hook that fires often but only speaks on a real violation is **not** noise.

## Summary

| Category | Count | Disposition |
|---|---|---|
| GATE | 14 | keep |
| BOUNDED-ADAPTER | 25 | keep |
| NOISE-INJECTOR | 14 | quiet (flip to opt-in default) or raise thresholds |

39 of 53 hooks (74%) already meet the 3.0 criterion. The 14 noise-injectors all already honor a per-hook disable env var and `AGENTOPS_HOOKS_DISABLED`; the reconciliation is to flip their **default** from opt-out to opt-in (or raise their thresholds) so the default experience is quiet.

## Noise-injectors (cut/quiet candidates)

Ranked by injection frequency × unconditionality. Each cites the emission site and the existing disable flag.

| Hook | Event | Emits additionalContext | Disable flag (exists) | Verdict |
|---|---|---|---|---|
| `standards-injector.sh` | PreToolUse:Edit/Write (6 file types) | always — JIT language standards on every edit | `AGENTOPS_STANDARDS_INJECTOR_DISABLED` | quiet: opt-in default |
| `commit-review-gate.sh` | PreToolUse:Bash (`git commit`) | always — staged diff + "SELF-REVIEW" every commit (misnamed: never blocks, line 4 "Non-blocking, always exit 0") | `AGENTOPS_COMMIT_REVIEW_DISABLED` | quiet: opt-in default + rename (not a gate) |
| `edit-knowledge-surface.sh` | PreToolUse:Edit | always — greps `.agents/learnings/` + "Relevant learnings" on every edit | `AGENTOPS_EDIT_KNOWLEDGE_DISABLED` | quiet: opt-in default or filename-filter |
| `research-loop-detector.sh` | PostToolUse:Read/Grep/Glob/Web* | escalating "you have made N read-only calls" from 8 reads | `AGENTOPS_RESEARCH_LOOP_DISABLED` | quiet: raise thresholds (8→16) or opt-in |
| `context-monitor.sh` | PostToolUse | context warnings at 35%/25% remaining (fires 3-5×/session) | (threshold env) | quiet: raise to ~15% remaining |
| `write-time-quality.sh` | PostToolUse:Edit/Write | advisory quality warnings on routine writes | (per-lang) | quiet: opt-in default |
| `go-vet-post-edit.sh` | PostToolUse (Go) | density warnings on every Go edit | (always-on by design) | quiet: only on real vet failure |
| (+7 lower-frequency conditional injectors) | various | conditional advisory prose | per-hook flags | quiet/monitor |

**Kept despite the "gate"/"warn" name (conditional, gated on a real violation — NOT noise):** `session-pr-counter.sh` (fires once at the 5-PR threshold), `check-test-pair-on-commit.sh`, `check-sibling-citation-on-commit.sh`, `update-principles-check.sh`, `codex-parity-warn.sh` (only on actual skills/ parity drift), `config-change-monitor.sh`, `context-guard.sh` (only on CRITICAL context). These speak only when a real condition holds, so they are bounded gates, not noise.

## Gates (14, keep)

`dangerous-git-guard`, `edit-scope-guard`, `go-test-precommit`, `go-complexity-precommit`, `holdout-isolation-gate`, `lead-only-worker-git-guard`, `git-worker-guard`, `pre-mortem-gate`, `stop-team-guard`, `task-validation-gate`, `skill-lint-gate`, `ao-agents-check`, `constraint-compiler`, `go-vet-post-edit`(blocking path) — all block on a real violation and inject nothing on the happy path.

## Bounded adapters (25, keep)

Silent or diagnostic side-effects: `session-start`, `ao-inject`, `ao-extract`, `ao-forge`, `ao-flywheel-close`, `ao-maturity-scan`, `ao-ratchet-status`, `ao-session-outcome`, `ao-task-sync`, `ao-feedback-loop`, `citation-tracker`, `compile-session-defrag`, `edit-audit`, `factory-router`, `finding-compiler`, `pending-cleaner`, `postedit-codex-refresh`, `precompact-snapshot`, `quality-signals`, `ratchet-advance`, `session-end-maintenance`, `subagent-stop`, `worktree-setup`, `worktree-cleanup`, `config-change-monitor`(bounded path).

## Follow-on (behavior change, tracked separately)

This audit is the analysis deliverable. Flipping the 14 noise-injectors to opt-in-by-default (or raising thresholds) is a behavior change to the operator's hook environment, tracked as a follow-on so the default-quiet transition is deliberate and reviewable. It satisfies the docs/3.0.md acceptance box once landed. The default-install-zero-hooks lift is ADR-0002 S2–S5 (separate, not a 3.0-close blocker).
