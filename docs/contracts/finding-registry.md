# Finding Registry Contract

> **Machine-readable mirror:** every registry line must validate against
> [`finding-registry.schema.json`](finding-registry.schema.json). There is no
> separate `*.example.json` fixture for this contract; the worked instance is
> the inline [Example Entry](#example-entry) below.

The canonical reusable-finding registry lives at `.agents/findings/registry.jsonl`.

This file is the canonical intake ledger for reusable findings in the AgentOps flywheel. Planning and judgment load it before rediscovering a known failure, and judgment writes back only the reusable findings that deserve to affect future work.

The registry line shape remains the canonical `version: 1` JSONL contract. V2 does not replace that ledger. Instead, V2 adds promoted Markdown finding artifacts and compiled prevention outputs defined in [finding-compiler.md](finding-compiler.md).

## Relationship to the V2 Compiler

The v2 prevention ladder has four layers:

1. `.agents/findings/registry.jsonl` — append-only intake ledger for normalized findings.
2. `.agents/findings/<id>.md` — promoted finding artifact with YAML frontmatter matching [finding-artifact.schema.json](finding-artifact.schema.json).
3. `.agents/planning-rules/<id>.md` and `.agents/pre-mortem-checks/<id>.md` — compiled advisory outputs consumed before planning or judgment.
4. `.agents/constraints/index.json` plus `.agents/constraints/<id>.sh` — compiled mechanical outputs governed by [finding-compiler.md](finding-compiler.md).

This contract governs only the registry ledger. The promotion ladder, executable constraint index, and runtime enforcement expectations are governed by [finding-compiler.md](finding-compiler.md).

## Canonical Shape

- One JSON object per line.
- Each line must validate against [finding-registry.schema.json](finding-registry.schema.json).
- Canonical contract version is `version: 1`.
- Canonical path is repo-local. Do not require a service or database.

## Required Fields

Each entry must include:

- `id`
- `version`
- `tier`
- `source.repo`
- `source.session`
- `source.file`
- `source.skill`
- `date`
- `severity`
- `category`
- `pattern`
- `detection_question`
- `checklist_item`
- `applicable_languages`
- `applicable_when`
- `status`
- `superseded_by`
- `dedup_key`
- `hit_count`
- `last_cited`
- `ttl_days`
- `confidence`

The `source.repo`, `source.session`, `source.file`, and `source.skill` fields are the provenance record for the finding.

## Controlled Vocabulary

`applicable_when` is the only controlled vocabulary in v1. Allowed values:

- `plan-shape`
- `classifier`
- `enum-parser`
- `struct-change`
- `pattern-matcher`
- `validation-gap`
- `test-gap`
- `docs-drift`

Writers must choose at least one value. Readers should treat the first item as the primary work-shape hint.

## Canonical Normalization

`dedup_key = <category>|<pattern-slug>|<primary-applicable-when>`

Normalization rules:

- `pattern-slug` is lowercase
- strip punctuation
- collapse repeated whitespace
- join tokens with `-`
- `primary-applicable-when` is the first controlled-vocabulary item in `applicable_when`

Example:

- `category = validation-gap`
- `pattern = Prior finding injection was skipped in planning`
- `applicable_when = ["plan-shape", "validation-gap"]`
- `dedup_key = validation-gap|prior-finding-injection-was-skipped-in-planning|plan-shape`

## Reader Ranking Rules

Readers must:

- consider only `status=active`
- cap the injected set at 5 entries
- sort by severity first
- break ties by explicit scope match
- break remaining ties by literal text match

Recommended scope match order:

1. `applicable_when` overlap with the current work shape
2. language overlap
3. path or changed-file overlap when available
4. literal keyword overlap with the goal, plan, or review target

Reader failure behavior is fail-open:

- missing registry -> skip silently
- empty registry -> skip silently
- malformed line -> warn and ignore that line
- unreadable file -> warn once and continue without findings

## Write-Path Semantics

Writers must:

- persist only reusable findings, not every transient comment
- merge by `dedup_key`
- preserve the most actionable current wording for `pattern`, `detection_question`, and `checklist_item`
- keep lifecycle fields (`status`, `superseded_by`, `ttl_days`, `confidence`) explicit in the merged record

Lifecycle notes:

- later retrieval and close-loop paths may increment `hit_count`
- later retrieval and close-loop paths may update `last_cited`
- post-mortem scoring inputs such as confidence, citations, and recency may guide promotion decisions
- the registry remains the source intake ledger even when higher-level v2 surfaces mutate these fields

## Retirement and Supersession

- `active` entries are eligible for injection
- `retired` entries stay in the registry for history but must not be injected
- `superseded` entries must set `superseded_by` to the replacement finding id
- `ttl_days` is advisory metadata in v1 and is not yet auto-enforced

## Atomic Update Rule

Writers must update `.agents/findings/registry.jsonl` with a temp file plus atomic rename:

1. Ensure `.agents/findings/` exists.
2. Read the current `registry.jsonl` if present.
3. Merge or append entries in memory by `dedup_key`.
4. Write the full result to a temp file under `.agents/findings/`.
5. Replace the live file with an atomic rename.

If a lock is used, the canonical lock path is `.agents/findings/registry.lock`, and writers must remove it on exit, including failure paths.

## Example Entry

```json
{
  "id": "f-2026-03-09-001",
  "version": 1,
  "tier": "local",
  "source": {
    "repo": "agentops/crew/nami",
    "session": "2026-03-09",
    "file": ".agents/council/2026-03-09-pre-mortem-finding-compiler-v1.md",
    "skill": "pre-mortem"
  },
  "date": "2026-03-09",
  "severity": "significant",
  "category": "validation-gap",
  "pattern": "Plans can omit prior-finding injection and rediscover the same failure mode.",
  "detection_question": "Did this plan load matching active findings before decomposition or review?",
  "checklist_item": "Verify the relevant skill reads registry.jsonl and cites applied finding IDs or known risks.",
  "applicable_languages": ["markdown", "shell"],
  "applicable_when": ["plan-shape", "validation-gap"],
  "status": "active",
  "superseded_by": null,
  "dedup_key": "validation-gap|prior-finding-injection-and-rediscovery|plan-shape",
  "hit_count": 0,
  "last_cited": null,
  "ttl_days": 30,
  "confidence": "high"
}
```

## Legacy V1 Deferrals and V2 Status

The old v1 slice described several follow-ons as deferred. Those deferrals are now split into two groups:

- **Superseded by the v2 compiler contract:** promoted finding artifacts, `ao findings`, citation updates, and active declarative constraints now belong to [finding-compiler.md](finding-compiler.md) and the downstream CLI/runtime contracts that implement it.
- **Still deferred beyond issue ag-8ki.1:** automatic TTL retirement and broader cross-repo transport policy remain follow-on implementation work even though the contract now leaves room for them.

The important compatibility rule is simple: the JSONL registry remains the canonical intake ledger, even as later v2 layers compile and consume richer prevention artifacts.
