---
name: skill-builder
description: 'Create a metadata-complete AgentOps skill source package, regenerate its derived projections, and check or repair structural hygiene in skill packages. Triggers: "create a skill", "scaffold skill", "absorb external skill", "new skill", "heal skill", "repair skill hygiene", "audit skill structure", "check skill package".'
practices:
- pragmatic-programmer
- refactoring
hexagonal_role: supporting
consumes: []
produces:
- skill-source-package
- skill-hygiene-report
context_rel: []
skill_api_version: 1
context:
  window: fork
  intent:
    mode: questions
  sections:
    exclude:
    - HISTORY
  intel_scope: topic
metadata:
  capabilities: [skill_builder, heal_skill]
  effects: [writes_skill_source, regenerates_skill_projections, optional_skill_projection_repair]
  canonical_status: canonical
  disposition: keep_specialist
  tier: meta
  dependencies: []
  stability: experimental
  contract_v3:
    schema_version: skill-contract.v3
    primary_layer: implementation
    lifecycle_seams: [implement_method]
    authority: [read_evidence, mutate_subject]
    effects:
    - id: source-read
      kind: filesystem.read
      scope: explicitly named skills/<slug> source package and compiler inputs
      authorization: caller
      cleanup: none
      receipt: optional
    - id: source-write
      kind: filesystem.write
      scope: explicitly named skills/<slug> source package
      authorization: caller
      cleanup: none
      receipt: required
    - id: projection-write
      kind: filesystem.write
      scope: explicitly invoked owned skill projections
      authorization: caller
      cleanup: none
      receipt: required
    - id: bounded-process
      kind: process.start
      scope: declared build, audit, compiler, and projection commands
      authorization: caller
      cleanup: required
      receipt: required
    artifacts:
      consumes:
      - name: build-request
        kind: intent
        semantics: factual
        schema_ref: null
        validator: null
      - name: skill-source-input
        kind: source
        semantics: factual
        schema_ref: schemas/skill-frontmatter.v2.schema.json
        validator: scripts/validate-skill-frontmatter.sh
      produces:
      - name: skill-source-package
        kind: source
        semantics: binding
        schema_ref: schemas/skill-frontmatter.v2.schema.json
        validator: scripts/validate-skill-frontmatter.sh
      - name: build-report
        kind: report
        semantics: factual
        schema_ref: skills/skill-builder/schemas/build-report.json
        validator: skills/skill-builder/scripts/build.sh
      - name: audit-report
        kind: report
        semantics: advisory
        schema_ref: skills/skill-builder/schemas/audit-report.json
        validator: skills/skill-builder/scripts/audit.sh
      - name: compile-receipt
        kind: receipt
        semantics: binding
        schema_ref: skills/skill-builder/schemas/compile-report.json
        validator: skills/skill-builder/scripts/compile_contracts.py
      - name: probe-receipt
        kind: receipt
        semantics: binding
        schema_ref: skills/skill-builder/schemas/probe-report.json
        validator: skills/skill-builder/scripts/run_contract_probe.py
    triggers:
      positive:
      - id: create-skill
        prompt: Create a metadata-complete skill source package.
        expected: route
      negative:
      - id: validate-candidate
        prompt: Issue a semantic validation verdict for this software candidate.
        expected: do_not_route
      ambiguity:
      - id: repair-unspecified
        prompt: Fix this skill.
        expected: clarify
      aliases:
      - id: skill-factory-alias
        alias: skill factory
        canonical_skill: skill-builder
      nearest_neighbors:
      - id: workflow-builder-boundary
        skill: workflow-builder
        distinction: Skill-builder owns one skill source package; workflow-builder composes reusable workflows.
    failure:
      unavailable:
        action: stop
        detail: Report the unavailable source, schema, or tool without selecting a substitute.
      timeout:
        action: stop
        detail: Stop the bounded command and return its timeout without retrying.
      partial_evidence:
        action: report_uncertainty
        detail: Emit no readiness claim when required compiler or audit evidence is incomplete.
      partial_mutation:
        action: rollback_then_stop
        detail: Preserve or restore the explicitly named source state and report the incomplete write.
      cleanup:
        action: stop
        detail: Report cleanup failure explicitly and make no completion claim.
    proof:
      class: mutating_isolation
      command: bash skills/skill-builder/scripts/test-contract-v3.sh
      harness_refs:
      - schemas/skill-contract.v3.schema.json
      - skills/skill-builder/fixtures/contract-v3/probe-harnesses/large-output.py
      - skills/skill-builder/fixtures/contract-v3/probe-harnesses/leave-descendant.py
      - skills/skill-builder/fixtures/contract-v3/probe-harnesses/mutate-copy.py
      - skills/skill-builder/fixtures/contract-v3/probe-harnesses/spawn-and-sleep.py
      - skills/skill-builder/schemas/compile-report.json
      - skills/skill-builder/schemas/probe-report.json
      - skills/skill-builder/scripts/compile_contracts.py
      - skills/skill-builder/scripts/contract_v3.py
      - skills/skill-builder/scripts/probe_runtime.py
      - skills/skill-builder/scripts/test-contract-v3.sh
      - skills/skill-builder/tests/test_contract_v3.py
      - skills/skill-builder/tests/test_probe_runner.py
      fixture_refs:
      - skills/skill-builder/fixtures/contract-v3/cases.json
      - skills/skill-builder/fixtures/contract-v3/invariants.json
output_contract: skills/skill-builder/schemas/build-report.json (create/build) or skills/skill-builder/schemas/audit-report.json (audit)
---

# Skill Builder — Create, heal, and audit skill packages

`skill-builder` owns the full structural lifecycle of one `skills/<slug>/`
source package: create it, verify its structure, repair owned projections, and
audit its content discipline. It does not schedule work, allocate writers,
operate Git, validate a software candidate, promote learnings, or decide what
happens after a failure.

Before creating a new root, search `skills/*/SKILL.md` for an existing owner.
Extend an existing skill when it already owns the requested behavior.

## Modes

| Trigger phrases | Mode | Entry point |
|---|---|---|
| "create a skill", "scaffold skill", "new skill" | create (build) | `scripts/build.sh` |
| "absorb external skill" | create (absorb-external) | `scripts/build.sh` |
| "check skill package" | check | `scripts/heal.sh --check [--strict]` |
| "heal skill", "repair skill hygiene" | heal | `scripts/heal.sh --fix` |
| "audit skill structure" | audit | `scripts/audit.sh` |
| "compile skill contract", "check contract v3" | compile (check) | `scripts/compile_contracts.py check` |
| "record skill contract receipt" | compile (record) | `scripts/compile_contracts.py record` |

## Constraints

- Create exactly one source package because metadata must have one canonical
  owner.
- Treat external skills as structural signals only because clean-room output
  must not copy names, prose, prompts, scripts, or examples.
- Regenerate projections once and stop because validation, revision, Git, and
  delivery remain caller-owned.
- Check and audit modes never mutate files; fix mode changes only an explicit
  source target and its owned projections, because source behavior remains
  human-authored.

## Create mode

Choose exactly one build input:

- `from-scratch <slug>` creates a blank source package.
- `from-template <slug> --like <existing-slug>` uses the existing skill only
  for metadata defaults; it does not copy its prose.
- `absorb-external <slug> --from <path>` verifies the source exists, then
  creates a clean-room blank package without copying names, prose, prompts,
  scripts, or examples.

The caller may set `SKILL_TIER`, `SKILL_DEPENDENCIES`,
`SKILL_CAPABILITIES`, and `SKILL_EFFECTS`. Values that represent lists must be
JSON arrays.

### Procedure

1. Run `scripts/build.sh` with one mode and one new slug.
2. Fill the generated placeholders with the skill's actual behavior.
3. Run `scripts/heal.sh --check --strict skills/<slug>`.
4. Run `scripts/generate-skill-mesh.py` to derive the catalog, registry,
   router, graph, maps, counts, and runtime image manifests from `SKILL.md`
   metadata.
5. Run `scripts/codex-sync.sh --only <slug>` and
   `scripts/regen-codex-hashes.sh --only <slug>` to derive the Codex twin.
6. Inspect the generated diff. Validation and delivery remain caller-owned.

`build.sh` performs steps 1, 3, 4, and 5 once. It never retries or chooses a
next action.

## Heal and check modes

```bash
bash skills/skill-builder/scripts/heal.sh --check [skills/<slug> ...]
bash skills/skill-builder/scripts/heal.sh --check --strict [skills/<slug> ...]
bash skills/skill-builder/scripts/heal.sh --fix [skills/<slug> ...]
```

Every explicit target must be a real, direct child of `skills/` or
`skills-codex/`. Missing paths, traversal, and symlink spellings are rejected.

### Procedure

1. Resolve and contain all requested target directories.
2. Parse each `SKILL.md` frontmatter.
3. Check the path/name match, description, API version, disposition metadata,
   and linked local references.
4. Print every finding once.
5. In `--fix` mode only, regenerate metadata-owned projections and scoped Codex
   twins, then stop.

`--check` is read-only. `--strict` makes any finding produce exit 1. A failed
fix is returned to the caller; the skill does not retry or select another
action. Structural findings are printed as:

```text
[FINDING_CODE] skills/example: concrete explanation
```

Generated Codex parity follows [codex-parity.md](references/codex-parity.md).
A second identical fix is idempotent, and remaining non-fixable findings stay
explicit.

## Audit mode

The optional read-only deep content audit is:

```bash
bash skills/skill-builder/scripts/audit.sh [--strict] [--json <path>] skills/<slug>
```

It combines the structural result with deterministic authoring checks and an
advisory quality score. It is not the core `Validate` phase, does not write a
`verdict.v2`, and has no delivery authority. Check definitions live in
[audit-checks.md](references/audit-checks.md); density scoring is described in
[context-density-checks.md](references/context-density-checks.md).

## Contract compile mode

The shadow compiler validates one explicitly named `metadata.contract_v3`
declaration without changing live API1 or catalog-v3 authority:

```bash
python3 skills/skill-builder/scripts/compile_contracts.py check \
  --skill <slug>
python3 skills/skill-builder/scripts/compile_contracts.py record \
  --skill <slug> --output <receipt-path>
```

`check` emits deterministic receipt bytes to stdout and never writes.
`record` writes those same bytes atomically to the explicit contained output.
Both modes emit the same schema-valid typed FAIL receipt when the contract is
rejected; stderr diagnostics accompany but never replace that receipt.
The compiler rejects duplicate YAML keys, unknown contract fields, malformed
semantics, forbidden authority, invalid references, and illegal hard
dependencies. The grammar and stable failure codes are documented in
[skill-contract-v3.md](references/skill-contract-v3.md).

Run and record the contract's declared isolated proof separately:

```bash
python3 skills/skill-builder/scripts/run_contract_probe.py check \
  --skill <slug>
python3 skills/skill-builder/scripts/run_contract_probe.py record \
  --skill <slug> --output <probe-receipt-path>
```

The probe runs only in a disposable repository copy. It content-binds the
entrypoint, harnesses, fixtures, compiler, and runner; bounds retained output;
records total output and truncation; reports changed paths; and terminates the
whole proof process group under a bounded TERM-to-KILL policy.

## Output

A created source package contains:

```text
skills/<slug>/
├── SKILL.md
└── scripts/validate.sh
```

The build report is `.agents/audits/<slug>-build.json` and conforms to
`schemas/build-report.json`. Deep audit JSON conforms to
`schemas/audit-report.json`. Contract compilation emits
`schemas/compile-report.json`; proof runs emit `schemas/probe-report.json`.
Generated inventories, the readiness ledger, and shadow contract receipts are
not live routing authority. The caller owns any subsequent edit or invocation.

## Checks

- The slug and frontmatter `name` match.
- Metadata declares `tier`, `dependencies`, `capabilities`, `effects`,
  `canonical_status`, and `disposition`.
- Every hard dependency names a live skill.
- The generated package contains no Git, tracker, queue, retry, release, or
  delivery behavior.
- External material is treated only as a signal that a clean-room skill may be
  useful; its content is not copied.
- Check mode never mutates files; fix mode changes only an explicit source
  target and its owned projections.
- Contract-v3 check mode is read-only, and its rendered receipt bytes equal
  record mode's bytes.
- The hostile contract corpus maps exactly to its source-owned invariant
  inventory; missing, duplicated, or unknown witnesses fail the proof.

## Failure behavior

Any invalid input, structural failure, projection failure, or Codex sync
failure exits nonzero after one attempt. The caller decides whether to revise
or invoke the builder again.

## References

- [skill template](references/skill-template.md)
- [shadow skill-contract.v3 grammar](references/skill-contract-v3.md)
- [authoring doctrine](references/authoring-doctrine.md) — prose-quality
  principles behind the advisory `authoring` audit block
- [heal.feature](references/heal.feature)
- [skill-auditor.feature](references/skill-auditor.feature)
