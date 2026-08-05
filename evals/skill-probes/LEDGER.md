# Behavioral probe ledger

> **This file is hand-maintained, never generated.** It used to live inside
> `skills/SKILL-TIERS.md`, which is regenerated from SKILL.md frontmatter — a
> regeneration wiped the ledger and the (non-blocking) coverage gate silently
> reported 0/11 measured for weeks. A measured result cannot be derived from
> frontmatter, so it does not belong in a generated file. Parsed by
> `scripts/check-skill-probe-coverage.sh` (heading + table format are load-bearing).
>
> **HONESTY.** A probe verdict records BEHAVIOR-CHANGE (did loading the skill
> change what the agent DID), never quality-uplift. BEHAVIORAL = loading it
> changed the action; INERT = it did not; small N is directional, not
> statistical (ADR-0011). One row per (skill, probe, run) — append, don't
> overwrite: history is the point.

## Behavioral Probe Ledger (MEASURED)

| Skill | Probe | Date | Verdict | Notes |
|---|---|---|---|---|
| `crank` | `crank` | 2026-07-08 | INERT | gpt-5.5 separated write-scope-colliding beads unaided (1.0 vs 1.0 — no headroom at frontier); evidence: docs/evals/2026-07-08-skill-probe-crank.md |
| `graphify` | `graphify-tool-preference` | 2026-06-30 | INERT | 0/2 treatment agents obeyed a verbatim doc instruction; evidence: docs/evals/2026-07-08-skill-probe-graphify-calibration.md |
| `premortem` | `premortem-self-validation` | 2026-08-04 | BEHAVIORAL | gpt-5.6-luna: xhigh 0.5→1.0, low 0.0→1.0 — effect grows as producer weakens; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `standards` | `standards-go-conventions` | 2026-08-04 | BEHAVIORAL | gpt-5.6-luna: xhigh 0.5→1.0, low 0.0→1.0 — perfect separation at low effort; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `validate` | `validate-not-proven` | 2026-08-04 | INERT | ceiling at xhigh AND low (luna returns NOT_PROVEN unaided) — scenario needs hardening, not the skill; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `security` | `security-coverage-gap` | 2026-08-04 | INERT | ceiling at xhigh AND low (GAPPED chosen unaided); scenario needs hardening; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `reality-check` | `reality-check-gap` | 2026-08-04 | INERT | ceiling at xhigh AND low (gap named unaided); scenario needs hardening; evidence: docs/evals/2026-08-04-probe-wave-1.md |
| `crank` | `crank-luna` | 2026-08-04 | INERT | third config (luna xhigh + low, after gpt-5.5 xhigh): collision invariant is native to frontier models — cull/reshape candidate; evidence: docs/evals/2026-08-04-probe-wave-1.md |
