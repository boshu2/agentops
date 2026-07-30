# Ponytail whole-repo contraction

- **Status:** Ready for execution-laptop handoff
- **Requested:** 2026-07-30
- **Planning base:** `efcf4879c8f3db32fce7e28cca4deb0760eb879f`
- **Intent:** Address every finding from the 2026-07-30 whole-repo
  `ponytail-audit` through multiple bounded beads that another machine can
  execute from cold context.
- **Estimated opportunity:** About 113,000 tracked lines. This estimate is a
  ranking aid, not acceptance; behavior and distribution preservation decide
  each bead.

## Bead specification manifest

This document defines the intended bead decomposition without creating or
mutating tracker state. The execution laptop may materialize the parent and
children from these sections under its repository policy. The stable slugs,
dependency edges, acceptance criteria, and write scopes are the portable
handoff; generated tracker IDs are deliberately left as `TBD`.

| Key | Stable slug | Type | Priority | Parent | Depends on |
|---|---|---:|---:|---|---|
| E | `ponytail-whole-repo-contraction` | epic | P1 | — | — |
| B1 | `ponytail-dead-knowledge-stack` | task | P1 | E | — |
| B2 | `ponytail-dead-types-quest` | task | P1 | E | — |
| B3 | `ponytail-flywheel-delegates` | task | P2 | E | — |
| B4 | `ponytail-config-contraction` | task | P1 | E | — |
| B5 | `ponytail-retired-entrypoints` | task | P2 | E | — |
| B6 | `ponytail-retired-gas-city-assets` | task | P1 | E | — |
| B7 | `ponytail-historical-doc-retention` | task | P2 | E | B5, B6 |
| B8 | `ponytail-codex-buildtime-projection` | task | P1 | E | B5 |
| B9 | `ponytail-gemini-buildtime-projection` | task | P1 | E | B5, B8 |

## Ground truth and experiment discipline

This is **Extend this project** work.

- **Ground truth:** Live executable behavior; the root and `cli/` `AGENTS.md`
  contracts; accepted ADRs, especially ADR-0016; Cobra command behavior;
  current release/install validators; and tests that exercise the affected
  surfaces.
- **Control experiment:** For deletion beads, capture the narrow tests and
  observable CLI output before deletion, then make the smallest deletion that
  preserves the live behavior. For runtime projections, build and hash the
  current checked-in bundle first, then require a clean checkout to generate
  an equivalent installable bundle without tracked copies.
- **Deviation ledger:** Default to deletion, direct calls, existing helpers,
  and existing generators. Every new abstraction, dependency, compatibility
  shim, or generated source surface is a deviation and must be justified in
  the owning bead before implementation. The expected ledger is empty.

## Initiative-wide acceptance and non-goals

Acceptance:

1. Every audit finding maps to exactly one child bead below.
2. Each child has one caller-visible behavior, explicit non-goals, a
   class-based write scope, and a first useful check.
3. Each child is implemented as its own bounded experiment and receives fresh
   validation over its exact subject before the next dependent bead starts.
4. No child adds a runtime dependency or replaces deleted code with a new
   framework.
5. Generated outputs are changed only through their owning generators.
6. The user's unrelated `skills/sbh/SKILL.md` work is never absorbed into a
   child subject.

Non-goals:

- Correctness, security, and performance review beyond regressions caused by
  these deletions.
- Rebranding or redesigning live CLI behavior.
- Removing useful thin adapters merely because they are small.
- Deleting `.agents/`, `.beads/`, operator data, release evidence, ADRs, or
  current intent sources.
- Publishing, committing, pushing, closing beads, or releasing as part of an
  implementation or validation invocation.

## Execution order

Use one writer and one worktree. “Wave” is sequencing, not authorization for
parallel writers.

1. **Wave 1 — isolated Go deletions:** B1, B2, B3.
2. **Wave 2 — user-facing and retired surfaces:** B4, B5, B6.
3. **Wave 3 — repository and distribution shape:** B7, B8, B9.

Within a wave, finish and freshly validate one bead before starting the next.
B8 and B9 are deliberately serialized: both change release packaging, but
they must not grow a shared framework unless the second implementation proves
one is necessary.

## E — Epic: contract the repository without changing live behavior

**Bead ID:** `TBD`

Behavior:

> Given the repository contains historical control packets, checked-in
> projections, and self-tested code with no production consumer, when the
> child contraction beads complete, then the default checkout contains only
> authoritative sources and behavior-earning implementations while current
> CLI and install paths remain usable.

Acceptance:

- All nine child beads are freshly validated at their exact accepted content.
- The final tracked-line delta is reported by child, not treated as proof by
  itself.
- `make -C cli build`, `go test ./...` from `cli/`, and
  `./cli/bin/ao gate check --scope worktree --full` pass on the aggregate
  branch before caller-directed integration.
- Any audit target intentionally retained names its concrete consumer in the
  parent close reason.

Write scope:

- No implementation scope. This epic owns only decomposition and roll-up.

First useful check:

```bash
git ls-files | wc -l
git ls-files '*.go' '*.sh' '*.py' '*.md' '*.json' | xargs wc -l | tail -1
```

## B1 — Delete the unused knowledge persistence and frontmatter stack

**Bead ID:** `TBD`

Audit findings covered:

- Unconstructed `storage.FileStorage`, its `Storage`/`Formatter` interfaces,
  and their self-only tests.
- Unused `parser.Extractor`.
- Unused wiki frontmatter codec, port DTOs, adapter, and tests.

Behavior:

> Given no production binary constructs `FileStorage`, `Extractor`, or
> `FrontmatterCodecPort`, when those implementations and self-only tests are
> removed, then transcript mining, atomic/JSONL storage helpers, wiki path
> resolution, and every live command still build and behave as before.

Acceptance:

- `FileStorage`, `NewFileStorage`, `Storage`, `Formatter`, `Extractor`,
  `FrontmatterCodecPort`, `PortCodec`, and their exclusive DTOs no longer
  exist.
- `storage.AtomicWriteFile`, the shared JSONL scanning/appending helpers, and
  constants used by `initapp`/`flywheelapp` remain.
- `parser.NewParser` transcript mining remains.
- `wiki.AgentsDirIn` and its sandbox/path behavior remain.
- No replacement interface, repository, codec wrapper, or compatibility
  alias is introduced.
- Production reference scans are empty and the full Go suite passes.

Non-goals:

- Rewriting transcript parsing.
- Changing citation, provenance, atomic-write, or JSONL behavior.
- Consolidating other ports that have live consumers.

Allowed write scope:

- The dead implementation/test files under `cli/internal/storage`,
  `cli/internal/parser`, `cli/internal/wiki`, and
  `cli/internal/ports`, plus direct compile fallout and comments/tests that
  name the deleted symbols.

First useful check:

```bash
rg -n 'NewFileStorage|FileStorage|NewExtractor|FrontmatterCodecPort|PortCodec' \
  cli --glob '*.go' --glob '!*_test.go'
```

Evidence commands:

```bash
cd cli
go test ./internal/storage ./internal/parser ./internal/wiki ./internal/ports
go test ./...
```

## B2 — Delete dormant domain types and the `quest` write wrapper

**Bead ID:** `TBD`

Audit findings covered:

- Unused candidate, pool, plan-manifest, knowledge-tier, MCP, and MemRL policy
  models in `internal/types`.
- `internal/types/quest`, which delegates atomic writes to
  `internal/storage` and carries a duplicated test suite.

Behavior:

> Given only transcript, citation, flywheel, and a small set of utility types
> have production consumers, when dormant domain models are removed and
> OpenClaw calls the canonical storage writer directly, then the live JSON
> contracts and OpenClaw snapshot behavior remain unchanged.

Acceptance:

- `memrl_policy.go` and its tests are removed after a production reference
  scan confirms no consumer.
- Candidate/pool/supersession/expiry/knowledge-tier/MCP/plan-manifest types
  and exclusive tests are removed.
- Transcript, token, tool-call, citation, flywheel, golden-signal, and other
  production-used types remain byte-compatible in JSON.
- `openclaw` calls `storage.AtomicWriteFile` directly with the existing
  permissions.
- `internal/types/quest` no longer exists.
- No new type package or alias layer replaces the deleted code.

Non-goals:

- Removing MemRL-named metrics fields that are part of live flywheel JSON.
- Redesigning citation or flywheel schemas.
- Changing OpenClaw snapshot paths, bytes, or permissions.

Allowed write scope:

- `cli/internal/types` dead declarations/tests,
  `cli/internal/openclaw` direct call sites, the canonical storage helper if a
  behavior-preserving exposure change is required, and direct compile fallout.

First useful check:

```bash
rg -n 'MemRLPolicy|PlanManifestEntry|Candidate|PoolEntry|types/quest|quest\.' \
  cli --glob '*.go' --glob '!*_test.go'
```

Evidence commands:

```bash
cd cli
go test ./internal/types/... ./internal/openclaw ./internal/doctor
go test ./...
```

## B3 — Remove duplicate flywheel golden-signal delegates

**Bead ID:** `TBD`

Audit finding covered:

- `flywheelapp/golden.go` delegates to `internal/quality`; its 449-line test
  file retests the delegate target.

Behavior:

> Given `internal/quality` owns golden-signal computation and tests, when the
> flywheel delegation/test layer is removed, then flywheel status and metrics
> continue to use the same quality functions and return the same results.

Acceptance:

- `flywheelapp/golden.go` and `golden_test.go` are removed.
- Any live flywheel call uses `quality` directly.
- Existing `quality` tests remain the single behavioral test owner.
- No local aliases are introduced to preserve old test names.

Non-goals:

- Changing formulas, thresholds, output fields, or verdict vocabulary.
- Moving unrelated flywheel code.

Allowed write scope:

- `cli/internal/flywheelapp`, direct `internal/quality` call sites, and focused
  tests/comments affected by the deletion.

First useful check:

```bash
rg -n 'computeVelocityTrend|computeCitationPipeline|computeResearchClosure|computeReuseConcentration|computeOverallVerdict|linearRegressionSlope' \
  cli/internal/flywheelapp
```

Evidence commands:

```bash
cd cli
go test ./internal/quality ./internal/flywheelapp
```

## B4 — Contract configuration to live consumers

**Bead ID:** `TBD`

Audit finding covered:

- RPI, Dream, Models, Forge, Search, Flywheel, and Compile configuration
  sections whose only consumers are configuration plumbing/tests; the
  explicitly dead `UseSmartConnections` knob; and hidden resolved fields.

Behavior:

> Given live commands consume output mode, verbosity, base directory, and the
> four eval corpus paths, when configuration is contracted to those consumers,
> then live settings keep their precedence and source reporting while retired
> sections are ignored on read and disappear on the next save.

Acceptance:

- `Config` retains only live fields, including exactly the path settings used
  by the eval sandbox.
- `ao config --show` reports only live settings in table, JSON, and YAML.
- Flag > environment > project > home > default precedence remains for live
  fields.
- A YAML file containing retired sections still loads without error because
  unknown keys are tolerated; the next typed save may drop those keys.
- Dead environment variables and merge/resolve plumbing are removed.
- Tests assert live behavior, not the continued existence of retired fields.
- Generated CLI documentation and command projections are regenerated through
  their owners if observable help/output changes.
- No generic map-based compatibility layer is added to preserve dead fields.

Non-goals:

- Redesigning config storage, locking, atomic writes, or path precedence.
- Removing the four eval sandbox path controls.
- Adding a migration command or deprecation framework.

Allowed write scope:

- The config command/application/adapter bounded context and its tests; direct
  consumers; and the complete class of Cobra-owned generated command/docs
  projections produced by the existing regeneration commands.

First useful check:

```bash
rg -n '\.(RPI|Dream|Models|Forge|Search|Flywheel|Compile)\b|UseSmartConnections' \
  cli --glob '*.go' --glob '!*_test.go'
```

Evidence commands:

```bash
cd cli
go test ./internal/config ./internal/commands/config ./internal/adapters/config ./cmd/ao
cd ..
bash scripts/regen-all.sh --check
```

## B5 — Remove retired and deprecated script entrypoints

**Bead ID:** `TBD`

Audit finding covered:

- Installer tombstones, retired eval/canary scripts, no-policy wrappers, the
  deprecated skill-flag wrapper, and tests/eval fixtures that exist only to
  prove those dead entrypoints remain dead.

Behavior:

> Given supported installation uses `npx skills`, managed plugins, or
> `ao skills link`, and retired eval surfaces have no live replacement, when
> their tombstone entrypoints and self-only contracts are removed, then every
> documented executable script invokes a live command and supported install
> paths remain covered.

Acceptance:

- Remove the five identical legacy installer tombstones
  (`install.sh`, `install-agy.sh`, `install-claude.sh`,
  `install-codex.sh`, `install-opencode.sh`) and their exclusive tests/refs.
- Remove retired `eval-agentops.sh`, `check-applied-ood-headroom.sh`, and
  `test-agentops-contract-canaries.sh` plus stale eval expectations.
- Remove `check-pillar-coverage.sh`, `release-cadence-check.sh`,
  `check-skill-flag-refs.sh`, and `refresh-codex-local.sh` if final reference
  scans confirm they have no supported caller.
- Update workflow-coverage exceptions, install docs, migration docs, and
  fixtures so no gate names a deleted path.
- Preserve live thin wrappers with real consumers, including policy dispatch,
  workflow installation, and Gas City maintainer glue.
- Supported installation and fresh-install conformance remain green.
- Do not add replacement tombstones or aliases.

Non-goals:

- Removing all shell wrappers.
- Changing supported installation semantics.
- Restoring a retired eval runner.

Allowed write scope:

- Retired/deprecated script entrypoints; their exclusive tests, eval fixtures,
  workflow/gate registrations, and documentation references; plus generated
  documentation/index outputs owned by existing generators.

First useful check:

```bash
rg -n 'install-(agy|claude|codex|opencode)\.sh|eval-agentops\.sh|check-applied-ood-headroom\.sh|test-agentops-contract-canaries\.sh|check-pillar-coverage\.sh|release-cadence-check\.sh|check-skill-flag-refs\.sh|refresh-codex-local\.sh' \
  . --glob '!docs/audits/**' --glob '!docs/plans/**' --glob '!CHANGELOG.md'
```

Evidence commands:

```bash
bash scripts/check-scripts-ao-invocations.sh
bash scripts/fresh-install-conformance.sh
bash scripts/ci-local-release.sh --quick
```

## B6 — Remove frozen Gas City prototype assets

**Bead ID:** `TBD`

Audit finding covered:

- `packs/agentops-executor`, `packs/agentops-factory`, and `deploy/gc`, which
  the live gate registry labels “frozen historical bytes” after the upstream
  factories pivot.

Behavior:

> Given AgentOps now uses the upstream Gas City build pack and the live
> `ao gc` maintainer surface, when frozen local prototype packs/deploy assets
> are removed, then current Gas City guidance and maintainer operations remain
> complete without referring to deleted local pack bytes.

Acceptance:

- The two prototype pack trees and `deploy/gc` are removed.
- Live code, gates, docs, skills, release fixtures, and manifests contain no
  dependency on those paths.
- Preserve `ao gc`, `cli/internal/gcmaintainer`,
  `scripts/gc-maintainer-ops.sh`, and `skills/using-gc`.
- Gas City documentation names upstream pack ownership and the remaining
  AgentOps boundary accurately.
- No archive copy is added elsewhere in the repository.

Non-goals:

- Removing Gas City support.
- Reimplementing upstream packs.
- Changing maintainer behavior.

Allowed write scope:

- The retired pack/deploy trees and all live references, tests, manifests,
  gates, and generated projections affected by their removal.

First useful check:

```bash
rg -n 'packs/agentops-executor|packs/agentops-factory|deploy/gc' \
  . --glob '!docs/audits/**' --glob '!docs/plans/**' --glob '!CHANGELOG.md'
```

Evidence commands:

```bash
cd cli
go test ./internal/gcmaintainer ./internal/commands/gc ./cmd/ao
cd ..
bash scripts/check-gc-maintainer-ops.sh
```

## B7 — Apply a retention rule to historical plans and audits

**Bead ID:** `TBD`

Audit finding covered:

- 54,794 lines under `docs/audits` and `docs/plans`, including raw model
  transcripts, repeated codebase reconstructions, and superseded plan packets.

Behavior:

> Given Git already retains deleted history and `docs-scope.sh` treats audit
> and plan trees as historical, when repository retention keeps only active
> intent and consumer-backed summaries, then the default checkout no longer
> ships raw or superseded control artifacts and live documentation links remain
> valid.

Acceptance:

- Add one concise retention rule to the existing documentation architecture:
  retain active plans, accepted decisions, and summaries cited by a live
  consumer; delete raw model output, superseded/completed packets, and repeated
  recon snapshots.
- Inventory current files by `active`, `consumer-backed-summary`, `raw`, or
  `superseded`; the inventory is temporary working evidence, not a new tracked
  archive.
- Delete eligible content rather than moving it to another tracked archive.
- Keep ADRs, current release notes, current intent sources, and this plan while
  the epic is active.
- Update live links and regenerate the documentation index through its owner.
- The docs build and live-doc gates pass with no broad new baseline waiver.

Non-goals:

- Deleting all plans/audits indiscriminately.
- Rewriting historical claims.
- Moving history into `.agents/`, a new archive directory, or another
  projection.

Allowed write scope:

- Historical plan/audit sources selected by the retention rule; all direct
  live references; documentation navigation/configuration; and the complete
  class of documentation-index/site projections produced by existing
  generators.

First useful check:

```bash
git ls-files docs/audits docs/plans | xargs wc -l | tail -1
git ls-files 'docs/audits/**/raw/**' | xargs wc -l | tail -1
```

Evidence commands:

```bash
python3 scripts/generate-documentation-index.py --check
bash scripts/check-doc-skill-refs.sh --all-docs --strict
bash tests/docs/validate-doc-release.sh
```

## B8 — Generate the Codex runtime projection only for packaging

**Bead ID:** `TBD`

Audit finding covered:

- The checked-in `skills-codex` projection duplicates the canonical skill
  corpus and overrides while acting as a release artifact.

Behavior:

> Given `skills/` and `skills-codex-overrides/` own Codex behavior, when a
> clean checkout builds a staged Codex plugin bundle, then the staged
> `skills-codex` artifact is semantically and manifest-equivalent to the
> current release bundle without tracking generated twins in Git.

Acceptance:

- Before changes, generate and record a control manifest containing paths,
  byte hashes, executable bits, and plugin metadata for the current Codex
  bundle.
- `scripts/codex-sync.sh` (or its owning Go replacement if separately
  authorized) accepts an explicit output root and never requires a tracked
  `skills-codex` tree.
- `.codex-plugin/plugin.json` resolves `./skills-codex` inside the staged
  bundle, not necessarily inside the source checkout.
- Release/plugin/fresh-install workflows stage the projection, run all current
  Codex validators against it, and package that exact validated directory.
- A clean source checkout contains no tracked `skills-codex` files.
- The staged artifact contains every live skill, override-derived prompt,
  mirrored reference, executable bit, and generated manifest expected by the
  control.
- Source-time tools, gates, docs, Windows checks, and tests no longer assume a
  checked-in projection.
- `skills/` and `skills-codex-overrides/` remain the only hand-edited behavior
  sources.
- No new dependency, template language, or second skill inventory is added.

Non-goals:

- Changing skill semantics or Codex wording.
- Removing Codex support or validation.
- Hand-editing staged output.

Allowed write scope:

- Codex projection generators and override sources; plugin/release/install
  packaging; all validators, tests, docs, and manifests that consume the
  projection; and the full generated `skills-codex` output class, which is
  deleted from source and created only under an ignored staging directory or
  release artifact.

First useful check:

```bash
bash scripts/validate-codex-install-bundle.sh
git ls-files skills-codex | xargs sha256sum > /tmp/agentops-codex-control.sha256
```

Evidence commands:

```bash
bash scripts/audit-codex-parity.sh
bash scripts/validate-codex-override-coverage.sh
bash scripts/validate-codex-generated-artifacts.sh --scope worktree
bash scripts/validate-headless-runtime-skills.sh
bash scripts/fresh-install-conformance.sh
bash scripts/ci-local-release.sh --quick
```

The implementer must adapt these commands to the staged output root rather than
weakening or skipping them.

## B9 — Generate the Gemini skill image only for packaging

**Bead ID:** `TBD`

Audit finding covered:

- `images/gemini/skills` contains byte-identical copies of canonical
  `skills/<slug>/SKILL.md`.

Behavior:

> Given the Gemini wrapper is zero-conversion over canonical skills, when a
> clean checkout stages the Gemini plugin image, then every staged skill is
> byte-identical to `skills/` and the plugin validates without tracking copied
> skill files.

Acceptance:

- Before changes, capture the current Gemini bundle manifest, byte hashes,
  executable bits, agents, rules, hooks, and MCP configuration as the control.
- The packaging command stages `images/gemini` metadata plus canonical skills
  into an ignored/output directory.
- `images/gemini/verify.sh` validates an explicit staged root and proves
  byte-identity against `skills/`.
- A clean source checkout contains no tracked `images/gemini/skills` files.
- Agents, rules, hooks, MCP configuration, plugin metadata, and skill count are
  unchanged in the packaged artifact.
- `agy plugin validate` runs against the staged image when `agy` is available;
  deterministic JSON/identity checks remain blocking without it.
- No conversion layer or Gemini-specific skill fork is introduced.

Non-goals:

- Changing canonical skill content.
- Removing Gemini/Antigravity support.
- Sharing a new packaging framework with Codex unless direct duplication
  remains after B8 and a separate justification is added to this bead.

Allowed write scope:

- Gemini image metadata, verifier, packaging/release/install workflows, tests,
  documentation, and the complete generated Gemini skills output class.

First useful check:

```bash
bash images/gemini/verify.sh
git ls-files images/gemini/skills | xargs sha256sum > /tmp/agentops-gemini-control.sha256
```

Evidence commands:

```bash
bash images/gemini/verify.sh
bash scripts/fresh-install-conformance.sh
bash scripts/ci-local-release.sh --quick
```

The post-change verifier must run over the staged image path.

## Execution-laptop handoff

If the execution-laptop operator chooses to materialize these specifications
in the repository tracker:

1. Create E with the epic behavior, initiative acceptance, and non-goals.
2. Create B1–B9 as children of E using the complete corresponding sections
   above as their descriptions.
3. Add dependency edges exactly as listed in the manifest table.
4. Replace every `TBD` with its generated bead ID.
5. Run `br lint` and `br dep cycles`.
6. Sync the private ledger through its own repository policy. Never stage
   `.beads` from this public repository.
7. Keep this committed plan as the shared sequencing and acceptance source;
   do not duplicate it into another public planning artifact.

For each implementation bead, the cold-start instruction is:

> Read the bead description, this plan, repository `AGENTS.md`, relevant
> subtree instructions, and the named ground-truth contracts. Implement one
> bounded RED/GREEN/refactor experiment, run the bead’s checks, then obtain a
> fresh validation result over the exact content. Report and stop; do not
> automatically start the next bead.
