# Skills Decision Tree

> Single source of truth for "which skill do I need next?"
> Linked from `skills/harvest/SKILL.md` and
> `skills/knowledge-activation/SKILL.md` (and their `skills-codex/`
> mirrors), plus `docs/index.md` and `docs/documentation-index.md`.

## Decision-Tree Naming Convention

Decision-tree docs follow `{subject}-decision-tree.md` (e.g., `skills-decision-tree.md`, `workflow-decision-tree.md`).

Rationale: groups by subject alphabetically in file listings; matches the existing `docs/skills-decision-tree.md`; keeps `decision-tree` as a suffix tag rather than a noisy prefix duplicated across many files.

## Global corpus flow (new users with `~/.agents/`)

1. **`$harvest`** — gather artifacts from many `.agents/` directories
   across your rigs, deduplicate cross-rig, promote high-value items
   into `~/.agents/learnings/`. Not a verbatim copy — an
   opinionated promotion of the unique, high-confidence artifacts.
2. **`$compile`** — synthesize the raw corpus into an interlinked
   wiki at `.agents/compiled/`. Large corpora are split into
   batches via `--batch-size` so a 2000+ file delta never lands in
   a single LLM prompt.
3. _(optional)_ **`$dream`** — overnight bounded compounding loop
   on top of the compiled corpus. Not interactive; runs to
   convergence or wall-clock, whichever comes first.
4. **`$knowledge-activation`** — lift compiled knowledge into
   playbooks, a belief book, and runtime briefings that future
   sessions read at bootstrap.

## Which skill do I need?

| I want to… | Use |
|------------|-----|
| Consolidate artifacts from many repos into one place | `$harvest` (writes `~/.agents/learnings/`) |
| Synthesize the raw corpus into an interlinked wiki | `$compile` (writes `.agents/compiled/`) |
| Overnight compounding + fitness-driven corpus improvement | `$dream` |
| Turn compiled knowledge into playbooks + beliefs for future sessions | `$knowledge-activation` |
| Check whether knowledge is actually compounding (velocity/friction) | `$flywheel` (reads `.agents/`, no writes) |
| Promote local learnings into shared team canon | `ao flywheel close-loop` (`.warmind/learnings/`) |
| Copy raw `.md` files verbatim without dedup | `rsync` (not AgentOps) |
| New project / new repo / first-time AgentOps setup | `ao quick-start`, then `$quickstart` |
| Full research → plan → implement → validate cycle | `$rpi` |
| Validate a plan or spec before implementation | `$pre-mortem` |
| Validate code quality after implementation | `$vibe` |

## Common "wait, which one?" disambiguations

**harvest vs compile.** Harvest moves artifacts between directories
(rig `.agents/` → global hub). Compile synthesizes artifacts into
higher-order output (wiki articles). Harvest is a physical operation;
compile is a semantic operation.

**~/.agents vs ~/.agents/learnings/.** Users often say "harvest all
to `~/.agents`" and mean the promotion hub. The promotion hub is the
`learnings/` subdirectory, which is why the harvest CLI emits
`--promote-to ~/.agents/learnings`. The outer `~/.agents/` directory
also contains `compiled/`, `playbooks/`, `packets/`, `knowledge/`,
`harvest/`, `mine/`, and `defrag/` — each owned by a different skill.

**compile vs knowledge-activation.** Compile builds the wiki.
Knowledge-activation turns the wiki into usable operator context
(beliefs, playbooks, briefings). Run compile first, then activation.
Running activation against an empty compiled dir is a no-op.

**compile vs dream.** Compile is interactive and bounded. Dream is
overnight and runs a compounding loop (harvest → compile → lint →
defrag → repeat until fitness plateaus). If you're sitting at the
terminal, use compile. If you're going to bed, use dream.

## Where durable state lands (skill → `ao` surface)

These skills carry the *judgment*; the `ao` binary carries the
*durable state*. Knowing which file each step writes keeps the flow
legible:

| Step | `ao` surface it shells into | Durable artifact |
|------|------------------------------|------------------|
| `$harvest` | `ao harvest`, `ao dedup`, `ao inject` | `~/.agents/learnings/` (promotion hub) |
| `$compile` | `ao compile` | `.agents/compiled/` (interlinked wiki) |
| `$dream` | `ao overnight` (same engine) | `.agents/overnight/*/summary.{json,md}` |
| `$knowledge-activation` | `ao knowledge`, `ao context` | playbooks + belief book + runtime briefings |

Two cross-cutting surfaces sit under every step:

- **Citations.** `ao inject` / `ao lookup` append a `CitationEvent`
  to `.agents/ao/citations.jsonl` every time corpus knowledge is
  read. Those citations feed confidence back into inject ranking —
  the loop closes through this file, not through any skill.
- **Promotion to team canon (`$flywheel`).** Local learnings flow
  `.agents/learnings/` → `.warmind/pool/staged/` → `.warmind/learnings/`
  (team canon). Tiers gate promotion: Gold (≥0.8) auto-promotes after
  24h; Silver (≥0.5) needs one external citation; Bronze needs three.
  Run `ao flywheel close-loop` (or `$flywheel` to inspect health)
  to promote + decay + contradict in one pass. This is the broadest
  single `ao` consumer — the RPI cluster, harvest, and compile all
  read through it.

## See also

- `skills/harvest/SKILL.md` — full harvest invocation
- `skills/compile/SKILL.md` — compile flags and runtimes
- `skills/knowledge-activation/SKILL.md` — activation surfaces
- `skills/dream/SKILL.md` — overnight compounding
- `skills/flywheel/SKILL.md` — knowledge-flywheel health + promotion
- `skills/quickstart/SKILL.md` — first-time setup
