---
id: research-2026-05-10-practice-citation-backfill-mapping
type: research
date: 2026-05-10
---

# Research: Practice-Citation Backfill Mapping for RPI-Core Skills

**Backend:** inline (4-file scope did not warrant explorer spawn)
**Scope:** Schema + slug catalog + per-skill mapping for a bounded 10-15 primitive backfill pass.

## Summary

`scripts/validate-practice-citations.sh` looks for `practices: [slug, slug, ...]` in the first 200 lines of each scanned file. Slugs must match `[a-z0-9-]+` and exist in the PRACTICE.md registry table. Today, **zero** primitives declare practices (754 missing). This pass adds declarations to 13 RPI-core and foundational skill SKILL.md files.

## Key Files

| File | Purpose |
|------|---------|
| `scripts/validate-practice-citations.sh` | Validator — defines the schema by regex |
| `PRACTICE.md` | Canonical 45-slug registry (lines 214-260) |
| `skills/<name>/SKILL.md` | Targets — frontmatter is parsed by the regex |

## Findings

### 1. Schema (validator regex, lines 97-106)

The validator does:
```
head -n 200 "$file" \
  | tr '\n' ' ' \
  | grep -oE '"?practices"?[[:space:]]*:[[:space:]]*\[[^]]*\]'
```

Implications:
- Declaration must appear in the **first 200 lines**.
- Format: `practices: [slug-a, slug-b, slug-c]` (inline YAML list). `"practices": ["a"]` also accepted (JSON style for `.json` primitives).
- Slugs are kebab-case `[a-z0-9-]+`.
- Empty `[]` would count as "declared" but with no slugs to validate — avoid; populate.
- Self-cite example from the script itself (line 23): `practices: [adr, snapshot-testing, ddd-bounded-context]`.

**Placement recommendation:** add `practices:` as a top-level YAML key in the SKILL.md frontmatter, right after `description:`. Frontmatters are well under 200 lines.

### 2. Canonical slugs (45)

```
adr, agile-manifesto, ai-assisted-dev, bdd-gherkin, cmm-process-maturity,
code-complete, containers, continuous-delivery, continuous-integration,
data-contracts, ddd-bounded-context, design-by-contract, design-patterns,
devops, distributed-systems-design, distributed-systems-foundations,
distributed-tracing, dora-metrics, ebpf-observability, event-sourcing-cqrs,
feature-flags, gitops, hermetic-builds, hexagonal-architecture,
infrastructure-as-code, lean-startup, legacy-code-seams, llm-eval-harness,
microservices, mythical-man-month, postels-law, pragmatic-programmer,
prompt-as-spec, property-based-testing, refactoring, resilience-patterns,
service-mesh, snapshot-testing, sre, supply-chain-integrity, tdd,
team-topologies, twelve-factor-app, wiki-knowledge-surface, xp
```

### 3. Existing declarations

`grep` across `skills/*/SKILL.md`, `hooks/*.sh`, `cli/cmd/ao/*.go`, `schemas/*.json` returns **zero** existing `practices:` declarations. Greenfield. The only self-citation lives inside `scripts/validate-practice-citations.sh` (which is not in the validator's own scan target list).

### 4. Skill → Practice mappings (proposed, 13 primitives)

Each mapping cites 2-4 slugs that the skill's stated purpose embodies, per its frontmatter `description` + Quick Ref + first section.

| Skill | Practices | Rationale |
|---|---|---|
| `rpi` | `continuous-delivery, dora-metrics, agile-manifesto, pragmatic-programmer` | Lifecycle pipeline orchestrator; tracer-bullet end-to-end |
| `discovery` | `adr, lean-startup, mythical-man-month` | Decision records + build-measure-learn discovery; Conway/no-silver-bullet awareness |
| `crank` | `continuous-delivery, xp, agile-manifesto` | Wave-based execution; XP small-batch shipping |
| `validation` | `llm-eval-harness, dora-metrics, sre` | Eval/canary gates; error-budget mindset |
| `plan` | `adr, agile-manifesto, pragmatic-programmer` | Just-enough planning + decision records + orthogonal decomposition |
| `implement` | `tdd, refactoring, code-complete` | Single-issue execution; red-green-refactor; construction practices |
| `vibe` | `ai-assisted-dev, llm-eval-harness, code-complete, pragmatic-programmer` | Council judgment as verification harness; readiness checks |
| `pr-prep` | `continuous-integration, continuous-delivery, gitops` | PR-as-CI gate; declarative branch-based delivery |
| `pr-validate` | `continuous-integration, code-complete, pragmatic-programmer` | Pre-merge gate; quality + orthogonal small PRs |
| `post-mortem` | `dora-metrics, sre, lean-startup` | Blameless retrospective; build-measure-learn close |
| `domain` | `ddd-bounded-context, wiki-knowledge-surface, pragmatic-programmer` | Ubiquitous language is core DDD; corpus IS a wiki surface |
| `flywheel` | `wiki-knowledge-surface, lean-startup, dora-metrics` | Knowledge-surface velocity/friction metrics |
| `handoff` | `adr, wiki-knowledge-surface, code-complete` | Decision capture + surface feed + completeness contract |

All cited slugs verified against the catalog above. No invalid slugs.

## Recommendations

1. Add `practices: [...]` as a top-level YAML key directly after `description:` in each target SKILL.md frontmatter.
2. Backfill batch order: `rpi`, `discovery`, `crank`, `validation`, `plan`, `implement`, `vibe`, `pr-prep`, `pr-validate`, `post-mortem`, `domain`, `flywheel`, `handoff` (13 primitives total).
3. After backfill: run `bash scripts/validate-practice-citations.sh` — expect `with practices field: 13`, `missing: 741`, `invalid: 0`.
4. Do **not** flip the gate to `--strict` this pass. Need all 754 declared before promotion.
5. Carry forward: next sessions repeat the same mapping pattern for the remaining 741 primitives (other skills, all hooks, evals, CLI commands, schemas).

## Coverage / depth / assumptions

- **Coverage:** validator script (full read), PRACTICE.md slug table (full read), 13 SKILL.md headers (top 50 lines). Sufficient — frontmatter is what we mutate.
- **Depth:** schema understanding is exact (regex read line-by-line). Mapping rationale derived from each skill's first-section purpose statement.
- **Assumption (verified):** SKILL.md frontmatter is YAML; adding a top-level `practices:` key won't break existing readers — none of the audited skills parse YAML strictly for unknown keys.
- **Assumption (verified):** PRACTICE.md table format matches the validator's awk parser — confirmed by running it (45 slugs extracted).
- **Gap (deferred):** mapping for the remaining 741 primitives. Out of scope for this pass.
