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

## Cycle-0 Baseline

Before the first improvement cycle runs, the system captures a baseline fitness snapshot (`fitness-0-baseline.json`). This serves as the comparison anchor for measuring session-wide progress.

The baseline includes:
- **All goals** from GOALS.yaml, measured in their initial state
- **Cycle-0 report** (`cycle-0-report.md`) — summary of which goals are failing and their weights
- **No regression comparisons** — this is the starting point

When the session ends (at Teardown), the system computes the **session fitness trajectory** by comparing the baseline against the final cycle snapshot. This produces `session-fitness-delta.md`, which shows which goals improved, regressed, or stayed unchanged over the entire $evolve session.

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

Use `$goals` to maintain the fitness specification:
- `$goals` — run all checks, report pass/fail by pillar
- `$goals generate` — scan repo for uncovered areas, propose new goals
- `$goals prune` — find stale/broken goals, propose removals or updates

## GOALS.md Format (Version 4)

GOALS.md extends the YAML format with strategic intent sections:

```markdown
# Goals

<Mission statement — one sentence. Describes outcomes, not features.>

## North Stars

- <Outcome-focused aspiration — what improves for users if this is achieved>
- <At least one star should describe a measurable user outcome>

## Anti Stars

- <What we explicitly avoid — best when derived from proven failure modes>
- <Each anti-star should reference a real failure pattern if .agents/ data exists>

## Directives

### 1. <Title — evidence-grounded>

<Description citing specific metrics or findings that motivated this directive.
 Good: "Close the multi-runtime promise gap — runtime-specific tests are quarantined (8 dirs in `tests/_quarantine/`)"
 Bad: "Improve test coverage">

**Steer:** increase | decrease | hold | explore
<Steer target should name the specific metric being steered, e.g., "increase (install scripts with smoke tests)">

### 2. <Title>

<Description — mix of engineering AND product/growth directives>

**Steer:** <direction> (<metric being steered>)

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| build-passing | `cd cli && make build` | 8 | CLI builds without errors |
| test-passing | `cd cli && make test` | 7 | All unit tests pass |
```

### Directive Dimensions

A healthy GOALS.md includes directives across multiple dimensions:

| Dimension | Focus | Example |
|-----------|-------|---------|
| Engineering | Code quality, test coverage, complexity | "Keep complexity regressions at zero" |
| Product | User experience, onboarding, gaps | "Gate the install path" |
| Growth | Adoption, retention, community | "Restructure quickstart for under 5 min" |
| Knowledge | Flywheel health, learning compounding | "Verify knowledge lifecycle end-to-end" |

If all directives fall in one dimension, the goals file is incomplete.

### Gate Dimensions

Similarly, gates should cover both code health and product health:

| Type | Examples |
|------|----------|
| Code health | `go-cli-builds`, `go-cli-tests`, `go-vet-clean`, `security-gate` |
| Product health | `flywheel-compounding`, `quickstart-under-5min`, `competitive-freshness` |
| Knowledge health | `athena-freshness`, `athena-no-oscillation`, `flywheel-proof` |

### Key Differences from YAML

| Feature | YAML (v1-3) | Markdown (v4) |
|---------|-------------|---------------|
| Goals/Gates | `goals:` array | `## Gates` table |
| Mission | `mission:` field | First paragraph after `# Goals` |
| Directives | Not supported | `## Directives` section |
| North/Anti Stars | Not supported | `## North Stars` / `## Anti Stars` |
| Version | `version: N` | Implicit (always 4) |

### CLI Commands

```bash
ao goals measure                  # Measure gates (both formats)
ao goals measure --directives     # Output directives as JSON
ao goals validate                 # Validate structure
ao goals init                     # Bootstrap GOALS.md interactively
ao goals steer add <title>        # Add directive
ao goals steer remove <number>    # Remove directive
ao goals steer prioritize <n> <p> # Reorder directive
ao goals migrate --to-md          # Convert YAML → Markdown
ao goals prune                    # Remove stale gates
```

### Format Auto-Detection

`LoadGoals()` auto-detects format:
1. `.md` extension → markdown parser
2. `.yaml`/`.yml` extension → check if `GOALS.md` exists alongside → prefer markdown
3. Default `GOALS.yaml` path → check if `GOALS.md` exists → prefer markdown
