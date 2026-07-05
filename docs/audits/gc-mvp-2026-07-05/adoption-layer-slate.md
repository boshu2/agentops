# Adoption-layer slate — making the membrane city usable (2026-07-05)

> Planning/research only. Companion to `out-of-box-gap-map.md` (stock gc vs the
> loop), `native-maximization-map.md` (gc's native features), and the shipped
> `packs/agentops-membrane/`. This file ranks the **ergonomics/adoption layer** —
> the pieces that let Bo *and other users* actually drive a membrane city, not
> just the pack that encodes the doctrine. Source for the beads that follow.

## The one-paragraph thesis

Stock Gas City is a strong substrate and the `agentops-membrane` pack adds the
one thing it lacks (a fail-closed, cross-family, verdict-bound close door). But
between "the pack exists on GitHub" and "a non-expert runs a reliable membrane
city" sits a wall of undocumented setup and two known nudge-stalls. What makes gc
usable for a non-expert is **one command that stands up a correct city** (right
GC_HOME isolation, bd/dolt native store proven `dolt_mode=server`, LAW-0
`print_args=[]` on every family, the pack imported and pinned, trinity+AGY
verifiers as always-on sessions, `gc doctor` green), **the two known stalls
pre-mitigated as shipped pack content** (idle-pane keepalive via `gc session
submit`; codex trust-modal pre-trusted via a clean `CODEX_HOME`), and **an
operating skill that teaches the day-to-day loop and routes to gc's own native
surface** (`gc events`, `gc doctor --json`, `gc costs`, the `pawl-verdict.v1`
artifacts) instead of re-wrapping it. Everything else — the store, DAG, worktrees,
orders, dashboard, provenance — the user should simply be *pointed at*, because
gc already ships it and self-describes.

## Hard constraint that shapes the whole slate (do not violate)

**AgentOps once had a `runtime=gc` bridge inside crank/swarm; it was severed and
deleted (soc-2rtm0), and `runtime=gc` is rejected by the CLI.** gc is now a
*coexisting* substrate for city-shaped work, **not** a swap for NTM (our swarm
substrate). Therefore every adoption piece here is **gc-native operator surface
that lives beside `ao`/NTM — never a re-coupling of `ao` or crank to gc.** In
particular: no `ao gc …` subcommand tree, no crank dispatch ladder re-add. The
bootstrap is a standalone script; the day-to-day driver is a skill; both route to
gc's own CLI, which is already self-describing.

## The ranked slate

| # | Piece | What it is | Who it helps | Removes (traced) | Built as | Effort | Priority |
|---|---|---|---|---|---|---|---|
| 1 | **`install-gc-city.sh`** | One command stands up a correct native membrane city from the RUNBOOK: dedicated `GC_HOME`, `env.sh`, verify bd/dolt version contract + `dolt_mode=server`, `gc init` (core+bd), import+pin `agentops-membrane`, 3 providers with `print_args=[]`, trinity+AGY always-on sessions, seed `CODEX_HOME`, git shim if network-git wedges, gate on `gc doctor` green | Both | 140-line RUNBOOK of exacting, order-dependent setup that only Bo can currently reproduce; the `dolt_mode`/version-contract cliff; LAW-0 print sinks; the git-hang host quirk | `scripts/install-gc-city.sh` (standalone; **not** `ao gc`) | M–L | **P1** |
| 2 | **Residual-gap pack content** | Turn `RESIDUAL-GAPS.md`'s *illustrative* snippets into *shipped, opt-in* pack files the bootstrap wires: `orders/membrane-lane-keepalive.toml` (re-nudge idle lanes via `gc session submit` only), a `CODEX_HOME` pre-trust seed in setup, and `[usage] provider="local"` so `gc costs` populates | Both | The two nudge-stalls users hit "the hard way" (idle-pane drain; codex trust modal) + the empty `gc costs` (usage.jsonl absent) | Pack content in `packs/agentops-membrane/{orders,assets/scripts}` + a bootstrap wire | S–M | **P1** |
| 3 | **`using-gc` operator skill** | The `ntm`/`using-atm` analog: doctrine for driving a membrane city day-to-day — sling a quest, watch the dispatcher + close gate through `gc events`, resolve the two known stalls, read `pawl-verdict.v1` verdicts, converge. **Routes to gc's native `--json`/events/doctor surface; never wraps it** | Both | The pack is inert knowledge without an operating loop; nobody knows the two nudge-fixes or how to read a verdict | `skills/using-gc/SKILL.md` (+ references) | M | **P1** |
| 4 | **Quickstart doc** | User-facing "your first gas city with the membrane": import line → run bootstrap → sling the canary quest → watch it CONFIRM → read the verdict. The external on-ramp | Other-users | External adopters have no narrative entry; the skill+script carry Bo | `packs/agentops-membrane/QUICKSTART.md` | S | **P2** |
| 5 | **Registry distribution** | `gc pack registry publish .` so `gc pack registry search agentops` discovers it, plus version-pin discipline. (The GitHub `[imports.agentops-membrane]` line already works today — this is the discoverability upgrade, not the enabling path) | Other-users | Discovery friction; the import URL works but is undiscoverable | `gc pack registry publish` + a `version` bump/pin runbook step | S | **P2** |
| 6 | **Tracker bridge (read-only rollup)** | Surface a city's closed quests + membrane verdicts back into agentops `br` for cross-tracker reporting. `.8` (bridge-lite). Thin `gc events`/`gc bd list --json` → br report, **not** a live sync | Bo-operator | No cross-city rollup once >1 membrane city runs | `scripts/gc-verdict-rollup.sh` (read-only) | M | **P3** |

## Recommended P1 sequence

1. **`install-gc-city.sh`** first — it unblocks everyone (including a fresh Bo
   session) standing up a correct city, and pieces 2–4 all reference it.
2. **Residual-gap pack content** second — the bootstrap wires it, so land it so
   the bootstrap can turn it on. This is what makes the stood-up city actually
   near-unattended instead of "works while Bo babysits."
3. **`using-gc` skill** third — teaches driving the city the bootstrap produced,
   with the two nudge-fixes as first-class unstick moves.

P2/P3 (quickstart, registry publish, bridge) follow once the P1 spine lets a
second person run a city end-to-end.

## What we should NOT build (explicit non-goals)

- **No `runtime=gc` bridge re-add** in crank/swarm. It was deleted (soc-2rtm0);
  NTM stays our swarm substrate. gc coexists for city-shaped work only.
- **No `ao gc …` subcommand tree.** It re-couples `ao` to gc's surface (the exact
  coupling soc-2rtm0 removed), and gc already ships `gc status --json`,
  `gc doctor --json`, `gc events`, `gc costs`. A status rollup is a **skill
  snippet over those**, not a new `ao` command. (This is the tempting-but-wrong
  piece.)
- **Don't wrap/duplicate gc's native operator surface** — `gc events`,
  `gc costs`, `gc dashboard`, `gc doctor`, `gc analyze reliability`,
  `gc session list/logs`, `gc mail`. The skill points at them; it does not
  re-implement them.
- **Don't build stall-detection or notification infra** — gc's 20 native orders +
  `gc events` + embedded dashboard already do it. The keepalive order is the *one*
  membrane-specific addition, not a new framework.
- **Don't fix the idle-pane drain / codex-modal at the root in-pack** — the durable
  fix is upstream in gascity (read-only fork). Ship the documented mitigations
  only; don't fake a root fix.
- **Don't auto-merge / auto-push.** The membrane never merges; a human merges. No
  adoption convenience may cross that line.
- **Don't gate on `gc costs` dollar thresholds** — list-price decision-support
  only; unpriced models drop from the total (would fail-open).

## Honest read — the 2–3 that most move the needle

1. **`install-gc-city.sh` (P1.1).** Highest leverage. Right now standing up a
   correct city is 140 lines of RUNBOOK that only Bo can reproduce, gated on the
   silent `dolt_mode=server`/version-contract cliff and the LAW-0 print-sink
   overrides. One command collapses that. Without it, "other users can use this"
   is false at step zero.
2. **`using-gc` skill (P1.3).** The membrane is inert as knowledge until someone
   knows the operating loop. The two nudge-fixes (`gc session submit` for a
   draining pane; `CODEX_HOME` pre-trust) are the literal difference between a run
   that converges and one that stalls — and they are non-obvious. The skill is
   where that lives, matching how `ntm`/`using-atm` carry NTM doctrine.
3. **Residual-gap pack content (P1.2).** Packages the exact "discover the hard
   way" fixes so a user never has to. Slightly lower than the other two only
   because the skill can *document* the mitigations even before they're shipped as
   files — but shipping them is what earns "near-unattended."

The quickstart, registry publish, and tracker bridge are real and worth doing,
but they are the second lap: they matter once the P1 spine has let a *second
person* run a membrane city start to finish.

---

## Draft bead specs (for filing)

> Suggested epic: **gc adoption layer** (parent of the six below). Tag with the
> gc-mvp lineage. Each carries acceptance in Given/When/Then terms.

### AL.1 — `install-gc-city.sh` one-command bootstrap  (P1, M–L)
- **Capability:** Stand up a correct native membrane city in one command.
- **G/W/T:** *Given* a host with gc, bd, dolt installed and no city, *when* an
  operator runs `scripts/install-gc-city.sh <dir>`, *then* the city boots with
  `gc status --json .beads = NativeDoltStore`, `gc doctor` green (incl.
  `law0-print-args` + `membrane-health` with ≥2 families), the `agentops-membrane`
  pack imported and pinned, and trinity+AGY verifiers as always-on sessions.
- **Edge:** *Given* the bd/dolt version contract is unmet (dolt_mode≠server),
  *when* run, *then* it fails fast with the exact remediation, not a silent perf
  cliff.
- **Non-goals:** not an `ao` subcommand; does not merge/push; does not touch
  `~/.gc` or any existing city.
- **Rollback:** removes the created `GC_HOME` + city dir; idempotent re-run.
- **Evidence:** a fresh-host run transcript reaching `gc doctor` green.

### AL.2 — Residual-gap mitigations as shipped pack content  (P1, S–M)
- **Capability:** The two known nudge-stalls + empty costs are pre-mitigated, not
  discovered.
- **G/W/T:** *Given* the membrane pack, *when* imported and the bootstrap wires
  it, *then* an idle reviewer lane past budget is re-nudged via `gc session
  submit` (never kill/reset/send-keys), city codex sessions inherit a pre-trusted
  `CODEX_HOME`, and `gc costs` returns non-empty (`[usage] provider="local"`).
- **Edge:** *Given* a 1-family city, *when* the keepalive fires, *then* nothing
  masks the `membrane-health` "fewer_than_two_families" block.
- **Non-goals:** no new stall-detection framework; no root fix of the upstream
  drain/modal gaps.
- **Evidence:** keepalive order fires in `gc events`; `gc costs` non-empty after a
  run.

### AL.3 — `using-gc` operator skill  (P1, M)
- **Capability:** Doctrine for driving a membrane city day-to-day (the
  ntm/using-atm analog).
- **G/W/T:** *Given* a running membrane city, *when* an operator/agent follows the
  skill, *then* they sling a quest, observe the close gate through `gc events`,
  resolve a draining pane with `gc session submit` and a wedged codex lane with
  the `CODEX_HOME` fix, read the `pawl-verdict.v1` verdict, and converge —
  routing to gc's native `--json` surface throughout.
- **Edge:** *Given* a REFUTED verdict, *when* read, *then* the skill directs the
  bounded-redo path, not a manual close.
- **Non-goals:** does not wrap `gc events`/`gc costs`/`gc doctor`; no `ao gc`.
- **Evidence:** a behavioral probe — the skill's unstick ladder resolves a
  seeded draining pane in a drill.

### AL.4 — Quickstart doc  (P2, S)
- **Capability:** External on-ramp "your first gas city with the membrane."
- **G/W/T:** *Given* a new adopter, *when* they follow `QUICKSTART.md`, *then*
  import → bootstrap → sling canary → watch CONFIRM → read verdict, with no prior
  gc knowledge.
- **Non-goals:** not a doctrine dump; links to gc's own docs for substrate.
- **Evidence:** a cold-reader (silent-novice) run reaches a CONFIRMED canary.

### AL.5 — Registry distribution  (P2, S)
- **Capability:** `gc pack registry search agentops` discovers the pack.
- **G/W/T:** *Given* a versioned pack, *when* `gc pack registry publish .` runs,
  *then* the pack is discoverable by search/show with a paste-ready import line
  and a pinned `version`.
- **Non-goals:** the GitHub import URL already works — this is discoverability,
  not the enabling path.
- **Evidence:** `gc pack registry show <handle>` prints the pinned import.

### AL.6 — Tracker bridge rollup (read-only)  (P3, M)
- **Capability:** Cross-city membrane-verdict rollup into agentops `br`.
- **G/W/T:** *Given* ≥1 membrane city, *when* `gc-verdict-rollup.sh` runs, *then*
  closed quests + verdicts surface as a br report — read-only, no live sync.
- **Non-goals:** not a two-way sync; does not mutate the city store.
- **Evidence:** a rollup report listing a city's CONFIRMED quests.
