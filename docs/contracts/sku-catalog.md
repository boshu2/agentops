# SKU Capability Catalog Contract

> **Status:** active. **Bead:** ag-cbm. **Artifact:** `registry.json` (schema_version 2),
> the `capabilities` / `capability_summary` / `cli_top_level_commands` blocks.
> **Gate:** `validate-sku-catalog-drift` (`.github/workflows/validate.yml`).

The SKU capability catalog is the single queryable inventory of every AgentOps
capability — skills, CLI commands, CI gates, and reference implementations. It is
a **4th derived projection** that sits alongside the three source axes:

| Axis | Source of truth | Generated to |
|---|---|---|
| DDD (vocabulary) | `skills/domain/references/` | BC names + ubiquitous language |
| Hex (structure) | `skills/*/SKILL.md` frontmatter | `docs/contracts/context-map.md` |
| Gherkin (acceptance) | `skills/*/references/*.feature` + bead `## Scenarios` | scenario hashes |
| **SKU (capability)** | **none — it is a JOIN** | **`registry.json` schema v2 `capabilities`** |

It is **never hand-authored** — the repo bans hand-edited inventory maps. It is a
JOIN of sources that already exist and are already gated.

## What it is not

It is not a 5th source of truth. Every field is projected from an upstream source;
the catalog adds no new authored datum. Even the skill↔command join key
(`drives_commands`) is **derived from the skill body**, not declared in frontmatter.

## Schema

`registry.json` schema_version 2 is a strict superset of v1 (existing
`summary` / `surfaces` / `cadence_recommendations` consumers keep working). It adds:

- `capabilities` — one SKU entry per capability.
- `capability_summary` — counts by type (`total`, `skills`, `cli_commands`, `gates`, `reference_impls`).
- `cli_top_level_commands` — the real top-level cobra command list (retires the
  bogus "163 cli_commands" file count — see below).

### SKU entry fields

| Field | Type | Meaning | Source |
|---|---|---|---|
| `sku` | string | stable id: `skill:<name>` / `cmd:ao.<path>` / `gate:<job>` / `ref-impl:<name>` | derived |
| `name` | string | display name | source |
| `type` | enum | `skill` \| `cli-command` \| `gate` \| `reference-impl` | derived |
| `bounded_context` | `BC1`..`BC5` | owning context | `skill-dispositions.yaml` (skills); same-named skill or `COMMAND_BC` map (commands) |
| `hex_role` | string | hexagonal role | `skill-dispositions.yaml` / SKILL.md frontmatter |
| `tier` | string | skill tier | `skills/SKILL-TIERS.md` |
| `purpose` | string | one-line purpose | SKILL.md `description` / cobra `Short` |
| `status` | string | `active` \| `deprecated` \| `planned` \| `alias-of:<sku>` | derived (see below) |
| `disposition` | string | editorial disposition | `skill-dispositions.yaml` |
| `consumes` / `produces` | list | data flow | SKILL.md frontmatter |
| `drives_commands` | list | the `ao` commands a skill drives (the join key) | **derived from skill body** |
| `driven_by_skills` | list | reverse of `drives_commands` | derived |
| `flags` | list | long flags of a cli-command | live cobra help |

### `status` derivation

1. Explicit SKILL.md frontmatter `status: deprecated|planned` wins.
2. Else `disposition: merge-review` with a rationale mentioning an alias absorbed
   into a sibling → `alias-of:skill:<sibling>` (e.g. `expert-council` →
   `alias-of:skill:council`).
3. Else `active`.

## `drives_commands` — the missing join key (oracle gap #1)

No prior artifact linked a skill to the `ao` commands it drives. The SKU catalog
derives this by scanning each skill's SKILL.md + `references/*` body for
`` `ao <command>` `` snippets and resolving each against the **live cobra tree**.
Only commands that actually resolve become edges, so a stale reference (e.g. the
removed `ao schedule`) is never silently promoted to a join edge. Implementation:
`scripts/lib/sku_extract.py`. This is the shared primitive both the generator and
the drift gate's linkage check reuse, so the two surfaces can never disagree about
what a "real command" is.

## Generation

- Engine: `scripts/lib/sku_catalog.py` (join logic) + `scripts/lib/sku_extract.py`
  (extractor + live command-tree scan).
- Entry points: `scripts/generate-sku-catalog.sh` (prints the SKU block) and
  `scripts/generate-registry.sh` (folds it into `registry.json` schema v2).
- Sources joined: `skills/*/SKILL.md` frontmatter, `docs/contracts/skill-dispositions.yaml`,
  `skills/SKILL-TIERS.md`, the live `ao capabilities` cobra tree,
  `.github/workflows/validate.yml` job ids, `packs/agentops/`.

Regenerate after touching any source:

```bash
bash scripts/generate-registry.sh
```

## Gate: `validate-sku-catalog-drift`

`scripts/validate-sku-catalog-drift.sh` enforces three checks (all required):

1. **Drift** — regenerate the SKU block and diff against committed `registry.json`;
   fail on any difference (mirrors `registry-check`).
2. **Linkage integrity** — every skill `drives_commands` edge must resolve to a
   real `ao` command in the live cobra tree (closes oracle gaps #1/#2/#3: stale
   skill→CLI references can no longer ship undetected).
3. **Coverage** — every bounded context (BC1–BC5) and every operating-loop move
   (1–7) has at least one **active** skill, and every BC has at least one
   cli-command SKU.

## Retired: the "163 cli_commands" count

Schema v1's `summary.cli_commands` counted `cli/cmd/ao/*.go` files containing a
`Command()` func (163) — a generation artifact, not a real command count. Schema v2
counts the **live top-level cobra command nodes** (the real surface), sourced from
`ao capabilities`. Any "≈163 commands" framing is retired.

## Relation to existing infra

- **Supersedes:** `registry.json`'s shallow v1 skill/CLI arrays (now enriched into
  SKU entries; v1 keys retained as a superset).
- **Extends / joins:** the 5-BC kernel (`bounded-contexts.yaml`), `skill-dispositions.yaml`,
  `context-map.md`'s frontmatter source, `SKILL-TIERS.md`, and the live cobra tree
  remain the upstream sources — the catalog is the downstream aggregate.
- **Distinct from:** `skills/catalog.json` (a skills-only frontmatter projection,
  gated separately by `check-skill-catalog-drift`).
