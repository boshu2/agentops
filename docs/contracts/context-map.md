<!-- generated from skills/*/SKILL.md frontmatter -->

# AgentOps Context Map

Generated from SKILL.md frontmatter. See [ADR-0001](https://github.com/boshu2/agentops/blob/main/docs/adr/ADR-0001-ddd-hexagonal-adoption.md)
and [`PRACTICE.md`](https://github.com/boshu2/agentops/blob/main/PRACTICE.md) line 80 for the architectural rationale.

## Skills by hexagonal role

### domain

- `brainstorm` — Separate goals from implementation.
- `bug-hunt` — Investigate bugs and root causes.
- `complexity` — Find focused refactor hotspots.
- `council` — Run multi-judge consensus.
- `crank` — Execute epics through waves.
- `design` — Validate product fit before discovery.
- `discovery` — Create execution packets.
- `domain` — Canonical vocabulary for human-AI software work.
- `flywheel` — Check knowledge flywheel health.
- `forge` — Mine transcripts into learnings.
- `goals` — Maintain AgentOps goals.
- `hooks-authoring` — Author AgentOps runtime hooks.
- `perf` — Profile and optimize hotspots.
- `plan` — Decompose goals into issue plans.
- `post-mortem` — Review completed work and learn.
- `pre-mortem` — Stress-test plans before work.
- `product` — Create or refine PRODUCT.md.
- `ratchet` — Record Brownian Ratchet gates.
- `retro` — Capture a session learning.
- `shared` — Shared AgentOps skill contracts.
- `standards` — Provide repo coding standards.
- `validation` — Run post-implementation validation.
- `vibe` — Validate code readiness.

### driving-adapter

- `bootstrap` — Initialize AgentOps project files.
- `implement` — Implement one tracked issue.
- `inject` — Load relevant .agents context.
- `pr-implement` — Implement a scoped OSS PR.
- `pr-prep` — Prepare PR commits and body.
- `pr-validate` — Validate PR scope and quality.
- `push` — Validate, commit, and push.
- `quickstart` — Show AgentOps next action.
- `recover` — Recover session context.
- `research` — Explore and write findings.
- `review` — Review diffs for risk, find mocks, scan for bugs, and audit codebases.
- `status` — Show AgentOps work status.
- `validate` — Produce PASS/WARN/FAIL verdicts for artifacts, plans, code, PRs, or gates.

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

- `autodev` — Manage bounded autonomous dev loops.
- `codex-team` — Coordinate multiple Codex agents.
- `compile` — Compile .agents knowledge wiki.
- `curate` — Mine transcripts, .agents, bd, and git for skill diffs, bd updates, or rare wiki entries.
- `doc` — Generate and validate repo docs.
- `dream` — Run overnight compounding sessions.
- `evolve` — Run autonomous improvement loops.
- `handoff` — Write compact session handoffs.
- `harvest` — Promote .agents knowledge.
- `heal-skill` — Repair skill hygiene.
- `knowledge-activation` — Activate mature .agents knowledge.
- `llm-wiki` — Build external-knowledge wikis.
- `pr-plan` — Plan an open source PR.
- `pr-retro` — Learn from PR outcomes.
- `red-team` — Probe docs and skills.
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
- `update` — Sync AgentOps skills.

### generic

- `converter` — Convert AgentOps skill formats.
- `oss-docs` — Scaffold or audit OSS docs.
- `readme` — Draft or improve README docs.
- `using-agentops` — Explain AgentOps workflows.

### unclassified

- (no unclassified skills)

## Context relationships

```mermaid
graph LR
  %% no context_rel edges declared yet
```

## Data flow (consumes / produces)

| Skill | Direction | Artifact |
|-------|-----------|----------|
| _(none)_ | _(none)_ | _(no consumes/produces declared yet)_ |
