# Estate ablation — delete everything, measure what earns its way back

> **Goal (Bo, 2026-08-05):** apply the Cherny method to this repo's own
> agent-facing estate: delete in the ablation harness, use the models, add
> back only what they repeatedly stumble without, then cull main with
> receipts. Target models: gpt-5.6-{luna, terra, sol} (codex), Opus 5 and
> Fable (Claude subagent lanes).
> **Ground rule:** deletion happens in ARMS, never on main; the deletion PRs
> come last, carry per-line evidence, and land on Bo's ratify. (The source
> method itself: delete → use → add back only on repeated measured stumble —
> and the product may keep UX-serving prose that pure capability doesn't need.)

## Surfaces under ablation

| ID | Surface | Deletion lever |
|---|---|---|
| S1 | Repo operating contract (CLAUDE.md / AGENTS.md) | contract file present/absent in fixture |
| S2 | Skills corpus (per-runtime: codex `CODEX_HOME/skills`, Claude listing, gemini image) | bare CODEX_HOME vs full; Claude arms via subagents |
| S3 | Rules files (`.claude/rules/*.md`) | rules file present/absent in fixture |
| S4 | Hooks + gates (deterministic layer) | ALREADY MEASURED — gates removed 6/12 false-PASS, doctrine 0/12; the deterministic layer is a keeper by evidence, matching the source's own report that what remains in their harness is safety/permissions code |
| S5 | Bo's personal `~/.claude` stack (dotfiles repo) | OUT OF SCOPE until Bo ratifies — different repo, same method |

One lever per sweep — clean attribution, no confounded arms.

## Corpus ("delete, then USE it")

Tier-2 execution tasks (t01/t03/t04/t05: real work + hidden holdouts +
false-PASS instrument) + probe scenarios (behavior) + routing scenarios
(delivery). Metrics per run: hidden_pass (capability), false_pass (trust),
tokens (cost), and skill-delivery evidence where applicable.

## Registered predictions (written BEFORE sweep-1 results; from this week's data)

P1. Capability: bare ≈ full on strong models (tier-2 controls already ran
    contract-less at high success; wave-1/2a ceiling effects). If bare is
    *slightly better* (the vendor's own simple-mode finding), context-rot
    predicted it.
P2. Trust: neither arm moves false-PASS (doctrine 0/12); only gates do.
P3. Skills' real value on codex is doctrine-DELIVERY (4/4 routing), visible
    in behavior shape (e.g. MEASURED-block application), not task pass-rate.
P4. Contract file (S1) adds ~0 capability; its live value, if any, is
    product-behavior (stop/boundary/honesty shapes) — measurable only by
    behavior canaries, not task success.
P5. Cost: full arm spends more tokens (skill reads + longer context).

## Sweep schedule

1. **S2 skills-delivery, codex** — {bare, full} × {luna, terra} × 4 tasks × n=2 (RUNNING).
2. S1 contract-file — {absent, present-as-AGENTS.md} on the winning baseline arm.
3. S3 rules-file — same shape, Go tasks.
4. Claude-side arms — same tasks via in-session subagents {opus, fable}, small n
   (Claude quota); bare lever = fixture outside repo (no CLAUDE.md auto-load),
   which the tier-2 pilot already instantiated.
5. Add-back rounds: any surface whose {bare} arm shows repeated measured
   stumbles gets its content restored CLASS by CLASS (not wholesale) and
   re-measured; survivors enter the ledger with evidence.

## Endgame

`SURVIVORS.md` — per line-class: {dead | product-behavior (canary-backed) |
enforcement-backed (lives in code, prose deletable) | survivor (measured
stumble without it)} with evidence links → deletion PRs per surface, Bo
ratifies each. Per-runtime differentiated: codex keeps its delivery channel;
prose that only restates what gates enforce dies everywhere.
