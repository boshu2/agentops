# AO / MTO Seam

> **Status:** V0 reduction contract.
> **Purpose:** keep AgentOps self-contained while relocating fleet orchestration
> behind the MTO/factory boundary.

This contract is the AgentOps-side pair for the control-plane boundary docs:

- `AO-MTO-BOUNDARY.md`
- `MTO.md`

AgentOps (`ao`) is the lean local image: one worker loop, local Themis, local
corpus, and the full local knowledge flywheel. MTO is the outer factory: fleet
cadence, dispatch, leases, fleet Themis/quorum, fleet corpus aggregation, ACFS
rollout, and kill switches.

## Boundary Rule

MTO may call AO. MTO must not import, source, or reimplement AO internals.

The supported external seam is:

```text
ao ready
ao claim <id>
ao verdict-gate <verdict>
ao council-gate <verdict-a> <verdict-b> [...]
ao close <id> <commit-message> <evidence-ref> [paths...]
```

An MTO worker dispatch is one whole AO loop invocation. It is not an expansion
of AO's inner `research -> plan -> implement -> validate` phases into factory
steps.

## AO-Owned Surfaces

These stay in the lean AO image:

| Surface | Reason |
|---|---|
| `ready`, `claim`, `close`, `tick`, `verdict-gate`, `council-gate`, guard status, chaos/self-test | State and local Themis ports. |
| `forge`, `mine`, `compile`, `curate`, `harvest`, `inject`, `lookup`, `corpus`, `knowledge`, `patterns`, `findings` | Local flywheel and corpus. |
| `goals`, `loop`, `mind`, `ratchet`, `validate`, `session`, `skills`, `beads` | Solo image operation and operator ergonomics. |
| Vendor-native image controls needed inside one worker image | Claude/Codex/AGY images must preserve their native affordances. |

AO must remain useful without MTO.

## MTO-Routed Surfaces

These are MTO/factory candidates after a compatibility port exists:

| Surface cluster | Route |
|---|---|
| `cron`, `ci`, `worktree`, `next_work`, `turn_verify`, `orchestrate` | MTO/factory coordination. |
| outer runtime supervision and worker dispatch | MTO/factory coordination. |
| fleet health, fleet leases, fleet corpus, fleet quorum | MTO/factory coordination. |

Route does not mean immediate deletion. Public commands need either a
compatibility stub, a documented replacement, or a generated-command contract
update in the same PR that moves them.

## Vendor-Adapter Surfaces

These route toward image-specific adapters rather than fleet MTO:

| Surface cluster | Route |
|---|---|
| `agent*`, `mcp_*`, `codex_runtime`, `harness*`, `session_spawn` | Vendor-image adapter seam. |
| Claude Workflow/subagents/CronCreate | Claude image adapter. |
| Codex Goals, plugins, `codex exec`, `codex doctor` | Codex image adapter. |
| AGY `/goal`, `/schedule`, Agent Manager, sidecars/`agentapi`, IDE/browser/terminal surfaces | AGY image adapter. |

Native affordances are part of the image contract. Relocation must not flatten
one vendor's native control surface into another vendor's implementation.

## Defer-Load-Bearing Surfaces

`rpi_*` is load-bearing. MTO may schedule a whole AO loop, but it must not
reimplement RPI phase logic. Move RPI files only after a stable AO port exposes
the same behavior and the generated CLI docs, command-surface inventory, and
skills that call the command are migrated together.

## Relocation Gate

Every RELOCATE PR must prove:

1. The moved surface cites its row(s) in `docs/reduction/ao-file-buckets.tsv`.
2. The replacement route is one of:
   - `mto-fleet`
   - `vendor-image-adapter`
   - `defer-load-bearing`
3. AO local flywheel commands still work.
4. The generated command surfaces are current.
5. No new dependency manifest entries were introduced unless the PR explicitly
   justifies them.
6. CI and the relevant local gates pass before merge.

## Not Allowed

- Moving local corpus promotion into MTO.
- Closing AO work through raw `br close` instead of `ao close`.
- Broad git staging in a close path.
- Replacing local Themis with fleet Themis.
- Treating Agent Mail as the durable bus.
- Removing a public command without either compatibility or a documented
  replacement route.
