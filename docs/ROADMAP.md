# Roadmap (planned, not committed)

> Features designed or discussed but **not yet implemented**. Nothing here has a committed timeline; items may change or be dropped. If a command or capability is documented elsewhere as available, that doc is the source of truth — this page is only for what is *not* built yet. The 3.0 north star is [docs/3.0.md](3.0.md).

## CLI — designed, not implemented

| Command | Intent | Status |
|---|---|---|
| `gt convoy` | Convoy monitoring (track a group of related runs/worktrees) | designed, not implemented |
| `bd mol` | Structured workflow "molecule" pour (see `bd formula`/`bd mol` design) | designed, not implemented |
| `bd cook` | Batch/recipe execution over beads | designed, not implemented |

If you hit an error invoking one of these, it does not exist yet — that is expected.

## Curation pipeline — later stages

The curation pipeline is a six-stage design: CATALOG, VERIFY, INDEX, SCORE, REJECT, CONSTRAIN. Today **CATALOG and VERIFY** ship as CLI commands (`ao curate catalog`, `ao curate verify`, `ao curate status`). The later stages (INDEX, SCORE, REJECT, CONSTRAIN) are roadmap, not built. The orthogonal, currently-active prevention lane is the finding-compiler (`.agents/findings/registry.jsonl` → constraints + planning rules), which is separate from this pipeline. See [docs/curation-pipeline.md](curation-pipeline.md).

## Hookless default install (ADR-0002 S2–S5)

The 3.0 north star demotes hooks to optional adapters ([docs/3.0.md](3.0.md)). The remaining lift — a default install path that requires **zero** hooks, and an RPI lifecycle that runs discovery→validation hookless — is tracked as ADR-0002 stages S2–S5. It is a follow-on, not part of the 3.0 reconciliation close.

## Legacy RPI lane removal

The legacy RPI lane (`rpi_parallel`, `rpi_loop_supervisor`, `rpi_phased_tmux` + the `ao rpi parallel` / supervised-loop surfaces) is superseded by the gc bridge but still load-bearing (live code references its helpers). Removing it requires a caller-migration refactor, tracked as `soc-1gbpz`.
