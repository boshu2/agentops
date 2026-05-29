# Skill-Flow Connectivity Contract

> Gate: `scripts/validate-skill-flow.sh` (CI job `validate-skill-flow`).
> Allowlist: `scripts/skill-flow-standalone.txt`.
> Source of truth: `skills/*/SKILL.md` frontmatter.
> Sibling: `scripts/audit-skill-metadata.sh` owns `context_rel.with` resolution;
> this contract owns the `consumes` vocabulary and connectivity.

## Why this exists

"Do all skills flow together?" was unanswerable until this gate. Skill
dependencies are declared in **three overlapping frontmatter fields** that
historically drifted apart:

| Field | Meaning | Read by context-map? |
|-------|---------|----------------------|
| `consumes` | upstream inputs (skill slug, external input, or produced artifact) | yes (data-flow table) |
| `context_rel` | DDD bounded-context relationship (`kind` + `with`) | yes (mermaid graph) |
| `metadata.dependencies` | upstream skill slugs | **no** |

Because the three are not reconciled, a skill can look "orphaned" in one field
while being well-connected in another (e.g. `trace` declares no `consumes` but
depends on `provenance` via `metadata.dependencies`). This contract defines a
single connectivity model that spans all three and a closed vocabulary for
`consumes`.

## Rules (enforced — gate fails on violation)

### 1. Closed `consumes` vocabulary

Every `consumes` token MUST resolve to exactly one of:

1. a **peer skill slug** (a directory under `skills/`), or
2. a **whitelisted external input**, or
3. an **artifact produced by some skill** (appears in another skill's `produces`).

Anything else is a typo or an undeclared dependency and fails the gate.

**External inputs** (the closed whitelist — extend deliberately, in both
`scripts/validate-skill-flow.sh` and this doc):

| Token | What it is |
|-------|-----------|
| `repo-context` | the repository working tree / source under analysis |
| `external-api` | an upstream API or doc site outside the corpus |
| `bd` | the beads issue store |
| `github-pr` | a GitHub pull request under review |
| `onboard` | the session onboarding handshake |

### 2. `metadata.dependencies` resolution

Every `metadata.dependencies` entry MUST name an existing skill slug.

### 3. Connectivity (no silent orphans)

A skill is **connected** if it shares at least one skill-to-skill edge with a
peer, counting all three layers (`consumes` skill-slugs, `context_rel.with`
skill-slugs, `metadata.dependencies`). A skill with **zero** skill-to-skill
edges is an **orphan** and fails the gate **unless** it is listed in
`scripts/skill-flow-standalone.txt` with a rationale.

Standalone skills are intentional leaves: boundary adapters (`push`,
`openai-docs`), orchestration/install adapters (`codex-team`,
`session-bootstrap`), and human-facing explainers (`using-agentops`). Listing a
skill there asserts "this is a leaf by design." If it later gains an edge, the
gate reports a **stale allowlist entry** — remove it.

## Reported, not enforced (informational)

The gate prints (without failing) two reconciliation signals:

- **`consumes` vs `metadata.dependencies` disagreement** — the two skill-slug
  fields that should agree but historically drifted. Reconciling them (picking
  one canonical field) is tracked work, not a blocker.
- **Dead-end produced artifacts** — artifacts in some skill's `produces` that no
  skill `consumes`. Most are output-type annotations (`result.json`,
  `verdict.json`, `stdout`), not edges, so this is informational.

## How to fix a failure

```bash
bash scripts/validate-skill-flow.sh          # human-readable findings
bash scripts/validate-skill-flow.sh --json   # machine-readable verdict
```

- **consumes-vocabulary**: fix the typo, or declare the producer, or (if it is a
  genuinely new external input) add it to the whitelist above and in the script.
- **metadata-dependencies**: point at a real skill slug or drop the entry.
- **orphan**: wire a real `context_rel`/`consumes`/`metadata.dependencies` edge,
  or add the slug to `scripts/skill-flow-standalone.txt` with a one-line reason.
