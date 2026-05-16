# AgentOps 3.0 YouTube Starter Series

This is the launch content plan for teaching AgentOps 3.0 through concrete
workflows. The series starts with council because the fastest product proof is
Claude and Codex judging one decision against the same domain/practice packet.

## Series Strategy

Audience: agent-heavy maintainers, technical founders, staff engineers, and
small teams who already use coding agents on real repos.

Promise: show how AgentOps turns isolated agent opinions into shared
engineering verdicts, then into tracked work and optional scheduled
compounding.

Primary CTA: follow [AgentOps 3.0 First-Value Path](first-value-path.md).

Secondary CTA: read the [AgentOps 3.0 Explainer Kit](agentops-3-explainer-kit.md)
and run `ao demo --quick`.

Evidence CTA: capture first-run and interview signals through
[AgentOps 3.0 PMF Evidence Loop](agentops-3-pmf-evidence-loop.md).

## Publishing Cadence

| Week | Asset | Goal |
|---|---|---|
| 1 | Episode 1 plus 2 short clips | Teach the council-first value path. |
| 1 | Episode 2 plus 2 short clips | Show the domain/practice packet as the product object. |
| 2 | Episode 3 plus 2 short clips | Show verdict-to-work with bd and `.agents/`. |
| 2 | Episode 4 plus 2 short clips | Show daemon/Dream as second-stage automation. |
| 3 | Episode 5 plus 2 short clips | Show the full 3.0 launch workflow and invite PMF interviews. |

Do not publish productivity or PMF claims in titles or thumbnails. The series
is a teaching funnel and evidence loop.

## Long-Form Episodes

### 1. AgentOps 3.0: Claude + Codex, One Engineering Verdict

**Thumbnail text:** Claude + Codex -> One Verdict

**Title options**

- AgentOps 3.0: Claude + Codex Reviewing One Decision
- How AgentOps Turns Agent Opinions Into Engineering Verdicts

**Cold open**

> Two agents can disagree. The problem is when they disagree without the same
> domain, evidence, or release bar. AgentOps gives them the same engineering
> operating layer.

**Demo beats**

1. Show `docs/examples/agentops-3-domain-practice-packet.md`.
2. Run `ao context assemble`.
3. Run `/council --mixed`.
4. Open `.agents/council/<run-id>/verdict.md`.
5. Create bd follow-up from the verdict.

**CTA**

Run `ao demo --quick`, then follow `docs/first-value-path.md`.

**Measurement fields**

- Comments asking what council is.
- Installs within 48 hours.
- GitHub stars within 48 hours.
- Reported council runs.
- Requests for setup help.

### 2. The Domain Packet: DDD, TDD, BDD, Review, And Release Rules For Agents

**Thumbnail text:** Give Agents A Domain

**Title options**

- Stop Prompting From Scratch: Give Agents A Domain Packet
- AgentOps Domain Packets: Engineering Discipline For Coding Agents

**Cold open**

> Software teams do not coordinate complex work by vibes. They use domain
> language, tests, review, issues, and release gates. AgentOps packages those
> practices for agents.

**Demo beats**

1. Open the domain/practice packet.
2. Open `docs/cdlc.md` as the foundation text behind the packet.
3. Point at product identity, target user, sources, practice sources, evidence
   rules, and non-goals.
4. Copy it into `.agents/packets/`.
5. Show the matching context briefing.
6. Show where the verdict cites the packet.

**CTA**

Copy the example packet into one repo and edit the target user, decision, and
evidence rules.

**Measurement fields**

- Viewers who can explain the packet in comments or PMF calls.
- Packet copies or adaptations shared back.
- Questions about how to map existing repo docs into packets.

### 3. From Verdict To Work: Making Agent Decisions Survive The Session

**Thumbnail text:** Verdict -> Work

**Title options**

- Agent Decisions Should Not Die In Chat
- Turn A Claude/Codex Verdict Into Tracked Engineering Work

**Cold open**

> The point is not that a council says something smart. The point is that the
> decision leaves a trace and becomes work the next agent can pick up.

**Demo beats**

1. Open the sample verdict.
2. Show PASS/WARN/BLOCK shape.
3. Run `bd create ... --description "From .agents/council/<run-id>/verdict.md"`.
4. Show `.beads/issues.jsonl`.
5. Re-run `ao context assemble` with the new issue as the task.

**CTA**

Create one issue from one verdict and note whether the next session starts
with clearer context.

**Measurement fields**

- Number of viewers who create first bd issue.
- Number of viewers who paste a verdict path into a follow-up issue.
- Questions about beads versus existing issue trackers.

### 4. The Daemon Is The Second Stage: Scheduled Compounding After Trust

**Thumbnail text:** Schedule After Trust

**Title options**

- AgentOps Daemon: The Always-On Lane After Your First Verdict
- Do Not Start With Automation. Start With A Verdict.

**Cold open**

> The daemon is powerful, but it is not the first thing to sell. First prove the
> packet and verdict. Then automate the loops you trust.

**Demo beats**

1. Recap the packet and verdict artifact.
2. Run `ao init --with-schedule`.
3. Run `ao daemon run --schedule-file .agents/schedule.yaml`.
4. Run `ao schedule list`.
5. Explain Dream/wiki/forge as compounding jobs, not unattended source mutation.

**CTA**

Only set up a schedule after you can inspect the artifact the schedule will
produce.

**Measurement fields**

- Daemon setup attempts.
- Schedule list screenshots or reports.
- Questions about safety boundaries and source mutation.

### 5. AgentOps 3.0 Full Path: First Verdict To PMF Evidence

**Thumbnail text:** First Verdict -> PMF Evidence

**Title options**

- The AgentOps 3.0 Launch Workflow
- Building An Engineering OS For Agent Teams

**Cold open**

> A product this dense needs evidence, not just a better tagline. The 3.0 loop
> is content, first verdict, reuse, interview, and claim discipline.

**Demo beats**

1. Show the explainer kit.
2. Show first-value path.
3. Run through the five commands quickly.
4. Show PMF evidence fields.
5. Invite users to run the path and report friction.

**CTA**

Open an issue or comment with: runtime used, setup time, council mode, verdict
path, whether the packet made sense, and what blocked first value.

**Measurement fields**

- PMF interview bookings.
- Activation reports with setup time and verdict path.
- Packet reuse in later work.
- Daemon adoption after first verdict.

## Short Clip Hooks

1. "Two agents arguing is not useful until they share the same domain."
2. "This file is the product: the domain packet your agents judge against."
3. "Claude and Codex can disagree. AgentOps makes the disagreement reviewable."
4. "The first artifact is not a dashboard. It is a verdict file."
5. "Do not start with the daemon. Start with one trusted verdict."
6. "Agent decisions should become issues, not vanish into chat."
7. "DDD, TDD, BDD, review, and release gates are agent context now."
8. "The killer demo is not automation. It is shared engineering judgment."
9. "Your `.agents/` folder is a local wiki for humans and agents."
10. "Bring your agent, bring your harness, keep the operating discipline."

## Recording Checklist

- Clean worktree or clearly labeled demo branch.
- `ao version` visible.
- Claude Code and Codex CLI authenticated if using mixed council.
- Fallback `/council --quick` command ready.
- `docs/examples/agentops-3-domain-practice-packet.md` open in editor.
- `docs/first-value-path.md` open for CTA.
- Terminal font large enough for command readability.
- No secrets, home paths, tokens, private transcripts, or unredacted `.agents/`
  content on screen.
- Demo verdict can be synthetic only if clearly labeled as sample output.
- Claim-safe language copied from the explainer kit.

## CTA Map

| Asset | Primary CTA | Secondary CTA |
|---|---|---|
| Episode 1 | Run `ao demo --quick`. | Follow `docs/first-value-path.md`. |
| Episode 2 | Copy and edit the domain packet. | Read `docs/domain-practice-packets.md`. |
| Episode 3 | Create one bd issue from one verdict. | Read the council verdict example. |
| Episode 4 | Set up a schedule only after first verdict. | Read scheduling docs and examples. |
| Episode 5 | Share a first-value report or book a PMF interview. | Star the repo if the path worked. |
| Short clips | Watch Episode 1 or read the explainer kit. | Run `ao demo --quick`. |

## PMF Evidence Fields

Collect these after content drives a first run:

| Field | Type |
|---|---|
| Viewer source | Episode, clip, README, gist, direct outreach, community post |
| Runtime | Claude Code, Codex CLI, Cursor, OpenCode, mixed |
| Setup time | Minutes to `ao quick-start` complete |
| Packet comprehension | Could the viewer explain the domain/practice packet? |
| Council mode | Mixed, quick fallback, blocked |
| Verdict artifact | Path or redacted exported sample |
| Follow-up work | bd issue id or external issue link |
| Daemon adoption | None, attempted, schedule listed, scheduled run completed |
| Reuse signal | Packet or verdict reused in later engineering decision |
| Friction | Free text |
| Interview status | Requested, scheduled, completed, declined |

## Claim Guardrails

Allowed:

- "engineering operating system for agent teams"
- "disciplined engineering layer for agentic software development"
- "from agent opinions to engineering verdicts"

Blocked without exported evidence:

- Product-market fit.
- Specific speed, quality, or productivity lift.
- Fully autonomous factory as the first-value promise.
- Regulated or certified-operation claims.
