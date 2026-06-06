<!-- generated from skills/*/SKILL.md frontmatter -->

# AgentOps Context Map

Generated from SKILL.md frontmatter. See [ADR-0001](https://github.com/boshu2/agentops/blob/main/docs/adr/ADR-0001-ddd-hexagonal-adoption.md)
and [CDLC](https://github.com/boshu2/agentops/blob/main/docs/cdlc.md) for the architectural rationale.

## Skills by hexagonal role

### domain

- `brainstorm` — Separate goals from implementation.
- `bug-hunt` — Investigate bugs and root causes.
- `burndown` — Drive a finite epic set to all-merged, then stop.
- `complexity` — Find focused refactor hotspots.
- `council` — Run multi-judge consensus. Use when: an irreversible or high-stakes decision needs independent judges before committing — architecture forks, one-way doors, scoring options.
- `crank` — Execute epics through waves.
- `design` — Validate product fit before discovery. Use when: framing a problem, checking product/market fit, or pressure-testing user value before writing a discovery packet or any code.
- `discovery` — Create dense execution packets.
- `domain` — Canonical vocabulary for human-AI software work.
- `flywheel` — Check knowledge flywheel health.
- `forge` — Mine transcripts into learnings.
- `goals` — Maintain AgentOps goals.
- `hooks-authoring` — Author AgentOps runtime hooks.
- `perf` — Profile and optimize hotspots.
- `plan` — Decompose goals into issue plans.
- `post-mortem` — Review completed work and learn. Use when: a task, PR arc, or session is finished and you want to extract learnings, or after ≥5 PRs (the scope checkpoint).
- `pre-mortem` — Stress-test plans before work. Use when: a plan is drafted but not yet executed and you want to surface failure modes, risks, and what would prove it wrong before committing.
- `product` — Create or refine PRODUCT.md.
- `ratchet` — Record Brownian Ratchet gates.
- `retro` — Capture a session learning.
- `shared` — Shared AgentOps skill contracts.
- `standards` — Provide repo coding standards.
- `validation` — Run post-implementation validation.
- `vibe` — Validate code readiness. Use when: doing a quick readiness or sanity check that code is ready to commit or ship, short of a full review.

### driving-adapter

- `bd-first-memory-migration` — Consolidate fragmented agent-memory layers into one bd-canonical store, then GC/retire the rest. Triggers: "memory migration", "consolidate agent memory", "beads-first memory".
- `bootstrap` — Initialize AgentOps project files.
- `implement` — Implement one tracked issue.
- `inject` — Load relevant .agents context.
- `operating-loop-workflow` — Install and run the operating-loop multi-agent Workflow (the seven-move loop) for AgentOps plugin users.
- `pr-implement` — Implement a scoped OSS PR.
- `pr-prep` — Prepare PR commits and body.
- `pr-validate` — Validate PR scope and quality.
- `push` — Validate, commit, and push.
- `quickstart` — Show AgentOps next action.
- `recover` — Recover session context.
- `research` — Explore and write findings.
- `review` — Review diffs for risk, find mocks, scan for bugs, audit codebases. Use when: reviewing a diff/PR for bugs and risk, hunting mocks/stubs/placeholders, or auditing for quality.
- `session-bootstrap` — Universal init prompt — every agent spawned into an AgentOps repo runs `ao session bootstrap` first.
- `ship-loop` — Bot-paired fast-lane cycle for coherent-arc internal PRs (one closable bead or small-epic slice): claim → test → impl → pre-push → push → squash auto-merge → close.
- `status` — Show AgentOps work status.
- `validate` — Produce PASS/WARN/FAIL verdicts for artifacts, plans, code, PRs, or gates. Use when: you need a structured verdict on an artifact, plan, code, PR, or CI gate before proceeding.

### driven-adapter

- `beads` — Track issues with bd/br, triage with bv, and convert plans to beads.
- `deps` — Audit dependency risks and updates.
- `grafana-platform-dashboard` — Validate OpenShift Grafana dashboards.
- `openai-docs` — Use official OpenAI docs.
- `pr-research` — Research an upstream repo.
- `provenance` — Trace artifact provenance.
- `scope` — Hard-block edits outside declared frozen directories via PreToolUse hook.
- `security` — Run repository security scans.
- `security-suite` — Run composable security analysis.

### supporting

- `agent-native` — Make an out-of-session Claude (Managed Agent or Agent SDK loop) AgentOps-native — via skills + the ao CLI + CI, not hooks.
- `autodev` — Manage the PROGRAM.md/AUTODEV.md contract that drives the loop — the config layer Evolve and Factory read each tick, not a loop itself.
- `automation-shape-routing` — Front door for agent automation — decide the SHAPE (Workflow vs NTM vs skill), then hand off. Triggers: "build automation", "convert skills to workflows", "which shape".
- `codex-team` — Coordinate multiple Codex agents.
- `compile` — Compile .agents knowledge wiki.
- `curate` — Mine transcripts, .agents, bd, and git for skill diffs, bd updates, or rare wiki entries.
- `doc` — Generate and validate repo docs (default), READMEs (--mode=readme), and OSS doc packs (--mode=oss).
- `dream` — Retired pointer — out-of-session compounding moved to the substrate (NTM + MCP + managed-agents).
- `eval-outcomes` — Grade against Outcomes as a holdout-safe projection of the locked eval substrate — one bar, many runtimes.
- `evolve` — Run autonomous improvement loops.
- `handoff` — Write compact session handoffs.
- `harvest` — Promote .agents knowledge.
- `heal-skill` — Repair skill hygiene.
- `knowledge-activation` — Activate mature .agents knowledge.
- `llm-wiki` — Build external-knowledge wikis.
- `red-team` — Probe docs and skills. Use when: adversarially probing a doc, skill, plan, or claim for weaknesses, gaps, or unstated assumptions before it ships.
- `refactor` — Execute safe refactors.
- `release` — Run release validation.
- `reverse-engineer-rpi` — Reverse-engineer product specs.
- `rpi` — Run discovery, crank, validation.
- `scaffold` — Create project, component, or boilerplate scaffolds.
- `scenario` — Manage holdout scenarios.
- `skill-auditor` — Audit an existing SKILL.md against the unified AgentOps template (15 checks). Triggers: "audit skill", "skill quality review", "is this skill ready".
- `skill-builder` — Scaffold or absorb new SKILL.md files against the unified AgentOps template. Triggers: "create a skill", "scaffold skill", "absorb external skill", "new skill".
- `swarm` — Dispatch parallel agents.
- `system-tuning` — Restore system responsiveness via safe, ordered process cleanup and agent-swarm hygiene.
- `test` — Generate tests and coverage plans.
- `trace` — Trace decisions through artifacts.
- `using-ntm` — Use NTM as the out-of-session substrate: spawn Claude/Codex panes running /rpi and /evolve over a bead queue, then tend the swarm to convergence.
- `workflow-builder` — Scaffold a new Claude Workflow script — deterministic multi-agent orchestration. Triggers: "build a workflow", "create a workflow", "scaffold workflow", "author a workflow".

### generic

- `converter` — Convert AgentOps skill formats.
- `using-agentops` — Explain AgentOps workflows.

### unclassified

- (no unclassified skills)

## Context relationships

```mermaid
graph LR
  automation-shape-routing -- "supplier-to" --> skill-builder
  automation-shape-routing -- "supplier-to" --> workflow-builder
  beads -- "supplier-to" --> crank
  beads -- "supplier-to" --> ratchet
  brainstorm -- "shared-kernel" --> standards
  bug-hunt -- "shared-kernel" --> standards
  burndown -- "shared-kernel" --> standards
  complexity -- "shared-kernel" --> standards
  council -- "shared-kernel" --> standards
  crank -- "shared-kernel" --> standards
  deps -- "supplier-to" --> vibe
  design -- "shared-kernel" --> standards
  discovery -- "shared-kernel" --> standards
  evolve -- "customer-of" --> rpi
  flywheel -- "shared-kernel" --> standards
  forge -- "shared-kernel" --> standards
  goals -- "shared-kernel" --> standards
  heal-skill -- "customer-of" --> skill-auditor
  hooks-authoring -- "shared-kernel" --> standards
  implement -- "customer-of" --> domain
  perf -- "shared-kernel" --> standards
  plan -- "shared-kernel" --> standards
  post-mortem -- "shared-kernel" --> standards
  pr-implement -- "customer-of" --> crank
  pr-prep -- "customer-of" --> domain
  pr-validate -- "customer-of" --> validation
  pre-mortem -- "shared-kernel" --> standards
  product -- "shared-kernel" --> standards
  provenance -- "supplier-to" --> trace
  quickstart -- "customer-of" --> rpi
  ratchet -- "shared-kernel" --> standards
  red-team -- "supplier-to" --> vibe
  release -- "supplier-to" --> ship-loop
  retro -- "shared-kernel" --> standards
  review -- "customer-of" --> validation
  rpi -- "customer-of" --> crank
  rpi -- "customer-of" --> discovery
  rpi -- "customer-of" --> validation
  scenario -- "supplier-to" --> validation
  scope -- "supplier-to" --> domain
  security -- "supplier-to" --> vibe
  security-suite -- "supplier-to" --> vibe
  session-bootstrap -- "customer-of" --> AGENTS-CI.md
  session-bootstrap -- "customer-of" --> AGENTS-CODEX.md
  session-bootstrap -- "customer-of" --> AGENTS-RUNTIME.md
  session-bootstrap -- "customer-of" --> AGENTS-WORKFLOW.md
  session-bootstrap -- "customer-of" --> AGENTS.md
  ship-loop -- "customer-of" --> post-mortem
  ship-loop -- "customer-of" --> rpi
  skill-auditor -- "supplier-to" --> heal-skill
  skill-auditor -- "customer-of" --> skill-builder
  skill-builder -- "customer-of" --> automation-shape-routing
  skill-builder -- "supplier-to" --> skill-auditor
  swarm -- "customer-of" --> crank
  trace -- "customer-of" --> provenance
  using-ntm -- "customer-of" --> swarm
  validate -- "customer-of" --> validation
  validation -- "shared-kernel" --> standards
  vibe -- "shared-kernel" --> standards
  workflow-builder -- "customer-of" --> automation-shape-routing
  workflow-builder -- "shared-kernel" --> operating-loop-workflow
```

## Data flow (consumes / produces)

| Skill | Direction | Artifact |
|-------|-----------|----------|
| `agent-native` | consumes | converter |
| `agent-native` | consumes | standards |
| `agent-native` | consumes | validation |
| `agent-native` | produces | docs/contracts/agent-runtime-profile.md |
| `autodev` | consumes | evolve |
| `autodev` | consumes | rpi |
| `bd-first-memory-migration` | consumes | repo-context |
| `bd-first-memory-migration` | produces | bd-memories |
| `bd-first-memory-migration` | produces | migration-report |
| `beads` | consumes | bd-issue |
| `beads` | produces | bd-issue |
| `bootstrap` | consumes | doc |
| `bootstrap` | consumes | goals |
| `bootstrap` | consumes | product |
| `bootstrap` | consumes | shared |
| `brainstorm` | consumes | standards |
| `brainstorm` | produces | result.json |
| `brainstorm` | produces | verdict.json |
| `bug-hunt` | consumes | beads |
| `bug-hunt` | consumes | standards |
| `burndown` | consumes | beads |
| `burndown` | consumes | implement |
| `burndown` | consumes | post-mortem |
| `burndown` | consumes | rpi |
| `burndown` | produces | .agents/burndown/*.json |
| `burndown` | produces | git-changes |
| `codex-team` | produces | .agents/swarm/results/*.json |
| `compile` | produces | .agents/compiled/lint-report.md |
| `complexity` | consumes | doc |
| `complexity` | consumes | standards |
| `complexity` | produces | stdout |
| `converter` | produces | converted-skill |
| `council` | consumes | standards |
| `council` | produces | result.json |
| `council` | produces | verdict.json |
| `crank` | consumes | beads |
| `crank` | consumes | implement |
| `crank` | consumes | post-mortem |
| `crank` | consumes | swarm |
| `crank` | consumes | vibe |
| `crank` | produces | .agents/swarm/results/*.json |
| `crank` | produces | git-changes |
| `curate` | produces | .agents/research/*.md |
| `deps` | consumes | repo-context |
| `deps` | produces | result.json |
| `design` | consumes | standards |
| `design` | produces | result.json |
| `discovery` | consumes | brainstorm |
| `discovery` | consumes | design |
| `discovery` | consumes | plan |
| `discovery` | consumes | pre-mortem |
| `discovery` | consumes | research |
| `discovery` | consumes | shared |
| `discovery` | produces | .agents/plans/*.md |
| `discovery` | produces | bd-issue |
| `discovery` | produces | execution-packet.json |
| `doc` | consumes | repo-context |
| `doc` | produces | documentation |
| `domain` | produces | stdout |
| `dream` | produces | .agents/research/*.md |
| `eval-outcomes` | consumes | council |
| `eval-outcomes` | consumes | ratchet |
| `eval-outcomes` | consumes | validation |
| `eval-outcomes` | produces | skills/council/schemas/verdict.json |
| `evolve` | consumes | compile |
| `evolve` | consumes | goals |
| `evolve` | consumes | post-mortem |
| `evolve` | consumes | rpi |
| `evolve` | produces | git-changes |
| `evolve` | produces | goals-fitness-delta |
| `flywheel` | produces | .agents/learnings/*.md |
| `forge` | produces | .agents/research/*.md |
| `goals` | produces | result.json |
| `grafana-platform-dashboard` | produces | dashboard-validation-report |
| `handoff` | produces | .agents/research/*.md |
| `harvest` | produces | .agents/research/*.md |
| `hooks-authoring` | produces | result.json |
| `implement` | consumes | domain |
| `implement` | produces | git-changes |
| `llm-wiki` | produces | documentation |
| `openai-docs` | consumes | external-api |
| `operating-loop-workflow` | produces | git-changes |
| `perf` | consumes | repo-context |
| `perf` | produces | result.json |
| `plan` | consumes | standards |
| `plan` | produces | .agents/plans/*.md |
| `plan` | produces | execution-packet.json |
| `post-mortem` | consumes | council |
| `post-mortem` | consumes | implement |
| `post-mortem` | consumes | vibe |
| `post-mortem` | produces | result.json |
| `pr-implement` | consumes | crank |
| `pr-implement` | produces | git-changes |
| `pr-prep` | consumes | domain |
| `pr-prep` | produces | git-changes |
| `pr-research` | consumes | external-api |
| `pr-research` | produces | result.json |
| `pr-validate` | consumes | validation |
| `pr-validate` | produces | result.json |
| `pre-mortem` | consumes | standards |
| `pre-mortem` | produces | result.json |
| `pre-mortem` | produces | verdict.json |
| `product` | produces | result.json |
| `provenance` | produces | result.json |
| `push` | consumes | git-changes |
| `push` | produces | git-changes |
| `quickstart` | consumes | rpi |
| `quickstart` | produces | stdout |
| `ratchet` | consumes | post-mortem |
| `ratchet` | consumes | validation |
| `ratchet` | consumes | vibe |
| `ratchet` | produces | .agents/rpi/*.md |
| `recover` | consumes | bd |
| `recover` | consumes | rpi |
| `recover` | produces | .agents/rpi/*.md |
| `red-team` | consumes | repo-context |
| `red-team` | produces | result.json |
| `refactor` | consumes | complexity |
| `refactor` | consumes | repo-context |
| `refactor` | produces | git-changes |
| `release` | produces | result.json |
| `research` | consumes | inject |
| `research` | consumes | repo-context |
| `research` | produces | .agents/research/*.md |
| `research` | produces | result.json |
| `retro` | consumes | standards |
| `retro` | produces | result.json |
| `reverse-engineer-rpi` | produces | .agents/research/*.md |
| `review` | consumes | github-pr |
| `review` | consumes | validation |
| `review` | produces | result.json |
| `rpi` | consumes | crank |
| `rpi` | consumes | discovery |
| `rpi` | consumes | domain |
| `rpi` | consumes | ratchet |
| `rpi` | consumes | validation |
| `rpi` | produces | .agents/rpi/*.md |
| `scaffold` | produces | converted-skill |
| `scenario` | produces | result.json |
| `scope` | produces | filesystem-gate |
| `security` | consumes | repo-context |
| `security` | produces | security-report.json |
| `security-suite` | consumes | repo-context |
| `security-suite` | produces | security-report.json |
| `session-bootstrap` | consumes | bd |
| `session-bootstrap` | consumes | onboard |
| `session-bootstrap` | produces | json |
| `session-bootstrap` | produces | stdout |
| `shared` | produces | stdout |
| `ship-loop` | consumes | beads |
| `ship-loop` | consumes | post-mortem |
| `ship-loop` | consumes | rpi |
| `ship-loop` | produces | git-changes |
| `ship-loop` | produces | merged-prs |
| `skill-auditor` | produces | result.json |
| `skill-builder` | produces | converted-skill |
| `standards` | produces | stdout |
| `status` | consumes | bd |
| `status` | produces | stdout |
| `swarm` | consumes | implement |
| `swarm` | consumes | vibe |
| `swarm` | produces | .agents/swarm/results/*.json |
| `test` | consumes | repo-context |
| `test` | consumes | standards |
| `test` | produces | result.json |
| `trace` | produces | result.json |
| `using-agentops` | produces | documentation |
| `using-ntm` | produces | documentation |
| `validate` | consumes | validation |
| `validate` | produces | result.json |
| `validation` | consumes | forge |
| `validation` | consumes | post-mortem |
| `validation` | consumes | retro |
| `validation` | consumes | shared |
| `validation` | consumes | vibe |
| `validation` | produces | .agents/research/*.md |
| `validation` | produces | result.json |
| `validation` | produces | verdict.json |
| `vibe` | consumes | standards |
| `vibe` | produces | result.json |
| `vibe` | produces | verdict.json |
| `workflow-builder` | produces | workflow-script |
