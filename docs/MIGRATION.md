# Cathedral Cut migration

AgentOps now owns one small product boundary:

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
```

The caller owns whether to revise, run another invocation, schedule work, use a
tracker, manage Git, or deliver the result. Deterministic repository checks stay
under `ao gate check`; semantic judgment is the Validate skill.

## Removed responsibilities

| Removed command or surface | Surviving alternative |
|---|---|
| `ao pawl` | Invoke the Validate skill for one independent semantic verdict. |
| `ao plan-pawl` | Invoke `premortem` when the caller wants an advisory plan challenge. |
| `ao validate` | Use the Validate skill; use `ao gate check` only for deterministic checks. |
| `ao land` | Use the repository's Git or CI delivery process. |
| `ao done`, `ao close` | Report the result to the caller; AgentOps does not close work. |
| `ao governor`, `ao converge` | The caller decides whether to start a new invocation or revision. |
| `ao yield` | Observe throughput in the selected runtime or external system. |
| `ao claim`, `ao next-work` | Use the caller's tracker or substrate directly. |
| `ao state`, `ao reconcile` | Inspect packets, verdicts, and generic provenance as read-only artifacts. |
| `ao worktree` | Use Git directly. |
| `ao membrane` | Record observations as Validate findings or generic provenance. |
| `ao crank` | Call an executor directly or use the optional `dispatch_once` adapter. |

These major public names have inert, nonzero-exit tombstones for this cut
release only. They do not forward to old code or mutate old state.

## Skills

- `plan` now contains the useful behavior from `discovery`,
  `behavior-first-planning`, and `goal-design`.
- `swarm` exposes only caller-directed `dispatch_once`; `crank` was removed.
- `learn` is an optional later consumer of verdict collections, never a
  lifecycle phase or authority.
- Canonical mortem names are `premortem` and `postmortem`. Hyphenated and
  underscored variants were removed.
- `beads-br` and `beads-bv` were removed from the bundle. Invoke `br` and `bv`
  directly if the caller selects those tools.

## Verdicts and identity

`verdict.v2` binds acceptance and a deterministic `subject-manifest.v1` to
distinct declared author and validator context identities. Freshness is an
attested trust fact, not cryptographic proof of process isolation. Verdicts are
stored atomically by content digest under `.agentops/verdicts/sha256/` unless a
caller supplies another directory.

Historical Pawl, queue, claim, landing, and lifecycle artifacts remain inert
evidence. They no longer influence phase sequencing, verdict validity, or CLI
outcomes.

## Optional runtimes

NTM, Agent Mail, Gas City, councils, and model-specific executors remain
caller-selected adapters or strategies. None is a hard dependency of RPI,
Plan, Implement, or Validate, and none may translate its own attempts, leases,
queues, or delivery state into AgentOps correctness state.
