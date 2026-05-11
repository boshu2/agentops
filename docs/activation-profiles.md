# Activation Profiles

Activation profiles are named first-value recipes for AgentOps 3.0. They tell an
operator which domain/practice packet to use, which skills or CLI commands to
run, what artifacts should appear, and what fallback path works when a runtime
does not support a feature.

## 3.0 Ship Decision

3.0 ships activation profiles as docs-backed workflow recipes, not as a new
`ao activate` command.

Reasons:

- The launch path must prove the language before freezing a CLI surface.
- Existing commands already cover the runtime path: `ao quick-start`,
  `ao context packet`, `ao context assemble`, `/council`, `/rpi`,
  `/validation`, `ao schedule`, and `ao daemon`.
- The current `docs/profiles/` directory contains role-based documentation
  groupings. It should not silently become executable configuration.

Future CLI work can add `ao activate <profile>` after the council-first demo and
PMF evidence prove which profiles users actually repeat.

## Relationship To Role Profiles

This page is not the same thing as [Role-Based Profiles](profiles/README.md).

| Concept | Purpose | 3.0 Status |
|---|---|---|
| Role-based profiles | Organize broad skill categories such as software development, platform operations, and content creation. | Existing docs-only taxonomy. |
| Activation profiles | Run one AgentOps workflow with explicit inputs, commands, and expected artifacts. | New 3.0 product recipe. |
| Model profiles | Select model quality/cost behavior for council or runtime commands. | Existing runtime setting. |

## Launch Profiles

### `product-council`

Use when validating a product, design, positioning, or architecture decision
against a shared domain.

This is the 3.0 launch demo profile.

Storyboard: [AgentOps 3.0 Council Demo Storyboard](examples/agentops-3-council-demo-storyboard.md).

**Inputs**

- `docs/examples/agentops-3-domain-practice-packet.md`
- `PRODUCT.md`
- `GOALS.md`
- `PRACTICE.md`
- Relevant bd issue or plan path
- `skills/council/SKILL.md`
- Claim or evidence rules when public copy is involved

**Commands**

```bash
ao context packet --goal "AgentOps 3.0 council-first launch demo"
ao context assemble \
  --phase planning \
  --task "Evaluate the AgentOps 3.0 launch demo against the domain/practice packet" \
  --output-file .agents/rpi/briefing-current.md
```

```text
/council --mixed validate "Given docs/examples/agentops-3-domain-practice-packet.md, should this product decision pass?"
```

**Artifacts**

- `.agents/rpi/briefing-current.md`
- `.agents/council/<run-id>/verdict.md`
- Optional committed summary under `docs/council-log/` for load-bearing public
  decisions

**Fallback**

If mixed Claude/Codex council is unavailable, run `/council --quick` with the
same packet and record the degraded runtime in the verdict notes.

### `engineering-team`

Use when moving from a real repo task to a validated implementation path.

**Inputs**

- Domain/practice packet for the task
- `GOALS.md`
- `PRACTICE.md`
- `AGENTS.md` or local runtime instructions
- One bd issue or `.agents/rpi/execution-packet.json`
- Repo standards and relevant tests

**Commands**

```bash
ao quick-start
ao context packet --goal "<task or issue title>"
ao context assemble --phase planning --task "<task or issue title>"
```

```text
/rpi "<small scoped objective>"
```

**Artifacts**

- `.agents/rpi/execution-packet.json`
- `.agents/plans/<date>-<goal>.md`
- `.agents/council/<run-id>/verdict.md`
- `.agents/rpi/phase-*-summary*.md`

**Fallback**

If `/rpi` is too heavy for the first pass, run `/research`, `/plan`, and
`/pre-mortem` manually, then implement one bd issue.

### `pr-review`

Use when an operator wants judgment before merging agent-produced work.

**Inputs**

- PR diff or local git diff
- Domain/practice packet for the repo or feature
- `GOALS.md`
- `PRACTICE.md`
- Relevant standards and tests
- Existing council or vibe findings, if any

**Commands**

```bash
git diff --stat
ao context assemble --phase validation --task "Review current diff before merge"
```

```text
/council validate this PR
/vibe recent
```

**Artifacts**

- `.agents/council/<run-id>/verdict.md`
- `.agents/findings/` extraction candidates when findings are promoted
- Optional PR body summary or `docs/council-log/` entry for load-bearing
  decisions

**Fallback**

If no PR exists, review the local diff and cite exact files changed.

### `release-discipline`

Use when preparing a release or public launch claim.

**Inputs**

- Release cut sheet
- `PRODUCT.md`
- `PRACTICE.md`
- `docs/releases/` or `evals/workbench/results/` evidence
- Claim ledger and claim markers
- Release validation checklist

**Commands**

```bash
bash scripts/check-factory-claim-ledger.sh --strict --no-fixtures
scripts/pre-push-gate.sh --fast
```

Optional final gate:

```bash
scripts/ci-local-release.sh
```

**Artifacts**

- Release cut sheet
- Claim-ledger validation output
- Release-readiness evidence
- Go/no-go note

**Fallback**

If the full local release gate is too expensive during shaping, run the fast
gate and mark the release gate blocked until final go/no-go.

### `nightly-factory`

Use after first trust exists and the operator wants scheduled compounding.

**Inputs**

- Existing `.agents/` corpus
- `.agents/schedule.yaml`
- Dream/forge schedule template
- Runtime credentials or local subscriptions selected by the operator

**Commands**

```bash
ao init --with-schedule
ao daemon run --schedule-file .agents/schedule.yaml
ao schedule list
```

**Artifacts**

- `.agents/overnight/<run-id>/summary.md`
- `.agents/wiki/forge/`
- `.agents/daemon/ledger.jsonl`

**Fallback**

Run `/dream` or `ao overnight start` manually and inspect the morning report.

## First-Value Path

The launch path uses `product-council` first because it demonstrates the
sharpest 3.0 idea:

1. Show the domain/practice packet.
2. Assemble or inspect the context that will feed the agents.
3. Run council against one product/design/engineering decision.
4. Inspect the verdict artifact.
5. Turn the verdict into tracked work or a public launch decision.

Nothing is hidden: the packet lists every product, goal, issue, standards, test,
and evidence source that enters the judgment.

For the viewer-facing command path with time budget and expected artifacts, see
[AgentOps 3.0 First-Value Path](first-value-path.md).

## Promotion Criteria For `ao activate`

Do not add `ao activate <profile>` until these are true:

- At least two launch or PMF runs reuse the same profile without major edits.
- The profile inputs and artifacts are stable enough to test.
- The command can print what it loads before running anything.
- The command has a dry-run mode.
- Existing `ao context` and `ao quick-start` commands cannot provide the same
  user value with clearer names.
