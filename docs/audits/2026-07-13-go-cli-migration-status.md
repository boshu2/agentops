# Go CLI Migration Status and Remaining Work

**Audit date:** 2026-07-13

**Integration base:** `origin/main` at `57f5c37436d4bd1f00517dfa2f46969d21a9b5f3`

**Validated integration candidate:** `564820f90528370829f471e2222cf1626258ac43`

**Tracker:** private `br` graph rooted at `age-nw28h`

**Supersedes for current status:** [Go CLI Production-Readiness Audit](2026-07-12-go-cli-production-readiness.md)

## Executive Summary

The Go CLI migration is substantial but unfinished. The current integration
candidate preserves ten sealed command-module families and passes their
architecture and four-profile compatibility checks. That is a real vertical
slice of the target architecture, not a completed migration.

The live CLI is still assembled by the package-main strangler: global Cobra
registration, ambient root effects, inferred recursive contracts, and
post-assembly profile pruning remain production behavior. Seventy-six top-level
family migrations remain open. Before more families are sealed, seven semantic
repair tracks must strengthen the proof membrane so it cannot certify code that
still violates cancellation, tracker execution, configuration, contract, or
output invariants. Seventeen measured hardening leaves and three final
composition/deletion leaves also remain.

The correct execution order is:

1. land the current ten-family integration stack without rewriting historical
   compatibility evidence;
2. complete semantic repair H1-H7;
3. complete the 17 measured hardening leaves;
4. migrate the remaining 76 command families one vertical slice at a time; and
5. execute the three final cutover leaves that delete the strangler and install
   anti-regrowth proof.

The overall `age-nw28h` program must remain open until all of those outcomes are
proved on landed code.

## Scope and Method

This status reconciles:

- the Git topology of current main and the ten-family integration candidate;
- the live `cmd/ao`, `cliapp`, contract, architecture-checker, tracker, and
  capability source;
- the private `br` dependency graph rooted at `age-nw28h`;
- current format, lint, static-analysis, complexity, architecture,
  compatibility, integration, and compiled fast-gate results; and
- the accepted family receipts and immutable compatibility lineage.

It is an execution-status audit, not a new architecture proposal. The existing
bead graph is the implementation plan.

## Current Shape

| Surface | Current fact |
|---|---|
| Mainline command modules | `origin/main` has no `cli/internal/commands` tree |
| Integrated sealed modules | 10: `beads`, `capabilities`, `claim`, `close`, `config`, `council_gate`, `doctor`, `done`, `eval`, `gate` |
| Remaining family roots | 76: 19 default-profile and 57 tagged-only families |
| Production `cmd/ao` | 287 production files and 67,297 lines |
| Global assembly residue | 187 production `init()` functions |
| Direct process launches | 72 production subprocess sites |
| Open program issues | 144 open `br` issues under `age-nw28h` |
| Semantic repair | 31 concrete leaves across H1-H7 |
| Measured hardening | 17 concrete leaves |
| Final cutover | 3 deletion/composition leaves |

The integration checker's `--all-migrated` spelling means “all ten families in
the sealed-family list.” It does not mean the whole CLI has migrated. Whole-CLI
completion requires the remaining family matrix and final strangler deletion.

## What the Ten-Family Candidate Proves

The candidate establishes a useful migration floor:

- explicit module ownership for the ten named families;
- preserved historical family ownership and compatibility blobs;
- current behavior checked across default, flywheel, legacy, and combined
  profiles;
- `origin/main` ancestry plus explicit overlap dispositions;
- ten descendant revalidation receipts bound to the integrated SHA; and
- a compiled fast gate with 32 passing checks, zero warnings, and zero failures
  on the final documentation/product scope.

These checks prove the bounded slice they name. They do not prove that every
command family has moved or that the current semantic checker covers every
runtime invariant.

## Why the Strangler Still Owns Production

The target `cliapp.BuildRoot` and command-module interface exist, but production
bootstrap still depends on package-main behavior:

- [`cli/cmd/ao/root.go`](../../cli/cmd/ao/root.go) owns global root state,
  ambient pre-run effects, and concrete process-exit mapping;
- [`cli/cmd/ao/zzz_default_spine.go`](../../cli/cmd/ao/zzz_default_spine.go)
  prunes an already assembled command tree for the default profile;
- [`cli/internal/cliapp/module.go`](../../cli/internal/cliapp/module.go) exposes
  one root contract per family rather than a complete runnable-node contract
  tree; and
- [`cli/internal/clicontract/inspect.go`](../../cli/internal/clicontract/inspect.go)
  still infers recursive capabilities instead of failing closed on missing
  declarations.

The result is cleanly factored code inside a still-active strangler, not yet a
cleanly factored CLI composition root.

## Semantic Repair Required Before More Family Seals

| Track | Current defect | Required outcome |
|---|---|---|
| H1 semantic seals | Architecture acceptance mostly checks source shape inside `internal/commands` | Prove context, tracker execution, output, declared effects, and runnable-node evidence |
| H2 recursive contracts | A module owns one root contract while live projection infers child behavior | Validate exactly one declaration per runnable node; reject missing, duplicate, extra, and unreachable contracts |
| H3 root purity | Every invocation can run ambient environment, cwd, Git, and app-injection effects | Build an immutable runtime view and apply preparation/effects only where declared |
| H4 tracker config | Read and YAML errors can silently fall through to lower-precedence sources | Ignore only absence; return source path and fail closed on permission, parse, and invalid-value failures |
| H5 cancellation | Six migrated beads handlers use `context.Background()` | Propagate `command.Context()` and prove cancellation with blocking fakes |
| H6 tracker execution | No shared `trackerexec` core exists; adapters can diverge on cwd/env/context | Add the shared execution core and migrate its 22 adoption/ownership leaves |
| H7 output truth | Global capability claims exceed command-specific format behavior | Declare formats per runnable node, use typed renderers, and reject unsupported formats before effects |

H1-H5 contribute five concrete leaves. H6 contributes 22 adoption/ownership
leaves, and H7 contributes four output-contract leaves, for 31 concrete semantic
repair leaves. H6 and H7 parent epics are aggregation nodes rather than extra
implementation.

## Measured Hardening Still Open

Seventeen hardening leaves cover generated command surfaces, presentation
cohesion, dependency validation, typed process results, compiled build-tag
reachability, exit/version taxonomy, runnable-node evidence, formatting, lint,
static analysis, complexity, and controlled performance baselines.

Current evidence shows that this is executable work, not stale bookkeeping:

- `gofmt -l cli` reports 17 files;
- the configured Go lint check reports three findings;
- standalone static analysis reports a dead-code/deprecation tail and two
  invalid self-comparison tests;
- the absolute-complexity gate is red, including
  `archcheck.checkFamily=25`, `main.Execute=22`, and `cliapp.BuildRoot=16`; and
- the shaped module-cohesion, performance, audit-closure, strangler-absence,
  tracker-ownership, and semantic-seal scripts do not exist yet.

The parent epic's final lint acceptance command also omits the required `run`
subcommand and can print help while exiting successfully. Leaf `age-nw28h.18`
contains the correct invocation and owns that proof repair.

## Remaining Family Matrix

The original matrix contains 86 family roots. Ten are sealed and 76 remain.
`goals` is the next in-progress family, but its partial implementation exists on
an older worktree and is deliberately excluded from the current integration
candidate. Only the focused H1 cancellation fixture from that worktree should be
ported onto the landed base; the stale branch must not be merged wholesale.

After H1-H7 and hardening, resume the matrix with Goals, the remaining 18 default
families, and the 57 tagged-only families. Each family remains a separate
vertical behavior slice with its own first failing acceptance test, module
ownership, current-profile compatibility, and exact-SHA seal.

## Final Cutover

Three leaves distinguish a completed migration from a permanent strangler:

1. `age-nw28h.6.1` deletes capability and contract inference after every
   runnable node is explicitly declared;
2. `age-nw28h.6.2` moves the complete manifest and fresh root assembly out of
   `package main`; and
3. `age-nw28h.6.3` deletes `App`, global command registration, argument
   backfill, post-assembly pruning, and remaining strangler scaffolding, then
   installs the anti-regrowth gate.

The program is complete only when those deletions pass on the landed SHA and no
legacy production owner remains.

## Execution Contract

### Acceptance

Given a candidate for any semantic, hardening, family, or cutover leaf, when its
scoped checks and repository-selected gate run, then the exact candidate SHA
must receive an independent evidence-bound verdict before landing. Historical
family evidence remains immutable; new behavior is represented by an append-only
current overlay or descendant receipt.

### Edge Cases

- A green ten-family `--all-migrated` check is never whole-CLI completion.
- A retry count does not justify stopping the goal. A breaker trip gets the
  repository's bounded fresh-context helper consultation before operator
  escalation unless a hard budget or human-only judgment is genuinely spent.
- A stale worktree is evidence to mine, not an integration source.
- A generated or bespoke runtime projection must remain semantically aligned
  with its canonical source.

### Non-goals

- Do not redesign the migration graph.
- Do not rewrite accepted family baselines.
- Do not merge multiple family owners into one broad cleanup sweep.
- Do not close `age-nw28h` when only the ten-family integration stack lands.

### Rollback

Each leaf rolls back to its accepted predecessor while retaining the failing
fixture and exact evidence that invalidated the candidate. Historical seal bytes
remain unchanged. The next attempt resumes from the earliest invalidated step
with a different approach.

## Evidence Commands

The following commands reproduce the program-level facts from the appropriate
candidate/worktree:

```bash
git ls-tree -r --name-only origin/main -- cli/internal/commands
find cli/internal/commands -mindepth 1 -maxdepth 1 -type d | sort
bash scripts/check-go-cli-integration-baseline.sh
bash scripts/check-go-cli-architecture.sh --all-migrated
bash scripts/check-go-cli-compatibility.sh \
  --oracle-version current --verify-frozen \
  --profiles default,flywheel,legacy,combined --all-migrated
BEADS_DIR="$(/Users/bo/go/bin/ao beads dir --require)" \
  br list --json
```

The private `br` graph is the durable source of execution state. This audit is
the human-readable continuity document for why that graph remains open and what
order completes it.
