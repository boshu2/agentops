# Sovereignty proof — how we know mixed-model review actually works

**The claim:** AgentOps lets you own the corpus that feeds your coding agents, run any vendor's model against it, and route review across vendors so one trio cannot rubber-stamp another's blind spots.

**The proof:** two committed verdicts where an independent vendor trio surfaced a load-bearing finding the other trio missed. Both are in this repository; both cite file:line evidence that you can resolve against the current tree.

This page is the falsifiable artifact behind the README hero. If the cited files don't exist, or the cited lines don't say what we claim, the proof is broken and CI fails (see [Verification](#verification) below).

---

## Why this matters (one paragraph)

Same-vendor judging is comfortable but soft: three Claude judges share the same training prior and converge on the same blind spots. The fungibility-first architecture (corpus-as-source-of-truth, stateless agents, any vendor can claim any bead) only pays off if you can **demonstrate** that swapping vendors catches things the resident vendor missed. The two case studies below are the receipts.

---

## Case study 1 — RPI-leanness reframe (2026-05-15)

**Setup.** Operator's thesis: "RPI has become too waterfall — too much context, too few cycles." Council ran `--mixed --deep` across one Codex (gpt-5.5) judge and four Claude-vendor judges (Agile coach, DDD architect, token economist, skeptic).

**The Codex reframe — independently load-bearing.** Codex did not vote with the operator's framing. Instead, gpt-5.5 reframed the problem at the diagnosis layer:

> "RPI is waterfall" is the wrong diagnosis. RPI's execution phase is already wave-based and fairly Agile. The real disease is narrower and more fixable: **discovery + planning manufacture `plan.md` as a shared god artifact**, then every later phase rehydrates pieces of it. Bloat is an *artifact-boundary* bug, not a *methodology* failure.

The skeptic seat agreed. The other three Claude voices then aligned. **Without the Codex reframe, the operator would have planned a methodology rewrite. With it, the planned slice became `ExecutionPacketPort` as a typed ≤8 KB contract — a one-port surgical change, not a meta-framework.**

**File:line evidence the verdict cited (each resolves today):**

| Claim | Citation |
|---|---|
| `/plan` requires symbol-level detail per file | [skills/plan/SKILL.md:112][plan-112] |
| `/plan` requires a full file-dependency matrix | [skills/plan/SKILL.md:160][plan-160] |
| `plan.md` is named as runtime input vs commentary | [skills/plan/SKILL.md:166][plan-166] |
| The canonical plan template | [skills/plan/SKILL.md:170][plan-170] |
| Per-worker "full project context" briefing | [skills/crank/SKILL.md][crank-skill] |
| `ao inject` is deprecated (v3.0.0 removal target) | [cli/cmd/ao/inject.go:155][inject-deprecation] |
| `trustTierWeight` is additive (soft prior, not gate) | [cli/cmd/ao/context_relevance.go:217][trust-tier] |

[plan-112]: https://github.com/boshu2/agentops/blob/main/skills/plan/SKILL.md#L112
[plan-160]: https://github.com/boshu2/agentops/blob/main/skills/plan/SKILL.md#L160
[plan-166]: https://github.com/boshu2/agentops/blob/main/skills/plan/SKILL.md#L166
[plan-170]: https://github.com/boshu2/agentops/blob/main/skills/plan/SKILL.md#L170
[crank-skill]: https://github.com/boshu2/agentops/blob/main/skills/crank/SKILL.md
[inject-deprecation]: https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/inject.go#L155
[trust-tier]: https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/context_relevance.go#L217

**Full verdict (committed):** [evidence/2026-05-15-rpi-leanness-codex-reframe.md](evidence/2026-05-15-rpi-leanness-codex-reframe.md).

**Outcome.** PR #275 (`crank/ddd-hexagonal-2026-05-12`) was already in-flight implementing this reframe; the council recovered it as canonical instead of re-deriving. `soc-etwf.1/.2/.3` were closed as superseded. The mis-scoped methodology epic was averted.

---

## Case study 2 — F6 + F7: Codex-trio findings the Claude trio missed (2026-05-16)

**Setup.** Research-mode council on the CDLC density machinery. Six judges: three Claude-runtime + three Codex-runtime, fully independent. Consensus verdict: **WARN (6/6, HIGH confidence)**.

**F6 — session-misjoin bug in the Adapt-phase feedback loop.** Two Codex judges *independently* flagged a real defect in `cli/cmd/ao/flywheel_citation_feedback.go`:

- The session-id fallback path could misjoin sessions when the runtime session id was timestamp-derived.
- Citation lookup did not filter by `session_id` before applying utility penalties — corrections were getting misattributed or over-applied.
- `skill_loaded` citations resolve `skills/<name>/SKILL.md`, but the feedback resolver only resolves `.agents/learnings|findings|patterns` — so skill-class corrections silently resolved to nothing.

The Claude trio's emphasis was framing ("reconcile the docs to the code"). The Codex trio's emphasis was correctness ("the code itself is mis-wired"). **F6 is a behavior bug, not a doc problem. No Claude judge surfaced it.**

**File:line evidence:**

| Claim | Citation |
|---|---|
| Session filtering in citation feedback | [cli/cmd/ao/flywheel_citation_feedback.go:591][f6-feedback] |
| `skill_loaded` citation type | [cli/cmd/ao/flywheel_citation_feedback.go:599][f6-skill-loaded] |
| Real hard gate the prior model omitted | [cli/cmd/ao/inject_learnings.go:387][quality-gate] |

[f6-feedback]: https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/flywheel_citation_feedback.go#L591
[f6-skill-loaded]: https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/flywheel_citation_feedback.go#L599
[quality-gate]: https://github.com/boshu2/agentops/blob/main/cli/cmd/ao/inject_learnings.go#L387

**F7 — hook manifest asymmetry between runtimes.** All three Codex judges noted that `hooks/citation-tracker.sh` exists but is *not* wired into `hooks/hooks.json` PostToolUse(Read), so the documented "passive read-citation tracking" is inactive. They also flagged that the Codex manifest omits SessionEnd / context-guard / context-monitor — so the unified lifecycle is asymmetric across runtimes.

| Claim | Citation |
|---|---|
| `citation-tracker.sh` exists | [hooks/citation-tracker.sh][citation-tracker] |
| Not wired in `hooks/hooks.json` (grep yields zero matches) | [hooks/hooks.json][hooks-json] |
| Codex manifest exists separately | [hooks/codex-hooks.json][codex-hooks] |

[citation-tracker]: https://github.com/boshu2/agentops/blob/main/hooks/citation-tracker.sh
[hooks-json]: https://github.com/boshu2/agentops/blob/main/hooks/hooks.json
[codex-hooks]: https://github.com/boshu2/agentops/blob/main/hooks/codex-hooks.json

**Full verdict (committed):** [evidence/2026-05-16-cdlc-f6-f7-codex-findings.md](evidence/2026-05-16-cdlc-f6-f7-codex-findings.md).

**Outcome.** F6 became a P1 follow-up bead; reconciliation work landed alongside the build work in the same wave (the cross-vendor split forced "lower the claim AND raise the implementation," not pick-one).

---

## What this is NOT

- **Not a benchmark claim.** The v1 retrieval workbench is saturated; we don't claim Δ>0 on it. The proof here is artifact-as-evidence, not score-as-evidence.
- **Not a vendor superiority claim.** Both case studies have findings where the Claude trio outperformed (e.g., the omitted hard quality gate in F2). The point is that *cross-vendor* review catches what *same-vendor* review misses.
- **Not a one-off.** The pattern is reproducible: run `/council --mixed --deep` against any high-stakes design or audit decision in this repo. The pattern is documented in `skills/council/SKILL.md`.

---

## Verification

The CI gate `validate-sovereignty-proof-citations` scans this page and the committed evidence files for `file:line` citations and verifies each resolves at HEAD. If a cited file is deleted or its line count shrinks below a cited line number, the gate fails and this page becomes a P0 fix.

**Run locally:** `bash scripts/validate-sovereignty-proof-citations.sh`

**Why the gate exists.** A proof page that lies about its citations is worse than no proof page at all. The gate is the mechanical enforcement that keeps this surface honest.

---

## See also

- [Council skill](https://github.com/boshu2/agentops/blob/main/skills/council/SKILL.md) — how to invoke `/council --mixed --deep`
- [Expert council skill](https://github.com/boshu2/agentops/blob/main/skills/expert-council/SKILL.md) — persona-debate variant for fungibility decisions
- [Operating loop](../architecture/operating-loop.md) — where council fits in the 7-move doctrine
