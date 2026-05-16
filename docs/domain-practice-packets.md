# Domain and Practice Packets

Domain/practice packets are the product-facing contract for the engineering
context AgentOps gives to agent teams before they make a decision, review a
plan, or change code.

They answer one question:

> What shared engineering domain should every agent judge this work against?

Put differently: the packet is AgentOps' linked intent object. It carries the
operator's intent, constraints, evidence rules, and provenance anchors from one
phase to the next, so discovery, planning, implementation, validation, and
release are judging the same work instead of re-inferring it from chat history.

This is the visible layer behind the 3.0 product promise. AgentOps is not just
multi-agent chat or a memory folder. It gives agents the same operating
materials human engineering teams use on complex codebases: product intent,
goals, standards, issues, tests, scenarios, review rules, verdict artifacts,
and release discipline.

## Contract

3.0 ships this as a docs-first product contract. Do not add a new top-level
`ao packet` or `ao activate` command until the council-first demo proves the
language and workflow. Use `ao context assemble`, `ao context packet`, and
existing skill invocations as the runtime path for now.

A domain/practice packet is a human-readable packet with these sections:

| Section | Required | Purpose |
|---|---|---|
| Decision or task | Yes | The specific decision, plan, PR, or task the agents are judging. |
| Target user | Yes | The persona or operator whose outcome matters. |
| Domain sources | Yes | Product, goals, repo instructions, issues, constraints, and prior decisions. |
| Practice sources | Yes | The engineering practices to enforce: tests, standards, skills, gates, BDD/Gherkin scenarios, release rules, or SRE posture. |
| Intent lineage | Yes | Packet/run IDs, phase handoff artifacts, and provenance or trace anchors that show how intent moves across phases. |
| Evidence rules | Yes | What proof counts, where verdicts should land, and what claims remain blocked. |
| Runtime path | Yes | The commands or skill invocations that consume the packet. |
| Outputs | Yes | Expected artifacts: council verdicts, context briefings, validation reports, handoffs, or release evidence. |
| Non-goals | Yes | What the agents must not optimize for during this run. |

The packet can be tracked documentation for a public demo, or a local runtime
artifact under `.agents/` for private work. Public launch examples belong in
`docs/`; local session packets belong in `.agents/`.

## Relationship To Existing AgentOps Surfaces

Domain/practice packets do not replace context packets. They sit one level
above them.

| Surface | Role |
|---|---|
| `PRODUCT.md` | Product intent, personas, value proposition, known gaps, and claim posture. |
| `GOALS.md` | Fitness spec and directives the repo must preserve. |
| `PRACTICE-REGISTRY.md` | Practice lineage and canonical `practices: [slug]` registry. |
| `AGENTS.md` or runtime instructions | Local operating rules for agents in this repo. |
| `docs/standards/` and `skills/standards/` | Coding and review conventions. |
| `bd` issues | Current work, dependencies, acceptance criteria, and ownership. |
| `ao context assemble` | Builds a phase-scoped briefing from goals, history, intel, task, and protocol. |
| `ao context packet` | Shows ranked findings, planning rules, pre-mortem checks, and next-work context. |
| `/council` | Turns the packet into a shared evidence frame for independent judges. |
| `/provenance` and `/trace` | Reconstruct where packet claims came from and how they moved through later artifacts. |
| RPI execution packet | Carries the accepted objective, plan path, contract surfaces, validation lanes, and done criteria across discovery, implementation, and validation. |
| `.agents/` | Local engineering wiki: traces, handoffs, verdicts, learnings, and run packets. |

The domain/practice packet tells the operator and the agents what should be in
scope. `ao context assemble` and `ao context packet` provide the runtime
machinery that prepares phase-specific context from that scope.

## Minimal Launch Path

For the 3.0 council-first demo, use this shape:

```bash
ao quick-start
ao context packet --goal "AgentOps 3.0 council-first launch demo"
ao context assemble \
  --phase planning \
  --task "Evaluate the AgentOps 3.0 launch demo against the domain/practice packet" \
  --output-file .agents/rpi/briefing-current.md
```

Then run council inside the packet:

```text
/council --mixed validate "Given docs/examples/agentops-3-domain-practice-packet.md, should the 3.0 launch demo lead with council-first engineering judgment?"
```

The expected result is not just a PASS/WARN/FAIL. The expected result is an
auditable verdict that shows each judge used the same product domain, practice
rules, and evidence bar.

## Validation Checklist

Before a packet is used in a demo, PMF scenario, or release claim, verify:

- The target user is named.
- The decision or task is specific enough for a judge to accept or reject.
- Product and goal sources are linked.
- Foundation practice sources are linked when the packet operates inside this
  repo.
- At least one standards or practice source is linked.
- Intent lineage links the packet to the handoff, execution packet, verdict, or
  trace artifact that carries the decision forward.
- Work context includes a bd issue, plan path, PR, or explicit task.
- Evidence rules say where artifacts land.
- Claims that require exported evidence are marked blocked until that evidence
  exists.
- The optional schedule or daemon lane is second-stage unless the demo is
  specifically about automation.

## Example

See [AgentOps 3.0 Domain/Practice Packet](examples/agentops-3-domain-practice-packet.md).
The matching launch storyboard is
[AgentOps 3.0 Council Demo Storyboard](examples/agentops-3-council-demo-storyboard.md).
The sample verdict artifact is
[AgentOps 3.0 Council Verdict Example](examples/agentops-3-council-verdict-example.md).
The viewer-facing first session is
[AgentOps 3.0 First-Value Path](first-value-path.md).
