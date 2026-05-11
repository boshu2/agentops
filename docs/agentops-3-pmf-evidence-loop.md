# AgentOps 3.0 PMF Evidence Loop

This loop turns the council-first content funnel into product discovery. It is
not a PMF claim. It is the operating plan for collecting evidence before public
copy says anything about PMF, productivity, speed, or user outcomes.

## Target Segment

Primary segment:

- Agent-heavy maintainers working repeatedly in one real codebase.
- Already using Claude Code, Codex CLI, Cursor, or OpenCode.
- Feeling context debt: repeated investigations, scattered decisions, weak
  review evidence, or cold starts between sessions.
- Comfortable running terminal commands and inspecting local artifacts.

Secondary segment:

- Technical founders or staff engineers evaluating agent workflows for a small
  engineering team.
- Open-source maintainers who need agent work to remain reviewable across PRs.

Exclude from this loop:

- People who have never used coding agents.
- Buyers primarily looking for hosted enterprise orchestration.
- Users who need compliance claims before a redaction/export path exists.

## Outreach List

Start with a small, high-signal list. Track each contact in a private sheet or
local notes until explicit permission exists to quote or publish.

| Channel | Who to recruit | Ask |
|---|---|---|
| YouTube comments | Viewers asking how council, packets, or daemon scheduling work | "Want to try the first-value path on one repo and tell me where it breaks?" |
| GitHub issues/discussions | Users who star, open install questions, or ask about Claude/Codex workflows | "Can we observe your first council verdict and capture setup friction?" |
| Discord/Slack communities | Engineers already showing coding-agent work in public | "Would a shared domain packet help your agent review workflow?" |
| Direct outreach | Maintainers known to use agents on active OSS or internal repos | "I am testing a council-first workflow for agent teams; can you run a 20-minute path?" |
| Existing users | People who have run AgentOps, Dream, daemon, or `/rpi` before | "Does the new packet/council path explain the product faster?" |

## First-Run Scenario

Ask each participant to follow [AgentOps 3.0 First-Value Path](first-value-path.md)
on a real repo or a public clone they are comfortable using.

Minimum scenario:

1. Install or update AgentOps.
2. Run `ao quick-start`.
3. Copy or edit a domain/practice packet.
4. Assemble context.
5. Run `/council --mixed` or documented fallback.
6. Inspect `.agents/council/<run-id>/verdict.md`.
7. Create tracked follow-up work from the verdict.

Second-stage scenario:

1. Reuse the same packet or verdict in a later engineering decision.
2. Try `ao init --with-schedule`, `ao daemon run`, and `ao schedule list`.
3. Report whether scheduled compounding felt useful or premature.

## Interview Script

Use this after the run. Do not lead the user toward a positive answer.

1. What were you trying to get your agent team to decide or validate?
2. Before AgentOps, where would that context and decision have lived?
3. Could you explain the domain/practice packet in your own words?
4. Which packet section was most useful or confusing?
5. Did council feel different from asking multiple chats separately?
6. Did the verdict artifact change what you trusted, changed, or tracked?
7. Did creating follow-up work from the verdict feel natural?
8. What blocked or slowed first value?
9. Would you reuse the packet or verdict in a later decision?
10. When, if ever, would you trust the daemon or a schedule?
11. What would make you recommend this to another maintainer?
12. What claim would feel false or overreaching if we put it on the homepage?

## Metrics

### Activation Metric

Activation is not install. Activation is:

```text
Participant produced or inspected .agents/council/<run-id>/verdict.md from a visible domain/practice packet.
```

Record:

- Runtime used.
- Setup time in minutes.
- Council mode: mixed, quick fallback, blocked.
- Verdict status: PASS, WARN, BLOCK, or unclear.
- First artifact path or redacted export path.

### Council/Domain Artifact-Reuse Metric

Reuse signal:

```text
Participant reused the same packet or verdict in a later decision, PR review, issue, or planning session.
```

Record:

- Reuse date.
- Reuse context.
- Whether the packet changed.
- Whether the verdict created follow-up work.
- Whether a later agent cited the packet or verdict.

### Daemon Adoption Metric

Daemon adoption is second-stage:

```text
Participant intentionally configured a schedule after first verdict trust existed.
```

Record:

- No daemon interest.
- Interested but blocked.
- Ran `ao init --with-schedule`.
- Ran `ao daemon run`.
- `ao schedule list` showed a schedule.
- Scheduled run completed and produced an inspected artifact.

Do not count daemon setup as first-value activation.

## Evidence Record Template

Exportable evidence belongs under:

```text
docs/releases/agentops-3-pmf-evidence/
```

Each participant record should be private by default. Export only redacted,
permissioned summaries using
[AgentOps 3.0 PMF Evidence Record Template](releases/agentops-3-pmf-evidence/record-template.md).

Required fields:

- Participant alias.
- Permission status.
- Source channel.
- Runtime.
- Repo type: public clone, OSS repo, private repo, synthetic demo.
- Setup time.
- Whether `PRACTICE.md` made the packet's engineering doctrine clearer.
- Packet path or redacted packet excerpt.
- Council mode.
- Verdict path or redacted verdict excerpt.
- Follow-up work created.
- Reuse signal.
- Daemon adoption signal.
- Friction notes.
- Quote, only with explicit permission.
- Claim posture: none, private learning only, public anonymized evidence, or
  public quoted evidence.

## Claim-Ledger Posture

Before public PMF claims:

1. Keep PMF, productivity, and speed claims blocked.
2. Allow product-shape claims such as "engineering operating system for agent
   teams" and "from agent opinions to engineering verdicts."
3. Export redacted evidence under `docs/releases/agentops-3-pmf-evidence/`.
4. Add or update claim markers only after exported evidence exists.
5. Run:

```bash
bash scripts/check-factory-claim-ledger.sh --strict --no-fixtures
```

Promotion from "learning signal" to "public claim support" requires:

- At least five completed first-value runs from target-segment users.
- At least three successful council verdict artifacts.
- At least two reuse signals on a later decision.
- At least one daemon adoption signal, if daemon claims are used.
- No unresolved claim-ledger failure for the public copy.

## Operating Cadence

| Cadence | Action |
|---|---|
| Daily during launch week | Review comments, issues, stars, install questions, and first-value reports. |
| Twice weekly | Conduct 2-3 PMF interviews or observed first runs. |
| Weekly | Update evidence summaries, friction list, and blocked claims. |
| Before launch copy changes | Re-run claim-ledger gate and cite exported evidence paths. |

## Follow-Up Outputs

The loop should produce:

- Redacted evidence records in `docs/releases/agentops-3-pmf-evidence/`.
- Follow-up bd issues for repeated setup friction.
- Copy changes only when claim posture allows them.
- Product changes when repeated users fail before first verdict.
