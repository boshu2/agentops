# ADR-0004: Domain-Slice Manifest Contract

- **Status:** Accepted (2026-05-17)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0003](ADR-0003-executable-spec-artifact-durability.md)
- **Tracking:** epic `soc-58nt` (Executable spec layer), bead `soc-58nt.3.8`

## Context

Epic `soc-58nt` F3 adds a scoped RPI loop that confines an agent to a declared
DDD domain slice: a bounded context with its own directives, scenarios,
implementation surface, and read fence. Before any F3 implementation starts
this design spike records the contract decisions so F3 implementation workers
share a stable target.

Three pre-existing surfaces touch domain concepts and must be reconciled before
a fourth one is invented:

1. **`skills/domain/SKILL.md`** — a library skill holding the canonical
   ubiquitous language (vocabulary, structural primitives, anti-patterns). It
   is a knowledge surface, not a registry.

2. **`docs/contracts/context-map.md`** — generated from `skills/*/SKILL.md`
   frontmatter (`hexagonal_role`, `consumes`, `produces`, `context_rel`). It
   maps how skills relate to each other architecturally (DDD bounded context
   topology, data flows). It is an *architecture view*, not a runtime scope
   declaration.

3. **Skill frontmatter fields** (`hexagonal_role`, `consumes`, `produces`) —
   per-skill metadata in `SKILL.md` YAML headers. These classify individual
   skills; they do not declare a domain slice's implementation surface or
   acceptance criteria.

None of these three surfaces declares the runtime scope of an agent operating
within a domain slice, nor records which directives and scenarios a slice owns.
A new, distinct artifact is needed: the **domain-slice manifest**.

## Decision

### Decision A — Command shape: `ao rpi phased --domain <name> <goal>`

The scoped domain RPI is invoked as:

```
ao rpi phased --domain <name> <goal>
```

**Not** a new top-level command (`ao rpi --domain`) and **not** a standalone
`ao domain` command. Rationale:

- `ao rpi phased` is the phased RPI engine. Domain scoping is a flag that
  restricts the context loaded by each phase — it extends `phased`, not
  replaces it.
- Keeping domain scoping as a `phased` flag preserves the existing
  `phaseManifest` machinery (per-phase context-budget control) and adds the
  domain read fence on top.
- A top-level `ao rpi --domain` would imply domain scoping applies to the
  *entire* RPI workflow class; `phased` is the right granularity.

### Decision B — Manifest is a durable tracked artifact at `docs/domains/<name>/manifest.yaml`

The domain-slice manifest lives at `docs/domains/<name>/manifest.yaml`, is
git-tracked, and is the **source of truth** for the domain slice.

Optional runtime mirrors (e.g. for hook caching) may sit under
`.agents/domains/`, but `.agents/` is gitignored; the tracked file is
authoritative. This follows the artifact-durability rule in ADR-0003 §3:
*"Domain manifests — tracked under `docs/domains/`"*.

Consequences:
- Changing a domain slice's read fence or owned directives is a reviewable
  git event.
- `ao rpi phased --domain` reads the manifest from `docs/domains/` and does
  not need `.agents/domains/` to exist.

### Decision C — Reconciliation with the three pre-existing domain surfaces

The domain-slice manifest is the **fourth** domain surface. It fills a gap
none of the three existing surfaces covers (runtime scope declaration). The
surfaces are complementary, not overlapping:

| Surface | What it does | What it does NOT do |
|---|---|---|
| `skills/domain/SKILL.md` | Defines ubiquitous language (nouns + primitives) | Does not declare which directives a slice owns |
| `docs/contracts/context-map.md` | Generated architecture view of skill relationships | Does not declare implementation paths or read fences |
| Skill frontmatter (`hexagonal_role` / `consumes` / `produces`) | Classifies individual skills by hexagonal role | Does not scope agent context loading to a slice |
| `docs/domains/<name>/manifest.yaml` (new) | Declares bounded context: owned directives, scenarios, context roots, read fence, validation commands | Does not replace or duplicate any of the above |

To avoid a third uncoordinated domain registry:

- The manifest **does not copy** skill-topology information from
  `context-map.md`. If a domain slice "owns" certain skills, that is expressed
  via `context_roots` pointing at the skill source directories.
- The manifest **does not redefine** terms from `skills/domain/SKILL.md`. The
  `bounded_context` field is a free-form sentence, not a new ontology.
- The manifest **does not replace** skill frontmatter. Frontmatter describes
  individual skills; the manifest declares the aggregate scope of a slice
  spanning multiple skills, CLI packages, schemas, and tests.
- `docs/contracts/context-map.md` remains the architecture view. The manifest
  is the operator-facing scope declaration.

### Decision D — Go model is named `domainSliceManifest`, explicitly distinct from `phaseManifest`

The Go struct that decodes a domain-slice manifest is:

```go
// domainSliceManifest declares the bounded DDD domain slice that scopes
// an ao rpi phased --domain run: owned directives, scenarios, context
// roots, read fence, and validation commands.
//
// This is DISTINCT from phaseManifest (rpi_phased_manifest.go), which is a
// per-phase context-budget declaration (token limits, handoff field selection)
// unrelated to DDD domain slicing.
type domainSliceManifest struct { ... }
```

`phaseManifest` (defined in `cli/cmd/ao/rpi_phased_manifest.go`) is a
per-phase context-budget struct: it controls which `phaseHandoff` fields each
RPI phase loads and the token cap. It is a runtime optimization artifact with
no domain semantics. The two structs are used together during
`ao rpi phased --domain`: `phaseManifest` controls context budgets per phase;
`domainSliceManifest` controls *which* context is in scope at all.

Using the same name or embedding one in the other would conflate two orthogonal
concerns — context breadth (domain fence) vs. context depth (per-phase token
budget).

## Schema

The manifest schema is defined in `schemas/domain-slice-manifest.v1.schema.json`.

Key fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `schema_version` | integer (const 1) | yes | Schema version |
| `domain` | string (`^[a-z][a-z0-9-]*$`) | yes | Short machine-readable name; matches directory under `docs/domains/` |
| `version` | string (semver) | yes | Manifest version (increment on structural change) |
| `bounded_context` | string | yes | One-sentence DDD bounded-context statement |
| `directive_ids` | array of `d-<slug>` | yes | Stable GOALS.md directive IDs this slice owns |
| `scenario_ids` | array of scenario IDs | yes | Promoted spec scenario IDs from `spec/scenarios/` |
| `context_roots` | array of paths | yes | Repo-relative dirs/files forming the implementation surface |
| `allowed_read_globs` | array of globs | yes | Read-fence allow list for agents in this slice |
| `denied_read_globs` | array of globs | yes | Read-fence deny list (precedence over allowed) |
| `validation_commands` | array of objects | yes | Ordered validation steps (label, command, optional working_dir and timeout_seconds) |
| `owner` | string | yes | Team or person responsible |

All fields are `required`. `additionalProperties: false` on the root object and
on each `validation_commands` item.

## Consequences

### Positive

- F3 implementation workers have a stable schema and clear command shape before
  any code is written.
- Domain manifests are durable (tracked) per ADR-0003 rule.
- The four domain surfaces are complementary; no duplication.
- The Go model name signals clearly to future readers that `domainSliceManifest`
  and `phaseManifest` are orthogonal.

### Negative

- Operators must author a `manifest.yaml` before using `ao rpi phased --domain`.
  The `docs/domains/example/` directory provides a template.
- Two Go struct types (`domainSliceManifest` + `phaseManifest`) compose during
  a phased-domain run; the interaction must be documented in the implementation
  bead.

## Acceptance

This ADR is accepted when:

- `schemas/domain-slice-manifest.v1.schema.json` is committed and validates the
  example manifest at `docs/domains/example/manifest.yaml`. ✓
- `docs/domains/` directory exists with `README.md` and `example/manifest.yaml`. ✓
- All four decisions (A–D) are recorded here. ✓
- ADR-0003 is cross-referenced. ✓

## References

- [ADR-0003](ADR-0003-executable-spec-artifact-durability.md) — artifact durability rule; §3 establishes `docs/domains/<name>/manifest.yaml` as tracked
- [ADR-0002](ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md) — hookless CDLC architecture
- `schemas/domain-slice-manifest.v1.schema.json` — the JSON Schema for this manifest
- `cli/cmd/ao/rpi_phased_manifest.go` — `phaseManifest` (the DISTINCT per-phase context-budget struct)
- `skills/domain/SKILL.md` — ubiquitous language (vocabulary, not registry)
- `docs/contracts/context-map.md` — generated skill architecture view (not replaced by this manifest)
- Epic `soc-58nt`, bead `soc-58nt.3.8` — design spike that produced this ADR
