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

The curation pipeline is a six-stage design: CATALOG, VERIFY, INDEX, SCORE, REJECT, CONSTRAIN. The former curation CLI was retired; none of these stages is a live `ao` command on current `main`. The orthogonal prevention lane is the finding-compiler (`.agents/findings/registry.jsonl` → constraints + planning rules), which is separate from this pipeline. See [docs/curation-pipeline.md](curation-pipeline.md).

## Hookless default install (ADR-0002 S2–S5)

The 3.0 north star demotes hooks to optional adapters ([docs/3.0.md](3.0.md)). The remaining lift — a default install path that requires **zero** hooks, and an RPI lifecycle that runs discovery→validation hookless — is tracked as ADR-0002 stages S2–S5. It is a follow-on, not part of the 3.0 reconciliation close.

## Legacy RPI lane removal

The legacy RPI lane's internal backends (`rpi_parallel`, `rpi_loop_supervisor`, `rpi_phased_tmux`) are load-bearing but live — the `ao rpi parallel` / supervised-loop **command surfaces were removed** in f61c5f0e7 (the RPI engine is gone; drive the seven-move operating loop via the `/rpi` skill instead), yet the phased engine kept its non-gc backends after the gc-bridge removal and live code still references the lane's helpers. Removing the remaining internal helpers requires a caller-migration refactor, tracked as `soc-1gbpz`.
