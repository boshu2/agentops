# Executable-Spec Chain — Reference

Detailed contracts for the executable-spec layer of `ao goals`: scenario
satisfaction (F2), trace/render (F4), and auto re-steer (F5). The `goals`
SKILL.md links here; this file holds the precise schemas and exit-code rules.

## Scenario satisfaction (F2)

`ao goals measure` aggregates the latest result of every behavioral scenario
linked to a directive and computes a satisfaction ratio. The producer reads
scenario result artifacts written by the `ao eval scenario` family.

### `scenario_satisfaction` JSON shape

Every directive object in `ao goals measure --json` and
`ao goals measure --directives` carries:

```jsonc
"scenario_satisfaction": {
  "linked": 4,         // count of scenarios linked to the directive
  "satisfied": 3,      // count whose latest result artifact is PASS
  "ratio": 0.75,       // satisfied / linked (0.0 when linked == 0)
  "threshold": 0.8,    // directive's required ratio (default in policy)
  "status": "RED"      // GREEN (ratio >= threshold)
                       // YELLOW (linked == 0 — nothing to satisfy yet)
                       // RED (ratio < threshold)
}
```

### `--scenarios-only`

`ao goals measure --scenarios-only` evaluates ONLY the executable-spec layer and
skips shell gate-command execution. Use it for fast iteration on scenarios
without paying for the full gate suite. Combine with `-o json` for CI.

### Result-artifact resolution order

Scenario results are resolved from result artifacts (ADR-0003 durability
contract):

1. Promoted spec scenarios — tracked `spec/scenarios/`.
2. Ad hoc holdout scenarios — `.agents/holdout/<id>.json`.

### Exit-code semantics

| Exit | Meaning |
|------|---------|
| 0 | All gates and all directive scenario thresholds satisfied. |
| 1 | One or more gates failed, or a directive is `RED` (ratio below threshold). |
| 2 | Partial result — a scenario artifact was missing or unreadable. |

## Trace and render (F4)

### `ao goals trace`

Renders and audits the directive → scenario → bead → verdict → learning chain.

- `--from <id>` — render the lineage tree rooted at a directive (`d-...`),
  scenario (`s-...`), or bead ID. Add `-o json` for a line-delimited JSON graph.
- `--orphans` — audit the whole chain. Broken references are **errors**;
  missing downstream yields (e.g. a scenario with no verdict) are **warnings**.
- `--strict` — escalate warning-class defects to a non-zero exit (ADR-0005
  §4.2). Errors always exit non-zero regardless of `--strict`.

Link anchors are stable directive IDs (`^d-[a-z0-9][a-z0-9-]*$`) — never the
display numbers, which are not stable across edits. The full link grammar and
defect taxonomy are in `docs/adr/ADR-0005`.

### `ao goals render`

Exports directive-linked scenarios as a Gherkin `.feature` file:

- bare — print Gherkin to stdout.
- `--out <path>` — write the Gherkin to a file instead.

## Auto re-steer (F5)

When a directive's scenarios fail chronically, the re-steer engine recommends a
directive mutation. This is the last and most safety-gated part of the chain.

### `ao goals steer recommend`

Read-only. Runs the re-steer policy engine over the verdict ledger and prints
recommended directive mutations plus skip reasons. GOALS.md is never modified.

### `ao goals steer apply`

Applies the top recommendation to GOALS.md. Two conditions must BOTH hold:

1. The policy's `auto_apply` is `true`.
2. The operator confirms — interactive prompt, or `--auto` / `--yes` for
   non-interactive scripted consent.

A run without confirmation never changes GOALS.md. Every mutation routes through
the non-lossy directive-block patcher (`cli/internal/goals/patcher.go`) — never
`RenderGoalsMD` / `WriteMDGoals`, which are lossy full re-renders.

- `--policy <path>` — re-steer policy file (default `docs/re-steer-policy.json`).
- `--auto` / `--yes` — pre-confirm for non-interactive use.

Policy schema, verdict-ledger format, mutation-safety invariants, and the
human-gate contract are in `docs/adr/ADR-0006`.
