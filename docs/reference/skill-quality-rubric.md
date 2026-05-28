# Skill Quality Rubric

Use this rubric to score AgentOps skills against a higher market-facing standard while preserving AgentOps repo-runtime constraints. The rubric is AgentOps-owned and derived from pattern-level external-corpus inspection plus existing AgentOps standards.

## Profiles

| Profile | Purpose | Required gate |
|---|---|---|
| Repo-runtime | Works inside this repository and AgentOps release pipeline | AgentOps skill, docs, and Codex artifact gates |
| Marketplace-export | Can be packaged for a marketplace lane | the target marketplace export validator |
| Mega-skill | Product-bundle skill that intentionally exceeds simple package-clean limits | explicit mega-skill classification and split/defer decision |

## Scoring

Score each category from 0 to 3.

| Score | Meaning |
|---:|---|
| 0 | Missing or unsafe |
| 1 | Present but weak, vague, or manual |
| 2 | Solid and usable |
| 3 | Product-grade and mechanically validated |

## Categories

| Category | 0 | 1 | 2 | 3 |
|---|---|---|---|---|
| Trigger quality | vague description | description says what but not when | clear what/when triggers | trigger phrases plus false-positive boundaries |
| Kernel clarity | `SKILL.md` is absent or overloaded | workflow exists but buries decisions | concise workflow with links | router-style kernel with phases, stop conditions, and references |
| Progressive disclosure | no references for complex material | references exist but are thin or unlinked | detailed linked references | reference map covers commands, failures, outputs, and variants |
| Helper scripts | no repeatable mechanics | scripts exist but are manual/fragile | scripts cover core checks | scripts provide scan, validate, doctor, score, or export gates |
| Validation | no runnable checks | manual checklist only | local commands prove behavior | automated repo and export gates with expected outputs |
| Self-test | no `SELF-TEST.md` | ad hoc examples only | trigger and behavior checks | self-test covers trigger, non-trigger, expected artifacts, and failure modes |
| Assets/templates | none where templates are needed | inline templates bloat `SKILL.md` | assets hold reusable payloads | assets are versioned, referenced, and validated |
| Subagents/roles | delegation absent despite broad scope | generic delegation guidance | role packets for complex work | subagents have bounded ownership and validation contracts |
| Safety boundaries | no non-goals or forbidden commands | partial warnings | explicit safe/unsafe operations | clean-room, auth, privacy, and mutation boundaries are mechanically checked |
| Packaging | unknown package readiness | package mostly works locally | package-clean or classified mega-skill | export validator proves mode normalization and marketplace validation class |

Maximum score: 30.

## Rating Bands

| Score | Rating | Meaning |
|---:|---|---|
| 0-10 | C | Basic local prompt, not product-grade |
| 11-20 | B | Useful repo skill with gaps |
| 21-26 | A | Strong skill ready for targeted hardening |
| 27-30 | S | Market-facing, mechanically validated skill |

## Required Gates By Profile

### Repo-Runtime

```bash
bash skills/heal-skill/scripts/heal.sh --strict
bash scripts/validate-skill-frontmatter.sh --strict
bash tests/docs/validate-skill-count.sh
```

When skill behavior or runtime UX changes:

```bash
bash scripts/refresh-codex-artifacts.sh --scope worktree
bash scripts/validate-codex-generated-artifacts.sh --scope worktree
```

### Marketplace-Export

Run the target marketplace's export validator against a temporary package copy. The export wrapper must:

- copy the skill to a temporary directory,
- normalize exported `scripts/` files to non-executable mode,
- run the marketplace validator against the temporary copy,
- classify file-count failures as mega-skill candidates,
- leave source file modes unchanged.

### Mega-Skill

A skill is a mega-skill candidate when it exceeds 50 files or requires broad `assets/`, `scripts/`, `references/`, and `subagents/` to operate.

Mega-skill handling requires one of:

- split into smaller package-clean skills,
- keep as repo-runtime only,
- document a product-bundle profile with explicit validation and distribution limits.

## Minimum Bar For New Market-Facing Skills

New market-facing skills should include:

- `SKILL.md` under the applicable AgentOps line cap,
- at least one linked `references/*.md` file when domain context is substantial,
- `SELF-TEST.md`,
- runnable validation command or script,
- explicit non-goals and forbidden operations,
- export validation against the target marketplace validator.

## Audit Method

For each skill:

1. Count files and structural directories.
2. Read frontmatter description and top-level headings.
3. Check for references, scripts, assets, subagents, and self-test.
4. Run repo-native skill gates.
5. Run export validation on a temporary copy when marketplace readiness matters.
6. Assign score and list the smallest upgrade that improves the rating.

---

**Source:** Pattern-only inspection of an external skill corpus and AgentOps skill standards. No proprietary source text copied.
