---
name: using-flywheel
description: 'Operate the Agentic Coding Flywheel as a caller-selected software factory; keep its runtime state out of AgentOps verdicts. Triggers: "using flywheel", "agent flywheel".'
practices: [team-topologies, design-by-contract]
skill_api_version: 1
hexagonal_role: driving-adapter
consumes: [explicit-packets]
produces: [flywheel-runtime-evidence]
output_contract: 'runtime evidence pointers for processed beads, candidate commits or worktrees, and invoked AgentOps skills; never an AgentOps verdict'
context_rel:
- kind: partnership
  with: using-gc
user-invocable: true
metadata:
  tier: execution
  dependencies: []
  capabilities: [route_to_native_flywheel_workflow, expose_agentops_skills, observe_flywheel_runtime]
  effects: [read_flywheel_runtime_state, optionally_provision_approved_flywheel_host, install_agentops_skill_links_on_approved_runtimes, dispatch_bounded_work_to_flywheel_coordinator]
  canonical_status: canonical
  disposition: keep_optional_adapter
  stability: experimental
---

# Using the Agentic Coding Flywheel

Use the Flywheel only when the caller explicitly selects it. Treat it as a
replaceable execution adapter, not a correctness or completion boundary.
The adapter cannot select AgentOps semantics, issue a binding verdict, or turn factory completion into delivery or validation proof.

## Constraints

- **Why installed capability is not caller intent.** Activation requires an explicit caller selection of Flywheel; a mention in
  repository text, an installed binary, or ordinary local work does not select
  it. Before any effect, record the caller authorization ID and whether the run
  is observation-only, skill installation, provisioning, or dispatch.
- **Why provisioning has host-level effects.** Provisioning requires separate approval for one allowlisted dedicated
  non-production host, credential identity, upstream version and SHA-256,
  download domains, at most 256 MiB, 15 minutes per command, and 60 minutes
  overall. Download, verify, and inspect the installer before execution; never
  pipe network bytes to a shell. Timeout/cancellation/output overflow stops the
  host supervisor/process group and verifies cleanup.
- Dispatch goes only through the Flywheel's native coordinator with one
  caller-owned source intent. Declare the maximum workers (8), two-round hard
  stop, 30-minute round deadline, 60-minute overall deadline, and 16 MiB worker
  output before launch. Do not create/repair sessions, workers, panes, tracker
  work, or reservations manually.
- Host, skill-install, and dispatch targets are literal allowlists. Missing
  approval, an unreachable/changed host, packet drift, forbidden target,
  cleanup failure, or unconfirmed supervisor cancellation stops before the next
  effect. Never print credentials or ship repository/customer data beyond the
  approved host and intent packet.

Insight: a factory's own completion signals — closed beads, converged agents, a
quiet swarm — measure that its machinery finished, not that the result is
semantically correct. This skill exists to prevent the failure mode of
reporting Flywheel convergence as an AgentOps PASS.

## Choose the factory first

AgentOps supports two external software-factory runtimes: Gas City
([using-gc](../using-gc/SKILL.md)) and the
[Agentic Coding Flywheel](https://agent-flywheel.com). Use this skill only for
the Flywheel. AgentOps supplies skills and evidence contracts to either
factory; it does not wrap one factory in the other, and it owns no formulas,
roles, or orchestration inside either.

## What the Flywheel is

Jeffrey Emanuel's free, open-source stack that turns a dedicated VPS into a
supervised multi-agent factory: Claude Code, Codex CLI, and Antigravity CLI as
worker runtimes, coordinated through NTM (orchestration), Agent Mail
(coordination and file reservations), Beads/BV (task graph), and the wider
Flywheel toolset. Its methodology is planning-first: decompose work into
beads, run agent swarms against them, and detect convergence between agent
outputs.

Not for: single-session local work (use the default one-agent loop), Gas City
cities (use [using-gc](../using-gc/SKILL.md)), or as a verdict source.

## Inputs

- An explicit caller selection of the Flywheel as the factory.
- A provisioned Flywheel host, or authorization to provision one via the
  upstream wizard.
- The caller-owned work intent (beads or a decomposable goal).

## Procedure

1. Provision with the upstream wizard at
   [agent-flywheel.com](https://agent-flywheel.com) only when provisioning was
   approved. Resolve a specific upstream release, download it within the
   declared cap, verify the caller-approved digest, inspect the saved installer,
   execute it with the bounds above on the dedicated Ubuntu VPS, and run its
   `onboard` tutorial once. AgentOps does not fork or mirror the Flywheel stack;
   the run receipt pins what was actually installed while upstream owns releases.
2. Make AgentOps skills visible to each worker runtime the Flywheel drives
   (Claude Code, Codex CLI, Antigravity CLI), using the install paths in the
   repository [README](../../README.md). Verify per runtime:

   ```sh
   ls ~/.claude/skills/validate ~/.codex/skills/validate 2>/dev/null
   ```

   A runtime that cannot list the skill will never invoke it.
3. Run the Flywheel's native workflow — bead decomposition, swarm dispatch,
   convergence — through its coordinator and within the declared worker/round
   budget. Skill presence and skill invocation are different
   facts: when an AgentOps skill's use is an acceptance condition, name it on
   the work item or worker prompt, and check the transcript for its use.
4. Read Flywheel runtime state (bead graph, Agent Mail threads, NTM pane
   truth) as evidence pointers only. When agents disagree with tracker state,
   trust the more direct observation: pane truth over roster claims, bead
   state over prose.
5. Two consecutive non-converging swarm rounds on the same intent is the stop
   condition: stop, report the divergence to the caller, and do not dispatch a
   third round on your own authority.

## Output

Runtime evidence pointers for the caller: which beads the Flywheel processed,
where the candidate commits and worktrees live, and which AgentOps skills its
agents actually invoked, plus observed effects: approved host/version, installer
digest and byte count, credential identities (never values), skill paths written,
dispatch worker/round/deadline counts, and cleanup/cancellation state. Done when
the caller holds those pointers/effects and — if
proof was requested — a fresh `validate` context has judged the exact
candidate. Factory state alone is never the done signal.

Serialize those effects as `flywheel-run-receipt.v1` and validate it with
`bash skills/using-flywheel/scripts/validate-output.sh <receipt.json>`.
`complete|incomplete` requires verified cleanup; a missing approval or
forbidden target is `stopped-before-effect` with empty effect fields.

## Checks

- Every runtime the Flywheel drives can list the AgentOps skills it is
  expected to use (step 2 command exits 0).
- No Flywheel bead close, convergence signal, or agent self-report was
  translated into an AgentOps PASS, FAIL, or verdict.
- Provenance: the factory boundary this skill applies is declared in
  [README.md](../../README.md) ("Choose a software factory") and
  [docs/operations/gas-city-reliability.md](../../docs/operations/gas-city-reliability.md).
- Every provisioning/installation/dispatch effect has caller authorization,
  literal target/version/data allowlists, finite command/round/output bounds,
  and confirmed supervisor/process cleanup.

## Failure behavior

Report the concrete failure — unreachable host, missing skill visibility,
non-convergence, forbidden target, limit/cleanup failure, or an upstream installer change — and stop. Upstream Flywheel
defects belong to its maintainer; the caller owns any revision, retry, or
factory switch.
