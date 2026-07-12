# AgentOps codebase pattern extraction — 2026-07-09

> **Model:** GPT-5 / Codex runtime; exact deployment ID unavailable
> **Pinned base:** `fbba8af5ace635104775ef18f34fef362ba368ce`
> **Scope:** full tracked repository, treating independently owned Go packages,
> Cobra commands, shell programs, generators, and gates as instances
> **Skill:** `codebase-pattern-extraction`

## Method and evidence boundary

This is an intra-repository recurring-pattern audit. I first collected and
classified current-tree evidence with bounded `rg`, `find`, `nl`, and Git reads;
only after fixing the candidate IDs did I read
`docs/audits/codebase-recon-2026-07-02/codebase-pattern-extraction.md`. The
required startup retrieval, `ao lookup --query "AgentOps recurring code patterns
duplication extraction opportunities prior audits"`, returned one generic memory
hit and did not supply code-pattern evidence. A supplementary `cass search` was
attempted, but robot mode returned `checkpoint_incomplete`; I did not rebuild its
external index during this read-mostly audit.

Counts below are lexical measurements at the pinned base. Production-Go counts
exclude `*_test.go` unless explicitly stated. They are evidence of scale, not a
claim that every textual match has identical semantics. Exact `path:line`
citations are the confirmed code facts. Sections labelled **Advice** are design
inferences; no recommendation is presented as implemented behavior.

Repository scale sampled for this audit: 5,163 tracked files; 1,445 Go files
(746 production, 699 tests); 300 production files directly under `cli/cmd/ao`;
401 files under `scripts/`; 526 under `tests/`; and 59 checked-in source skills.

## Measured inventory

| ID | Recurring pattern | Measured current-tree signal | Classification | Existing canonical surface | Priority |
|---|---|---:|---|---|---|
| P-01 | Declarative gate triple | 105 production `gates.Check` literals; 168 top-level `check-*`/`validate-*` scripts; 247 Bats files | healthy reuse | `gates.Check` + `ScriptRunner` + Bats | preserve |
| P-02 | Source → deterministic generator → drift check | 11 write steps and 13 no-write checks in `regen-all.sh`; 25 generated-marker files | healthy reuse | `regen-all.sh` / `regen-changed-scope.sh` | preserve |
| P-03 | Effective project/repo-directory resolution | 160 direct `os.Getwd()` calls in 102 command files versus 40 `resolveProjectDir()` occurrences in 30 | harmful duplication / adoption gap | `resolveProjectDir`, `repoRootOrCwd`, root `App.WorkDir` | high |
| P-04 | YAML frontmatter decoding | 22 parser-named functions in 17 production files; codec referenced by 10 production files | harmful semantic duplication | `wiki.FrontmatterCodec` + port | high |
| P-05 | Durable atomic replacement | 7 production caller files use `storage.AtomicWriteFile`; 13 files contain both `CreateTemp` and `Rename` (including canonical/specialized writers) | extraction already exists; adoption gap | `storage.AtomicWriteFile`, `FsyncDir` | high |
| P-06 | JSONL line scanning and size policy | 84 scanner constructions in 64 files; 46 custom `.Buffer` calls; 44 grandfather entries; zero external callers of `storage.ScanJSONL*` | harmful policy duplication | `storage.ScanJSONL`, `ScanJSONLFile`, shrink ratchet | high |
| P-07 | Shared shell preamble | 345 top-level shell scripts; 9 actual preamble source sites; 325 grandfather data lines | healthy packaged pattern, low adoption | `scripts/lib/preamble.sh` + advisory ratchet | medium |
| P-08 | Typed outcome error → process exit code | 11 command-local `*ExitError` types and 11 `ExitCode()` methods | extraction opportunity | convention only; capabilities documents three command maps | medium |
| P-09 | Thin Cobra compatibility facade over internal package | 23 production command files explicitly identify thin/test-compat wrappers | healthy strangler pattern with cleanup debt | domain packages (`search`, `lifecycle`, `goals`, etc.) | medium |
| P-10 | Boolean-like value normalization | 14 true-only idiom sites; zero `strconv.ParseBool` sites; three different parsers inside `internal/config` alone | harmful semantic duplication | unexported `verifycfg.parseBool` / `config.getEnvBoolValue` | medium |
| P-11 | Two-stage hash-chained records | four independent hash implementations, plus one package delegating to another | security-critical duplication | no generic helper; `drwitness` partially reuses `drrebuild` | highest |

## P-01 — Declarative gate triple

### Confirmed instances

| Independent gate | Registry selection/policy | Executable check | Acceptance test |
|---|---|---|---|
| Shell portability | `cli/internal/gates/checks/seed.go:248` | `scripts/check-shell-portability.sh:1` | `tests/scripts/check-shell-portability.bats:1` |
| JSONL scanner ratchet | `cli/internal/gates/checks/seed.go:391` | `scripts/check-jsonl-scanner-ratchet.sh:1` | `tests/scripts/check-jsonl-scanner-ratchet.bats:1` |
| Live-doc CLI references | `cli/internal/gates/checks/seed.go:381` | `scripts/check-docs-cli-snippets.sh:1` | `tests/scripts/check-docs-cli-snippets.bats:1` |

**Invariant.** A check has a stable ID and routing/blocking policy, one executable
decision surface, and isolated behavioral tests. The Go registry owns selection
and reporting; the check owns domain logic; Bats pins failures, exclusions, and
escape behavior. `.github/workflows/validate.yml:224` consumes the registry as a
whole with `ao gate check --full`, avoiding a second per-check CI switch.

**Variance.** Some checks are always-run, others path-routed; some are advisory,
others blocking; a native `Run` may replace shell backing. The tests may use a
fixture repo, a real-tree smoke, or both.

**Classification and current packaging.** Healthy reuse. The canonical library
shape already exists in `cli/internal/gates`, and the three examples above are
complete gate triples. The registry contains 105 check literals at this base.

**Advice.** Preserve the triple as a gate-authoring template rather than add
another framework. Validation for a new instance should assert: registry ID
uniqueness, path self-reference, positive and planted-negative Bats cases, and
full-gate workflow coverage. Escape hatch: use a native `Run` for logic that is
safer in Go, `Blocking:false` for an explicitly advisory heuristic, and exit 75
only for a genuine structural skip.

## P-02 — Source → deterministic generator → drift check

### Confirmed instances

| Projection | Source/invariant | Generator | Drift proof / composed route |
|---|---|---|---|
| CLI reference | live Cobra help tree | `scripts/generate-cli-reference.sh:4` | `scripts/generate-cli-reference.sh:288`; generated banner at `cli/docs/COMMANDS.md:3` |
| Context map | `skills/*/SKILL.md` frontmatter | `scripts/generate-context-map.sh:4` | `scripts/validate-context-map-drift.sh:2`; generated marker at `docs/contracts/context-map.md:1` |
| SKU registry | skills, workflows, evals, CLI, gates | `scripts/generate-registry.sh:2` | `scripts/generate-registry.sh:8`; composition at `scripts/regen-all.sh:63` and `scripts/regen-all.sh:88` |
| Codex parity twins | source skills plus override catalog | `scripts/codex-sync.sh:2` | check mode at `scripts/codex-sync.sh:37`; composition at `scripts/regen-all.sh:69` and `scripts/regen-all.sh:93` |

**Invariant.** A declared source of truth is transformed deterministically into a
tracked projection; a no-write check reproduces or compares it; the repair
command is explicit. `scripts/regen-all.sh:59` orders the writers and
`scripts/regen-all.sh:86` composes the no-write sweep.

**Variance.** Inputs range from the Cobra tree to YAML/frontmatter and filesystem
trees. Some generators emit one file, while Codex sync emits a tree with a
bespoke opt-out catalog. Some checks compare bytes; others validate a join.

**Classification and current packaging.** Healthy reuse. `regen-all.sh` currently
composes 11 write steps and 13 checks; `regen-changed-scope.sh` is the bounded
repair route. This is stronger than a hand-maintained inventory.

**Advice.** Preserve this as a repository template: every new generated surface
declares source, deterministic ordering, writer, `--check`, repair hint, and
changed-scope routing in one change. Validation: run the generator twice for a
zero diff, mutate one source fixture to prove drift detection, and confirm the
projection is selected by the Go gate. Escape hatch: hand-maintained bespoke
Codex twins stay catalogued opt-outs; no unregistered manual projection.

## P-03 — Effective project/repo-directory resolution

### Confirmed instances

| Command instance | Current behavior |
|---|---|
| Umbrella validation | `cli/cmd/ao/validate.go:121` calls `os.Getwd()` and constructs repo-rooted validation from the raw result |
| Quickstart | `cli/cmd/ao/quickstart.go:95` calls `os.Getwd()` independently |
| Lookup | `cli/cmd/ao/lookup.go:69` calls `os.Getwd()` independently |
| Canonical test seam | `cli/cmd/ao/projectdir.go:21` implements `resolveProjectDir()` |
| Canonical repo-root seam | `cli/cmd/ao/projectdir.go:42` implements `repoRootOrCwd()` and documents subdirectory/worktree behavior at line 39 |

**Invariant.** A command needs an effective working project and often the Git
top-level before joining `.agents/`, `docs/`, or contract-relative paths.

**Variance.** Some commands truly need the invocation directory; repo-artifact
commands need the Git top-level; tests need injection without global `chdir`.
Error wording also varies.

**Classification and adoption.** Harmful duplication and incomplete migration.
There are 160 direct `os.Getwd()` calls across 102 production command files,
while the explicit test seam appears in 30 files. Root pre-run already captures
`App.WorkDir` at `cli/cmd/ao/root.go:73`, but production handlers do not consume
it. The canonical helper exists; proposing another resolver would be duplicate.

**Advice.** Package this as one command-context directory policy: invocation dir
and repo root are named, injectable values, with `resolveProjectDir`/
`repoRootOrCwd` as the migration bridge. Migrate repo-relative joins first.
Validation: table-test root, subdirectory, linked worktree, non-Git directory,
and test override; add a shrink-only static check for new `os.Getwd()` inside
handlers. Escape hatch: a handler may request the literal invocation directory
when that distinction is part of its contract, but must name that choice.

## P-04 — YAML frontmatter decoding

### Confirmed instances

| Package instance | Local semantics |
|---|---|
| Harvest | `cli/internal/harvest/extract.go:230` locates delimiters with substring searches and performs a salvage YAML pass |
| Corpus fitness | `cli/internal/corpus/fitness.go:193` requires a leading delimiter and returns `(map, bool)` on malformed input |
| Provenance | `cli/internal/provenance/provenance.go:164` decodes a typed learning subset and errors on an unterminated block |
| Evolve templates | `cli/internal/evolve/template.go:200` uses a regexp and requires a leading block |
| Resolver field probe | `cli/internal/resolver/resolver.go:250` scans one scalar field line-by-line without YAML decoding |

**Invariant.** Leading `---`-delimited YAML metadata is separated from Markdown,
decoded, and exposed either as a map, typed struct, body offset, or selected
fields.

**Variance.** Callers intentionally differ on strict versus tolerant failure,
typed versus untyped output, missing-closing-delimiter behavior, scalar salvage,
and body preservation.

**Classification and adoption.** Harmful semantic duplication around an existing
canonical helper. `cli/internal/wiki/frontmatter.go:130` declares
`FrontmatterCodec` the single parser; its decode implementation is at line 146
and a port exists at `cli/internal/ports/frontmatter_codec.go:46`. Ten production
files reference the codec, yet 22 parser-named functions remain across 17 files.
The five instances above are independent implementations, not thin wrappers.

**Advice.** Do not add a second frontmatter package. Extend the codec with named
policies (`Strict`, `Tolerant`, optional salvage) and typed decode over the same
delimiter engine, then migrate package by package. Validation must replay each
package's current golden cases: CRLF, no block, missing closer, malformed YAML,
colon-bearing scalar, body round-trip, and typed-field mismatch. Escape hatch:
callers may supply a policy/typed target, but delimiter discovery and YAML
boundary handling remain canonical.

## P-05 — Durable atomic replacement

### Confirmed instances

| Package instance | Local writer behavior |
|---|---|
| Config | `cli/internal/config/config.go:693` writes a fixed `.tmp` path then renames; no file or directory fsync |
| Wiki source registry | `cli/internal/wiki/source.go:106` creates a sibling temp and renames at line 119; no fsync |
| RPI phased state | `cli/internal/rpi/phased_state.go:77` implements temp + file sync + rename; no parent-directory sync |
| Constraint index | `cli/internal/search/constraint.go:167` implements temp + chmod + rename; no file or parent-directory sync |

**Invariant.** Serialize complete content to a same-directory temporary file,
apply permissions, close it, and atomically rename over the target so readers do
not observe a partial file.

**Variance.** Required mode, whether parent directories are created, whether
file/dir fsync is required, whether a cross-process lock exists, and whether the
operation is a content replacement or a real move/transaction.

**Classification and adoption.** The extraction exists; adoption remains
partial. `storage.AtomicWriteFile` is canonical at
`cli/internal/storage/atomicfile.go:26`, including file fsync, chmod, rename, and
parent-directory fsync at line 59. Seven production caller files invoke it. A
lexical intersection still finds 13 production files containing both
`os.CreateTemp` and `os.Rename`; that set includes canonical and specialized
writers and therefore is a candidate set, not 13 proven defects.

**Advice.** Migrate plain single-file replacement sites to
`storage.AtomicWriteFile`; keep a small serialization wrapper in the owning
package. Validation: fault-inject write/sync/chmod/close/rename/dir-sync, assert
old-or-new bytes only, correct modes, and no leaked temp file. Escape hatch:
multi-file transactions, lock-coupled appends, and genuine renames may retain a
specialized writer, but should call `storage.FsyncDir` where durability requires
it and document why `AtomicWriteFile` is insufficient.

## P-06 — JSONL line scanning and size policy

### Confirmed instances

| Reader instance | Local policy |
|---|---|
| Yield ledger | `cli/internal/yieldledger/loader.go:36` uses a 4 MiB maximum |
| Provenance graph | `cli/internal/provenancegraph/store.go:50` uses a 1 MiB maximum |
| Context cycle history | `cli/cmd/ao/context_assemble.go:271` uses a 256 KiB maximum and logs scanner errors as warnings |
| Canonical scanner | `cli/internal/storage/file.go:251` uses one loud 8 MiB inclusive policy and reports line number |

**Invariant.** Open/receive a JSONL stream, scan nonblank lines, decode each
record, report a line number, and surface oversized or malformed records.

**Variance.** Current maximums span 256 KiB, 1 MiB, 2 MiB, 4 MiB, and 8 MiB;
some callers warn, some fail; some need raw bytes and some typed decode.

**Classification and adoption.** Harmful policy duplication with an existing
canonical implementation. There are 84 `bufio.NewScanner` constructions in 64
production files and 46 custom `.Buffer` calls. The shrink list contains 44 data
lines (one is a test file). `storage.ScanJSONL` and `ScanJSONLFile` exist at
`cli/internal/storage/file.go:251` and line 277, but no production file outside
the storage package calls them. The advisory prevention surface is complete:
registry `cli/internal/gates/checks/seed.go:391`, check
`scripts/check-jsonl-scanner-ratchet.sh:1`, and Bats
`tests/scripts/check-jsonl-scanner-ratchet.bats:1`.

**Advice.** Finish adoption rather than invent another scanner. Add an
error-returning callback variant if typed decoders need to abort immediately,
then migrate ledgers in risk order and shrink the grandfather file. Validation:
blank lines, malformed JSON at a named line, the checked-in ~67.5 KiB transcript
case, exactly-at-cap, cap+1, read error, and close error. Escape hatch: non-JSONL
protocol streams may use a separately named configurable line scanner; they
must not silently inherit the JSONL policy.

## P-07 — Shared shell preamble

### Confirmed instances

| Script instance | Adoption point |
|---|---|
| ADR registry gate | `scripts/check-adr-registry.sh:20` sources the preamble |
| JSONL scanner ratchet | `scripts/check-jsonl-scanner-ratchet.sh:82` sources the preamble |
| Skill probe | `scripts/probe-skill.sh:53` sources the preamble |
| Docs duplicate check | `scripts/check-docs-duplicates.sh:30` sources the preamble |

**Invariant.** Strict mode, a CWD-hijack-proof `REPO_ROOT`, portable find/stat,
temporary-directory cleanup, and dependency checks are sourced once. The
helper implementations live at `scripts/lib/preamble.sh:17`, line 37, line 50,
line 105, and line 133.

**Variance.** Individual scripts still own arguments, output contracts, and
domain logic. Some genuinely cannot assume repository context.

**Classification and adoption.** Healthy packaged extraction with low adoption.
Nine scripts contain an actual source statement, versus 345 top-level shell
scripts and 325 grandfather entries. The shrink-only advisory gate is registered
at `cli/internal/gates/checks/seed.go:257`, implemented at
`scripts/check-new-scripts-use-preamble.sh:1`, and tested at
`tests/scripts/check-new-scripts-use-preamble.bats:1`.

**Advice.** Preserve the preamble and let touched-file migration shrink the
snapshot; do not mass-rewrite 300 scripts. Validate each helper on macOS and
Linux plus CWD hijack, missing dependency, multiple temp dirs, and trap cleanup.
Promote the ratchet from advisory only after one measured clean cycle. Escape
hatch: `# preamble-exempt: <non-empty reason>` is already supported; keep it
reviewable and shrink-only.

## P-08 — Typed outcome error → process exit code

### Confirmed instances

| Command instance | Type and mapping |
|---|---|
| Validation | `cli/cmd/ao/validate.go:40` defines `gateExitError`; `ExitCode()` at line 48 |
| Beads verdicts | `cli/cmd/ao/beads.go:51` defines `beadsExitError`; `ExitCode()` at line 58 |
| Governor budget | `cli/cmd/ao/governor.go:130` defines `governorExitError`; `ExitCode()` at line 136 |
| Wiki health | `cli/cmd/ao/wiki_health.go:26` defines `wikiHealthExitError`; `ExitCode()` at line 31 |

**Invariant.** A Cobra handler returns a typed error carrying a machine verdict
code; root `Execute()` recognizes it and exits without converting a normal
negative verdict into generic stderr noise.

**Variance.** Code vocabularies and whether a message is printed vary. Some
negative decisions are silent because the command already rendered detail;
internal/usage failures may need stderr.

**Classification and adoption.** Extraction opportunity. Eleven command-local
types each repeat `Error`/`ExitCode`, and root has type-specific `errors.As`
branches. No canonical error type exists. A per-command capabilities map now
documents three default-build commands at `cli/cmd/ao/capabilities.go:81`, which
partially closes the July 2 contract gap but does not cover the full repeated
mechanism.

**Advice.** Package a small `exitcode.Error` (code, message, quiet flag) and have
root unwrap the `interface{ ExitCode() int }` once; retain type-specific handling
only where there is actual extra behavior. Derive capabilities entries from a
single command exit-code registry or gate them against constants. Validation:
table-test every current type/code/message behavior and assert ordinary errors
remain exit 1. Escape hatch: a command may define a richer type that implements
the interface when it carries additional structured state.

## P-09 — Thin Cobra compatibility facade over an internal package

### Confirmed instances

| Command facade | Canonical implementation |
|---|---|
| Dedup | wrappers begin at `cli/cmd/ao/dedup.go:77` and delegate to `internal/lifecycle` |
| Search | `cli/cmd/ao/search.go:391` delegates grep/JSONL helpers to `internal/search` |
| Session parsing | `cli/cmd/ao/inject_sessions.go:52` delegates three parsers to `internal/search` |
| Learning parsing | alias/wrappers begin at `cli/cmd/ao/inject_learnings.go:654` and delegate to `internal/search` |

**Invariant.** Domain logic moves from package `main` to a testable internal
package while a one-line alias/wrapper preserves command-local call sites and
historical tests during migration.

**Variance.** Some wrappers only rename; some adapt globals such as verbosity or
feedback flags; some retain a type alias. Their legitimate lifetime differs.

**Classification and adoption.** Healthy strangler pattern with cleanup debt,
not a reason to create another library. Twenty-three production command files
explicitly identify thin or test-compatibility wrappers. The canonical code is
already in `internal/search`, `internal/lifecycle`, `internal/goals`,
`internal/quality`, and related packages.

**Advice.** Keep wrappers as a time-bounded migration device: move tests to the
canonical package, make Cobra tests assert wiring only, then delete zero-value
facades. Validation: package tests own behavior; command tests cover flags,
stdio, and exit mapping; a temporary inventory test can ensure wrappers remain
one-line delegation. Escape hatch: retain a facade when it is a deliberate
adapter for globals, compatibility, or output shaping, and document that reason
instead of calling it merely “for tests.”

## P-10 — Boolean-like value normalization

### Confirmed instances

| Parser instance | Accepted vocabulary |
|---|---|
| Legacy config `envBool` | `cli/internal/config/config.go:478` accepts exact lowercase `true`/`1` only |
| Config `getEnvBool` | `cli/internal/config/config.go:723` repeats exact lowercase `true`/`1` and cannot distinguish explicit false from unset |
| Config `getEnvBoolValue` | `cli/internal/config/config.go:732` accepts trimmed/case-folded `true/1/yes/on` and false forms |
| Verify config | `cli/internal/verifycfg/verifycfg.go:438` independently implements the same broad vocabulary |
| Forge legacy switch | `cli/cmd/ao/forge.go:1067` accepts `1/true/yes`, but not `on` or false forms |

**Invariant.** Normalize a string-like configuration value into `(value,
recognized)` so precedence logic can distinguish unset, explicit false, and an
invalid token.

**Variance.** Some kill switches intentionally accept only `1`; others are
user-facing booleans and should accept the documented common vocabulary. Invalid
values may warn, fail, or fall through.

**Classification and adoption.** Harmful semantic duplication. Fourteen
production lexical sites use true-only idioms and no production code uses
`strconv.ParseBool`. Even `internal/config` carries three different behaviors.
The best current broad implementation is private to a package, so there is no
cross-package canonical helper.

**Advice.** Package `envparse.Bool(value) (bool, recognized)` with one documented
vocabulary and a separate, explicitly named `EnabledOnlyByOne` for hard kill
switches. Validation: case/whitespace matrix; all true/false forms; invalid and
unset distinction; precedence behavior. Escape hatch: security-sensitive
one-token switches remain strict, but the strictness must be visible in the
helper name and help text.

## P-11 — Two-stage hash-chained records

### Confirmed instances

| Package instance | Hash implementation |
|---|---|
| Provenance graph | `cli/internal/provenancegraph/edge.go:222` marshals an explicit payload subset and hashes it plus `PrevHash` |
| Turn state | `cli/internal/turnstate/turnstate.go:90` repeats the two-stage SHA-256 shape over a transition payload |
| RPI ledger | `cli/internal/rpi/ledger.go:152` normalizes details, marshals a ledger payload, and chains it |
| Disaster-rebuild witness | `cli/internal/drrebuild/drrebuild.go:244` repeats the provenance-style payload and chain calculation |
| Partial reuse | `cli/internal/drwitness/drwitness.go:136` delegates witness hashes to `drrebuild.ComputeEventHashes` |

**Invariant.** Canonical record bytes produce a SHA-256 `payload_hash`; the
record's chain hash is SHA-256 of `payload_hash + "\n" + prev_hash`; verification
recomputes both and checks link order.

**Variance.** Payload schemas, excluded join fields, identity/dedup keys, schema
versions, and append locking differ. A crucial semantic difference refines the
July 2 report: `rpi.LedgerPayload` includes `PrevHash` in the payload at
`cli/internal/rpi/ledger.go:61`, while provenance and turn-state payload structs
explicitly exclude it at `cli/internal/provenancegraph/edge.go:128` and
`cli/internal/turnstate/turnstate.go:71`. These implementations therefore share
a shape, not byte-identical payload semantics.

**Classification and current packaging.** Security-critical duplication. There
is no generic `internal/hashchain` package. Four packages independently own the
calculation; `drwitness` demonstrates that reuse is possible by delegating to
`drrebuild`.

**Advice.** Extract only the cryptographic chain kernel, parameterized by a
caller-supplied canonical payload encoder and legacy policy; do not force one
record schema or silently change historical bytes. Validation must replay every
checked-in/fixture chain byte-for-byte, verify mixed schema versions, mutation,
reorder, deletion, duplicate append, genesis, and the RPI prev-hash-in-payload
variant. Roll out one ledger at a time behind a dual-compute assertion. Escape
hatch: a legacy codec remains registered indefinitely for already-committed
records; rollback is switching the writer to the old codec while leaving the
shared verifier able to read both.

## Packaging and priority decision

| Order | Pattern | Recommended artifact | Why now | Validation gate / escape |
|---:|---|---|---|---|
| 1 | P-11 | `cli/internal/hashchain` kernel + per-ledger codecs | duplicated security invariant with discovered semantic drift | golden historical chains; dual-compute rollout; legacy codec escape |
| 2 | P-04 | finish adoption of `wiki.FrontmatterCodec` policies | 17 parser files can disagree on document boundaries | cross-package golden matrix; named strict/salvage policy |
| 3 | P-06 | migrate to `storage.ScanJSONL*` and shrink grandfather | 44 heuristic debt entries and five size policies | existing advisory ratchet; non-JSONL scanner escape |
| 4 | P-03 | one invocation-dir/repo-root command context | 102 command files bypass the test/root seam | new-site ratchet; literal invocation-dir escape |
| 5 | P-05 | migrate simple writers to `AtomicWriteFile` | canonical durability now includes directory fsync | fault tests; specialized transaction escape |
| 6 | P-08 | generic exit-code error + one capabilities registry | 11 local types and a growing root switch | table tests; richer interface implementation escape |
| 7 | P-10 | shared boolean parser + strict kill-switch helper | user-visible vocabulary already differs inside one package | token matrix; explicitly strict helper escape |
| preserve | P-01, P-02 | gate triple and generator/check template | these are working anti-drift assets | native/advisory and bespoke catalog escapes |
| continue | P-07, P-09 | shrink-only adoption / wrapper retirement | both are sound migration mechanisms, not missing abstractions | reasoned exemption / documented adapter escape |

## Delta versus the 2026-07-02 pattern audit

The earlier report was read only after the current inventory above was fixed.
The following are current-code deltas or refinements, not inferred chronology.

| July 2 item | Current pinned-base result |
|---|---|
| P1 hash-chained ledger | Still no generic helper. This audit confirms four independent implementations and refines the claimed invariant: RPI includes `prev_hash` in `payload_hash`, while provenance/turnstate do not. Extraction must preserve both codecs. |
| P2 typed exit error | Expanded from the prior report's 9 named sentinel types to 11 current `*ExitError` types. `capabilities` now publishes command-specific maps for three commands (`cli/cmd/ao/capabilities.go:81`), partially closing prior A7; generic unwrapping remains open. |
| P3 deterministic deciders | Still a healthy architectural pattern. No new abstraction is warranted by the current collection; retain pure, fail-closed decision functions and table tests. |
| P4 fail-closed absence / A1 | The cited fail-open gate defect is fixed: `cli/internal/gates/orchestrator.go:191` makes blocking UNKNOWN/unrecognized verdicts fail, and `cli/internal/gates/report.go:26` routes that predicate into the process result. |
| P5 decentralized registration | The pattern remains healthy and has grown: current lexical counts are 353 `AddCommand` calls, 398 Cobra command literals, and 105 gate check literals versus the earlier approximate 343/~90. |
| P6 generated contracts | Strengthened and made more explicit: `regen-all.sh` now composes 11 writers and 13 no-write checks; command exit codes are partly represented in capabilities. P-02 records the reusable generator/check triple. |
| P7 atomic write | The earlier parent-directory fsync gap is closed in the canonical helper at `cli/internal/storage/atomicfile.go:59`. Adoption is still incomplete; P-05 avoids equating every remaining rename with a defect. |
| P8 capability degradation | Still a healthy BC6 design pattern; not promoted into this audit's extraction backlog because it already has a canonical selector/output-contract seam. |
| P9 no-self-grade | Still a healthy centralized invariant. The current direct Go calls are visible at `cli/internal/evidencedturn/evidencedturn.go:351` and `cli/internal/ports/claim_evidence_binder.go:61`; shell/contract mirrors remain separate adapters. |
| P10 SKIP verdict | The 75 (gate) versus 77 (goals) split remains at `cli/internal/gates/scriptrunner.go:169` and `cli/internal/goals/measure.go:39`; reconcile only with an explicit compatibility decision. |
| Newly surfaced here | P-03 project-dir adoption, P-04 incomplete codec adoption, P-06 scanner-policy migration, P-07 preamble adoption, P-09 compatibility-facade cleanup, and P-10 boolean normalization were not measured as standalone patterns in the July 2 report. |

## Validation plan for this report

This report itself is complete when:

1. the artifact is non-empty and names the full pinned SHA;
2. every P-ID has at least three exact `path:line` instances;
3. each pattern separates invariant, variance, classification/current helper,
   packaging advice, validation, and escape behavior;
4. the delta section names both resolved and still-open July 2 findings; and
5. only this tracked report is changed by this worker.

No source extraction is implemented here. Every proposed migration is reversible
by retaining the current package-local implementation until its golden cases pass
through the canonical helper; P-11 additionally requires legacy codecs to remain
readable permanently.
