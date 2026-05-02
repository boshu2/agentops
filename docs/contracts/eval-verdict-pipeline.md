# Contract: Eval Verdict Pipeline

> **Status: Wave 1 stub.** Full contract documentation deferred to Wave 1.5.

## Purpose

Closed-loop edge of the AgentOps self-modification flywheel. Run manifests
carry `verdict` records (per `~/.agents/evals/SCHEMA.md` §4); the
verdict-compiler hook reads those and mutates `.agents/learnings/`
frontmatter so the corpus learns from outcomes.

## Pipeline

```
Run produces verdict (kind, utility, applicable_artifacts[])
        ↓
hooks/eval-verdict-compiler.sh
        ↓
.agents/learnings/<artifact>.md frontmatter mutation
   (utility EMA, harmful_count++, reward_count++, last_verdict stamp)
        ↓
.agents/rpi/next-work.jsonl (when threshold breached: queue retire)
```

## Inputs

- Run manifest at `~/.agents/evals/runs/<run-id>/manifest.json`,
  `status=complete`, non-null `verdict`. Verdict accepts both legacy
  rc2 string form (`"verdict": "improved"`) and rc3 struct form
  (`{kind, delta_point, ci_low, ci_high, utility, applicable_artifacts[],
  notes}`) per pre-mortem C1.
- Watermark at `~/.agents/evals/processed.jsonl` (idempotency).

## Outputs

- Frontmatter mutations: `utility:` (EMA), `harmful_count:`,
  `reward_count:`, `last_verdict:`.
- Next-work queue when `harmful_count ≥ HARMFUL_THRESHOLD` (3) AND
  `utility < LOW_UTILITY_THRESHOLD` (0.3).

## Path resolution

Per pre-mortem C2: absolute paths honored as-is; relative paths
resolved against agentops repo root; `~` expanded.

## Seed source

Per pre-mortem C3: when `applicable_artifacts == []`, derive from the
run's `harness.lock.json` files under `.agents/learnings/`,
`.agents/patterns/`, or `.agents/playbooks/`. Wave-5 Layer-4
attribution will replace this stub.

## Normative constants

Defined in SCHEMA.md §6 (rc3). Override via env or GOALS.md:

| Constant | Default | Env override |
|---|---|---|
| `EMA_ALPHA` | `0.7` | `AGENTOPS_VERDICT_EMA_ALPHA` |
| `EMA_BETA` | `0.3` | `AGENTOPS_VERDICT_EMA_BETA` |
| `HARMFUL_THRESHOLD` | `3` | `AGENTOPS_VERDICT_HARMFUL_THRESHOLD` |
| `LOW_UTILITY_THRESHOLD` | `0.3` | `AGENTOPS_VERDICT_LOW_UTILITY` |

## Wiring

- **SessionEnd**: `hooks/session-end-maintenance.sh` invokes when
  `AGENTOPS_EVAL_VERDICT_ENABLED=1` (kill-switch-independent gate per
  pre-mortem H1).
- **Manual**: `bash hooks/eval-verdict-compiler.sh [--dry-run] [--manifest <path>]`.

## Cross-links

- Sibling: [Finding Registry Contract](finding-registry.md)
- Schema: `~/.agents/evals/SCHEMA.md` §4 / §6 / §11 entry #18
- Plan: `.agents/plans/2026-05-01-eval-as-self-pruning-corpus.md`
- Pre-mortem: `.agents/council/2026-05-01-pre-mortem-eval-corpus*.md`

## Wave 1.5 follow-ups

- Full contract doc replacing this stub
- `lib/finding-compiler-helpers.sh` extraction
- Two-phase commit (`WATERMARK_TMP` buffering) for crash recovery
- 9-marker test (current: 5)
- `ao eval verdict apply` Go subcommand for cross-runtime parity
