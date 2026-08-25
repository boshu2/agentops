# Skill API Reference

> Definitive reference for AgentOps SKILL.md frontmatter fields. Schema: `schemas/skill-frontmatter.v1.schema.json`.

Publicly, AgentOps talks about bookkeeping, validation, primitives, and flows. This document describes the internal API and taxonomy behind that operating model.

## Frontmatter Format

Every skill has a YAML frontmatter block between `---` delimiters at the top of `SKILL.md`:

```yaml
---
name: my-skill
description: 'What this skill does. Triggers: "keyword1", "keyword2".'
skill_api_version: 1
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
metadata:
  tier: execution
---
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Skill identifier (must match directory name) |
| `description` | string | What the skill does, including trigger phrases |
| `skill_api_version` | integer | Always `1` (const) |

## Optional Fields

### `context`

Legacy metadata that once controlled the now-retired lookup command. Nothing
reads it today; it stays in the schema so skills written against the old shape
keep validating. New skills can omit it — 28 of the 52 shipped skills do. Two
forms remain accepted:

**String form:**
```yaml
context: fork
```

**Object form:**
```yaml
context:
  window: isolated
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
```

#### `context.window`

How the skill's execution context relates to the parent session.

| Value | Meaning |
|-------|---------|
| `isolated` | Fresh context, no parent inheritance. For validation and mechanical skills. |
| `fork` | Copy parent context as starting point. For skills that need to know what you're working on. |
| `inherit` | Use full parent context as-is. For session utilities (`status`, `handoff`, `bootstrap`). |

**v1 status:** Declaration-only. Nothing parses or enforces it.
Do not rely on this field for RPI phase isolation: the freshness requirement
for Validate is a property of the traversal, described in
[`docs/architecture/rpi-traversal.md`](architecture/rpi-traversal.md) and
enforced by the validator's own context-identity checks, not by this key.

#### `context.sections`

Named which knowledge sections a retired injection step would filter.

```yaml
sections:
  include: [INTEL, TASK]     # Allowlist — only these sections
  exclude: [HISTORY]         # Blocklist — everything except these
```

If both `include` and `exclude` are set, `include` takes precedence.

Valid section names:

| Section | What it named |
|---------|---------------|
| `HISTORY` | Past session summaries |
| `INTEL` | Mined learnings and patterns. The surface that produced them was retired with `ao flywheel` and `ao knowledge`; the name is still accepted so old declarations parse, and it now names nothing. |
| `TASK` | Current bead ID and predecessor context |

**v1 status:** Metadata compatibility only. The lookup command was removed and
no shipped code injects any of these sections, so a `sections` declaration
neither adds nor removes context at runtime. Two skills (`converter`, `scope`)
still carry `exclude: [HISTORY, INTEL, TASK]` as inert legacy declarations.

#### `context.intent.mode`

Declares what the skill is doing.

| Value | Meaning |
|-------|---------|
| `task` | Executing work (`implement`, `plan`, `validate`) |
| `questions` | Exploring or researching (`research`, `skill-builder`, `workflow-builder`) |
| `none` | Operational utility (`status`, `handoff`, `scope`) |

**v1 status:** Declaration-only. Nothing reads it at runtime.

#### Retired: `context.intel_scope`

`context.intel_scope` declared how much of the knowledge flywheel to inject.
That surface is gone — `ao flywheel`, `ao knowledge`, `ao patterns`, and
`ao inject` were all removed (see [MIGRATION.md](MIGRATION.md)), and a search
of `cli/` and `scripts/` finds no reader for the key. No shipped skill declares
it any more. `schemas/skill-frontmatter.v1.schema.json` still accepts it so
third-party skills written against the old shape keep validating; a declaration
has no effect.

### `allowed-tools`

Restricts which tools the skill can auto-approve. This one is enforced by the
host agent runtime that loads the skill, not by `ao`. Declared today by
`research` and `status`.

```yaml
# Array form
allowed-tools:
  - Read
  - Grep
  - Glob
  - Bash

# String form (comma-separated)
allowed-tools: Read, Grep, Glob, Bash
```

### `model`

Preferred model for skill execution.

```yaml
model: haiku    # Use cheaper/faster model for lightweight skills
```

Declared today only by `status` (`model: haiku`). Declaration-only — `ao` does
not read it, and model choice belongs to the caller's runtime (`ao config
models` was removed for the same reason).

### `user-invocable`

Whether the skill appears in the slash-command list.

```yaml
user-invocable: true   # Shows as /skill-name
user-invocable: false  # Hidden from user, used by other skills
```

### `metadata`

Skill classification and dependency information.

```yaml
metadata:
  tier: execution              # See tier values below
  dependencies: [standards]    # Sibling skills this one delegates to
  capabilities: [run_the_thing] # What the skill can do
  effects: [write_report]      # Observable side effects, [] for read-only
  canonical_status: canonical  # canonical | alias | deprecated
  disposition: keep_specialist # Why the corpus keeps it
  graph_root: true             # Entry point in `ao skills graph`
  internal: false              # If true, not published externally
  stability: experimental      # experimental | stable (default stable)
```

Every shipped skill declares `tier`, `dependencies`, `capabilities`, `effects`,
and `canonical_status`; `disposition` is on 50, `graph_root` on 11,
`stability` on 5, `internal` on 2. The schema also still permits `version`,
`author`, `triggers`, and `replaces`, which no shipped skill declares.
`dependencies` feeds the delegation edges rendered by `ao skills graph`.

#### Tier Values

Tiers in use by the shipped corpus, with real member skills. The generated
inventory is [SKILL-ROUTER.md](SKILL-ROUTER.md); `skills/catalog.json` is the
machine-readable form (`ao skills list`).

| Tier | Purpose | Skills that declare it |
|------|---------|------------------------|
| `judgment` | Legacy internal tier name for validation and review gates | anti-ceremony, council, craft-goal, postmortem, premortem, reality-check, validate |
| `execution` | Single-task implementation and runtime adapters | account-rotation, agent-mail, cass, cc-hooks, codebase-recon, dcg, idea-genie, implement, learn, ms, ntm, pattern-mining, plan, rch, refactor, research, reverse-engineer, sbh, scaffold, swarm, test, using-flywheel, using-gc |
| `orchestration` | Multi-skill coordination | codex-exec |
| `session` | Session lifecycle | bootstrap, handoff, status |
| `knowledge` | Reference corpora loaded on demand | domain, standards |
| `product` | Product strategy and product-surface work | doc, fitness, goals, product, security |
| `library` | Shared references | shared |
| `meta` | System-level (skills, workflows, scope, the traversal itself) | agent-native, automation-shape-routing, operationalize, rpi, scope, skill-builder, toil-mining, workflow-builder |
| `cross-vendor` | Cross-runtime | agy-native, converter |

The schema's tier enum also still accepts `background`, `contribute`, and
`experimental`. No shipped skill uses them: the `background` family
(`push`, `ratchet`, `flywheel`, `forge`) and the `contribute` family (`pr-*`,
`oss-docs`) were retired, and `ao ratchet`, `ao flywheel`, and `ao forge` are
removed verbs — see [MIGRATION.md](MIGRATION.md).

### `output_contract`

Path to a JSON Schema file that defines the skill's structured output format.

```yaml
output_contract: skills/council/schemas/verdict.json
```

Paths are relative to repo root. Several skills instead use it as a prose
description of their output (`learn`, `using-flywheel`), which the schema
allows — it is typed as a plain string.

**v1 status:** Declaration-only. `scripts/validate-skill-schema.sh` allowlists
the key; nothing resolves the path or checks a skill's output against it.

### Other Fields

| Field | Type | Description |
|-------|------|-------------|
| `license` | string | License identifier (e.g., `MIT`) |
| `compatibility` | string | Runtime requirements (e.g., `Requires git, gh CLI`) |

## Context Declaration Quick Reference

`context` is optional and most skills omit it. These 24 are every skill in
`skills/` that declares one; the remaining 28 declare no `context` block at
all. Regenerate this view with `rg -A5 '^context:' skills/*/SKILL.md` — the
frontmatter is the source of truth, this table is a convenience copy.

| Skill | Tier | Window | Sections | Intent |
|-------|------|--------|----------|--------|
| craft-goal | judgment | inherit | — | task |
| postmortem | judgment | fork | exclude: HISTORY | task |
| codebase-recon | execution | fork | exclude: HISTORY | task |
| pattern-mining | execution | fork | exclude: HISTORY | task |
| refactor | execution | fork | exclude: HISTORY | task |
| research | execution | fork | exclude: HISTORY, TASK | questions |
| reverse-engineer | execution | fork | exclude: HISTORY | task |
| scaffold | execution | fork | exclude: HISTORY | task |
| test | execution | fork | exclude: HISTORY | task |
| codex-exec | orchestration | inherit | exclude: HISTORY | none |
| bootstrap | session | fork | — | task |
| handoff | session | inherit | — | none |
| status | session | inherit | — | none |
| domain | knowledge | isolated | — | none |
| doc | product | fork | exclude: HISTORY | task |
| fitness | product | fork | — | task |
| product | product | inherit | — | task |
| security | product | fork | exclude: HISTORY | task |
| automation-shape-routing | meta | inherit | — | task |
| scope | meta | isolated | exclude: HISTORY, INTEL, TASK | none |
| skill-builder | meta | fork | exclude: HISTORY | questions |
| toil-mining | meta | fork | exclude: HISTORY | task |
| workflow-builder | meta | fork | exclude: HISTORY | questions |
| converter | cross-vendor | isolated | exclude: HISTORY, INTEL, TASK | none |

## Enforcement Summary (v1)

Only two frontmatter fields change behavior at runtime, and neither is
enforced by `ao`: `name`/`description` drive skill discovery in the host agent
runtime, and `allowed-tools` narrows that runtime's auto-approval. Everything
under `context` is inert metadata kept so existing skills keep validating.

| Field | Runtime enforcement | Enforced by |
|-------|--------------------|-------------|
| `allowed-tools` | **Active** — narrows tool auto-approval | host agent runtime |
| `name`, `description` | **Active** — skill discovery and trigger matching | host agent runtime |
| `context.window` | None — declaration-only | — |
| `context.intent.mode` | None — declaration-only | — |
| `context.sections` | None — the injection surface was removed | — |
| `context.intel_scope` | None — retired; no skill declares it | — |
| `model` | None — declaration-only | — |
| `output_contract` | None — declaration-only | — |

What `ao` itself reads is a different set of fields. `ao skills list`,
`consumers`, `producers`, and `graph` read the generated
`skills/catalog.json` — `hexagonal_role`, `consumes`, `produces`,
`context_rel`, `practices`, `dependencies`, `user_invocable`, `graph_root`.
`ao skills find` bypasses the catalog and scores `name`, `description`, and
best-effort triggers parsed straight off each `SKILL.md`.
`ao skills check` and `ao skills resolve` validate the frontmatter itself.
Nothing in that set is `context`.

## See Also

- [Skills Reference](SKILLS.md) — Skill descriptions and router
- [Skill Router](SKILL-ROUTER.md) — Generated inventory of the shipped corpus
- [MIGRATION.md](MIGRATION.md) — Removed `ao` verbs and their replacements
- [Skill Tiers](https://github.com/boshu2/agentops/blob/main/skills/SKILL-TIERS.md) — Taxonomy and dependency graph
- Schema: `schemas/skill-frontmatter.v1.schema.json`
