# Go CLI Production-Readiness Audit

**Audit date:** 2026-07-12
**Source:** `goal/go-cli-clean-architecture-repair` at `a07bc8f44675e74fad2b0703c66cd370baf26354`
**Tracker:** `age-nw28h.7`, under program `age-nw28h`
**Domain:** CLI architecture, correctness, operability, and Go maintainability

## Executive Summary

The Go CLI is moving in the right direction, but it is not yet production-ready
under its own stated contracts. Foundation work and ten family migrations have
materially reduced the `cmd/ao` monolith. Builds, deterministic tests, race
tests, the vulnerability scan, and the four-profile compatibility oracle are
green. Those are real strengths.

The main risk is now the **proof boundary**, not merely the remaining volume of
legacy code. A migrated family can satisfy the current architecture and
compatibility gates while still violating the intended semantics. This is not
hypothetical:

- the accepted `beads` command module discards Cobra cancellation in six paths;
- the accepted `claim` adapter does not honor the canonical tracker resolver's
  `WorkDir` and `ChildEnv`;
- malformed tracker configuration silently falls through despite the closed S3
  bead promising fail-closed behavior;
- the architecture checker passes all ten sealed families because it verifies
  local syntax and ownership, not those end-to-end invariants;
- explicit contracts exist, but the live capability projection ignores them
  and the singular family-root contract cannot describe recursive subcommands;
- the machine output contract advertises behavior that representative live
  commands do not implement.

The current 101-bead strangler program still contains the correct large-scale
direction: explicit assembly, one family at a time, final deletion of globals,
and an immutable compatibility baseline. It is not sufficient as the entire
production-hardening program. Before more family seals accrue, its proof system
needs a semantic repair. A later goal-design pass should preserve the useful
strangler work while adding conformance suites for context, tracker execution,
recursive contracts, output formats, generated surfaces, and integration with
`main`.

## Finding Count

| Severity | Count |
|---|---:|
| Critical | 0 |
| High | 8 |
| Medium | 9 |
| Low | 0 |

No immediately exploitable vulnerability or demonstrated data-loss path was
found. The high findings are release-blocking because they can make the CLI or
its verification membrane assert a stronger guarantee than the code proves.

## Scope and Method

The audit reconciled these artifact classes against the current tree:

- the source-pinned [Go CLI architecture guide](../architecture/go-cli-architecture-guide.md);
- the July 10 research, derived specification, validated plan, pre-mortem, and
  101-node private `br` graph;
- all current command modules, composition shims, contracts, tracker resolution,
  architecture checker, compatibility fixtures, and generated command reference;
- compiled default-profile behavior for help, output modes, exit codes, and
  hidden build-tag introspection;
- Go build, vet, test, race, formatting, lint, static analysis, dependency
  vulnerability, complexity, compatibility, architecture, build-tag, generated
  reference, and regeneration checks.

This is a deep audit of the CLI architecture and its proof system. It is not a
claim that every business rule in all 833 production Go files has received a
manual line-by-line correctness review.

## Current Shape and Movement

The migration has made measurable progress from execution base
`a681d7e7c00a876a17a51efbf239f843806ff9c0`:

| Measure | July 10 base | Current | Delta |
|---|---:|---:|---:|
| `cmd/ao` production files | 305 | 284 | -21 (-6.9%) |
| `cmd/ao` production lines | 77,691 | 66,430 | -11,261 (-14.5%) |
| production `init()` functions | 205 | 186 | -19 (-9.3%) |
| direct `cmd/ao` subprocess sites | 89 | 72 | -17 (-19.1%) |
| direct `cmd/ao` internal imports | 596 | 382 | -214 (-35.9%) |
| Go packages | 115 | 152 | +37 |
| concrete adapter directories | 20 | 33 | +13 |
| explicit command-module packages | 0 | 11 | +11 |

The ledger reports 15 of the original 101 program issues closed, 85 open, and
`goals` in progress. P0 through P4A and the first ten default-profile families
are closed. The current branch is not an integration branch: it is 122 commits
behind and 115 commits ahead of `origin/main`, with nine overlapping CLI paths.

## What Is Already Strong

- `go build ./...`, `go vet ./...`, and `go test ./...` pass.
- `go test -race ./...` passes across all packages.
- `govulncheck ./...` reports no vulnerabilities.
- `go mod tidy -diff` is clean.
- Four-profile compatibility passes for default, flywheel, legacy, and combined.
- The immutable fixture lineage and accepted-family ownership seals are a
  meaningful defense against casually blessing drift.
- `cliapp.BuildRoot` returns fresh trees, validates modules, detects duplicate
  IDs/names/aliases, and selects profiles before assembly.
- `trackerresolve` is now the single selection/workspace owner and correctly
  models linked worktrees plus distinct `br`/`bd` child environments.
- The race-clean result and extensive package tests are a better baseline than
  the monolith usually has at this point in a strangler migration.

These strengths should be retained. The findings below identify where their
scope is narrower than the program currently implies.

## High Findings

### H1. Architecture acceptance proves local syntax, not the promised semantics

- **Locations:** `tests/go_cli_clean_architecture.bats:70`,
  `cli/internal/archcheck/checker.go:135`,
  `cli/internal/archcheck/family.go:300`,
  `scripts/check-go-cli-compatibility.sh:209`
- **Issue:** S2 says a command handler is a thin driving adapter and every effect
  crosses an owned adapter. Its executable acceptance only checks that a module
  exists, lacks a short list of direct-effect spellings, and passes the family
  architecture/compatibility scripts. `CheckSource` explicitly returns unless
  the file is under `internal/commands`; `checkModuleShape` counts constructors,
  contracts, and profile literals. It does not inspect transitive use cases,
  compare declared effects with observed effects, require command-context
  propagation, or establish that the dimension named in a compatibility file
  is actually exercised by the referenced shell command.
- **Evidence:** All ten sealed family checks pass today while H4, H5, and H6
  below remain true. The compatibility validator only checks that a dimension's
  `evidence` IDs name existing executable checks; it cannot prove that those
  checks cover the dimension.
- **Root cause:** The gate was designed as an AST ownership boundary and then
  used as evidence for a broader behavioral assertion.
- **Production risk:** A family can receive a sealed, independently confirmed
  migration verdict while cancellation, tracker targeting, output isolation, or
  application-layer effect ownership is wrong.
- **Recommended outcome:** Split the guarantee into explicit properties. Add
  context-propagation checks, adapter conformance suites, declared-versus-observed
  effect checks, recursive contract validation, and typed evidence schemas whose
  assertions bind to each claimed dimension. A family seal must state exactly
  which properties it proves.
- **Current-program coverage:** P4A/P7 partially cover syntax and dependency
  direction; the semantic portion is missing and should be repaired before more
  family leaves are sealed.

### H2. The contract model cannot yet be the recursive source of truth

- **Locations:** `cli/internal/cliapp/module.go:9`,
  `cli/internal/clicontract/inspect.go:11`,
  `cli/internal/adapters/capabilities/cobra.go:66`,
  `cli/internal/clicontract/metadata.go:79`
- **Issue:** A `Module` exposes one `Contract()` for an entire top-level family,
  while the capability document is recursive. `Inspect` never calls
  `ContractFor` or `ProjectContract`; it derives IDs from paths and defaults
  output, effects, and exits for every node. Attaching a contract to a family
  root therefore does not affect the live machine projection. Even after that
  wiring is added, one root contract cannot describe child commands with
  different arguments, outputs, effects, or exits.
- **Evidence:** The compiled capability rows for all eleven modules still show
  inferred values. For example, `ao close` reports `output=none`,
  `effects=mixed`, and only exits 0/1 despite its explicit root contract
  declaring text output and additional conflict/partial exits. `ao gate` and
  `ao eval` appear pure at the root because they are containers, while their
  children are effectful.
- **Root cause:** P1 established a dependency-neutral vocabulary, but P5 assumes
  that deleting inference is enough without defining how every recursive node
  receives a declaration.
- **Production risk:** Agents and generated docs consume a contract that is
  stable in shape but semantically false.
- **Recommended outcome:** Make contract ownership recursive. Either every
  command constructor attaches a node contract, or a module returns a validated
  contract tree keyed by stable node ID. `BuildRoot` should reject missing,
  duplicate, unreachable, or extra node contracts before execution. Capability
  generation and docs must fail closed on an uncontracted runnable node.
- **Current-program coverage:** P5 names inference deletion but is underspecified;
  its design must be expanded before implementation.

### H3. Root pre-run effects make declared-pure commands stateful

- **Locations:** `cli/cmd/ao/root.go:51`,
  `cli/internal/adapters/worktreeconfig/worktree_config.go:14`,
  `cli/internal/adapters/worktreeconfig/worktree_config.go:25`,
  `cli/internal/commands/capabilities/module.go:24`
- **Issue:** Every normal invocation passes through a root pre-run that mutates
  process environment, obtains the working directory, probes Git, and can rewrite
  shared/per-worktree Git configuration. `capabilities` declares `EffectPure`,
  but invoking it executes those root effects before its handler.
- **Root cause:** Cross-cutting repository repair and configuration propagation
  live in the universal executable lifecycle rather than in command-scoped
  requirements.
- **Production risk:** Read-only introspection can mutate caller state or Git
  configuration, and effect metadata describes only the leaf handler rather
  than the end-to-end invocation.
- **Recommended outcome:** Build an immutable runtime/environment view at the
  composition boundary. Apply repository preparation only to commands that
  declare it. Move repair into an explicit, idempotent operation or a narrowly
  scoped interceptor. Contract effects must include all root interceptors.
- **Current-program coverage:** P6/P7 remove the old root shape but do not yet
  state a pure-command or root-interceptor invariant.

### H4. Closed tracker resolution silently accepts malformed configuration

- **Location:** `cli/internal/trackerresolve/resolve.go:211`
- **Issue:** `configValue` ignores all `os.ReadFile` errors and YAML unmarshal
  errors, then falls through to a lower-precedence config, ledger, or binary.
  This contradicts S3's closed bead, which says failures name exact recovery
  without fallback, and the validated plan, which explicitly required
  malformed-config tests.
- **Root cause:** The rendered P3 bead acceptance lost requirements from the
  validated plan. Its focused test pattern covers tracker/worktree names but no
  malformed configuration or cancellation case.
- **Production risk:** A typo or unreadable explicit config can silently select
  the wrong backend or ledger.
- **Recommended outcome:** Return `(value, source, error)` from config loading.
  Ignore only `fs.ErrNotExist`; report permission, parse, and invalid-value errors
  with the exact path. Add precedence-table tests for malformed high-priority
  config and prove no fallback occurs.
- **Current-program coverage:** P3 is closed and no remaining original bead owns
  this repair.

### H5. The accepted `beads` module discards command cancellation

- **Locations:** `cli/internal/commands/beads/module.go:378`, `:393`, `:443`,
  `:485`, `:629`, `:656`
- **Issue:** Six handlers call use cases with `context.Background()` instead of
  `command.Context()`. Ctrl-C, parent cancellation, and caller deadlines do not
  propagate to tracker execution, verification, lint, harvest, stale-claim, or
  resume work.
- **Root cause:** The S2 checker forbids selected effect APIs but has no context
  propagation invariant.
- **Production risk:** Long-running child processes and repository work can
  outlive the invocation that started them.
- **Recommended outcome:** Pass `command.Context()` at every handler boundary;
  add cancellation conformance tests with a blocking fake; add an architecture
  rule rejecting `context.Background/TODO` in command modules except a documented
  process root.
- **Current-program coverage:** The `beads` family bead is sealed; this requires
  a new corrective bead and a ratchet applied to all future families.

### H6. `claim` bypasses the canonical tracker execution context

- **Locations:** `cli/internal/adapters/claim/tracker.go:49`, `:53`, `:54`, `:55`,
  `cli/internal/trackerresolve/resolve.go:147`, `:177`
- **Issue:** `trackerresolve` returns canonical `WorkDir` and `ChildEnv`, including
  removal of `BEADS_DIR` for `bd`. The claim adapter instead runs in the caller's
  cwd and reconstructs the inherited environment, preserving a foreign
  `BEADS_DIR` in its current test. The `beads exec` path honors the resolver.
- **Root cause:** Resolution policy and process-launch policy are separate, and
  adapters are not checked against one shared execution-context conformance
  suite.
- **Production risk:** `ao claim` can address a different repository context
  than other tracker commands.
- **Recommended outcome:** Set `command.Dir = resolution.WorkDir` and
  `command.Env = resolution.ChildEnv`. Better, expose a resolved tracker command
  factory used by claim, close, done, and beads adapters, with `br`/`bd` table
  tests for cwd, env, argv, cancellation, stdout, stderr, and exit mapping.
- **Current-program coverage:** S3 and `claim` are both closed; immutable
  compatibility currently preserves the defect rather than correcting it.

### H7. The machine output contract advertises unsupported behavior

- **Locations:** `cli/cmd/ao/root.go:225`,
  `cli/internal/capabilities/document.go:75`, `:134`,
  `cli/internal/commands/capabilities/module.go:38`,
  `cli/internal/commands/beads/module.go:112`,
  `cli/cmd/ao/status.go:276`
- **Issue:** The root advertises `table`, `json`, and `yaml` globally and the
  capability document says `--json` or `-o json` works on any read-side command.
  Actual support is command-specific and often ignores `--output`.
  `capabilities --output yaml` contradicts help that says output is always JSON;
  because its document types have only JSON tags, YAML emits keys such as
  `schemaversion` rather than `schema_version`.
- **Compiled evidence:** `beads dir --output json` prints plain text while its
  local `--json` prints JSON; `status --output yaml` prints the human table;
  `ready --output yaml` emits JSON; `capabilities --output yaml` emits the
  non-contract YAML key shape.
- **Root cause:** Output is a mutable global hint, not a validated command
  capability. The contract reports global possibility rather than per-command
  support.
- **Production risk:** Automation can receive syntactically valid data in the
  wrong format or parse the wrong field names.
- **Recommended outcome:** Declare supported formats per command node. Reject an
  unsupported requested format with a usage error. Use one typed renderer per
  format and add both JSON and YAML tags when YAML is supported. Generate the
  capability statement from the same declaration.
- **Current-program coverage:** GOALS.md already acknowledges the broader
  read-side `--json` fidelity gap, but the current strangler goal forbids public
  contract changes and has no output-conformance workstream.

### H8. The long-running goal branch has accumulated integration risk

- **Location:** Git graph for `goal/go-cli-clean-architecture-repair` versus
  `origin/main` at audit time
- **Issue:** The branch is 122 commits behind and 115 ahead. Both sides changed
  nine of the same CLI paths, including `root.go`, `capabilities.go`,
  `gate_check_test.go`, and generated `cli/docs/COMMANDS.md`.
- **Root cause:** Ten sequential family migrations accumulated on the isolated
  branch without regularly integrating current `main`.
- **Production risk:** A late rebase can invalidate compatibility lineage,
  ownership digests, accepted SHA seals, or behavior assumptions across many
  already-closed leaves.
- **Recommended outcome:** Establish an integration checkpoint before further
  migration. Reconcile `main`, re-run all sealed-family proofs, and define how
  freeze/accepted lineage survives an integration merge or rebase. Future
  tranches need a maximum divergence policy measured in commits and overlapping
  paths, not only a 30-minute status heartbeat.
- **Current-program coverage:** Final acceptance requires landing on
  `origin/main`, but no intermediate divergence gate exists.

## Medium Findings

### M1. The strangler remains the dominant production architecture

- **Locations:** `cli/cmd/ao/root.go:19`, `cli/cmd/ao/root.go:210`,
  `cli/cmd/ao/zzz_default_spine.go:10`
- **Issue:** Current `cmd/ao` still contains 284 production files, 66,430 lines,
  186 `init()` functions, 544 package-level variable declarations, 283 global
  Cobra command declarations, and 72 direct subprocess sites. The default
  production binary still assembles the broad graph and prunes it afterward.
- **Root cause:** This is expected transitional state, not a failed slice.
- **Recommended outcome:** Continue one-family deletion, then complete P6/P7.
  Preserve the current no-big-bang strategy, but do not call the architecture
  complete until production uses one fresh explicit root.
- **Current-program coverage:** Fully covered by remaining family leaves and
  P6/P7.

### M2. A sealed family left generated CLI documentation red

- **Locations:** `cli/internal/commands/gate/module.go:78`,
  `cli/docs/COMMANDS.md:254`
- **Issue:** `scripts/check-cli-contract.sh` fails because six `ao gate`
  descriptions differ from the generated command reference. `make regen-check`
  is also red, including unrelated skill-catalog drift.
- **Root cause:** Per-family acceptance runs compatibility, architecture, focused
  Go tests, and the fast gate, but generated command/reference parity is deferred
  to tranche/final validation.
- **Recommended outcome:** A family cannot seal after changing help unless the
  generated command reference and surface projections are regenerated and
  checked in the same slice. Add the generated paths to reviewed write scope.
- **Current-program coverage:** Final verification catches it too late.

### M3. Repository Go quality gates are not green

- **Locations:** `cli/internal/archcheck/checker.go:135`,
  `cli/internal/archcheck/inventory.go:32`,
  `cli/cmd/ao/pawl.go:364`,
  `cli/internal/commands/capabilities/module_test.go:62`,
  `cli/internal/commands/claim/module_test.go:40`
- **Issue:** The configured lint reports five findings. Twenty Go files are not
  `gofmt`-clean. Standalone staticcheck also reports deprecated `strings.Title`,
  many unused production/test symbols, and the two suspicious freshness tests.
  `go-complexity-ceiling` is red with 13 functions over the CLI threshold and 18
  over the internal threshold.
- **Root cause:** The full quality suite is tranche/final work, and current lint
  configuration does not enforce formatting or all staticcheck classes.
- **Recommended outcome:** Make format, lint, staticcheck policy, and absolute
  complexity green before calling any tranche production-ready. Refactor the
  architecture checker first; a quality gate should exemplify the quality it
  enforces.
- **Current-program coverage:** Final acceptance includes golangci-lint but not a
  direct gofmt check; GOALS.md separately names the complexity ceiling.

### M4. Several "thin" command modules are presentation monoliths

- **Locations:** `cli/internal/commands/eval/module.go:1` (923 lines),
  `cli/internal/commands/beads/module.go:1` (741),
  `cli/internal/commands/goals/module.go:1` (443),
  `cli/internal/commands/gate/module.go:1` (429)
- **Issue:** These files mostly contain legitimate Cobra presentation, but size
  and broad `UseCases`/`HostOptions` structs make review and local reasoning
  difficult. The checker equates absence of direct effects with thinness.
- **Root cause:** One file per family was used as a convenient migration target,
  and the architecture has no cognitive-size criterion.
- **Recommended outcome:** Keep one package per family but split constructors by
  cohesive subcommand group. Prefer constructor arguments or small option types
  near the consumer; reject host callbacks that merely reach back into mutable
  package-main globals.
- **Current-program coverage:** Not explicitly covered.

### M5. Root error and dependency handling remain centralized and ambient

- **Locations:** `cli/cmd/ao/app.go:12`, `cli/cmd/ao/app.go:50`,
  `cli/cmd/ao/root.go:101`
- **Issue:** `App` remains a broad flag/service bag and silently creates
  production dependencies when missing from context. `Execute` has cyclomatic
  complexity 21 and a long concrete-error type switch with repeated `os.Exit`.
- **Root cause:** Transitional compatibility seams were retained while families
  move.
- **Recommended outcome:** Return a process result from a testable runner, map a
  single typed exit interface at `main`, and fail constructor validation rather
  than creating ambient defaults from context.
- **Current-program coverage:** P7 covers App/globals deletion, but the final
  runner/result shape should be made explicit.

### M6. The default build-tag introspection test does not test the binary

- **Locations:** `cli/cmd/ao/buildtags.go:25`,
  `cli/cmd/ao/buildtags_test.go:13`,
  `cli/cmd/ao/zzz_default_spine.go:24`
- **Issue:** The unit test calls `buildtagsCmd.RunE` directly and proves that the
  handler would print `spine`. The real default binary prunes `buildtags`, so
  `ao buildtags` returns unknown command. The build-tag verification script is
  green because it verifies archive membership, not default introspection.
- **Root cause:** Package tests deliberately skip default-spine pruning.
- **Recommended outcome:** Add a compiled default-binary test or explicitly make
  the hidden command retained in every profile. Do not use a direct handler test
  to claim executable reachability.
- **Current-program coverage:** The compatibility oracle can cover this once the
  hidden-command policy is deliberately chosen.

### M7. Usage and global CLI ergonomics are inconsistent

- **Locations:** `cli/cmd/ao/root.go:101`, `cli/internal/capabilities/document.go:95`
- **Issue:** Unknown commands, unknown flags, and rejected positional arguments
  exit 1, while several command-specific dictionaries call 2 a usage result.
  `--version` works but `-V` does not; `--verbose` exists but there is no global
  `--quiet`. These are not correctness bugs by themselves, but the contract is
  inconsistent and automation must special-case families.
- **Root cause:** Exit semantics were accumulated command by command.
- **Recommended outcome:** Define one CLI-wide exit taxonomy and only override it
  for documented diagnostic verdicts. Decide explicitly whether `-V` and
  `--quiet` belong in the public contract, then test the decision.
- **Current-program coverage:** P5 can project exit declarations but does not
  reconcile the taxonomy.

### M8. New command-module test depth is uneven

- **Locations:** `cli/internal/commands/config`,
  `cli/internal/commands/doctor`, `cli/internal/commands/gate`
- **Issue:** Focused statement coverage is 32.6% for config, 46.8% for doctor,
  and 48.4% for gate. The larger eval and beads modules are 58.3% and 68.2%.
  Coverage percentage is not a quality target by itself, but the untested paths
  include exactly the flag/render/error/cancellation behavior the syntactic
  architecture checker cannot establish.
- **Root cause:** Compatibility checks sample representative behavior while
  module tests prioritize constructor delegation.
- **Recommended outcome:** Add behavior-oriented tests for every runnable child:
  accepted/rejected args, flag-to-request mapping, output writer, error-to-exit
  mapping, and cancellation. Measure scenario coverage, not a universal line
  percentage.
- **Current-program coverage:** Per-family leaves require focused tests but do
  not define a complete runnable-node inventory.

### M9. CLI efficiency has no explicit budget or regression proof

- **Locations:** `cli/cmd/ao/zzz_default_spine.go:24`,
  `cli/internal/cliapp/root.go:19`
- **Issue:** The default binary still assembles a global command graph and then
  prunes it. Five warm local `capabilities --json` samples completed in roughly
  20-60 ms, so the audit found no demonstrated startup emergency, but the repo
  has no stable cold-start, allocation, memory, or large-repository benchmark
  guarding the migration. `BuildRoot` also has no benchmark proving that the
  explicit target improves or at least preserves startup cost.
- **Root cause:** The program correctly prioritized behavior compatibility and
  architecture, but treated efficiency as an assumption rather than a fitness
  function.
- **Recommended outcome:** Add deterministic benchmarks for root assembly,
  `--help`, `capabilities`, tracker resolution, and representative repository
  scans. Record allocations and wall-clock budgets on controlled fixtures; use
  profiles before optimizing. Do not turn incidental laptop timings into a
  release gate.
- **Current-program coverage:** Not explicitly covered.

## Artifact Discrepancies

| Artifact claim | Current evidence | Disposition |
|---|---|---|
| S2 seal proves thin handlers and adapter-owned effects | It proves direct-effect absence in command files plus ownership/profile shape | Over-claimed; H1 |
| S3 fails without silent fallback | Malformed/unreadable YAML is ignored | Contradicted; H4 |
| P3 includes cancellation cases | Closed bead acceptance and tests contain none | Lost during plan-to-bead render; H4/H5 |
| Explicit contracts are the future recursive source | Singular root contract cannot describe descendants; live Inspect ignores attachment | Design incomplete; H2 |
| Capabilities is pure | Universal root pre-run mutates environment and may repair Git config | False end-to-end; H3 |
| Any read-side supports JSON and global YAML is available | Representative commands ignore or misrender requested formats | False; H7 |
| Default `buildtags` reports spine | Only direct handler test does; compiled command is pruned | False executable claim; M6 |
| Each accepted leaf leaves generated surfaces green | Gate help and `COMMANDS.md` currently drift | False; M2 |
| Per-leaf revalidation controls long-run drift | Branch has 122/115 divergence and nine overlapping CLI paths | Operational control insufficient; H8 |

## Existing Program Coverage

| Workstream | Current 101-bead program | Audit judgment |
|---|---|---|
| Immutable four-profile surface | P0, green | Retain; strong for discovery/help shape |
| Explicit metadata vocabulary | P1, green | Retain; insufficient without recursive ownership |
| Fresh profile-aware root skeleton | P2, green | Retain; production lifecycle still undefined |
| One tracker resolver | P3, closed | Reopen via corrective work for fail-closed config, context, and adapter conformance |
| Architecture checker | P4A, closed | Repair before more seals; current claim is broader than checker |
| Remaining 76 families | P4 leaves | Continue only after proof repair and integration checkpoint |
| Remove inferred contracts | P5 | Redesign around recursive command nodes |
| Production composition cutover | P6 | Add root interceptor/effect and process-result invariants |
| Delete package-main strangler | P7 | Retain, with quality and anti-regrowth checks |
| Output-format conformance | none | New workstream required |
| Cross-family tracker execution | none | New shared conformance workstream required |
| Generated surface per leaf | deferred | Move into each family acceptance |
| Branch integration/divergence | final-only | Add tranche/integration gates |
| Go quality/complexity cleanup | final-only/GOALS | Make a staged ratchet, not a final surprise |

## Finding Beads

The eight high findings are persisted as children of audit bead
`age-nw28h.7`; they are evidence packets for goal design, not a claim that their
implementation order is already settled.

| Finding | Bead |
|---|---|
| H1 — semantic architecture proof | `age-nw28h.7.1` |
| H2 — recursive live contracts | `age-nw28h.7.2` |
| H3 — root effects and purity | `age-nw28h.7.3` |
| H4 — fail-closed tracker config | `age-nw28h.7.4` |
| H5 — beads cancellation | `age-nw28h.7.5` |
| H6 — claim tracker context | `age-nw28h.7.6` |
| H7 — output-format truth | `age-nw28h.7.7` |
| H8 — branch integration | `age-nw28h.7.8` |

## Goal-Design Inputs

The next goal-design pass should not discard the strangler and start over. It
should reshape it around these dependency-ordered outcomes:

1. **Re-establish an integrated green base.** Reconcile `origin/main`, preserve
   or deliberately rebind lineage, and rerun all ten accepted-family proofs.
2. **Repair the membrane before producing more seals.** Narrow the existing S2
   claim or strengthen its checks so context, effects, trackers, output, and
   evidence dimensions are actually proven.
3. **Establish shared runtime conformance.** One tracker command factory; one
   context/cancellation rule; one stdout/stderr and exit mapping contract.
4. **Make contracts recursive and executable.** Every runnable command node has
   exact profiles, args, output formats, effects, and exits; assembly rejects
   gaps before effects.
5. **Continue vertical family migration.** Use the improved proof for each
   remaining family and regenerate all public projections in the same slice.
6. **Cut over composition and delete globals.** Production, tests, docs, and
   inspection build the same fresh root; pure commands do not run ambient repair.
7. **Run a dedicated hardening tranche.** Reconcile public output/exit behavior,
   formatting, lint, staticcheck, complexity, compiled-profile reachability,
   coverage gaps, startup behavior, and failure injection.
8. **Close only on main-bound evidence.** Full build/vet/test/race/lint,
   vulnerability scan, four-profile black box, generated surfaces, architecture
   properties, and independent verdict must bind the exact integrated SHA.

## Verification Snapshot

| Check | Result |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test ./...` | PASS |
| `go test -race ./...` | PASS |
| `go mod tidy -diff` | PASS |
| `govulncheck ./...` | PASS — no vulnerabilities found |
| four-profile compatibility, all migrated | PASS |
| build-tag mechanism | PASS, but does not cover M6 |
| clean-architecture scenarios | 5 PASS / 3 expected RED (S1 final, S4 final, S6) |
| all-migrated architecture | FAIL — 373 expected `goals` legacy-symbol violations |
| generated CLI contract | FAIL — six gate help descriptions drift |
| golangci-lint | FAIL — 5 findings |
| gofmt | FAIL — 20 files |
| standalone staticcheck | FAIL — dead/deprecated/suspicious code findings |
| absolute complexity gates | FAIL |
| `make regen-check` | FAIL — command/reference plus unrelated skill-catalog drift |

## Conclusion

The codebase is healthier than it was at the start of the strangler, and the
direction is sound. The next step is not simply to migrate the other 76 command
families faster. It is to make the proof membrane match the semantics it claims,
then continue the migration under that stronger proof. Otherwise the program can
produce beautifully organized packages with sealed evidence while preserving
wrong tracker context, uncancellable work, misleading machine contracts, and
late integration failures.
