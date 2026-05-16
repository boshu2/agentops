# AgentOps 3.0 Explainer Kit

This is the public text kit for explaining AgentOps 3.0 in a gist, README
section, launch post, or video description.

## One-Sentence Positioning

AgentOps is the engineering operating system for agent teams: a disciplined
engineering layer that gives coding agents shared domain context, review
verdicts, tracked follow-up work, and optional scheduled compounding.

## Problem Statement

Coding agents are useful, but most teams still operate them like isolated chat
sessions. A model can make a reasonable product, design, or engineering call in
one window, while the next run loses the domain language, prior concerns,
review evidence, and follow-up decisions.

Human engineering teams solved this class of problem with shared domain models,
specs, tests, code review, issue trackers, release gates, runbooks, and wikis.
AgentOps encodes that operating discipline for agents.

The packet is the linked intent object. It is the thing that moves through the
AgentOps lifecycle: product/domain intent becomes a context briefing, then a
council verdict, then tracked work, then an execution packet, validation
evidence, and a handoff. Provenance and trace make that movement inspectable.

Canonical doctrine for that discipline lives in `docs/cdlc.md`. `PRACTICE-REGISTRY.md`
backs the packet with practice lineage and stable `practices: [slug]`
citations.

## Target User

AgentOps 3.0 is for agent-heavy maintainers, staff engineers, technical
founders, and small teams who already use Claude Code, Codex CLI, Cursor, or
OpenCode on real repositories and want repeated agent work to become more
coherent instead of resetting every session.

They do not need a hosted control plane to see value. The first proof is one
decision becoming a shared packet, a council verdict, and tracked work.

## The 5-Command Path

After installing AgentOps and restarting the agent runtime:

```bash
ao quick-start
cp docs/examples/agentops-3-domain-practice-packet.md .agents/packets/agentops-3-launch.md
ao context assemble --phase planning --task "Evaluate the AgentOps 3.0 launch demo against the domain/practice packet" --output-file .agents/rpi/briefing-current.md
```

```text
/council --mixed validate "Given .agents/packets/agentops-3-launch.md, should the AgentOps 3.0 launch demo lead with council-first engineering judgment?"
```

```bash
bd create "Apply council verdict to launch demo" --description "From .agents/council/<run-id>/verdict.md" --json
```

Expected artifacts:

- `.agents/packets/agentops-3-launch.md`
- `.agents/rpi/briefing-current.md`
- `.agents/council/<run-id>/verdict.md`
- `.beads/issues.jsonl`

For the full first-session path with time budget and fallbacks, see
[AgentOps 3.0 First-Value Path](first-value-path.md).

For launch video outlines, clip hooks, CTAs, and measurement fields, see
[AgentOps 3.0 YouTube Starter Series](agentops-3-youtube-starter-series.md).
For observed first-run evidence and claim promotion rules, see
[AgentOps 3.0 PMF Evidence Loop](agentops-3-pmf-evidence-loop.md).

## Domain Packet Example

Use [AgentOps 3.0 Domain/Practice Packet](examples/agentops-3-domain-practice-packet.md)
as the launch example.

The packet makes these things visible before the agents judge:

- Product identity and target user.
- The decision under review.
- Product, goal, issue, standards, and evidence sources.
- The practice lineage and citations from `PRACTICE-REGISTRY.md`.
- Engineering practices to enforce, including DDD/TDD/BDD/review/release
  discipline where relevant.
- Non-goals and claims that require external evidence before public use.

## Council Verdict Example

Use [AgentOps 3.0 Council Verdict Example](examples/agentops-3-council-verdict-example.md)
as public sample output.

The important shape:

- Every judge sees the same domain/practice packet.
- Claude and Codex can disagree, but they are disagreeing inside the same
  engineering frame.
- The consolidated verdict is PASS/WARN/BLOCK, not loose advice.
- The verdict creates follow-up work or a launch decision.

## Why This Is Not Just Multi-Chat Prompting

| Ad hoc multi-chat | AgentOps 3.0 path |
|---|---|
| Paste context into several chats manually. | Put the domain/practice packet in a reviewable artifact. |
| Ask each model for an opinion. | Ask judges for a verdict against the same evidence bar. |
| Copy useful parts back by hand. | Record `.agents/council/<run-id>/verdict.md` and create bd follow-up work. |
| Lose the reasoning after the session. | Keep local packets, briefings, verdicts, issues, and learnings inspectable. |
| Re-explain intent at every phase. | Hand the packet lineage through briefing, verdict, execution, validation, and handoff artifacts. |
| Automation starts as a giant promise. | Automation is second-stage, after the packet and verdict earn trust. |

## Daemon Expansion Path

The daemon is the deeper lane, not the first proof.

After the user sees one packet and one verdict:

```bash
ao init --with-schedule
ao daemon run --schedule-file .agents/schedule.yaml
ao schedule list
```

Use the daemon for approved recurring work such as Dream reports, wiki
curation, release checks, or other compounding jobs where the operator has
already accepted the artifact shape.

## Evidence-Gated Claims

Use these claims now:

- AgentOps is the engineering operating system for agent teams.
- AgentOps is a disciplined engineering layer for agentic software development.
- AgentOps turns agent opinions into engineering verdicts.
- AgentOps keeps agent work local, inspectable, and repo-native unless the
  operator chooses external services.

Do not claim these without exported evidence:

- Product-market fit.
- Specific productivity or speedup numbers.
- Safety-critical, regulated, or certified operation.
- Fully autonomous source mutation as the first-value promise.
- A public customer outcome that only exists in local `.agents/` notes.

## Launch CTA

Use one CTA at the end of public content:

```text
Install AgentOps, run ao demo --quick, then follow docs/first-value-path.md to get your first council verdict.
```

The goal is not to explain every skill. The goal is to get a maintainer to see
one shared engineering domain become one verdict artifact they can inspect.
