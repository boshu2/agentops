# Schemas

AgentOps exposes JSON Schemas for every inter-component contract: installable manifests, runtime artifacts, and skill frontmatter. This page catalogs them and points to the right file when you need to validate or extend a contract.

Schemas live in two places. Both are source of truth:

- [`schemas/`](https://github.com/boshu2/agentops/tree/main/schemas) — user-facing manifests and artifacts versioned with `.v1.schema.json`
- [`lib/schemas/`](https://github.com/boshu2/agentops/tree/main/lib/schemas) — internal runtime contracts used by the `ao` CLI and swarm runners

Narrative documentation of the contracts (who writes and reads each artifact) lives under [`contracts/`](contracts/index.md) and is indexed from [`contracts/index.md`](contracts/index.md).

## Manifests

Manifests describe installable units — skills, plugins, marketplace entries, hook bindings.

| Schema | Purpose |
|--------|---------|
| [`skill-frontmatter.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/skill-frontmatter.v1.schema.json) | YAML frontmatter block at the top of every `SKILL.md`. Validated by `heal.sh --strict` and CI. |
| [`plugin-manifest.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/plugin-manifest.v1.schema.json) | Claude Code plugin manifest (`plugin.json`). |
| [`codex-plugin-manifest.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/codex-plugin-manifest.v1.schema.json) | Codex plugin manifest — thin variant used by the Codex marketplace. |
| [`codex-marketplace.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/codex-marketplace.v1.schema.json) | Top-level marketplace index consumed by `claude plugin marketplace add`. |
| [`hooks-manifest.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/hooks-manifest.v1.schema.json) | Schema for [`hooks/hooks.json`](https://github.com/boshu2/agentops/blob/main/hooks/hooks.json). See [`HOOKS.md`](HOOKS.md). |

## Runtime artifacts

These describe data written and consumed at runtime — handoffs between sessions, evidence for closure, quality signals.

| Schema | Purpose |
|--------|---------|
| [`handoff.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/handoff.v1.schema.json) | Session-boundary handoff artifact written by `ao handoff`, read by the `SessionStart` hook. |
| [`memory-packet.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/memory-packet.v1.schema.json) | Boundary-memory packet emitted by lifecycle hooks for cross-session continuity. |
| [`evidence-only-closure.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/evidence-only-closure.v1.schema.json) | Proof artifact for issue closures that rely on validation or policy evidence instead of a code delta. |
| [`session-quality-signal.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/session-quality-signal.v1.schema.json) | Per-session quality signal rolled up into the knowledge flywheel. |
| [`scenario.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/scenario.v1.schema.json) | Behavioral validation scenarios stored in `.agents/holdout/`. |
| [`eval-suite.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/eval-suite.v1.schema.json) | Public canary and private holdout evaluation suite manifests. See [`Eval Environment`](contracts/eval-environment.md). |
| [`eval-run.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/eval-run.v1.schema.json) | Evaluation run records, scorecards, baseline comparisons, runtime hygiene, and artifacts. See [`Eval Environment`](contracts/eval-environment.md). |
| [`release-readiness.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/release-readiness.v1.schema.json) | Release readiness score artifact with SIL/VIL/HIL evidence and waiver state. See [`Release Readiness`](contracts/release-readiness.md). |
| [`routing-policy.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/routing-policy.v1.schema.json) | Agentopsd factory routing policy for model/provider/runtime lanes, authority levels, concurrency caps, and manual-merge defaults. See [`Routing Policy`](contracts/routing-policy.md). |
| [`factory-yield.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/factory-yield.v1.schema.json) | Factory yield observations for baseline/treatment comparison across routing, validation, merge, cost, latency, defects, interventions, and artifacts. See [`Factory Yield Ledger`](contracts/factory-yield-ledger.md). |
| [`hook-lease.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/hook-lease.v1.schema.json) | Generated hook lease inventory entries for the AgentOps 3.0 hookless-first migration. See [`Hook Lease Inventory`](contracts/hook-lease-inventory.md). |
| [`remote-compute-target.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/remote-compute-target.schema.json) | Product-neutral GasCity-backed remote compute target configuration. See [`Remote Compute`](contracts/remote-compute.md). |
| [`remote-session-event.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/remote-session-event.schema.json) | Remote session events and idempotent command delivery records. See [`Remote Compute`](contracts/remote-compute.md). |
| [`swarm-evidence.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/swarm-evidence.schema.json) | Permissive shape for files written by swarm workers to `.agents/swarm/results/<task>.json`. Companion strict schema: [`contracts/swarm-worker-result.schema.json`](contracts/swarm-worker-result.schema.json). |
| [`finding.json`](https://github.com/boshu2/agentops/blob/main/schemas/finding.json) | Canonical finding-item schema for validation skills. Compatible subset of [`contracts/finding-artifact.schema.json`](contracts/finding-artifact.schema.json). |

## Internal runtime contracts

Used by the `ao` CLI team runner and worker pipeline. See [`reference.md`](reference.md) for the team execution model.

| Schema | Purpose |
|--------|---------|
| [`lib/schemas/team-spec.json`](https://github.com/boshu2/agentops/blob/main/lib/schemas/team-spec.json) | Team specification consumed by `lib/scripts/team-runner.sh` when launching parallel workers. |
| [`lib/schemas/worker-output.json`](https://github.com/boshu2/agentops/blob/main/lib/schemas/worker-output.json) | Worker artifact written by Codex and Claude workers; watched by `watch-{codex,claude}-stream.sh`. |

## Related contracts

Machine-readable schemas that live under [`contracts/`](contracts/index.md) (narrative + schema paired in one directory):

- [`contracts/repo-execution-profile.schema.json`](contracts/repo-execution-profile.schema.json) — repo bootstrap/validation/tracker/done-criteria for autonomous runs
- [`contracts/rpi-phase-result.schema.json`](contracts/rpi-phase-result.schema.json) — RPI phase result artifacts
- [`contracts/rpi-c2-events.schema.json`](contracts/rpi-c2-events.schema.json) / [`contracts/rpi-c2-commands.schema.json`](contracts/rpi-c2-commands.schema.json) — per-run events/commands JSONL
- [`contracts/next-work.schema.md`](contracts/next-work.schema.md) — `.agents/rpi/next-work.jsonl` shape
- [`contracts/swarm-worker-result.schema.json`](contracts/swarm-worker-result.schema.json) — strict completion contract for swarm workers
- [`contracts/finding-artifact.schema.json`](contracts/finding-artifact.schema.json) — full finding-artifact schema
- [`contracts/factory-yield-ledger.schema.json`](contracts/factory-yield-ledger.schema.json) — contract-local fixture schema for `factory.yield_observation`
- [`contracts/factory-claim-ledger.schema.json`](contracts/factory-claim-ledger.schema.json) — contract-local schema for public claim posture rows, evidence levels, and promotion states

## Validating against a schema

Most schemas follow JSON Schema Draft 2020-12. Any compatible validator will work. Inside this repo:

```bash
# Validate skill frontmatter across all skills
scripts/validate-skills.sh

# Validate hooks manifest
jq -e . hooks/hooks.json           # well-formed JSON
ao hooks show --validate           # schema-aware check

# Validate swarm evidence artifacts
scripts/validate-swarm-evidence.sh
```

CI enforces schema validity for everything shipped in a release — see [`CI-CD.md`](CI-CD.md).

## Versioning

Schemas are versioned in the filename (`.v1.schema.json`). Breaking changes bump the version and publish a new file; the previous version stays in place until deprecation is announced in [`UPGRADING.md`](UPGRADING.md).

If you are adding a new schema, follow the conventions in [`CONTRIBUTING.md`](CONTRIBUTING.md) and link it from this page plus [`contracts/index.md`](contracts/index.md).
