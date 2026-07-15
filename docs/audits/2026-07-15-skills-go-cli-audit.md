# Skills and Go CLI Audit: AgentOps

**Date:** 2026-07-15

**Baseline:** `main` at `16b4dbe0d` after fast-forwarding to `origin/main`

**Domains:** skill behavior and contracts, CLI correctness and operator ergonomics

**Method:** source inspection, focused regression tests, generated-projection checks,
CLI probes, and one frozen-subject full gate

## Summary

- **Total findings:** 19
- **Critical:** 0
- **High:** 11 (8 fixed, 3 open)
- **Medium:** 8 (3 fixed, 5 open)
- **Current deterministic result:** `ao gate check --full --json` passed 65/65
- **Current behavioral result:** not proven; all 10 product/judgment skills are
  still unmeasured by the repository's behavioral-probe ledger

The important correction is architectural, not procedural: the caller's bead,
issue, or conversation owns intent. AgentOps no longer asks a model to duplicate
that intent into Plan, Candidate, or Revision packets. The runtime may persist
content-addressed intent, subject, check, and verdict receipts because those are
derived facts needed for exact identity and independent judgment; they are not
parallel work-management artifacts.

The audit initially drifted into validation theater: proof artifacts and repeated
gates were multiplying faster than product evidence. The rollback from that work
also removed the evidence floor from `verdict.v2`, which made evidence-free PASS
possible again. The final repair separates the useful invariant from the packet
machinery: PASS requires evidence, but no packet is required.

## Fixed High Findings

### 1. Core packets duplicated caller-owned intent and tracker state

- **Location:** `AGENTS.md:42-67`, `skills/plan/SKILL.md:24-50`,
  `skills/implement/SKILL.md`, `skills/rpi/SKILL.md:35-84`
- **Issue:** The operating loop required model-authored Plan, Candidate, and
  Revision packets even when a bead or issue already owned acceptance and scope.
  This created two sources of truth and rewarded artifact production over work.
- **Root cause:** AgentOps had absorbed work-management and orchestration
  responsibilities that belong to the caller.
- **Fix:** The core loop now reads or refines the caller's intent source, performs
  one bounded experiment, derives runtime facts, validates once in fresh context,
  reports, and stops. Packet schemas remain compatibility-marked rather than
  being live prerequisites.

### 2. `verdict.v2` could persist an evidence-free PASS

- **Location:** `schemas/verdict.v2.schema.json:63-84`,
  `skills/validate/scripts/validate.py:232-271`,
  `skills/validate/scripts/validate.py:363-373`
- **Issue:** A PASS criterion with empty `evidence_refs` could be stored, directly
  contradicting the evidence-backed verdict contract.
- **Root cause:** Evidence enforcement had been coupled to packet validation and
  was accidentally removed during the packet rollback.
- **Fix:** Schema and semantic validation now require nonempty top-level evidence,
  nonempty checked scope, and evidence for every PASS criterion. A deficient PASS
  is downgraded to `NOT_PROVEN`; direct persistence validation rejects malformed
  PASS artifacts.

### 3. Tracker-less intent was invisible to a fresh validator

- **Location:** `skills/plan/SKILL.md:26-50`,
  `skills/validate/scripts/validate.py:409-455`,
  `skills/validate/scripts/validate.py:480-544`
- **Issue:** When the caller had no usable tracker, the conversation was the
  effective intent source, but it had no durable identity a fresh context could
  verify.
- **Root cause:** The rewritten boundary named caller intent without defining a
  tracker-less runtime representation.
- **Fix:** The runtime now atomically snapshots the exact resolved intent bytes at
  `.agentops/intents/sha256/<digest>.intent`. `store-verdict` performs this
  automatically and binds the acceptance digest to those exact bytes. This is a
  derived receipt, not a model-authored planning artifact.

### 4. Go gates could execute with the wrong interpreter and misstate failures

- **Location:** `cli/internal/gates/gates.go`,
  `cli/internal/gates/scriptrunner.go`, `cli/internal/gates/orchestrator.go`
- **Issue:** Gate execution did not consistently select Python 3 versus Bash and
  could lose actionable failure state.
- **Root cause:** Interpreter choice and blocking/advisory status were spread
  across execution paths.
- **Fix:** Python gates use `python3`, shell gates use Bash, missing interpreters
  report `UNKNOWN` with diagnostic output, blocking non-PASS results fail closed,
  and advisory results remain explicit.

### 5. Config writes were vulnerable to torn or lost concurrent updates

- **Location:** `cli/internal/config/config.go`,
  `cli/internal/config/filelock_unix.go`,
  `cli/internal/config/filelock_windows.go`
- **Issue:** Save operations could expose partial content or overwrite another
  writer's disjoint update; dry-run did not exercise the exact save path.
- **Root cause:** Read/merge/write was not one locked atomic operation.
- **Fix:** Config save and preview now use a locked merge, same-directory atomic
  replacement, symlink-safe temporary handling, and the same validation path.
  Concurrency, malformed-input, missing-target, and dry-run cases have regression
  tests.

### 6. Release evidence could manufacture success after evaluator retirement

- **Location:** `scripts/eval-agentops.sh`,
  `scripts/check-applied-ood-headroom.sh`,
  `scripts/test-agentops-contract-canaries.sh`,
  `scripts/ci-local-release.sh:766-835`
- **Issue:** Retired packet-era evaluator surfaces still looked callable, and the
  release path risked treating their absence as evaluation success.
- **Root cause:** Compatibility scripts outlived the behavior they claimed to
  measure.
- **Fix:** Retired surfaces are explicit exit-2 tombstones. Release evidence now
  records `not_applicable` with a reason, and readiness handles that status without
  awarding evaluation points or fabricating a PASS.

### 7. Compile redaction could silently fall back to identity output

- **Location:** `scripts/lib/flywheel-compile.sh`,
  `cli/embedded/skills/compile/scripts/compile.sh`,
  `cli/cmd/ao/zzz_default_spine.go`
- **Issue:** If the redactor was unavailable, compile could emit sensitive input
  unchanged.
- **Root cause:** Redaction was treated as an optional helper while the live
  `ao redact` command had been removed from the default spine.
- **Fix:** `ao redact` is restored as a retained non-lifecycle command. Compile
  locates the product or repository binary and fails closed when neither exists;
  the identity fallback is gone.

### 8. CLI capability inspection omitted attached command contracts

- **Location:** `cli/internal/clicontract/inspect.go`,
  `cli/cmd/ao/gate_composition.go`
- **Issue:** Capability inspection could report an incomplete surface, including
  a gate command with no declared contract projection.
- **Root cause:** Inspection walked commands without projecting the attached
  contract consistently.
- **Fix:** Inspection now includes attached contracts and the gate command attaches
  its declared contract; focused tests cover both paths.

## Fixed Medium Findings

### 9. Retired command families failed clean-clone architecture checks

- **Location:** `cli/internal/archcheck/family.go`, retired lineage fixtures under
  `cli/testdata/compatibility-baseline/families/`
- **Issue:** Retired `beads`, `claim`, `close`, `council-gate`, and `done` lineages
  still expected live owner directories.
- **Root cause:** The compatibility checker did not distinguish historical lineage
  from a current source owner.
- **Fix:** Lineages are explicitly retired and archcheck skips nonexistent live
  owner directories only for retired families, with regression coverage.

### 10. Generated docs, backlinks, baselines, and tests had accumulated drift

- **Location:** generated CLI/skill docs, `scripts/.docs-*`,
  `scripts/check-test-isolation.sh`, and migrated Go tests
- **Issue:** Stale links, overgrown grandfather baselines, and process-wide
  directory mutation obscured real regressions.
- **Root cause:** Generated projections and test isolation had not been maintained
  through the CLI and skill cut.
- **Fix:** Projections were regenerated from owners, obsolete baseline entries were
  removed rather than expanded, backlinks were repaired, and affected Go tests
  now use test-scoped directory changes.

### 11. Core scenarios still asserted packet-era behavior

- **Location:** core skill feature files and validators under `skills/{plan,implement,rpi,validate}/`,
  `scripts/check-cathedral-cut-conformance.py`, `scripts/.scenario-linkage-allow`
- **Issue:** Scenarios and conformance probes could pass while checking the old
  packet loop.
- **Root cause:** Contract changes were not propagated to behavior examples and
  their linkage inventory.
- **Fix:** Core scenarios now assert intent/subject/runtime binding and evidence
  semantics. Validate has linked executable tests rather than an allowlist entry.

## Open High Findings

### 12. Global config selection and output format are not honored consistently

- **Location:** `cli/internal/adapters/config/gateway.go:12-41`,
  `cli/internal/config/command_service.go:67-83`,
  `cli/cmd/ao/config_module.go:11-12`
- **Issue:** `AGENTOPS_OUTPUT=json ao config models` and
  `ao --output yaml config models` still emit the human table. An explicit
  nonexistent `--config` can warn yet `config --show` reports the current working
  directory's config and exits successfully.
- **Root cause:** The adapter calls `Resolve(output, "", verbose)` and `Load(nil)`,
  discarding the selected config path; `Show` does not load the selected config;
  rendering remains command-specific.
- **Recommended fix:** Thread one resolved global options object through the config
  port, load exactly the selected path, and centralize human/JSON/YAML rendering.
  Add CLI tests covering env, flag, missing explicit path, stdout, stderr, and exit
  code together.

### 13. Global dry-run does not protect provenance writes

- **Location:** `cli/cmd/ao/provenance_add.go:123-145`,
  `cli/cmd/ao/provenance_mine_session.go:142-170`
- **Issue:** `provenance add` always appends, and `provenance mine-session --state`
  always persists its watermark even when global dry-run is active.
- **Root cause:** These commands bypass the command contract's mutation policy and
  call storage directly.
- **Recommended fix:** Pass dry-run into both application services, compute and
  render the proposed mutation without writing, and test the ledger and state file
  byte-for-byte before and after dry-run.

### 14. Structural gates do not prove that the skills improve model behavior

- **Location:** `scripts/check-skill-probe-coverage.sh:1-32`,
  `scripts/check-skill-probe-coverage.sh:107-160`,
  `cli/internal/gates/checks/seed.go:267-273`, `skills/SKILL-TIERS.md:13-35`
- **Issue:** All 10 product/judgment skills—`council`, `doc`,
  `dueling-idea-genies`, `goals`, `postmortem`, `premortem`, `product`,
  `reality-check`, `security`, and `validate`—lack a behavioral-probe result.
  The full gate therefore records a green run while the advisory script prints
  that the entire tier is unmeasured.
- **Root cause:** Schema, generated parity, and scenario linkage prove repository
  integrity, not that a loaded skill changes behavior usefully. The coverage check
  intentionally exits zero in advisory mode.
- **Recommended fix:** Run small control/treatment probes for the core spine and
  record honest `BEHAVIORAL` or `INERT` results. Stop when a pass finds no new
  behavior signal. Do not answer this gap with more schemas, packet fields, or
  blocking gates.

## Open Medium Findings

### 15. Codex reference-file rewriting is specified but not implemented

- **Location:** `docs/contracts/codex-skill-api.md:197-207`,
  `scripts/codex-sync.sh:395-445`,
  `skills-codex/doc/references/validation-rules.md:18-22`
- **Issue:** The contract requires Markdown reference files to pass through the
  Codex text rewriter, but the sync script byte-compares and exact-copies all
  sibling content. A generated Codex reference still points to
  `~/.claude/scripts/doc-validate.py`.
- **Root cause:** Transformation is applied to `SKILL.md`, while reference payloads
  are treated as parity bytes.
- **Recommended fix:** Rewrite `.md` files during reference copy, compare the
  transformed bytes in check mode, preserve non-Markdown payloads exactly, and add
  a fixture containing Claude-only tools and paths.

### 16. The Go skill-catalog decoder is lossy and permissive

- **Location:** `cli/internal/skills/catalog.go:21-65`,
  `schemas/skill-catalog.schema.json:7-29`
- **Issue:** The schema-v3 catalog requires capabilities, effects, canonical
  status, disposition, and tier, but the Go entry type drops them. Plain
  `json.Unmarshal` also accepts unknown fields, stale schema versions, and a
  `skill_count` that differs from the array length.
- **Root cause:** The consumer model predates the schema while its comment claims
  exact parity; loading performs syntax decoding without contract validation.
- **Recommended fix:** Model every v3 field, use a decoder that rejects unknown
  fields, validate schema version and count, and add stale/extra/missing fixtures.

### 17. Skill reference counts ignore nested files

- **Location:** `scripts/generate-skill-mesh.py:45-61`,
  `schemas/skill-catalog.schema.json:101-105`
- **Issue:** `references_count` uses non-recursive `glob("*")`, so nested reference
  files are undercounted even though the schema describes the count as files under
  the reference directory.
- **Root cause:** The generator counts immediate directory entries instead of
  recursive files.
- **Recommended fix:** Count `p for p in references.rglob("*") if p.is_file()` and
  add a nested-reference fixture.

### 18. `ao skills link` reports semantic conflicts but exits zero

- **Location:** `cli/cmd/ao/skills_link.go:35-43`,
  `cli/cmd/ao/skills_link.go:209-243`,
  `cli/cmd/ao/skills_link.go:274-290`
- **Issue:** Wrong symlinks and foreign directories are correctly preserved and
  printed as conflicts, but only storage errors contribute to `anyErr`; a caller
  cannot distinguish a clean install from an incomplete one by exit status.
- **Root cause:** Conflicts are modeled only as renderable result data, not as an
  unsuccessful command outcome.
- **Recommended fix:** After all destinations are attempted, return a typed
  conflict error or documented distinct exit code when any conflict remains;
  preserve the current non-destructive behavior and complete fan-out.

### 19. The release lane has no live behavior/effectiveness evaluator

- **Location:** `scripts/ci-local-release.sh:766-835`
- **Issue:** Release readiness honestly records evaluation as `not_applicable`, so
  it can prove build/release mechanics but not AgentOps effectiveness.
- **Root cause:** The legacy evaluator was packet-coupled and had to be retired;
  no replacement is wired yet.
- **Recommended fix:** Define a small outcome-based evaluator over caller intent,
  runtime receipts, and verdicts. Keep absence explicit until that evaluator
  exists; do not restore the retired scripts or mint substitute PASS evidence.

## Operational Blocker: Beads Schema

The local `bd` executable is `/opt/homebrew/bin/bd`; `/Users/bo/bin/bd` is absent.
The shared remote database is schema v49 while this binary requires v53. The audit
did not migrate or bypass that shared state. Creating audit beads is therefore
blocked on an operator-selected migration owner. The safe choices are a designated
migrator using `BD_ALLOW_REMOTE_MIGRATE=1 bd migrate` followed by the repository's
normal push policy, or `bd bootstrap` after adopting a migration performed
elsewhere. This does not justify recreating tracker state as AgentOps packets.

## Verification

### Checked

- `./cli/bin/ao gate check --full --json`: **65 passed, 0 failed, 0 warned,
  0 skipped, 0 unknown**; elapsed 146.532 seconds on the frozen code subject.
- Focused Validate/RPI Python suite: **20 passed**.
- Release/readiness/native-skill BATS selection: **71 passed**.
- Focused Go race tests for the changed gate, config, contract, architecture, and
  command packages passed.
- Core skill self-validation, cathedral-cut conformance, scenario linkage, strict
  documentation references, generated projection checks, and `git diff --check`
  passed at closeout.
- The JSM corpus is preserved at
  `docs/audits/jsm-pattern-corpus-2026-07-15/` as comparative evidence only; its
  scores are not work authorization.

### Not checked

- No live behavioral probes were run; skill usefulness remains explicitly
  unproven as finding 14 states.
- The full `scripts/ci-local-release.sh` release workflow was not run end to end.
- No deep dependency/security scanner, external runtime, publishing, merge,
  release, or rollback operation was performed.
- The shared beads database was not migrated.
- No commit or push was created.

## Process Guardrail Adopted

For this class of work, freeze the subject once, run targeted checks while editing,
run the expensive integration gate once after the code is frozen, and publish the
remaining uncertainty. A fresh finding reopens only the affected code and its
targeted checks unless it changes the whole subject. Runtime receipts establish
identity and evidence; they do not schedule work, mirror beads, or prove that a
skill is valuable. Behavioral value requires behavioral evidence.
