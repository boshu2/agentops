# Skills Decision Tree

> Single source of truth for "which skill do I need next?"
> Linked from `skills/harvest/SKILL.md`, `skills/compile/SKILL.md`,
> `skills/knowledge-activation/SKILL.md`, `skills/quickstart/SKILL.md`,
> and their `skills-codex/` mirrors.

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
| Copy raw `.md` files verbatim without dedup | `rsync` (not AgentOps) |
| New project / new repo / first-time AgentOps setup | `$quickstart` |
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

## See also

- `skills/harvest/SKILL.md` — full harvest invocation
- `skills/compile/SKILL.md` — compile flags and runtimes
- `skills/knowledge-activation/SKILL.md` — activation surfaces
- `skills/dream/SKILL.md` — overnight compounding
- `skills/quickstart/SKILL.md` — first-time setup
