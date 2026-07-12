---
name: ntm
description: Orchestrate NTM tmux agent swarms and robot
---
<!-- TOC: One Rule | Constraints | Outcome | Cold Start | Software Factory Mesh | Mandatory Loop | NTM Action Card | Surface Selection | Pattern Tiers | Anti-Patterns | Pre-Flight Checklist | Output | Quality | Operating Notes | Reference Index | Related Skills -->

> **Scope:** this skill is AgentOps operating doctrine for the external NTM binary. The binary is self-describing — query its live robot help, capability, schema, and snapshot surfaces before state changes. Never trust remembered syntax over the executable contract. `agent-native` owns the portable worker lifecycle; this skill owns NTM-specific pane mechanics only.

# NTM — external pane adapter

> **The One Rule:** Discover the live NTM contract first, then use the least interactive surface that can prove and execute the action. No `--robot-capabilities` / `--robot-snapshot` evidence -> no automation assumption.

The most common NTM mistake is treating it like a tmux macro runner. NTM is a control plane: robot API, attention feed, work graph, locks/mail, pipelines, safety, approvals, serve API, and durability all have explicit contracts. Use the contract.

## Constraints

- Keep NTM opt-in because the operator chooses the orchestration substrate; never start, register, or probe it merely because it is available.
- Keep `agent-native` as lifecycle owner and NTM as pane-mechanics adapter because portable factory policy must not depend on tmux.
- Give every writing pane a disjoint worktree or reserved write scope because shared-path collisions are the primary multi-agent failure mode.
- Keep producer, tester, fresh-context refuter, and integrator roles distinct because a producing pane cannot independently certify its own candidate.
- Route admission through `ao pawl review`, not pane consensus, because NTM transports work while the AgentOps membrane owns verdicts.

## Outcome — When an NTM Action Has Delivered

A state-changing NTM action is complete only when **all** of the following hold:

- The intended state transition is **visible in `ntm --robot-snapshot`** — not just acknowledged by the command's exit code. (NTM commands can succeed at the API layer while panes/work/locks remain unchanged; trust the snapshot, not the return value.)
- The **attention feed** (`--robot-attention` / `--robot-tail`) shows the expected event(s) — pane output, work-graph movement, lock acquire/release, mail delivery. Absence is evidence of failure.
- Adjacent state (git, br/beads, mail, pipelines) reflects the action's downstream effects within one observation window — otherwise the action fired in isolation and likely didn't accomplish its real purpose.
- Locks and pipelines you opened are either **released / completed** by you, or explicitly handed off via mail with a thread the next operator can claim. Orphaned locks block the swarm.
- For dispatched marching orders: the targeted pane has acknowledged (printed the order, started the work, or replied via mail). A sent-but-not-acknowledged order is not "done."

If the snapshot or attention feed disagrees with what the command said happened, **trust the snapshot** and re-discover the contract — the local model of NTM is stale.

## Cold Start: Which NTM Skill?

| Situation | Start here |
|---|---|
| You need NTM doctrine, then exact syntax via `--robot-docs` / references | This skill |
| You are tending an already-running worker factory | `$agent-native` for lifecycle policy, then this skill for NTM mechanics |
| You are running a Brenner-style hypothesis investigation or incident RCA through NTM panes | `brennerbot-with-ntm` |
| You only need Beads or BV mechanics | `$beads-br` or `$beads-bv` |

For any state-changing action, verify the live contract with `ntm --robot-capabilities` before executing.

### Folded triggers (ag-s43tg wave 1): `ntm-browser-test-coordination` + `ntm-review-worker-orchestration` route here

- **Browser/UI test coordination.** Use when coordinating browser or UI tests through NTM panes
  with screenshots and handoffs: dispatch the test run as marching orders to a dedicated pane,
  reserve the surfaces under test via `agent-mail`, keep screenshot/artifact paths in the pane
  output, and confirm completion in `--robot-snapshot` + the attention feed before handing off.
- **Review/analysis workers.** Use when operating an NTM review or
  analysis worker with bounded inputs and evidence-backed output: scope the worker
  to an explicit input set (files, diff, bead),
  require artifact-backed findings (paths + line refs, not impressions), and treat a worker that
  emits conclusions without evidence as not-done — re-dispatch with the bounded-input contract
  restated.

## Tending doctrine (single owner)

**`agent-native` owns lifecycle policy; `ntm` owns NTM mechanics.** Apply the suspect → bounded nudge → replace policy from `agent-native`; use the recovery commands, liveness truth stack, boot-race warnings, and executable robot contract documented here to enact it.

## Software Factory Mesh

**Operator-choice invariant:** NTM is an optional substrate selected explicitly by the operator; a cold `ao pawl review` and ordinary in-session work do not require an NTM session.

**Pane-role contract:** Producer panes own disjoint worktrees and write scopes; tester panes run deterministic checks; fresh-context refuter panes judge without mutation; integrator panes act only after an `ao pawl review` `CONFIRMED` verdict. Never collapse producer and refuter for one candidate.

**Agent Mail handoff:** Before two or more writers run, reserve disjoint paths. Handoff on one Agent Mail thread with bead, pane role, worktree, reserved paths, exact HEAD, evidence paths, and next action; require recipient acknowledgement and release reservations at completion.

**Pawl authority:** NTM may host or tend warm reviewer panes, but `ao pawl review` owns independent verdict and admission. Do not inject keys into an in-flight pawl pane, and do not treat pane agreement as confirmation.

**Failure routing:** A plain `REFUTED` verdict returns to the producer for automatic repair and revalidation. Only a tripped circuit breaker enters `HOLD` and receives exactly one bounded helper consultation before the candidate re-earns an independent verdict.

## The Loop (Mandatory)

```
1. DISCOVER   -> ntm --robot-capabilities; ntm --robot-tools; repo AGENTS.md/README.md
2. SNAPSHOT   -> ntm --robot-snapshot; inspect sources/degraded_sources, cursor, sessions, panes
3. SELECT     -> choose the smallest surface: work/assign/send/wait/pipeline/locks/mail
4. PROVE      -> fill the NTM action card: target, contract, safety, ownership, rollback
5. EXECUTE    -> prefer --robot-* for automation; avoid human-only TUIs
6. VERIFY     -> attention/events/causality/tail plus git/br/mail evidence changed as expected
7. CLEANUP    -> release/renew locks, checkpoint/handoff, prune old pipeline state when appropriate
8. REPEAT     -> re-snapshot on cursor expiry or after any state-changing action
```

## NTM Action Card

For every state-changing NTM action, be able to answer this before running it:

```markdown
## NTM action: <command>
- Target session/project: <name/path>; resolved by: `ntm config get projects_base` / `ntm quick` / snapshot
- Live contract checked: `ntm --robot-capabilities` contains <flag>; schema/docs checked if unfamiliar
- Evidence before: cursor=<N>; sources=<fresh/degraded>; panes=<count>; locks=<summary>
- Ownership/safety: Agent Mail reservation or worktree policy is clear; user pane inclusion is intentional
- Blast radius: panes/files/sessions affected; destructive/safety/policy approvals required? <yes/no>
- Verification after: <robot event / tail movement / bead state / git change / pipeline status>
- Recovery: <smart restart / interrupt / checkpoint restore / cancel pipeline / handoff>
```

If you cannot fill the card, do a read-only discovery pass first.

## Surface Selection

Score candidate surfaces when several could work:

```
Score = (ContractFit x Observability x Reversibility) / BlastRadius

ContractFit    1-5: exact robot/schema match beats human help text
Observability  1-5: action emits cursor/event/status/causality evidence
Reversibility  1-5: easy cancel/retry/restore/checkpoint
BlastRadius    1-5: one pane/file is low; whole session/process tree is high
```

Pick the highest score. In ties, prefer the surface that produces structured output. Enumerate candidates from `ntm --robot-docs=commands`, not from memory. Standing preferences:

- `--robot-*` for anything machine-driven; `ntm dashboard` / `ntm palette` / `ntm view` are human-only TUIs.
- `--robot-format=toon` (or `NTM_ROBOT_FORMAT=toon`) and `--robot-verbosity=terse` when context is tight.
- Recovery order: diagnose -> probe / is-working -> smart-restart -> explicit restart. Never kill before a liveness proof.
- Event-driven tending (`--robot-wait` / `--robot-attention`) over fixed sleep/poll loops.

## Pattern Tiers

Escalate only with the action card filled; each tier raises the proof bar:

1. **Tier 1 — safe read-only** (capabilities/schema, snapshot, events/digest/attention, work triage/queue-dry, locks list/check). Always permitted; proof = fresh `sources` / `degraded_sources` reviewed, cursor advancing, no conflicting reservation.
2. **Tier 2 — reversible control** (directed send, interrupt, smart-restart, assign, pipeline cancel/resume). Guard = explicit pane/type scoping, tail/liveness evidence that intervention is warranted, run id and state confirmed.
3. **Tier 3 — durable orchestration** (pipelines, Agent Mail locks, checkpoint/handoff, serve API, safety/policy/approvals). Guard = dry-run first, lease/thread/run ids captured, auth and exposure understood, exit/recovery path named.

## Anti-Patterns (Never Do)

| Bad move | Why it fails | Use instead |
|---|---|---|
| Call `ntm view` from automation | Retiles the human layout and returns nothing useful | `--robot-tail`, `--robot-snapshot`, or `--robot-dashboard` |
| Trust old notes over `--robot-capabilities` | NTM surface changes quickly | Discover first |
| Send to `--all` without naming the user-pane intent | Can hit the operator pane | use type/panes or `-s/--skip-first` |
| Treat cursor values as portable | Cursors are per-server monotonic | checkpoint/handoff for portability |
| Kill/restart before a liveness proof | Destroys partial work | diagnose -> smart restart -> explicit restart |
| Conflate pipeline status and run | `--robot-pipeline=<id>` is status | `--robot-pipeline-run=<file>` |
| Retry degraded mail/CASS forever | Burns the session | record degraded source, use fallback, continue |
| Infer abandoned beads from silence | NTM deliberately does not implement `bead_orphaned` | explicit status/mail/reservation evidence |
| Trust a fresh `spawn --cod` pane blind (bare shell) | Some builds leave a **bare zsh**; prompts execute as shell text | verify with `--robot-tail`; relaunch the CLI, or fall back to `codex exec -C <worktree>` per lane |
| Fire a separate `send` right after a bare `spawn` (boot race) | `spawn` returns **before** the agent boots to its input box; the first send is silently dropped → pane is never-engaged (CLI alive, 0.0% CPU) | wait for input-ready first: `--assign` / `--init-prompt` / `--robot-wait=ready`; if already dropped, **re-dispatch, don't restart** (OC-047) |

## Pre-Flight Checklist

- [ ] Repo `AGENTS.md` / README read when operating inside a codebase (repo-local rules override this skill).
- [ ] `ntm --robot-capabilities` checked for any unfamiliar flag.
- [ ] `ntm --robot-snapshot` captured and `sources` / `degraded_sources` reviewed.
- [ ] Session/project resolution verified; labels and `projects_base` make sense.
- [ ] User pane inclusion/exclusion is explicit.
- [ ] File ownership is clear: Agent Mail reservation, bead assignee, or approved worktree policy.
- [ ] For pipelines: dry-run passed; run id/state file plan known.
- [ ] For recovery: liveness truth stack supports intervention.
- [ ] For destructive/risky actions: safety/policy/approval surfaces checked.
- [ ] Post-action verifier named before execution.

## Output Specification

- **Path:** structured robot state is emitted on `stdout`; durable evidence belongs under the bead-named `.agents/` path, never inside this skill or the NTM index.
- **Filename:** NTM creates no default report; when the surrounding arc requires one, use its declared filename or `<bead>-ntm-handoff.md`.
- **Format:** robot evidence is JSON or TOON; durable handoff is Markdown containing the action card, exact HEAD, snapshot/attention proof, reservations, and Agent Mail thread id.
- **Validation command:** run `skills/ntm/scripts/validate.sh` for this mesh contract; for a live action, verify `ntm --robot-snapshot` plus attention/mail/git/bead state.
- **Downstream handoff:** send the verified state and evidence on the existing Agent Mail thread to the named next role; acknowledgement and released/transferred reservations are the completion marker.

## Quality Checklist

- Operator choice is explicit: no NTM session or pawl service is started merely because the substrate exists.
- Pane roles remain separated, write scopes are disjoint, and every multi-writer handoff is acknowledged through Agent Mail.
- Deterministic evidence and a fresh-context `ao pawl review` verdict—not producer or pane consensus—control admission.
- Plain `REFUTED` work auto-repairs; only a tripped breaker gets one helper, and every lock or reservation is released or transferred.

## Operating Notes (doctrine-critical facts)

- **Project resolution is the #1 cross-tool breakage:** session name MUST equal the directory basename under `projects_base` (`NTM_PROJECTS_BASE`), or agent-mail/beads/reservations register under a different key than NTM sees. If tools "see different projects," fix this first.
- **Coordination default:** Agent Mail reservations are the primary primitive; `--worktrees` isolation is allowed when repo policy permits. If mail/reservations are degraded, record it and use bead assignee/status as the soft lock — no retry loops.
- **Cross-machine continuity** is checkpoint export/import or handoff bundles — never shipped cursors.
- **Safety surfaces are first-class:** use `ntm safety` / `ntm policy` / `ntm approve` (approve takes a *token*, not a bead id) instead of ad hoc shell habits; obey repo rules that route builds through `rch` or similar.
- **The standing pawl-service is NTM's largest persistent-pane consumer — tend it, don't nudge it.** `ao pawl up` spawns a session named `<repo>--pawl-service` (e.g. `agentops--pawl-service`), one warm reviewer pane per model family: `cc`=claude/opus (pane 1), `cod`=codex (pane 2), `agy`=Gemini (pane 3); it obeys the same `projects_base`-basename rule above. **Do NOT send keys to a pawl-service pane while a route is in flight** — the route loop deliberately never injects keys into a reviewing pane (a stray nudge breaks the verdict). Tend it read-only with `ao pawl doctor` / `ao pawl health`; bring it up with `ao pawl up` (`--dual` cc+cod is the default panel; `--tri` adds agy) and reap the idle account slot with `ao pawl reap`. Full contract: [`docs/contracts/pawls.md` §Operating the warm pawl-service](../../docs/contracts/pawls.md). The pawl panes are operator machinery — a plain `ao pawl review` needs no NTM at all (it runs cold from any git repo).
- The full distilled trip-wire list (CASS dedup blocking sends, `--` label separator, send-vs-spawn flag parsers, attention flag namespacing, PATH precedence for safety wrappers, and more) lives in [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md).

## Reference Index

Keep depth out of this file. The binary's own docs (`ntm --robot-docs=...`) are the first stop for syntax; load these for operator-handbook detail:

| Topic | Reference |
| --- | --- |
| `ntm send` deep reference (selectors, templates, CASS dedup, error modes) | [SEND.md](references/SEND.md) |
| `ntm spawn` deep reference (counts/variants, labels, worktrees, recipes, stagger) | [SPAWN.md](references/SPAWN.md) |
| Work intelligence & assignment (`ntm work *`, `ntm assign`, bv integration) | [WORK-AND-ASSIGN.md](references/WORK-AND-ASSIGN.md) |
| Ensemble mode (reasoning modes, presets, `--robot-ensemble-*`) | [ENSEMBLE.md](references/ENSEMBLE.md) |
| Pipelines (YAML schema, run IDs, resume/cancel, robot flags) | [PIPELINES.md](references/PIPELINES.md) |
| Serve API (auth modes, REST route map, OpenAPI, SSE) | [SERVE.md](references/SERVE.md) |
| Safety, policy, approvals (policy.yaml, tokens, what `safety install` drops) | [SAFETY.md](references/SAFETY.md) |
| Durability stack (checkpoint vs timeline vs handoff vs resume) | [DURABILITY.md](references/DURABILITY.md) |
| Integration surfaces (DCG, SLB, CAAM, RCH, mail, cass, quota) | [INTEGRATIONS.md](references/INTEGRATIONS.md) |
| Environment variables (`NTM_*`, `TOON_*`) | [ENV-VARS.md](references/ENV-VARS.md) |
| Troubleshooting (symptom / root cause / fix, full gotcha entries) | [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) |
| Self-test / trigger phrases | [SELF-TEST.md](references/SELF-TEST.md) |
| High-leverage command patterns, output capture, reusable assets | [COMMANDS.md](references/COMMANDS.md) |
| Attention feed, robot formats, wait conditions, full `--robot-*` index | [ROBOT-MODE.md](references/ROBOT-MODE.md) |
| Human dashboard, palette, keybindings, TUI notes | [DASHBOARD.md](references/DASHBOARD.md) |
| Project resolution, `projects_base`, config paths, project-local assets | [CONFIG.md](references/CONFIG.md) |

### Assets

Drop-in examples live under `assets/`:

- [`pipeline-example.yaml`](https://github.com/boshu2/agentops/blob/main/skills/ntm/assets/pipeline-example.yaml) — a review pipeline with parallel step + retry
- [`policy-example.yaml`](https://github.com/boshu2/agentops/blob/main/skills/ntm/assets/policy-example.yaml) — opinionated `~/.ntm/policy.yaml` starter
- [`envrc.example`](https://github.com/boshu2/agentops/blob/main/skills/ntm/assets/envrc.example) — recommended `direnv`/shell env vars

## Related Skills

- **`agent-native`** — portable factory roles, worker lifecycle, bounded recovery, evidence, and retirement.
- `agent-mail` for inboxes, contact handshakes, and file reservations
- [`scripts/validate.sh`](scripts/validate.sh) for the operator-choice, factory-role, Agent Mail, and pawl mesh contract
- `br` for bead state changes and syncing
- `bv` for graph-aware task prioritization
- `cass` for prior-session retrieval
- `caam` for account rotation across providers (paired with `--robot-switch-account`)
- `dcg`, `slb` for destructive-command and two-person approval policy
