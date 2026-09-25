---
name: skill-builder
description: 'Create, adapt, consolidate or repair skill packages and projections. Use when: authoring guidance, descriptions or structure; Skill Eval measures behavioral benefit.'
practices:
- pragmatic-programmer
- refactoring
hexagonal_role: supporting
consumes: []
produces:
- skill-source-package
- skill-hygiene-report
- converted-skill
- operationalization-proposal
context_rel: []
skill_api_version: 1
user-invocable: true
context:
  window: fork
  intent:
    mode: questions
  sections:
    exclude:
    - HISTORY
metadata:
  capabilities: [skill_builder, heal_skill, export_skill, distill_expertise]
  effects: [write_skill_source, write_build_report, regenerate_skill_projections, repair_skill_projections, write_converted_skill_projection, write_advisory_proposal]
  canonical_status: canonical
  disposition: keep_specialist
  tier: meta
  dependencies: []
  stability: experimental
output_contract: build-report.json for creation, audit-report.json for audit, target-valid exported files for conversion, or an advisory expertise proposal
---
# Skill Builder

Create, repair, audit or export one canonical skill package, or turn supported
expertise into a small authoring proposal. Search existing owners before adding
a root. Extend the owner that already handles the behavior.

## Choose the requested operation

| Need | Entry point |
|---|---|
| Create a source package | `scripts/build.sh` with `from-scratch`, `from-template` or `absorb-external` |
| Check package structure | `scripts/heal.sh --check [--strict] skills/<slug>` |
| Repair owned projections | `scripts/heal.sh --fix skills/<slug>` |
| Inspect audit evidence | `scripts/audit.sh [--profile canonical|portable|external-observation] [--json <path>] skills/<slug>` |
| Export to another platform | [Conversion](#conversion) |
| Make repeated expertise reusable | [Distill expertise](#distill-expertise) |

Run only the selected operation. Skills remain optional tools within the native
caller's authorized outcome; this skill does not add execution phases, own work,
operate Git, validate a software candidate, or decide delivery and retries.

## Create and maintain

Treat external skills as structural signals only. Clean-room output must not
copy their names, prose, prompts, scripts or examples. `from-template` reuses
metadata defaults; `absorb-external <slug> --from <path>` verifies an input and
creates a blank source package. Neither imports another skill's content.

For creation, supply one input to `scripts/build.sh`, then replace placeholders
with the actual behavior. The caller can supply `SKILL_TIER`,
`SKILL_DEPENDENCIES`, `SKILL_CAPABILITIES` and `SKILL_EFFECTS`; lists are JSON
arrays. The result is one incomplete source package containing only `SKILL.md`;
helpers, references and assets are conditional on the actual behavior. An inline
answer needs no output file. State applicability, inputs, authority, result,
completion and failure in the layout that makes them clear.

The shell entrypoints delegate creation and source checks to `ao skills build`
and `ao skills check-source`. Development checkouts run their Go source; installed packages need an `ao`
built from this version. Tests can set
`AO_SKILL_BUILDER_BIN` to an explicit binary. Build JSON goes to stdout. To save
it, pass `--report /absolute/external/directory/build.json` in an existing
protected non-Git directory. Existing report paths are never replaced. The
[build-report schema](schemas/build-report.json) retains its old fields, permits
a one-file source list, and adds `authoring_state: scaffold` and
`semantics_evaluated: false`. `structure_check_pass` describes mechanical
creation/projection only, even when true. There is no default workspace report;
consumers of the former `.agents/scratch/skill-builder/` path must select a
report destination or read stdout. Creation wrapper syntax errors remain exit 2;
Go rejects invalid creation inputs and existing destinations with exit 1.
This includes invalid slugs and missing template/external inputs that the old
initializer classified as usage errors. Successful creation remains exit 0 and
always reports scaffold state. Check/heal target errors remain exit 2; strict
source findings remain exit 1.

Replace placeholders and remove `metadata.authoring_state: scaffold` only after
authoring the behavior. Strict source checks reject that explicit incomplete
state. Removing it is an author assertion, not proof of semantic completeness;
a fresh reviewer must judge the actual behavior.

Edit `skills/<slug>/` as the source owner. Check the completed source with
`scripts/heal.sh --check --strict skills/<slug>`, then regenerate its owned
projections through the repository's owning commands. `scripts/regen-all.sh`
is the integrated projection recipe; `scripts/generate-skill-mesh.py`,
`scripts/codex-sync.sh --only <slug>` and
`scripts/regen-codex-hashes.sh --only <slug>` are the existing scoped surfaces.
Do not repeat work already performed by `build.sh` unless source changes
require it. Inspect the generated diff; hand-edit no projection.

Creation is staged, not atomic across source, catalogs, projections and reports.
If a later stage fails, retain the created source and any report, capture the
command's exit and diagnostic, and inspect which outputs exist. Report source
creation separately from projection/check completion. `structure_check_pass:
false` does not mean no files were created; a report-write failure can also
leave source behind. Never call that partial result a completed package or
remove it just to rerun creation. An existing target is deliberately rejected.

Recover from the observed stage within existing authority: repair the named
obstruction, finish authoring the retained source if it is still a scaffold,
then run the strict source check, owning projection commands and audit above.
Retain the failed report as evidence and use a new authorized report path if
one is needed. Verify the retained source and final generated output; report
remaining failures instead of resetting completion history. Missing required
scripts, references or runtime support block their dependent operation; name
that resource and continue only work that does not depend on it.

Check/heal targets must be real direct children of `skills/`; reject missing
paths, traversal and symlink spellings. Check mode is read-only. Fix mode
regenerates owned projections for explicit targets and does not invent source
behavior. Findings name their code, target and concrete issue; `--strict`
returns nonzero for findings. Check the slug/name match, description, API
version, metadata, live dependencies and linked resources.

Deep audit defaults to [skill-audit.v2](schemas/audit-report.json): separate
static conformance, located effect observations, behavioral evidence and non-gating
authoring suspicions. There is no total, rating or aggregate quality verdict.
Effects and behavior remain `NOT_PROVEN`: this command runs no skill or trial.
Profile selection follows package location, or explicit `--profile`; canonical
source metadata is not portable host metadata. Installed host behavior and
invocation policy still need their own checks.

Exit 0 means the selected static checks found no conformance defect, not that the
skill is safe or effective. Exit 1 means a concrete conformance failure; exit 2
means invalid invocation, input or report destination. `--strict` does not promote
wording suspicions to failures. Default output is JSON on stdout. `--json` creates
a new report only in an existing external non-Git directory, never overwrites one.

Consumers needing the old `verdict`, `pass1`, `pass2`, `density`, `rubric`, `craft`
and `authoring` fields must explicitly use `--legacy` and
[audit-report-legacy.json](schemas/audit-report-legacy.json). That opt-in preserves
the accepted old schema, scores and exit behavior, including nonblocking canonical
lexical WARNs under `--strict` and the external-observation strict behavior.
Do not use legacy scores to rank or optimize packages. Shared trigger CI remains
unchanged. Remove compatibility only when the remaining field consumers migrate.

Exact checks live
in [audit checks](references/audit-checks.md),
[authoring doctrine](references/authoring-doctrine.md), and
[Codex parity](references/codex-parity.md).

## Conversion

Use `bash skills/skill-builder/scripts/converter/convert.sh <skill-dir> <target>
[output-dir]` for an explicit out-of-tree export. Targets are `codex`, `cursor`
and `test`; `--all` selects all source packages, and `--codex-layout inline`
selects the legacy inline Codex layout. Read
[SkillBundle](references/converter/skill-bundle-schema.md) when format details
matter. Parse the source once, render the target, then validate resource parity
and target format. Report layout and any omitted Cursor references.

The default export is `.agents/projections/converter/<target>/<skill-name>/`.
The exporter clean-writes its output directory, so use only the explicit derived
target: refuse a source package, its ancestor, or the repository root. Preserve
the source unchanged and fix the source or adapter instead of editing output.
A parse, write, format or required-resource failure leaves an incomplete export.
The shipped `skills-codex/**` remains owned by `scripts/codex-sync.sh` through
`scripts/regen-all.sh`; this ad-hoc exporter never replaces that authority.

## Distill expertise

When the caller wants a reusable rule, begin with cited occurrences or a named
authoritative source. State the trigger, desired behavior, inputs, outputs,
negative example and limits. Prefer an addition to an existing reference or
skill over a new root, library, gate or workflow; no action is a valid result.

An abstraction needs three independently evidenced real occurrences and a
successful reapplication to a source case without missing context. Preserve
short source excerpts or command results with resolvable citations. Fewer
occurrences support a narrow reference note; an authoritative source substitutes
only for a faithful statement of that source, not a wider generalization.
Use Research's [pattern mode](../research/SKILL.md#pattern-evidence) when the
claim needs exemplars and a holdout before packaging.

A proposed process artifact must have a concrete consumer, a subject or release
decision it informs, an observed defect and a retirement condition. If any is
missing, omit the artifact. Code written only to consume it supplies no consumer.
Minimal recovery state needs a named evidence-loss or corruption risk. Show a
negative/holdout case and how the proposed rule returns the right decision.

Return the proposal inline unless a durable proposal was requested. Respect
[Memory's source and destination rules](../memory/SKILL.md) for mined material.
Evidence cannot publish itself as policy. Build an artifact only when the
caller's authorization includes adoption; a proposal-only request ends with the
proposal. Repair ordinary known defects within existing authority; tool failures
remain explicit facts for the native caller, not an automatic helper chain.

For an actual package edit, use the [source template](references/skill-template.md)
for required fields and [context density guidance](references/context-density-checks.md)
when deciding which prose earns a place. Neither requires adding a new skill.
