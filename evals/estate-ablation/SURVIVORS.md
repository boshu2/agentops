# Survivors ledger — repo estate, line-class granularity

> Verdicts: DEAD (delete; no measured stumble without it) · ENFORCED (a gate/
> hook carries it; prose deletable, pointer stays) · PRODUCT (keeps product
> behavior; needs a canary to stay) · SURVIVOR (measured stumble without it)
> · TBD. Every non-TBD verdict must cite a sweep/probe/pilot result.

## S1 — repo CLAUDE.md (operating contract), seeded predictions

| Section/class | Predicted | Existing evidence | Verdict |
|---|---|---|---|
| Core loop (RPI narrative) | DEAD for capability | tier-2 controls executed plans fine without it; doctrine 0/12 mid-task | TBD (sweep 2) |
| Runtime floor (`claude -p` ban) | ENFORCED | no-claude-p-guard hook exists; prose is a pointer | TBD |
| Constraint floor (python ratchet etc.) | ENFORCED | check-skill-python-ratchet.sh gate | TBD |
| Authority/trust + source precedence | PRODUCT | shapes conflict-resolution behavior; no capability effect expected | TBD (needs canary) |
| Product boundary / closeout / report shapes | PRODUCT | premortem probe: models don't do report-shape rules unprompted at low effort | TBD (needs canary) |
| Concurrency rules | ENFORCED-ish (reservations/worktrees tooling) + rt-01 showed native competence | routing rt-01: correct collision handling unaided | TBD |
| Triggered-sources table | DEAD (lookup docs, not instruction) | never observed load-bearing in any transcript | TBD |

## S2 — skills corpus: per-runtime, seeded from routing + probes

| Runtime | Prediction | Evidence |
|---|---|---|
| codex | KEEP (sole delivery channel; routes 4/4; applied MEASURED blocks) | routing batch 3 |
| Claude-in-repo | REDUNDANT for measured cores (rules auto-load + native) | routing batch 2, rt-06 |
| gemini image | unmeasured | — |

Sweep results append below.

## Sweep 1 — S2 skills-delivery, codex ({bare CODEX_HOME} vs {full}), 2026-08-05

n=8 per arm per model, tier-2 tasks t01/t03/t04/t05, effort low, directional.
Arm isolation asserted: 16/16 bare transcripts free of the skills-context
injection, 0/16 contain it. Raw: results/sweep1-{luna,terra}.jsonl.

| Model | Arm | hidden_pass | false_pass | avg tokens |
|---|---|---|---|---|
| gpt-5.6-luna | bare | 0/8 | 8/8 | 16,814 |
| gpt-5.6-luna | full | 1/8 | 7/8 | 18,157 |
| gpt-5.6-terra | bare | 2/8 | 5/8 | 21,071 |
| gpt-5.6-terra | full | 1/8 | 5/8 | 23,615 |

Registered predictions, tested as written: **P1 HOLDS** (capability delta ≈ 0
both models; terra's bare arm slightly leads — the simple-mode direction).
**P2 HOLDS** (false-PASS unmoved by arm; varies by MODEL — terra 5/8 vs luna
7-8/8 — more than by skills-presence). **P5 HOLDS** (full arm +8% luna /
+12% terra tokens for no measured execution benefit).

**S2-codex verdict: SPLIT.** KEEP the corpus as the sole doctrine-delivery
channel (routing batch 3: 4/4, MEASURED-block application) — but the
always-on skills injection into every execution run is the measured waste
(~1.3-2.5k tokens/run, zero effect). The deletion-shaped fix is SCOPING
(load skills for advice/routing-shaped work, not unconditionally), not
corpus deletion. Candidate mechanism for the proposal PR: task-class-gated
skill injection on the codex adapter.

## Sweep 2 — S1 operating contract ({fixture AGENTS.md = repo CLAUDE.md} vs absent), 2026-08-05

Skills held constant (bare CODEX_HOME both arms); n=8 per arm per model,
effort low, directional. Raw: results/sweep2-contract.jsonl.

| Model | Arm | hidden_pass | false_pass | avg tokens |
|---|---|---|---|---|
| gpt-5.6-luna | contract | 2/8 | 6/8 | 19,108 |
| gpt-5.6-luna | no contract | 2/8 | 6/8 | 17,451 |
| gpt-5.6-terra | contract | 0/8 | 6/8 | 21,313 |
| gpt-5.6-terra | no contract | 2/8 | 5/8 | 18,939 |

**P4 HOLDS, strengthened:** capability identical (luna) or directionally
WORSE with the contract (terra — the context-rot signature); trust unmoved
or slightly worse; +9-12% tokens per run. The decisive detail: the contract
carries the fresh-validation doctrine verbatim ("the context that authors a
candidate cannot issue its binding PASS"), it sat in context for all 16
contract-arm runs, and false-PASS never once improved relative to absence.
**S1 verdict class confirmed at surface granularity: ENFORCED-or-nothing —
the contract's execution-relevant prose only works where a gate carries it.
Its remaining candidate value is product-behavior shaping (report formats,
boundaries) for interactive/orchestration sessions — canary territory, not
execution territory.** Line-class add-back rounds can now target ONLY the
product-behavior sections; the capability/trust sections are measured dead
on two models.
