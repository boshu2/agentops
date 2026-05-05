# Routing Policy Contract

> **Status:** Draft
> **Decision:** Factory routing is a schema-backed policy consumed by
> `agentopsd`, not a prose strategy or a raw `--executor-policy` switch.
> **Consumers:** `cli/internal/daemon/routing_policy.go`, daemon job routing,
> future factory status projections, bounded pilot fixtures

The routing policy maps a task class to a model/provider/runtime lane and an
authority level. It is intentionally conservative in milestone 1: cloud/frontier
coding lanes are allowed, local lanes are advisory, and GasCity / Mt. Olympus
production coding lanes are disabled.

## Schema

Machine-readable schema:

- `schemas/routing-policy.v1.schema.json`

Validated fixtures:

- `cli/internal/daemon/testdata/routing-policy/default.json`
- `cli/internal/daemon/testdata/routing-policy/invalid-gascity-production.json`

## Required Shape

```json
{
  "schema_version": 1,
  "policy_id": "repo-default",
  "default_lane": "frontier-codex",
  "max_total_concurrency": 2,
  "auto_merge_enabled": false,
  "manual_merge_by_default": true,
  "lanes": [
    {
      "id": "frontier-codex",
      "enabled": true,
      "authority": "DELEGATED",
      "provider": "openai",
      "runtime": "codex",
      "model": "frontier-default",
      "task_classes": ["code_change", "test_repair", "docs_change"],
      "max_concurrency": 2,
      "cost_hint_usd_per_hour": 0,
      "latency_hint": "interactive",
      "quality_prior": "default",
      "yield_gate": {
        "min_accepted_patches_per_hour": 0,
        "min_sample_size": 0
      },
      "merge_eligibility": {
        "manual_merge_required": true,
        "validation_commands": [
          "cd cli && go test ./internal/daemon -run 'RoutingPolicy|FactoryProjection'",
          "scripts/pre-push-gate.sh --fast"
        ],
        "validation_failure_terminal_event": "factory.job_terminal",
        "retain_artifacts_on_failure": true,
        "retain_worktree_on_validation_failure": true
      }
    },
    {
      "id": "local-observer",
      "enabled": true,
      "authority": "ADVISORY",
      "provider": "local",
      "runtime": "mlx-or-ollama",
      "model": "local-configured",
      "task_classes": [
        "scout",
        "retrieve",
        "summarize",
        "classify",
        "preflight",
        "critique"
      ],
      "max_concurrency": 1,
      "promotion_gate": {
        "requires_yield_evidence": true
      }
    },
    {
      "id": "gascity-reference",
      "enabled": false,
      "authority": "OBSERVE",
      "provider": "gascity",
      "runtime": "mt-olympus",
      "model": "provider-selected",
      "task_classes": ["reference_runtime"],
      "max_concurrency": 0,
      "disabled_reason": "Not production-critical for milestone 1"
    }
  ]
}
```

## Authority Levels

| Authority | Allowed in milestone 1 | Meaning |
|---|---:|---|
| `OBSERVE` | Yes | May inspect and report. Output cannot change routing or merge decisions automatically. |
| `ADVISORY` | Yes | May critique, preflight, summarize, or recommend. Another authority must make decisions. |
| `DELEGATED` | Yes | May perform scoped coding work in an owned worktree. Cannot merge automatically. |
| `AUTHORITATIVE` | No | May make routing or merge decisions only after explicit yield promotion. |

## Validation Rules

Daemon validation fails closed when:

- `schema_version` is not `1`;
- `policy_id`, `default_lane`, or `lanes` is missing;
- unknown JSON fields are present;
- lane IDs are duplicated;
- `default_lane` is missing or disabled;
- `auto_merge_enabled` is not `false`;
- `manual_merge_by_default` is not `true`;
- an enabled lane has `max_concurrency == 0`;
- lane `max_concurrency` exceeds `max_total_concurrency`;
- authority is not one of the four known values;
- a local provider lane is above `ADVISORY`;
- an `AUTHORITATIVE` lane lacks a promotion gate requiring yield evidence;
- a disabled lane lacks `disabled_reason`;
- a GasCity / Mt. Olympus lane is enabled for production task classes;
- an enabled `DELEGATED` or `AUTHORITATIVE` lane serving a production task
  class lacks `merge_eligibility`;
- `merge_eligibility.validation_commands` is empty or contains blank commands;
- `merge_eligibility.validation_failure_terminal_event` is not the known
  `factory.job_terminal` event;
- `merge_eligibility.manual_merge_required`,
  `merge_eligibility.retain_artifacts_on_failure`, or
  `merge_eligibility.retain_worktree_on_validation_failure` is not `true`.

## Production Boundaries

Production task classes are:

- `code_change`
- `test_repair`
- `docs_change`
- `merge_decision`

GasCity / Mt. Olympus lanes may appear as disabled reference lanes, but they
must not route production task classes in milestone 1.

Local lanes may run useful sidecar work such as `scout`, `retrieve`,
`summarize`, `classify`, `preflight`, and `critique`. They cannot own code
changes or merge decisions until yield evidence supports promotion.

## Merge Policy

`auto_merge_enabled` must be `false` and `manual_merge_by_default` must be
`true`. The routing policy does not grant merge authority. A `DELEGATED` coding
lane can produce a candidate patch and validation artifacts; an operator or a
future explicitly promoted gate decides whether to merge.

Factory admission runs before routing. A blocked admission decision must not be
converted into a routed worker lane or yield sample. Once admission allows a
work order, routing selects the lane for the child execution job and keeps the
same manual-merge defaults.

Any route or pilot lane that can produce a merge candidate must declare
`merge_eligibility`. That gate is not permission to merge. It is a machine
readable contract for manual review:

- `manual_merge_required: true`;
- `validation_commands`: exact commands the operator must see passed before
  merge review;
- `validation_failure_terminal_event: "factory.job_terminal"`;
- `retain_artifacts_on_failure: true`;
- `retain_worktree_on_validation_failure: true`.

When validation fails, the lane must leave the worker job terminal through
`factory.job_terminal` with `status: "failed"`, retain validation artifacts,
and keep the worktree for operator recovery. The factory projection represents
that state through `terminal_jobs`, `blocked_validations`, and
`retained_failed_worktrees`.

Legacy `ao rpi parallel` now follows the same safety default: it preserves
worktrees and requires `--auto-merge` to opt into compatibility-mode merging.
