# Documentation Index

> Master table of contents for AgentOps documentation.

## Getting Started

- [README](../README.md) — Project overview and quick start
- [FAQ](FAQ.md) — Comparisons, limitations, subagent nesting, uninstall
- [CONTRIBUTING](../CONTRIBUTING.md) — How to contribute
- [AGENTS.md](../AGENTS.md) — Local agent instructions for this repo
- [Changelog](../CHANGELOG.md) — Release history
- [Security](../SECURITY.md) — Vulnerability reporting

## Architecture

- [How It Works](how-it-works.md) — Brownian Ratchet, Ralph Wiggum Pattern, agent backends, hooks, context windowing
- [Architecture](ARCHITECTURE.md) — System design and component overview
- [Architecture Folder Index](architecture/README.md) — Architecture subdocs overview
- [AO-Olympus Ownership Matrix](architecture/ao-olympus-ownership-matrix.md) — Responsibility split for skills, runtime, and bridge contracts
- [PDC Framework](architecture/pdc-framework.md) — Prevent, Detect, Correct quality control approach
- [FAAFO Alignment](architecture/faafo-alignment.md) — FAAFO promise framework for vibe coding value
- [Failure Patterns](architecture/failure-patterns.md) — The 12 failure patterns reference guide

## Skills

- [Skills Reference](SKILLS.md) — Complete reference for all AgentOps skills
- [Skill Tiers](../skills/SKILL-TIERS.md) — Taxonomy and dependency graph
- [Testing Skills](testing-skills.md) — Guide for writing and running skill tests
- [Claude Code Skills Docs](https://code.claude.com/docs/en/skills) — Official Claude Code skills documentation (upstream)

## Workflows

- [Workflow Guide](workflows/README.md) — Decision matrix for choosing the right workflow
- [Complete Cycle](workflows/complete-cycle.md) — Full Research, Plan, Implement, Validate, Learn workflow
- [Session Lifecycle](workflows/session-lifecycle.md) — Complete guide to working with Claude across sessions
- [Quick Fix](workflows/quick-fix.md) — Fast implementation for simple, low-risk changes
- [Debug Cycle](workflows/debug-cycle.md) — Systematic debugging from symptoms to root cause to fix
- [Knowledge Synthesis](workflows/knowledge-synthesis.md) — Extract and synthesize knowledge from multiple sources
- [Assumption Validation](workflows/assumption-validation.md) — Validate research assumptions before planning
- [Post-Work Retro](workflows/post-work-retro.md) — Systematic retrospective after completing work
- [Multi-Domain](workflows/multi-domain.md) — Coordinate work spanning multiple domains
- [Continuous Improvement](workflows/continuous-improvement.md) — Ongoing system optimization and pattern refinement
- [Infrastructure Deployment](workflows/infrastructure-deployment.md) — Orchestrate deployment with validation gates
- [Meta-Observer Pattern](workflows/meta-observer-pattern.md) — Autonomous multi-session coordination

### Meta-Observer

- [Meta-Observer README](workflows/meta-observer/README.md) — Complete workflow package overview
- [Pattern Guide](workflows/meta-observer/pattern-guide.md) — Autonomous multi-session coordination guide
- [Example Session](workflows/meta-observer/example-today.md) — Real example from 2025-11-09
- [Showcase](workflows/meta-observer/SHOWCASE.md) — Distributed intelligence for multi-session work

## Concepts

- [Knowledge Flywheel](knowledge-flywheel.md) — How every session makes the next one smarter
- [The Science](the-science.md) — Research behind knowledge decay and compounding
- [Brownian Ratchet](brownian-ratchet.md) — AI-native development philosophy

## Standards

- [Standards Overview](standards/README.md) — Coding standards index
- [Go Style Guide](standards/golang-style-guide.md) — Go coding conventions
- [TypeScript Standards](standards/typescript-standards.md) — TypeScript coding conventions
- [Python Style Guide](standards/python-style-guide.md) — Python coding conventions
- [Shell Script Standards](standards/shell-script-standards.md) — Shell script conventions
- [Markdown Style Guide](standards/markdown-style-guide.md) — Markdown formatting conventions
- [JSON/JSONL Standards](standards/json-jsonl-standards.md) — JSON and JSONL conventions
- [YAML/Helm Standards](standards/yaml-helm-standards.md) — YAML and Helm chart conventions
- [Tag Vocabulary](standards/tag-vocabulary.md) — Standard tag definitions

## Levels

- [Levels Overview](levels/README.md) — Progressive learning path

### L1 — Basics

- [L1 README](levels/L1-basics/README.md) — Single-session work with Claude Code
- [Research](levels/L1-basics/research.md) — Explore a codebase to understand how it works
- [Implement](levels/L1-basics/implement.md) — Make changes, validate, commit
- [Demo: Research Session](levels/L1-basics/demo/research-session.md) — Example research session
- [Demo: Implement Session](levels/L1-basics/demo/implement-session.md) — Example implement session

### L2 — Persistence

- [L2 README](levels/L2-persistence/README.md) — Cross-session memory with .agents/
- [Research](levels/L2-persistence/research.md) — Explore codebase and save findings
- [Retro](levels/L2-persistence/retro.md) — Extract session learnings
- [Demo: Research Session](levels/L2-persistence/demo/research-session.md) — Example persistent research
- [Demo: Retro Session](levels/L2-persistence/demo/retro-session.md) — Example retro session

### L3 — State Management

- [L3 README](levels/L3-state-management/README.md) — Issue tracking with beads
- [Plan](levels/L3-state-management/plan.md) — Decompose goals into tracked issues
- [Implement](levels/L3-state-management/implement.md) — Execute, validate, commit, close
- [Demo: Plan Session](levels/L3-state-management/demo/plan-session.md) — Example planning session
- [Demo: Implement Session](levels/L3-state-management/demo/implement-session.md) — Example implement session

### L4 — Parallelization

- [L4 README](levels/L4-parallelization/README.md) — Wave-based parallel execution
- [Implement Wave](levels/L4-parallelization/implement-wave.md) — Execute unblocked issues in parallel
- [Demo: Wave Session](levels/L4-parallelization/demo/wave-session.md) — Example wave execution

### L5 — Orchestration

- [L5 README](levels/L5-orchestration/README.md) — Full autonomous operation with /crank
- [Crank](levels/L5-orchestration/crank.md) — Execute epics to completion
- [Demo: Crank Session](levels/L5-orchestration/demo/crank-session.md) — Example crank session

## Profiles

- [Profiles Overview](profiles/README.md) — Role-based profile organization
- [Profile Comparison](profiles/COMPARISON.md) — Workspace profiles vs 12-Factor examples
- [Meta-Patterns](profiles/META_PATTERNS.md) — Patterns extracted from role-based taxonomy
- [Example: Software Dev](profiles/examples/software-dev-session.md) — Software development session
- [Example: Platform Ops](profiles/examples/platform-ops-session.md) — Platform operations session
- [Example: Content Creation](profiles/examples/content-creation-session.md) — Content creation session

## Comparisons

- [Comparisons Overview](comparisons/README.md) — AgentOps vs the competition
- [vs SDD](comparisons/vs-sdd.md) — AgentOps vs Spec-Driven Development
- [vs GSD](comparisons/vs-gsd.md) — AgentOps vs Get Shit Done
- [vs Superpowers](comparisons/vs-superpowers.md) — AgentOps vs Superpowers plugin
- [vs Claude-Flow](comparisons/vs-claude-flow.md) — AgentOps vs Claude-Flow orchestration

## Positioning

- [Positioning Overview](positioning/README.md) — Product and messaging foundations
- [DevOps for Vibe-Coding](positioning/devops-for-vibe-coding.md) — Strategic foundation document
- [12 Factors Validation Lens](positioning/12-factors-validation-lens.md) — Shift-left validation for coding agents

## Plans

- [Plans Overview](plans/README.md) — Time-stamped plans index
- [Validated Release Pipeline](plans/2026-01-28-validated-release-pipeline.md) — Release pipeline design (2026-01-28)
- [AO-Olympus Bridge Next Steps](plans/2026-02-13-ao-olympus-bridge-next-steps.md) — Follow-up work to make the AO↔OL bridge enforceable (2026-02-13)

## Templates

- [Templates Overview](templates/README.md) — Templates index
- [Workflow Template](templates/workflow.template.md) — Template for new workflows
- [Agent Template](templates/agent.template.md) — Template for new agents
- [Skill Template](templates/skill.template.md) — Template for new skills
- [Command Template](templates/command.template.md) — Template for new commands
- [Kernel Template](templates/kernel.template.md) — Template for new project kernels
- [Product Template](PRODUCT-TEMPLATE.md) — Template for writing a PRODUCT.md

## Reference

- [Glossary](GLOSSARY.md) — Definitions of domain-specific terms (Beads, Brownian Ratchet, RPI, etc.)
- [CLI Reference](../cli/docs/COMMANDS.md) — Complete `ao` command reference
- [Reference](reference.md) — Deep documentation and pipeline details
- [Releasing](RELEASING.md) — Release process for ao CLI and plugin
- [Troubleshooting](troubleshooting.md) — Common issues and quick fixes
- [Incident Runbook](INCIDENT-RUNBOOK.md) — Operational runbook for incidents and recovery
- [AO Command Customization Matrix](architecture/ao-command-customization-matrix.md) — External command dependencies and customization policy tiers
- [OL-AO Bridge Contracts](ol-bridge-contracts.md) — Olympus-AgentOps interchange formats
- [MemRL Policy Integration](contracts/memrl-policy-integration.md) — AO-exported deterministic MemRL policy contract for Olympus hooks
- [MemRL Policy Schema](contracts/memrl-policy.schema.json) — Machine-readable schema for MemRL policy package
- [MemRL Policy Example Profile](contracts/memrl-policy.profile.example.json) — Example deterministic policy profile
