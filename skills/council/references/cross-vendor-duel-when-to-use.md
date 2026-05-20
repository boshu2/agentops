# Cross-Vendor Duel — When To Use It

[dueling-route.md](dueling-route.md) covers the **mechanics** of
`/council --mode=debate`. This doc covers the **strategic call**: when
the cross-vendor variant (Opus + Codex, not all-Claude) is worth the
~30-minute model cost, plus the score-symmetry diagnostic that tells you
the duel itself worked.

## When To Use The Cross-Vendor Variant

| Decision shape | All-Claude (persona-only)? | Cross-vendor (Opus + Codex)? |
|---|---|---|
| Naming, tone, copy | ✓ Sufficient | overkill |
| Implementation pattern with established precedent | ✓ Sufficient | overkill |
| Contested **design** decision, two capable models would plausibly disagree | weak | **✓ Worth it** |
| AGENTS.md / CLAUDE.md / branch-protection edits | weak | **✓ Worth it** |
| Architecture trade-off with no obvious right answer | weak | **✓ Worth it** |
| Domain where one model is decisively more capable | one-sided | wasted spend |
| Already aligned with operator's preference | confirmation theater | confirmation theater |

The rule: single-model proposals embed the model's bias unexamined.
Cross-vendor cross-scoring forces each model to defend specifics,
surfaces real misses (model A invents something B catches as
fictitious; B literalizes a failure mode A named in the briefing), and
produces a verdict no single proposal could have.

## The Score-Symmetry Diagnostic

When you run a cross-vendor duel, the **whole-ballot scores** tell you
whether the duel itself worked:

| Score pattern | Interpretation |
|---|---|
| Both ballots within **12 points** of each other (e.g., 632/620) | **Healthy** — neither model dominated; mutual concessions happened |
| Both ballots above **800** | **Love-fest** — re-score with explicit candor instruction; current scores embed politeness, not adversarial judgment |
| Gap **> 200 points** | One model was decisively more capable for this domain, OR the briefing was biased toward one. Re-read the briefing's "tension to confront" section |
| Symmetric **kills** (each model kills ≥2 of the other's recommendations) | **Healthy** — real adversarial engagement |
| Zero mutual kills | Suspicion: tribal defensiveness or shared blind spot. Probe the reveal harder |

## Evidence (anchored)

> "The 2026-05-17 SDLC-shape council ran Opus 4.7 vs Codex gpt-5.5 in 3
> rounds: independent proposals, adversarial cross-scoring 0-1000 per
> recommendation, then reveal + concessions. Whole-ballots came in at
> 632/1000 (Codex→Opus) and 620/1000 (Opus→Codex) — symmetric, neither
> dominated."
— `.agents/learnings/2026-05-17-cross-vendor-duel-converges.md`

> "The central tension (one canonical doc vs three thin docs) converged
> in Round 3: both moved to 'one short pocket doc + operating-loop
> spine for agents; three axis owners generated or schema-gated as
> sources.' Codex caught Opus's invented `bd lease`. Opus caught
> Codex's 'three thin docs' as the literal failure mode the briefing
> named. The synthesis is in DUEL.md."
— `.agents/learnings/2026-05-17-cross-vendor-duel-converges.md`

The concrete proof: Codex caught a fabrication (Opus's `bd lease` —
not a real command). Opus caught a literalization mistake (Codex
proposing exactly the failure mode the briefing named as a thing to
avoid). Neither would have surfaced under a single-model run; both
surfaced naturally under adversarial cross-scoring.

## How To Apply

1. **Triage the decision.** Is it on the "Worth it" row above? If not,
   default to all-Claude personas (faster, cheaper, fine for most
   things).
2. **Write the briefing with explicit tensions.** The duel's fuel is the
   "tension you must confront" section in BRIEFING.md. Without a
   contestable claim, the duel converges politely and produces nothing.
   See [dueling-route.md](dueling-route.md) Phase 3.
3. **Spawn one Opus pane + one Codex pane.** Codex needs a real API key
   (NTM rejects `gpt-*-codex` on ChatGPT-billed accounts — see
   `dueling-route.md` NTM gotchas). If unavailable, fall back to
   all-Claude.
4. **Run all 3 rounds.** Independent verdicts → adversarial cross-score
   → reveal + blind-spot probe. **Don't skip the reveal** — that's
   where concessions happen.
5. **Read the score-symmetry diagnostic above.** If healthy, the
   synthesis is binding. If love-fest or one-sided, re-score with
   explicit candor or re-write the briefing.
6. **Synthesize the binding `DUEL.md`** from both ballots + mutual kills
   + blind-spot answers. Persist to `.agents/council/<topic>-<date>/`.

## Failure Modes To Watch

+ **Models too aligned.** If Opus and Codex agree on everything for
  your domain, the duel produces no friction (love-fest scoring). The
  fix is to pick personas that have real disagreements, not to bigger
  the model gap.
+ **Briefing without tension.** A briefing that frames the choice
  neutrally ("which option is better?") lets both models hedge. Frame
  the contestable claim: "I'm going to do X; tell me why I'm wrong."
+ **Skipping the reveal.** This is the one rule with no exception. The
  reveal is where each model confronts how the other scored *its* work;
  concessions only happen here.
+ **Orchestrator editorializing.** Report scores faithfully. The
  orchestrator's opinion goes only in the meta-analysis section.

## See Also

+ [dueling-route.md](dueling-route.md) — the mechanics this doc layers
  on top of
+ [adversarial-protocol.md](adversarial-protocol.md) — the
  cross-scoring protocol used in Phase 5
+ [multi-agent-architecture.md](multi-agent-architecture.md) — pane
  orchestration shape
+ Empirical reference: `.agents/council/sdlc-shape-2026-05-17/DUEL.md`
  (local, gitignored) — the 632/620 SDLC-shape result this doc anchors
