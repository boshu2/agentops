# AgentOps codebase archaeology — 2026-07-09

> **Skill:** `codebase-archaeology`
> **Snapshot:** `fbba8af5ace635104775ef18f34fef362ba368ce`
> **Model:** GPT-5 / Codex runtime; exact deployment id is unavailable.
> **Scope:** the full tracked repository at the pinned snapshot. Private `_beads/`, uncommitted files, remediation, full gates, and full test suites are out of scope.
> **Comparison:** [`../codebase-recon-2026-07-02/codebase-archaeology.md`](../codebase-recon-2026-07-02/codebase-archaeology.md).

## Executive summary

AgentOps is a local verification membrane, not an agent host. Its shipped center is a Cobra CLI plus skills and deterministic gates. The product path converts work into independently checked evidence, binds that evidence to commits in a hash-chained provenance ledger, and refuses release when a blocking check cannot produce an authorizing verdict. The recurring architectural rule is literal in the executable: **no verdict = not done**.

This snapshot is a validation-centered hybrid rather than a fully hexagonal application. The newer BC-scoped code has strong ports, typed domain data, append-only ledgers, fail-closed decisions, and deterministic generators. The older and larger command layer still owns substantial orchestration and direct filesystem/subprocess behavior. That partial retrofit is not hidden by the source, but several narrative pages overstate its completeness or describe an obsolete command surface.

The most important change since the 2026-07-02 archaeology is the new canonical `ao land` path. It builds and re-executes a fresh in-checkout binary, pins that binary through review and gates, runs the trusted pawl review, rebases/restamps, and performs one gated push (`cli/cmd/ao/land.go:30-61`, `150-234`). The verification membrane also now has deterministic preflight, live smoke support, cost telemetry, outage-class failover, an honest strict-quorum unavailable state, and a proof-carrying `REBOUND` path.

The most important fresh risks are contract drift:

1. `docs/architecture/ports-and-adapters.md` describes four packet invariants that no longer exist and says there are three ports; the executable has schema-driven validation and 29 declared port interfaces.
2. `schemas/pawl-verdict.v1.schema.json` says `REBOUND` never authorizes, while `scripts/pawl-verdict.sh check` explicitly authorizes a `REBOUND` after full lineage, byte-identity, and gate proof.
3. `ao capabilities` omits pawl/verify exit code 5 and exposes only three environment variables even though the default CLI honors many more. The prior archaeology's assertion that these dictionaries were drift-proof is no longer true.
4. Malformed `.aoverify.yaml` policy warns and falls back to defaults, including `strict=false`; this can weaken an intended committed verification posture rather than fail closed.

### Confidence vocabulary

- **CONFIRMED** — directly observed in the pinned source/generated artifact or reproduced by a bounded command.
- **INFERENCE** — an architectural interpretation supported by several confirmed facts.
- **RISK** — a confirmed mismatch or behavior with a plausible failure consequence; impact was not exercised end to end here.
- **UNKNOWN** — deliberately not established within this audit's non-mutating, bounded scope.

## Measured snapshot

All counts below were recomputed at the pinned commit. They are measurements, not copied documentation.

| Dimension | 2026-07-09 measurement |
|---|---:|
| Tracked files | 5,163 |
| Go files / production / tests | 1,445 / 746 / 699 |
| Go LOC / production / tests | 402,880 / 164,946 / 237,934 |
| Go packages in `cli` default profile | 109 |
| `cmd/ao` files / test files | 670 / 369 |
| Default macOS `cmd/ao` production files / LOC | 259 / 66,988 |
| Shell files / LOC | 753 / 120,608 |
| Shell files under `scripts/` | 363 (345 at the directory root) |
| Python files / LOC | 59 / 15,141 |
| Markdown files / LOC | 1,854 / 268,887 |
| Root `schemas/` JSON documents | 64 |
| Active skill contracts / Codex twins | 59 / 58 |
| Claude workflow scripts | 4 |
| Registry top-level commands / capabilities | 72 / 145 |
| Documented default CLI nodes | 74 top-level including help/completion; 201 nested |
| CLI coverage inventory leaf rows | 257 (174 covered, 83 explicitly allowlisted) |
| Registered Go gate checks | 112 (103 shell-backed, 9 native Go) |
| `validate.yml` / nightly jobs | 14 / 6 |
| Bats / Gherkin feature files | 283 / 62 |
| Test-like tracked files / Go test functions | 1,340 / 7,586 |
| Commits in last 7 / 30 / 90 days | 231 / 1,217 / 1,304 |
| Provenance ledger records | 422 |

The tracked provenance ledger appears in 99 of the last 200 commits, more than any other path. That supports the inference that proof binding is routine repository behavior, not only product language. It does not by itself establish that every historical verdict was correct.

The generated `registry.json` distinguishes two quantities that narrative docs often conflate: 112 executable checks exist in `cli/internal/gates/checks`, while the capability catalog exposes 14 workflow gate capabilities. The catalog's 145 total is `59 skills + 72 CLI commands + 14 gate capabilities`, not the executable gate-check count.

### Reproduction commands for the snapshot

```bash
git rev-parse HEAD
git ls-files | wc -l
git ls-files 'cli/*.go' 'cli/**/*.go' | sort -u | wc -l
git ls-files 'cli/*_test.go' 'cli/**/*_test.go' | sort -u | wc -l
git ls-files '*.sh' | xargs wc -l | tail -1
git ls-files '*.py' | xargs wc -l | tail -1
git ls-files '*.md' | xargs wc -l | tail -1
(cd cli && go list ./... | wc -l)
jq '.summary, .capability_summary' registry.json
rg '\bID:\s*"[^"]+"' cli/internal/gates/checks --glob '*.go' | wc -l
rg '\bBacking:\s*"' cli/internal/gates/checks --glob '*.go' | wc -l
rg '\bRun:\s*' cli/internal/gates/checks --glob '*.go' | wc -l
```

## System shape

```text
operator / coding agent
        |
        v
skills + ao Cobra commands -----------------------------+
        |                                                |
        | intent / work / changed files                  | direct shell, git,
        v                                                | filesystem, br/bd,
operating loop + bead contract                           | tmux, codex/agy
        |                                                |
        v                                                |
ao verify / pawl review -----> independent reviewer      |
        |                         |                      |
        | verdict + evidence <---+                      |
        v                                                |
provenance ledger + yield ledger                         |
        |                                                |
        v                                                |
ao gate check -> declarative registry -> ports/adapters <-+
        |
        v
ao land -> rebase -> restamp proof -> gated push
```

The repository declares six bounded contexts: Corpus, Validation, Loop, Factory, Runtime, and Orchestration. In code, their boundaries are conventions and packages rather than independently deployed services. The CLI root is the common process, scripts remain first-class implementations, and several packages bridge more than one bounded context.

### Architectural assessment

**AR-01 — CONFIRMED: validation-centered hybrid.** The default binary is one Cobra process. `rootCmd` installs global worktree sanitation and an `App` context (`cli/cmd/ao/root.go:27-78`), and typed errors become process exit codes (`root.go:80-183`). Commands self-register with `init()` rather than through a central command table. The architecture combines:

- a large command adapter layer;
- BC-scoped internal packages;
- 29 port interfaces in `cli/internal/ports`;
- 20 adapter packages under `cli/internal/adapters`;
- one explicit domain package, `cli/internal/domain/packet`;
- hundreds of deterministic shell programs that remain production implementations.

**AR-02 — INFERENCE: the hexagonal migration is valuable but incomplete.** The default active `cmd/ao` build is 66,988 production LOC and directly contains 68 `exec.Command` calls and 259 `os` file-operation calls. Only one domain package exists. These measurements do not imply those calls are defective; they show that the command layer is still an orchestration/application layer, not uniformly a thin adapter over domain ports.

**AR-03 — CONFIRMED strength: explicit compatibility profiles.** The default build excludes `flywheel` and `legacy` command sets; `make build-flywheel` restores both (`cli/Makefile:11-20`, `38-56`). There are 109 build-tagged Go files across legacy, flywheel, and platform profiles. Therefore source presence is not proof of shipping command availability. The generated command reference and registry are built from a live default binary (`scripts/generate-cli-reference.sh:33-39`, `247-303`; `scripts/generate-registry.sh:334-395`).

Notable default omissions include `ao corpus`, `ao codex`, `ao rpi`, `ao loop`, `ao autodev`, and `ao orchestrate`; their source is archived/tagged. `ao inject`, `ao compile`, and `ao forge` remain default commands. This is the key interpretive rule for reading the repository.

## Entry points and registration

### Shipped executables

1. `cli/cmd/ao/main.go:5-12` is the main CLI entry and identifies version `3.2.0-rc` at this snapshot.
2. `cli/cmd/skill-frontmatter-json/main.go:14-53` is a small frontmatter conversion helper.
3. `cli/cmd/witness-crosscheck/main.go:29-66` performs independent witness cross-checking.

Fixtures and workbench mains exist but are not equivalent to shipped operator entry points.

### CLI startup

`rootCmd.PersistentPreRunE` performs four global actions before a leaf runs: translates `--json`, materializes the config flag into the environment, sanitizes inherited Git worktree process state, repairs shared worktree config, and injects an `App` into the command context (`cli/cmd/ao/root.go:50-77`). `App` holds flag state and dependency-injection seams for command execution, IO, randomness, clock, environment, and exit behavior (`cli/cmd/ao/app.go:12-55`).

This is a strong testability pattern, but it coexists with mutable package globals and per-command seams throughout `cmd/ao`; the migration is incremental.

### Command discovery

There is no handwritten canonical command list. Each command attaches itself to `rootCmd` from `init()`. The generated reference builds a temporary binary, scrapes live help, normalizes absolute paths, and recursively renders subcommands. `registry.json` similarly builds the live binary and joins command nodes with skill, disposition, tier, and CI sources (`scripts/generate-registry.sh:334-395`, `538-664`). This makes default-build presence authoritative and is one of the strongest anti-drift mechanisms in the repository.

## Core types

| Type | Location | Role and invariant |
|---|---|---|
| `App` | `cli/cmd/ao/app.go:12-55` | Per-command process context: flags plus injectable effects. |
| `packet.ExecutionPacket` | `cli/internal/domain/packet/aggregate.go:4-58` | Rich execution/handoff aggregate. Raw JSON is validated against the embedded canonical schema; unknown fields fail. `EffectiveVerdict` maps absent or unknown values to FAIL (`aggregate.go:112-123`). |
| `packet.Criterion` | `aggregate.go:141-153` | Acceptance criterion with check type/command, evidence path, weight, and judge. |
| `gates.Check` | `cli/internal/gates/gates.go:53-86` | Declarative gate: unique ID, tier bitmask, path routing, blocking flag, and exactly one shell backing or native function. |
| `ports.GateVerdict` / `GateRunnerPort` | `cli/internal/ports/gate_runner.go:11-20`, `73-75` | Typed PASS/WARN/FAIL/SKIP/UNKNOWN result and execution seam. |
| `provenancegraph.Edge` | `cli/internal/provenancegraph/edge.go:70-126` | Hash-protected PROV-O relationship with reviewer/cost enrichment and chain fields. |
| `provenancegraph.Store` | `cli/internal/provenancegraph/store.go:12-23`, `107-223` | Append-only, idempotent, cross-process-locked authority for `docs/provenance/ledger.jsonl`. |
| `yieldledger.Event` / `GateVerdictBody` | `cli/internal/yieldledger/yieldledger.go:103-177` | Runtime event stream for accepts, verdicts, usage, escape classification, and affected paths. |
| `config.Config` | `cli/internal/config/config.go:19-53` | General CLI configuration with layered resolution; also retains archived RPI/Dream fields. |

### Execution packet contract

`ExecutionPacket.Validate` serializes and applies the embedded JSON schema; `ValidateJSON` validates raw bytes so struct unmarshalling cannot hide unknown properties (`cli/internal/domain/packet/invariants.go:30-54`). `DecodeJSON` also migrates the old slim form, then validates the migrated packet before admitting it (`invariants.go:56-83`). The required rich fields are currently only `schema_version` and non-empty `objective` (`schemas/execution-packet.schema.json:8-22`); `plan_path`, complexity, test levels, and provenance are optional.

The filesystem adapter validates run IDs against traversal, writes the archive before `latest`, and uses the repo's fsync/rename atomic writer (`cli/internal/adapters/storage_fs/packet_repo.go:21-78`, `96-115`). A bounded byte comparison confirmed the root and embedded execution-packet schemas are identical.

## End-to-end flow 1: verify and land

### Generic verification front door

`ao verify` is intentionally a thin alias to `runPawlReview`, not a parallel reviewer (`cli/cmd/ao/verify.go:16-22`, `149-185`). It supports zero-argument HEAD labels and config/rebind short circuits. The trust boundary is physical binary containment:

1. If the running `ao` binary lives inside the resolved AgentOps checkout, the command may run live repository scripts (`cli/cmd/ao/pawl.go:119-140`).
2. Otherwise, it extracts an embedded pawl bundle and runs that against the user's Git repository (`pawl.go:142-146`, `202-229`).
3. The stranger path strips relative and repository-owned PATH entries, clears `BASH_ENV`, `ENV`, and `GIT_EXTERNAL_DIFF`, prevents service startup and repo-local CLI execution, and pins `AO_BIN` to the trusted invoking binary (`pawl.go:231-260`).
4. Bash is resolved to an absolute executable on the sanitized path, and the script's typed exit code propagates unchanged (`pawl.go:268-299`).

**AR-04 — CONFIRMED strength: this is a meaningful RCE boundary.** A foreign repository cannot obtain live-script execution merely by forging AgentOps marker files. The stated threat model remains single-operator code the user chose to check out, not hostile multi-tenant isolation (`cli/cmd/ao/verify.go:66-77`).

### Review engine

The shell engine is a substantive state machine, not a single model call:

- shared guarded Codex execution, diff identity, trivial-bind handling, deterministic preflight, and amend guards are sourced at entry (`scripts/pawl-review.sh:65-103`);
- reviewer names map to explicit binary/family specifications, while unknown names fail closed (`pawl-review.sh:492-504`);
- a routed warm result is trusted only after the same `pawl-verdict.sh check` used by the release gate validates it (`pawl-review.sh:1535-1562`);
- cold reviewer failures are classified so only outage-class failures fail over; substantive/garbled outcomes do not silently switch judges;
- incomplete reviewer processes cannot be mined for an early `CONFIRMED` token;
- staged review intentionally does not write an authorizing commit-bound verdict;
- converge mode records both the rebase-stable patch ID and whitespace-significant content identity;
- output writes are followed by the verdict checker before success is returned.

Default cold review is available. Strict review is not currently available: the strict voter set contains only Codex, while strict requires two distinct eligible families. The engine exits 5 rather than degrade (`cli/cmd/ao/verify.go:38-64`; `scripts/pawl-review.sh:45-62`). This is honest fail-closed behavior, but it means the strongest advertised posture is machinery awaiting a second eligible adapter, not active protection.

### Canonical landing path

`ao land <bead>` is specific to the AgentOps checkout. Its sequence is:

1. Resolve a genuine AgentOps repository; reject foreign repositories.
2. Build `cli/bin/ao` from the current checkout.
3. Re-exec the entire verb through that binary so the physical-containment trust test passes.
4. Pin `AO_BIN` so preflight, verdict emission, and the pre-push gate use one fresh executable.
5. Best-effort warm the pawl service; cold review remains possible.
6. Run `ao pawl review <bead> --scope head`; any non-confirming outcome stops the land.
7. On confirmation, run `scripts/pawl-land.sh` for fetch, rebase, restamp, and one gated push.
8. Best-effort emit the trunk-bound landed provenance edge.

The implementation and failure propagation are at `cli/cmd/ao/land.go:150-234`. This closes a real stale-installed-binary gap and is the largest operational improvement since the prior archaeology.

### Verdict binding and REBOUND

A normal verdict is commit-bound and evidence-bound, then projected as a provenance edge. `provenancegraph.Store` seals and appends under a cross-process sidecar lock, rejects malformed prior rows, and makes duplicate identities a no-op (`cli/internal/provenancegraph/store.go:26-48`, `107-223`). `VerifyChain` recomputes every payload and link (`edge.go:282-307`).

`REBOUND` permits review reuse after a no-op rebase only when:

- the original lineage verdict itself passes the full authorizing check;
- stable patch IDs match;
- whitespace-significant added/removed content is byte-identical;
- the new tip passed the full gate at write time;
- lineage and evidence paths remain present.

The executable check is deliberately more than a disposition string comparison (`scripts/pawl-verdict.sh:714-825` and subsequent identity checks). This is a careful optimization with a documented caveat: even identical diff bytes on a new base can have different semantics; the green gate is the compensating control.

## End-to-end flow 2: `ao gate check`

`cli/cmd/ao/gate_check.go` imports the check packages for registration, creates the default registry, wires a shell adapter and changed-files port, runs the orchestrator, renders a report, and maps blocking failures to a typed exit.

The gate model has a useful single-registry invariant: Fast and Full are bitmask filters over the same `Check` records, so separate lists cannot drift (`cli/internal/gates/gates.go:21-35`). The selection algorithm is:

1. filter by tier;
2. Full selects every matching-tier check;
3. Fast obtains the requested changed-file scope;
4. changes to gate-invalidating surfaces force all Fast checks;
5. otherwise, always-run checks and matching path globs are selected;
6. run serially to avoid scripts racing on shared generated state;
7. a blocking UNKNOWN/error is fail-closed, not a silent skip.

Evidence: `cli/internal/gates/orchestrator.go:45-105`, `129-180`, `183-209`.

**AR-05 — CONFIRMED strength: one declarative registry now contains 112 unique checks.** Of these, 103 delegate to scripts and nine are native Go. The shell adapter maps exit 0/2/75 to PASS/WARN/SKIP and other exits to FAIL. The CLI uses exactly the same typed `GateVerdict` abstraction for both implementations.

**AR-06 — persistent naming risk.** `ao gate` itself is also the human pool promotion surface; validation hangs below it as `ao gate check`. This semantic collision was noted in the prior archaeology and remains. It is understandable in help, but searches for “gate” still mix two different state machines.

CI is a backstop rather than the routine local authority. `.github/workflows/validate.yml` has 14 jobs and triggers on pull requests/merge groups, version tags, or manual dispatch; it does not run on every direct push to main. The repository contract therefore depends on the local pre-push/cockpit path being installed and honored.

## Data stores and external integrations

| Surface | Storage/integration | Behavior |
|---|---|---|
| Provenance authority | `docs/provenance/ledger.jsonl` | Tracked, hash-chained, append-only, flock-serialized. Malformed rows hard-fail reads. Not cryptographically signed. |
| Yield ledger | `.agents/yield/yield-ledger.jsonl` | Private append-only JSONL, mode `0600`, one `O_APPEND` write per event. Used for verdict/escape/usage telemetry (`cli/internal/yieldledger/writer.go:9-85`). |
| Execution packets | `.agents/rpi/execution-packet.json` and run archives | Schema-validated atomic file replacement; naming remains from archived RPI (`cli/internal/adapters/storage_fs/packet_repo.go:1-5`, `cli/internal/adapters/storage_fs/packet_repo.go:60-115`). |
| General config | YAML home/project/explicit file | Layered merge with environment and flags. |
| Verify config | root `.aoverify.yaml` | Six policy keys bridged to `PAWL_*`; per-key source retained. |
| Skills/contracts | Markdown + YAML frontmatter | Source of truth for runtime process contracts and generated catalogs. |
| Git | subprocess and pure-Go root discovery | Change scopes, commit identities, worktrees, rebases, pushes, ledger snapshots. |
| Trackers | `br`/private ledger; `bd` for Gas City substrate | Mostly shell integration; generic legacy fields still mention `bd`. |
| Review runtimes | Codex cold; AGY degraded/fallback; warm families via service | Explicit adapter/family roster in pawl shell. Claude headless print path is unavailable/forbidden. |
| Orchestration | tmux/NTM, MCP Agent Mail, managed agents, optional Gas City | External substrate; AgentOps ships no scheduler daemon. |

The yield ledger is intentionally fail-open observability (`cli/internal/yieldledger/writer.go:9-13`), while the provenance verdict is release authority. That separation is coherent: loss of telemetry should not prevent a merge, but loss of the authorizing verdict should.

## Configuration model

### General CLI config

`config.Load` declares and implements `flags > env > project > home > defaults` (`cli/internal/config/config.go:357-407`). An explicit `--config`/`AGENTOPS_CONFIG` is exclusive: ambient home and project files are skipped, and missing/invalid explicit content fails closed (`config.go:362-383`). Without an explicit path, the project file is resolved only as `$PWD/.agentops/config.yaml`, not by walking to the Git root (`config.go:422-432`). Commands invoked from a repository subdirectory can therefore miss a root project config.

**AR-07 — RISK: archived configuration remains in the default type.** Defaults still include `RPI.RuntimeCommand="claude"`, `BDCommand="bd"`, and Dream/overnight settings (`config.go:297-354`), and environment parsing still honors RPI/Dream variables (`config.go:484-523`). Those command families are absent from the default binary. This is primarily maintenance and discovery debt rather than proof that default commands execute Claude or `bd`, but it conflicts with the current Codex/`br` doctrine and enlarges the public machine contract.

### Verify policy

`.aoverify.yaml` has six keys: reviewer chain, timeout, strict, smoke, autobind, and author family. Resolution walks to the repository root and applies `env > file > default` (`cli/internal/verifycfg/verifycfg.go:60-118`, `252-294`).

**AR-08 — RISK: malformed committed policy weakens to defaults.** `verifycfg.LoadDir` never returns an error; malformed YAML and invalid typed fields generate warnings and fall through (`verifycfg.go:252-266`, `307-349`). The test explicitly expects malformed YAML to retain default values (`cli/internal/verifycfg/verifycfg_test.go:251-267`). Since `strict` defaults false and `autobind` true, a malformed file intended to set `strict: true` can run the weaker default verifier and authorize rather than hold. Recommended policy is to fail closed when a verify config file exists but cannot be parsed, at minimum for safety-strengthening keys.

## Generated surfaces and synchronization

The generation pipeline is unusually explicit:

- source skill contracts, dispositions, tiers, the live Cobra tree, workflow ledger, and CI jobs feed `registry.json`;
- wall-clock timestamps are excluded so generation is content-deterministic (`scripts/generate-registry.sh:538-545`);
- root scripts feed an embedded pawl bundle (`cli/Makefile:25-36`);
- parity-only Codex twins are generated while bespoke overrides remain hand-maintained;
- `scripts/regen-all.sh` encodes dependency order and a no-write check mode (`regen-all.sh:59-102`).

This audit byte-compared the root and embedded execution-packet schema, pawl review script, pawl verdict script, and pawl verdict schema. All matched. `generate-cli-reference.sh --check` and `generate-registry.sh --check` also passed.

**AR-09 — CONFIRMED strength: generated docs are executable-derived.** The command reference and registry both build the default binary instead of parsing filenames. This correctly handles build tags, aliases, and self-registration. The previous “command-file count equals command count” class is structurally prevented (`scripts/generate-registry.sh:380-395`).

## Documentation and contract drift

### AR-10 — high: packet/ports narrative is obsolete

`docs/architecture/ports-and-adapters.md:9-16` claims `ExecutionPacket` enforces non-empty `plan_path`, constrained complexity, non-empty test levels, and required provenance via named Go errors. None of those named errors exist. The live packet is schema-driven, permits empty `plan_path`, and requires only schema version plus objective. The current test deliberately sets `PlanPath=""` and expects validation success (`cli/internal/domain/packet/aggregate_property_test.go:37-46`).

The same page says three ports exist (`ports-and-adapters.md:45-53`); `rg '^type .* interface' cli/internal/ports` finds 29. It calls Git, beads, and provider adapters “future” even though many adapter packages and direct integrations now exist. This page should be rewritten from the current source, not patched count-by-count.

### AR-11 — high: REBOUND schema text contradicts authorization code

The verdict schema admits `REBOUND` but says only `CONFIRMED` authorizes and that current `pawl-verdict.sh check` rejects `REBOUND` (`schemas/pawl-verdict.v1.schema.json:40-43`). The executable checker explicitly admits `{CONFIRMED, REBOUND}` and then performs extensive REBOUND lineage and identity proof (`scripts/pawl-verdict.sh:714-825`). The code is safer than the prose, but downstream consumers generated from the schema description can make the wrong authorization decision.

### AR-12 — high: `ao capabilities` is incomplete

`ao verify --help` and the pawl script define exit 5 as strict HOLD/UNAVAILABLE (`cli/cmd/ao/verify.go:35-43`; `scripts/pawl-review.sh:45-62`). `capabilitiesCommandExitCodes` publishes only 0–4 for `pawl review` and has no `verify` entry (`cli/cmd/ao/capabilities.go:73-99`). The machine-readable contract therefore cannot safely branch on the strict result it advertises elsewhere.

The capabilities document calls its environment map the variables “the CLI honors,” but lists only `AGENTOPS_CONFIG`, `NO_COLOR`, and `AO_DOCTOR_LOG_LEVEL` (`capabilities.go:101-106`). General config alone honors more than 20 `AGENTOPS_*` variables (`config.go:484-523`), and verify honors six `PAWL_*` policy variables. Either the field needs complete generation/validation or its semantics must be narrowed to “global discovery variables.”

This is a regression against the prior archaeology's conclusion that exit-code and env dictionaries were drift-proof. Live-tree command reflection still prevents command-name drift; the handwritten auxiliary dictionaries do not share that guarantee.

### AR-13 — medium: the canonical overview is numerically and behaviorally stale

`docs/architecture/codebase-overview.md:51-62` reports 73 skills, about 88 commands, about 77 checks, about 280 shell scripts, 139 Bats files, and 172 capabilities. Current values are 59, 72, 112, 363 scripts under `scripts/`, 283, and 145. Its directory map also claims 73 Codex twins and about 280 scripts (`codebase-overview.md:89-105`).

More importantly, the “active waist” lists `ao corpus inject` and `ao codex *` as active default commands (`codebase-overview.md:122-151`). Neither appears in the live default generated command reference; both live behind archived build profiles. The page itself correctly states executable/generated precedence (`codebase-overview.md:110-118`), so its own active-command section violates its stated rule.

### AR-14 — medium: `cli/README.md` teaches archived products

`cli/README.md:5-58` centers factory start, phased RPI, and overnight commands. These are not present in the default CLI. The README should describe the shipped spine first and move tagged restoration to an explicit archive section.

### AR-15 — low/known: two gate meanings share one name

The prior audit's naming-collision finding persists: human pool approval and validation execution both live under `ao gate`. No functional defect was reproduced, but the collision raises search and onboarding cost.

## Testing and proof posture

The repository's test investment is substantial: test Go LOC exceeds production Go LOC, 699 Go test files define 7,586 test/example/benchmark/fuzz functions, and the tree contains 283 Bats and 62 Gherkin files. `docs/cli-surface.json` inventories 257 default leaf commands; 174 have direct coverage evidence and 83 are allowlisted with reasons. An allowlist is an explicit gap disposition, not equivalent to a direct behavior test.

The 112-check Go registry and 14-job workflow create broad structural coverage. The architecture also has important fail-closed properties:

- blocking UNKNOWN is failure (`cli/internal/gates/orchestrator.go:191-209`);
- malformed provenance and yield ledger rows hard-fail reads;
- packet legacy migration must satisfy the rich schema;
- generated artifacts have byte-drift checks;
- the stranger review path neutralizes common shell and path injection routes.

What this audit did **not** prove:

- that the full 112-check gate is green;
- that all 14 workflow jobs are green on this commit;
- that a real external reviewer produces a correct verdict;
- that `ao land` succeeds against a live remote under contention;
- that the corpus/flywheel improves future agent outcomes;
- that the ledger is unforgeable against a hostile local writer (the project explicitly disclaims cryptographic signatures).

## Delta from the 2026-07-02 archaeology

The prior report was a useful map, but its measurements and several conclusions have moved.

| Area | 2026-07-02 report | Current observation | Classification |
|---|---|---|---|
| Go scale | ~392K LOC, ~1,300 files | 402,880 LOC, 1,445 files | Expanded |
| Shell/Python | ~112K / ~14K LOC | 120,608 / 15,141 LOC | Expanded |
| Skills | 64 | 59 | Consolidated; history records an 8-skill retirement wave followed by additions |
| Tests | 682 Go test, 236 Bats | 699 Go test, 283 Bats | Expanded |
| Go packages | ~80 internal | 109 default packages | Expanded/more precise basis |
| Gate registry | ~90 | 112 unique checks | Expanded |
| Partial hexagon | Reported | Still observable | Persistent |
| Gate naming collision | Reported | Still observable | Persistent |
| Pawl | Review + bind + gate | Added strict-unavailable semantics, failover, smoke, preflight, meter, REBOUND, catch memory | Expanded |
| Landing | Manual conceptual flow | `ao land` fresh-binary trusted path | New, material improvement |
| Capabilities | Described auxiliary dictionaries as drift-proof | exit 5 and env variables are missing | Regressed/previously overclaimed |
| Generated command truth | Live-tree based | Still live-tree based and check-green | Persistent strength |

Repository history since the comparison includes the skill-retirement wave (`9a23ba9cc`), strict cold quorum (`cc559b21c`), REBOUND enforcement (`fe1d8fee8`, `b426c613d`), preflight/metering, and canonical land (`b9d2103b2`). The pinned head adds land audit hardening and a goal-design workflow (`fbba8af5a`). These history references explain the delta; executable state remains the authority.

## Recommended reading order

For a new maintainer, the shortest reliable route through the current code is:

1. `cli/cmd/ao/root.go` and `app.go` — process spine and typed exit behavior.
2. `cli/Makefile` — which source is actually in the default product.
3. `cli/docs/COMMANDS.md` and `registry.json` — generated default surface.
4. `cli/cmd/ao/verify.go`, `pawl.go`, `land.go` — verification and release path.
5. `scripts/pawl-review.sh`, `pawl-verdict.sh`, `pawl-land.sh` — actual membrane state machine.
6. `cli/internal/gates/{gates,orchestrator}.go` and `checks/` — deterministic release gate.
7. `cli/internal/provenancegraph` and `yieldledger` — durable proof and telemetry planes.
8. `cli/internal/domain/packet` and `ports` — newer domain/hexagonal direction.
9. `skills/**/SKILL.md` and `schemas/**` — declared process/data contracts.
10. Narrative architecture only after checking the executable/generated surfaces above.

## Bounded validation performed

The audit intentionally did not run the full gate or full repository tests. It ran these focused checks:

```bash
cmp schemas/execution-packet.schema.json cli/internal/domain/packet/schemas/execution-packet.schema.json
cmp scripts/pawl-review.sh cli/embedded/pawl/scripts/pawl-review.sh
cmp scripts/pawl-verdict.sh cli/embedded/pawl/scripts/pawl-verdict.sh
cmp schemas/pawl-verdict.v1.schema.json cli/embedded/pawl/schemas/pawl-verdict.v1.schema.json
bash scripts/generate-cli-reference.sh --check
bash scripts/generate-registry.sh --check
cd cli && go test \
  ./internal/domain/packet \
  ./internal/provenancegraph \
  ./internal/gates \
  ./internal/gates/checks \
  ./internal/verifycfg \
  ./internal/yieldledger
```

Result: all comparisons passed; both generation checks reported up to date; all six targeted Go package suites passed.

## Final mental model

AgentOps is best understood as three concentric layers:

1. **A process contract:** skills describe how intent becomes a bounded, testable, independently verified change.
2. **A local enforcement membrane:** `ao verify`, pawl scripts, provenance, `ao gate check`, and `ao land` turn that contract into typed, fail-closed release decisions.
3. **An experimental learning plane:** corpus, yield telemetry, escape compilation, and ratchets attempt to improve future work; the repository correctly treats measured uplift as unproven.

The membrane is the mature center. The strongest code patterns are executable-derived generation, typed exit semantics, explicit build profiles, sanitized foreign-repo review, declarative gates, and append-only proof. The dominant maintenance risk is not hidden algorithmic complexity; it is **contract multiplicity** across Go, Bash, embedded copies, schemas, generated catalogs, and narrative docs. The embedded and generated surfaces are well synchronized. The manually written semantic descriptions are where the current snapshot drifts.
