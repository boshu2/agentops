# Council verdict — CDLC F6 + F7 findings (2026-05-16)

> **Provenance.** Excerpt from the original local council verdict at `.agents/council/2026-05-16-research-cdlc-claims.md` (gitignored — runtime corpus). Committed here so the [sovereignty proof page](../index.md#case-study-2-f6-f7-codex-trio-findings-the-claude-trio-missed-2026-05-16) has a falsifiable, in-repo source.

**Date:** 2026-05-16
**Mode:** research · mixed · deep (6 judges: 3 Claude-runtime + 3 Codex-runtime, independent / no preset)
**Target:** How the CDLC density machinery works in practice, and how to bridge claims → working skills/code/hooks.
**Consensus verdict:** **WARN (6/6 WARN, all HIGH confidence)** — cross-vendor consensus, no DISAGREE.

---

## Headline

The CDLC is **not** mostly aspirational — it has substantial working machinery (decay-ranked retrieval, a hard learnings quality gate, citation/MemRL feedback, an implemented σ×ρ metric). It loses PASS for two reasons every judge converged on:

1. The CDLC's **only named invariant — the Context Density Rule — has zero mechanical enforcement.** No code, hook, skill, or eval checks the six payloads.
2. The framing **mischaracterizes soft signals as hard gates**, and the flagship doc **points at a deprecated command** (`ao inject`, v3.0.0 removal target) for 3 of 7 phases.

The prior "three proxies" model was directionally right but **had the hardness backwards** — it credited a soft ranking bias as admission control and omitted the one genuine hard gate.

---

## F6 — CRITICAL: the Adapt-phase feedback loop has a misattribution bug *(codex-1, codex-3 — independent)*

Two Codex judges *independently* flagged the citation/quality-signal penalty path:

- `cli/cmd/ao/flywheel_citation_feedback.go` can **mis-join sessions** (timestamp-generated fallback session id, exact-match session filtering) and **does not filter citations by session** before applying utility penalties — corrections get misattributed or over-applied. (codex-1)
- `skill_loaded` citations point at `skills/<name>/SKILL.md`, but the feedback resolver only resolves `.agents/learnings|findings|patterns` — so **skill-correction penalties silently resolve to nothing.** (codex-3)

This is a real defect, not a framing issue: the "Adapt" phase the leverage hierarchy calls highest-leverage is partly mis-wired.

**File:line evidence (each resolves at HEAD):**
- Session filtering in citation feedback: `cli/cmd/ao/flywheel_citation_feedback.go:591` — "Load citations for this session and filter for skill_loaded entries."
- `skill_loaded` citation type literal: `cli/cmd/ao/flywheel_citation_feedback.go:599`.
- Real hard gate the prior model omitted: `cli/cmd/ao/inject_learnings.go:387` — `if !passesQualityGate(l) { ... }`.

---

## F7 — Observability claims outrun the manifest *(codex-1, codex-2, codex-3)*

`hooks/citation-tracker.sh` exists but is **not wired** into `hooks/hooks.json` PostToolUse(Read), so passive read-citation tracking is claimed but inactive. Hook coverage is also runtime-asymmetric — the Codex manifest at `hooks/codex-hooks.json` omits SessionEnd / context-guard / context-monitor — while CDLC prose presents one unified lifecycle.

**File:line evidence (each resolves at HEAD):**
- `hooks/citation-tracker.sh` exists (80 lines).
- `hooks/hooks.json` — `grep -n citation-tracker hooks/hooks.json` returns zero matches.
- `hooks/codex-hooks.json` — runtime-specific manifest exists separately.

---

## Cross-vendor comparison (`--mixed`)

Both vendor trios returned **unanimous WARN / HIGH** — strong cross-vendor consensus on the verdict. The split is in **emphasis and remedy**, and it tracks vendor cleanly:

| | Claude trio (judges 1–3) | Codex trio (judges 4–6) |
|---|---|---|
| Sharpest finding | trust_policy hardness-backwards; σ=0 in baselines vs present-tense docs | the execution-packet `density` schema gap; the F6 feedback misattribution **bug** |
| Highest-leverage fix | **Reconcile** — fix docs/paths to match reality | **Build** — make the six-field density contract schema-enforced |
| Unique catch | `passesQualityGate` is the omitted real hard gate | session-misjoin bug (F6); `citation-tracker.sh` not wired (F7); Claude/Codex hook asymmetry |

This is the high-signal disagreement: **lower the claim vs. raise the implementation.** It is not a contradiction — they address different gaps — but it is the real fork in "how do we make it work."

---

## Why this is the sovereignty proof artifact (case 2)

The F6 misattribution bug is a *behavior* defect — `flywheel_citation_feedback.go` was silently mis-applying utility penalties to wrong sessions and to skill citations that resolved to nothing. **No Claude judge surfaced it.** Two independent Codex judges did. F7 — the citation-tracker.sh wiring gap and the cross-runtime hook asymmetry — was a Codex-only catch as well.

If the resident vendor (Claude) had been the only reviewer, both findings would have shipped unobserved. The Codex trio is what made the verdict load-bearing.

---

## Outcome

F6 became a P1 follow-up bead. The build/reconcile split was resolved as "do both in the same wave" because the Codex `build` work and the Claude `reconcile` work addressed different gaps — they did not conflict. The packet schema was extended to encode the six-field `density` block (`schemas/execution-packet.schema.json` line 47 — `density: $ref Density` — was the gap-close).
