# Reference

Deep documentation, runbooks, and lookup tables. These are the pages you open when something breaks, when you need the exact name of an environment variable, or when you want to understand why a specific failure mode exists and how to avoid it.

Three groups worth knowing:

- **Lookup** — [Glossary](../GLOSSARY.md), [Environment Variables](../ENV-VARS.md), [CLI ↔ Skills Map](../cli-skills-map.md). Skim once, then search-on-demand.
- **Operations** — [Testing](../TESTING.md), [CI/CD](../CI-CD.md), [Releasing](../RELEASING.md), [Incident Runbook](../INCIDENT-RUNBOOK.md). Load these before you ship a release or page someone.
- **Field guides** — [Agent Footguns](../agent-footguns.md), [Troubleshooting](../troubleshooting.md), [AgentOps Brief](../agentops-brief.md). Read before onboarding a new teammate.
- **Curation** — [JSM Skill Standards Snapshot](jsm-skill-standards.md), [JSM CLI Capability Map](jsm-cli-capability-map.md), [JSM Clean-Room Extraction Policy](jsm-clean-room-extraction-policy.md), [Skill Quality Rubric](skill-quality-rubric.md), and [JSM Skill Absorption Matrix](jsm-skill-absorption.md). Use these when checking JSM package standards, extraction boundaries, or what was absorbed from the Bushido standalone skill set.
- **Evolution control** — [AgentOps Domain Evolution BDD](agentops-domain-evolution-bdd.md), [AgentOps Skill Domain Map](agentops-skill-domain-map.md), [AgentOps Hexagonal Architecture Map](agentops-hexagonal-architecture-map.md), and [AgentOps Domain Evolution Plan](agentops-domain-evolution-plan.md). Use these before running broad skill, CLI, or hook evolution loops.

<div class="grid cards" markdown>

-   :material-book-alphabet: **[Glossary](../GLOSSARY.md)**

    ---

    Definitions of domain-specific terms (Beads, Brownian Ratchet, RPI, etc).

-   :material-variable: **[Environment Variables](../ENV-VARS.md)**

    ---

    All configuration variables with defaults and precedence.

-   :material-test-tube: **[Testing Guide](../TESTING.md)**

    ---

    Umbrella guide for all test types, tiers, and conventions.

-   :material-source-branch: **[CI/CD Architecture](../CI-CD.md)**

    ---

    Workflow map, job graph, blocking vs soft gates, local CI.

-   :material-tag: **[Releasing](../RELEASING.md)**

    ---

    Release process for ao CLI and plugin.

-   :material-ambulance: **[Incident Runbook](../INCIDENT-RUNBOOK.md)**

    ---

    Operational runbook for incidents and recovery.

-   :material-wrench: **[Troubleshooting](../troubleshooting.md)**

    ---

    Common issues and quick fixes.

-   :material-alert: **[Agent Footguns](../agent-footguns.md)**

    ---

    Common agent failure modes and mitigations.

-   :material-briefcase: **[AgentOps Brief](../agentops-brief.md)**

    ---

    Executive summary of AgentOps.

-   :material-map: **[System Map](../agentops-system-map.md)**

    ---

    Visual system map.

-   :material-file-tree: **[CLI ↔ Skills Map](../cli-skills-map.md)**

    ---

    Which commands are called by which skills and hooks.

-   :material-book-open-variant: **[Deep Reference](../reference.md)**

    ---

    Deep documentation and pipeline details.

-   :material-source-merge: **[JSM Absorption](jsm-skill-absorption.md)**

    ---

    Disposition matrix for the Bushido JSM-managed skill set.

-   :material-clipboard-check: **[JSM Skill Standards](jsm-skill-standards.md)**

    ---

    Pattern-only snapshot of the 118-skill local JSM corpus and packaging standards.

-   :material-console: **[JSM CLI Capability Map](jsm-cli-capability-map.md)**

    ---

    Read-only and mutating `jsm` command surfaces for clean-room analysis.

-   :material-shield-check: **[JSM Clean-Room Policy](jsm-clean-room-extraction-policy.md)**

    ---

    Allowed observations, forbidden source material, and review checklist.

-   :material-star-check: **[Skill Quality Rubric](skill-quality-rubric.md)**

    ---

    Scoring model for repo-runtime, JSM-export, and mega-skill readiness.

-   :material-clipboard-list: **[JSM Gap Audit](jsm-agentops-gap-audit.md)**

    ---

    Structural AgentOps skill gaps and priority upgrade queue.

-   :material-format-list-checks: **[JSM Pilot Backlog](jsm-pilot-upgrade-backlog.md)**

    ---

    First candidate AgentOps skill upgrades.

-   :material-graph-outline: **[Domain Evolution BDD](agentops-domain-evolution-bdd.md)**

    ---

    Gherkin acceptance contract for auditing, domain-mapping, and evolving AgentOps.

-   :material-sitemap: **[Skill Domain Map](agentops-skill-domain-map.md)**

    ---

    All checked-in AgentOps skills mapped to BC1-BC5 with first dispositions.

-   :material-hexagon-multiple: **[Hexagonal Architecture Map](agentops-hexagonal-architecture-map.md)**

    ---

    Domain, port, adapter, and proof-gate target for skill, CLI, and hook evolution.

-   :material-progress-check: **[Domain Evolution Plan](agentops-domain-evolution-plan.md)**

    ---

    Sequenced bootstrap and evolution plan anchored to `soc-y5vh`.

</div>
