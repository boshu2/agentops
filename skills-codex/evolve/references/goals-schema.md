# GOALS.yaml Schema

```yaml
version: 1
mission: "What this repo does"

goals:
  - id: unique-identifier
    description: "Human-readable description"
    check: "shell command — exit 0 = pass, non-zero = fail"
    weight: 1-10  # Higher = fix first
```

Goals are checked in weight order (highest first). The first failing goal with the highest weight is selected for improvement.

## Fitness Snapshot Format

Each cycle writes a fitness snapshot with **continuous values** (not just pass/fail):

```json
{
  "cycle": 1,
  "timestamp": "2026-02-12T15:45:00-05:00",
  "cycle_start_sha": "abc1234",
  "goals": [
    {
      "id": "go-coverage-floor",
      "result": "pass",
      "weight": 2,
      "value": 86.1,
      "threshold": 80
    },
    {
      "id": "doc-coverage",
      "result": "pass",
      "weight": 2,
      "value": 20,
      "threshold": 16
    },
    {
      "id": "go-cli-builds",
      "result": "pass",
      "weight": 5,
      "value": null,
      "threshold": null
    }
  ]
}
```

- **value**: The continuous metric extracted from the check command (null for binary-only goals)
- **threshold**: The pass/fail threshold (null for binary-only goals)
- **cycle_start_sha**: Git SHA at cycle start, used for multi-commit revert on regression

Pre-cycle snapshot: `fitness-latest.json` (rolling, overwritten each cycle)
Post-cycle snapshot: `fitness-latest-post.json` (rolling, for regression comparison)

## Era Baselines

Before the first improvement cycle of a new goal era runs, evolve captures an
immutable baseline snapshot under
`.agents/evolve/fitness-baselines/goals-<hash>/`, where `<hash>` is the first
12 hex characters of the SHA-256 digest of the active GOALS.md or GOALS.yaml
file. A new goal era gets a new directory automatically; an existing era is
not recaptured once that directory contains a JSON snapshot.

Each era baseline includes:
- **All goals** from GOALS.yaml or GOALS.md, measured in their initial state for that era
- **Baseline metadata** embedded in the snapshot JSON written by `ao goals measure`
- **No regression comparisons** — this is the starting point for that era

When the session ends (at Teardown), the system computes the **session fitness trajectory** by comparing the current era baseline against the final cycle snapshot. This produces `session-fitness-delta.md`, which shows which goals improved, regressed, or stayed unchanged over the entire /evolve session.

## Meta-Goals

Meta-goals validate the validation system itself. Use them to prevent exception lists (allowlists, skip lists) from accumulating stale entries unnoticed.

```yaml
# Meta-goals validate the validation system itself
goals:
  - id: allowlist-hygiene
    description: "Every dead-code allowlist entry should have 0 non-test callers"
    check: "bash scripts/check-allowlist-hygiene.sh"
    weight: 7

  - id: skip-list-hygiene
    description: "Every skip-list entry should still reference an existing test"
    check: "bash scripts/check-skip-list-hygiene.sh"
    weight: 5
```

**When to add a meta-goal:** After pruning any allowlist or exception list, always add a corresponding meta-goal that fails if entries have callers/references. Allowlists without meta-goals are technical debt magnets — they grow silently across epics.

## Maintaining GOALS.yaml

Use `/goals` to maintain the fitness specification:
- `/goals` — run all checks, report pass/fail by pillar
- `/goals generate` — scan repo for uncovered areas, propose new goals
- `/goals prune` — find stale/broken goals, propose removals or updates

## GOALS.md Format (Version 4)

GOALS.md extends the YAML format with strategic intent:

```markdown
# Goals

<Mission statement>

## North Stars
- <Aspiration>

## Anti Stars
- <What to avoid>

## Directives

### 1. <Title>
<Description>
**Steer:** increase | decrease | hold | explore

## Gates
| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| id | `command` | N | Description |
```

### Evolve Integration

When GOALS.md is detected, evolve uses the directive-based cascade (Step 3.1):
1. `ao goals measure --directives` returns the directive list as JSON
2. Top-priority directive (lowest number) is assessed for gaps
3. If gap found → generates work item from directive description + steer
4. Directive becomes the work source for the cycle

When `--beads-only` is passed, directive assessment is skipped entirely.

### Format Detection

`ao goals measure` auto-detects format. When both GOALS.yaml and GOALS.md exist, GOALS.md takes precedence.
