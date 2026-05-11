# AgentOps 3.0 Council Demo Storyboard

This storyboard is the canonical 3.0 launch demo. It is designed to become a
video script, docs quickstart, and PMF scenario seed.

## Demo Promise

AgentOps makes agents work like a disciplined engineering team.

In the demo, the operator gives Claude and Codex the same domain/practice
packet. They judge the same product/design/engineering decision, produce one
verdict artifact, and turn that verdict into tracked work. The daemon appears
only after first trust, as the second-stage compounding lane.

## Repo, Task, And Setup

| Field | Value |
|---|---|
| Demo repo | `boshu2/agentops` |
| Demo branch | Any clean 3.0 worktree rebased on current `main` |
| Demo profile | `product-council` from `docs/activation-profiles.md` |
| Domain packet | `docs/examples/agentops-3-domain-practice-packet.md` |
| Decision | Should the 3.0 launch demo lead with council-first engineering judgment instead of daemon-first automation? |
| Target issue | `soc-m6v5.9.7.8` after packet/profile/storyboard work closes |

Pre-flight:

```bash
ao version
bd show soc-m6v5.9.7
bd ready --parent soc-m6v5.9.7
```

Screen setup:

- Terminal pane 1: commands.
- Terminal pane 2: packet and verdict artifacts.
- Browser or editor: `PRODUCT.md`, this storyboard, and the domain/practice
  packet.

## Before State

Show the problem without AgentOps:

1. One agent says "lead with daemon because automation is differentiated."
2. Another agent says "lead with council because it is easier to understand."
3. Neither answer cites product posture, goals, evidence rules, or release
   constraints.
4. No verdict artifact survives for the next session.

Do not dramatize this as agent failure. The point is that the agents are missing
the shared engineering domain.

## Act 1: Show The Shared Domain

Open:

```bash
sed -n '1,140p' docs/examples/agentops-3-domain-practice-packet.md
```

Narration:

> This is the domain the agents operate inside: product identity, goals,
> standards, issues, evidence rules, and non-goals. AgentOps makes the context
> explicit before the models argue.

Show these packet sections:

- Decision or task.
- Target user.
- Domain sources.
- Practice sources.
- Evidence rules.
- Non-goals.

## Act 2: Assemble Runtime Context

Run:

```bash
ao context packet --goal "AgentOps 3.0 council-first launch demo"
```

Then:

```bash
ao context assemble \
  --phase planning \
  --task "Evaluate the AgentOps 3.0 launch demo against the domain/practice packet" \
  --output-file .agents/rpi/briefing-current.md
```

Show:

```bash
sed -n '1,120p' .agents/rpi/briefing-current.md
```

Narration:

> The packet is the product layer. The briefing is the runtime layer. AgentOps
> separates the operating domain from the per-phase context that enters the
> model window.

## Act 3: Run Cross-Model Council

Primary command:

```text
/council --mixed validate "Given docs/examples/agentops-3-domain-practice-packet.md, should the AgentOps 3.0 launch demo lead with council-first engineering judgment instead of daemon-first automation?"
```

Fallback command:

```text
/council --quick validate "Given docs/examples/agentops-3-domain-practice-packet.md, should the AgentOps 3.0 launch demo lead with council-first engineering judgment instead of daemon-first automation?"
```

The fallback is acceptable for docs or offline recording only. A public
cross-model demo should record whether Claude and Codex both completed.

## Expected Verdict Shape

The demo should not require a predetermined PASS. The expected shape is:

| Judge | Expected assessment |
|---|---|
| Claude product/design judge | Council-first is clearer first value because the user sees taste and judgment immediately. |
| Claude engineering-practice judge | The packet encodes DDD/TDD/BDD/release discipline into the decision context. |
| Codex implementation judge | The demo is feasible with existing `ao context`, `/council`, bd, and `.agents` surfaces. |
| Codex release-risk judge | WARN unless public copy avoids PMF/productivity claims without exported evidence. |
| Consolidated verdict | PASS or WARN: lead with council-first, keep daemon as second-stage, and gate launch claims on exported evidence. |

Expected artifact:

```text
.agents/council/<run-id>/verdict.md
```

Public sample shape:
[AgentOps 3.0 Council Verdict Example](agentops-3-council-verdict-example.md).

If the verdict is WARN, that is still a good demo. It proves AgentOps is doing
engineering judgment, not just producing agreement.

## Act 4: Turn Verdict Into Work

Show the existing tracked work:

```bash
bd show soc-m6v5.9.7.8
```

Then show how the verdict becomes implementation context:

```bash
ao context assemble \
  --phase planning \
  --task "Rebuild ao demo around the council-first engineering OS golden path" \
  --output-file .agents/rpi/briefing-current.md
```

Narration:

> The verdict does not disappear into chat. It becomes a durable artifact and
> feeds the next planning or implementation step.

## Act 5: Show The Deeper Automation Lane

Only after the verdict:

```bash
ao init --with-schedule
ao daemon run --schedule-file .agents/schedule.yaml
ao schedule list
```

Narration:

> Once you trust the packet, verdict, and evidence trail, the same operating
> layer can run on a schedule. That is the factory lane, not the first thing you
> need to understand.

## Engineering Practice On Screen

The demo should explicitly show these practices encoded into the packet:

- DDD: the domain is named before the agents judge.
- TDD/BDD: tests or scenarios define expected behavior.
- XP/review: independent judges review before implementation.
- SRE/release: claims and release gates define what cannot ship.
- Wiki/agile lineage: `.agents` becomes the local engineering wiki for agents
  and humans.

## Claim-Safe Boundaries

Use:

- "engineering operating system for agent teams"
- "disciplined engineering layer for agentic software development"
- "from agent opinions to engineering verdicts"
- "bring your agent, bring your harness"

Do not use:

- old reliability-framed agent copy
- "superpowers"
- "fully autonomous factory" as the first-value promise
- PMF, productivity, or speedup claims without exported evidence

## Recording Outline

| Time | Beat |
|---:|---|
| 0:00 | State the promise: agents need a shared engineering domain. |
| 0:30 | Show the before state: opinions without shared context. |
| 1:15 | Open the domain/practice packet. |
| 2:15 | Assemble runtime context with `ao context packet` and `ao context assemble`. |
| 3:15 | Run `/council --mixed`. |
| 5:00 | Inspect the verdict artifact. |
| 6:00 | Show the verdict becoming bd work. |
| 7:00 | Show schedule/Dream as second-stage automation. |
| 8:00 | Close with the install or gist CTA. |

## PMF Evidence Fields

When this storyboard becomes a PMF scenario, record:

- Runtime and models used.
- Setup time.
- Commands run.
- Whether mixed council completed.
- Verdict path.
- Whether the viewer/operator could explain the packet.
- Whether the operator reused the verdict or packet later.
- Any friction before first verdict.

## Done Criteria

This storyboard is ready for the `ao demo` rebuild when:

- The packet path is stable.
- `product-council` is the named first-value profile.
- The expected verdict shape is accepted.
- The daemon lane is second-stage.
- Claim boundaries are copied into README/docs/demo work.
