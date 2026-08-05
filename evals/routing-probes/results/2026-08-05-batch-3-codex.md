# Routing batch 3 — codex runtime, 2026-08-05 (seed 2)

> **HONESTY.** N=4, `codex exec` workers (gpt-5.6-sol @ xhigh, read-only, repo
> cwd), template-instantiated fresh strings. Runtime and model family change
> TOGETHER vs batches 1–2 (sonnet subagents) — runtime effect and model effect
> are confounded in this comparison; labeled accordingly. Scoring is
> transcript-verified (explicit invocation line + skill-body read), not
> self-report alone: one self-report was malformed (rt-06 echoed the TOOLS
> placeholder) and one initial grep pattern missed reads via the canonical
> `skills/` path — both caught by transcript inspection.

| Scenario | Applicable | Outcome | Evidence in transcript |
|---|---|---|---|
| rt-02 claim-audit | reality-check | **ROUTED** | "I'm using the `agentops:reality-check` skill because…" + read `skills/reality-check/SKILL.md` |
| rt-03 plan-challenge | premortem | **ROUTED** | invocation + read SKILL.md **including the wave-1 MEASURED block** + finding #1 = self-certification, explicitly applying "the skill's key check" |
| rt-04 verdict-request | validate | **ROUTED** | "I'm using `agentops:validate`" invocation |
| rt-06 conventions | standards | **ROUTED** | "applying `agentops:standards`" + SKILL.md + the `references/common-standards.md` + `references/go.md` chain (progressive disclosure worked) |

**Codex routing: 4/4. Claude-subagent clean routing (batches 1–2): 2/6.**

## Findings

1. **The loop closed on camera (rt-03).** Wave 1 measured premortem's
   evidence-shape doctrine (BEHAVIORAL) → the measured core was hoisted into
   the SKILL.md (PR #1033) → an unprompted codex worker routed to the skill →
   loaded the hoisted MEASURED block → and named the planted self-certification
   flaw first, attributing it to the skill's key check. Measure → improve →
   route → apply, across two runtimes, inside one day.
2. **Runtime shapes routing more than task shape.** The same judgment-posture
   scenarios that never routed on Claude subagents (premortem, validate)
   routed 100% on codex, with an announce-then-load protocol ("I'm using X
   because Y") the Claude side never exhibited. Plausible mechanisms, not yet
   separated: codex's compact invocable skills list + selection norm vs the
   Skill tool among many tools; absence of alternate channels on codex (no
   .claude/rules auto-load) raising the perceived need; model-family habits
   (sol vs sonnet — confounded). A sonnet-on-bare-fixture arm would separate
   channel absence from runtime norm.
3. **Product implication:** on the codex runtime the skill library IS the
   doctrine-delivery channel — routing there is load-bearing and currently
   strong. On Claude-in-this-repo, delivery is redundant across channels
   (rules auto-load, memory, native competence). Skill investment pays
   differently per runtime; the eval program should report per-runtime
   delivery, not one routing number.
