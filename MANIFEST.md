# Knowledge OS Plugin Manifest v2.0

A comprehensive Claude Code plugin providing the Research/Plan/Implement workflow, session management, vibe-coding metrics, and 95+ specialized agents organized into a 3-tier modular architecture.

## Architecture Overview

```
FOUNDATION (Required - Install First)
├── foundation-core           RPI workflow, sessions, vibe-coding
├── foundation-bundles        Context persistence
├── foundation-quality        Code review, testing, architecture
├── foundation-learning       Retrospectives, pattern extraction
├── foundation-meta           Multi-agent orchestration
└── foundation-skills         Domain knowledge packs

DOMAIN (Pick Your Role - 1-3)
├── domain-software-engineer  Frontend, backend, fullstack, UI/UX
├── domain-devops            CI/CD, containers, pipelines
├── domain-platform          GitOps, Crossplane, Kyverno, monitoring
├── domain-data              Data engineering, ML, MLOps [NEW]
├── domain-security          Scanning, compliance, incidents
├── domain-documentation     Diataxis, diagrams, optimization
├── domain-mobile            iOS, Android, cross-platform [NEW]
└── domain-innovation        Architecture review, automation

SPECIALIST (Deep Expertise - Optional)
├── specialist-golang
├── specialist-python
├── specialist-typescript
├── specialist-shell
├── specialist-rust          [NEW]
├── specialist-java          [NEW]
├── specialist-performance   [NEW]
└── specialist-accessibility [NEW]
```

## Plugin Summary

| Tier | Plugins | Description |
|------|---------|-------------|
| Foundation | 6 | Core workflow, bundles, quality, learning, meta, skills |
| Domain | 8 | Role-based plugins for your work domain |
| Specialist | 8 | Deep expertise in languages and quality |
| **Total** | **22 plugins** | **28 commands, 95+ agents, 40+ skills** |

## Installation

### From Marketplace (Recommended)

```bash
# Add this marketplace
/plugin marketplace add fullerbt/claude-code-marketplace

# Browse and install plugins
/plugin
```

### Install by Profile

```bash
# Minimal - Core workflow only
/plugin install fullerbt/claude-code-marketplace/foundation-core
/plugin install fullerbt/claude-code-marketplace/foundation-bundles

# Developer Profile
/plugin install fullerbt/claude-code-marketplace --profile developer

# DevOps Profile
/plugin install fullerbt/claude-code-marketplace --profile devops

# Data Profile
/plugin install fullerbt/claude-code-marketplace --profile data

# Full Installation
/plugin install fullerbt/claude-code-marketplace
```

### Install by Tier

```bash
# Foundation tier (recommended first)
/plugin install fullerbt/claude-code-marketplace/foundation-core
/plugin install fullerbt/claude-code-marketplace/foundation-bundles
/plugin install fullerbt/claude-code-marketplace/foundation-quality

# Domain tier (pick 1-3 based on your role)
/plugin install fullerbt/claude-code-marketplace/domain-software-engineer
/plugin install fullerbt/claude-code-marketplace/domain-devops

# Specialist tier (optional deep expertise)
/plugin install fullerbt/claude-code-marketplace/specialist-python
```

---

## Foundation Tier (6 plugins)

### foundation-core

Core RPI workflow commands:

| Command | Description |
|---------|-------------|
| `/research` | Phase 1: Deep exploration before planning |
| `/plan` | Phase 2: Specify exact changes |
| `/implement` | Phase 3: Execute with validation |
| `/session-start` | Initialize session with context |
| `/session-end` | Clean session closure |
| `/session-resume` | Single-command resume |
| `/vibe-check` | Run metrics analysis |
| `/vibe-level` | Classify task trust level (0-5) |
| `/progress-update` | Update progress files |

### foundation-bundles

Context persistence for multi-session work:

| Command | Description |
|---------|-------------|
| `/bundle-save` | Save compressed context bundle |
| `/bundle-load` | Load saved bundle |
| `/bundle-search` | Search bundles by topic |
| `/bundle-list` | List all bundles |
| `/bundle-prune` | Clean stale bundles |
| `/bundle-load-multi` | Load multiple bundles |

### foundation-quality

Code review, testing, and architecture (7 agents):

| Component | Description |
|-----------|-------------|
| `/code-review` | Comprehensive code review |
| `/architecture-review` | Architecture analysis |
| `/generate-tests` | Generate test suites |
| `code-reviewer` | Expert code review for quality and security |
| `code-review-improve` | Systematic code review with conventions |
| `code-explorer` | Systematically explore code structure |
| `test-generator` | Generate comprehensive test cases |
| `testing-integration-e2e` | Integration and E2E test creation |
| `testing-onboarding-audit` | Audit onboarding experience |
| `continuous-validator` | Continuous validation throughout lifecycle |

### foundation-learning

Pattern extraction and retrospectives:

| Command | Description |
|---------|-------------|
| `/learn` | Extract reusable patterns |
| `/retro` | Post-work retrospective |
| `/maintain` | Weekly knowledge maintenance |

### foundation-meta

Multi-agent orchestration (8 agents):

| Component | Description |
|-----------|-------------|
| `/ultra-think` | Deep multi-dimensional analysis |
| `/project-init` | Initialize 2-Agent Harness projects |
| `/research-multi` | Parallel 3-agent research |
| `meta-implement-agent` | Execute approved plans with validation |
| `meta-observer` | Monitor N autonomous workers |
| `meta-retro-analyzer` | Extract patterns from past sessions |
| `meta-workflow-auditor` | Audit multi-agent workflows |
| `ai-engineer` | LLM applications and RAG systems |
| `autonomous-worker` | Domain-specific independent work |
| `context-manager` | Context management for multi-agent |
| `task-decomposition-expert` | Complex goal breakdown |

### foundation-skills

40+ domain knowledge packs:

| Domain | Skills |
|--------|--------|
| GitOps | argocd-gitops-operations, harmonize, git-workflow |
| Infrastructure | crossplane-infrastructure-dev, edb-postgres-lifecycle |
| Security | compliance-scanning, incident-diagnostics |
| Automation | ansible-automation-platform, gpu-workload-management |
| Platform | keycloak-sso-integration, kyverno-policy-suite |
| Observability | observability-stack, manifest-validation |
| Release | release-engineering |

---

## Domain Tier (8 plugins)

### domain-software-engineer (6 agents)

Full-stack development:

| Agent | Description |
|-------|-------------|
| `frontend-developer` | React, responsive design, accessibility |
| `backend-architect` | API design, microservices, databases |
| `fullstack-developer` | End-to-end application development |
| `ui-ux-designer` | User research, wireframes, design systems |
| `api-documenter` | OpenAPI specs, SDK generation |
| `prompt-engineer` | LLM prompt optimization |

### domain-devops (8 agents)

CI/CD, containers, deployments:

| Agent | Description |
|-------|-------------|
| `deployment-engineer` | CI/CD and deployment automation |
| `deployments-progressive-delivery` | Canary, blue-green, rolling deploys |
| `deployments-rollback-automation` | Automated rollback mechanisms |
| `containers-build-modify` | Container image building |
| `pipelines-gitlab-ci` | GitLab CI/CD pipeline modification |
| `pipelines-troubleshooting` | Pipeline failure diagnosis |
| `network-engineer` | Network connectivity and infrastructure |
| `networking-nsx-load-balancer` | NSX-T load balancer configuration |

### domain-platform (23 agents)

GitOps, platform services, playbooks:

| Agent | Description |
|-------|-------------|
| `applications-create-app` | Create GitOps applications |
| `applications-modify-app` | Modify existing GitOps apps |
| `applications-debug-sync` | Debug Argo CD sync failures |
| `argocd-debug-sync` | ArgoCD sync debugging |
| `services-crossplane-dev` | Crossplane infrastructure development |
| `services-edb-databases` | EDB Postgres database management |
| `services-kyverno-policies` | Kyverno policy creation |
| `monitoring-alerts-runbooks` | Prometheus alerts and runbooks |
| `playbooks-gitops-operations` | GitOps operations guide |
| `playbooks-keycloak-sso` | Keycloak SSO deployment |
| ... and 13 more platform agents |

### domain-data (4 agents) [NEW]

Data engineering, ML, and MLOps:

| Agent | Description |
|-------|-------------|
| `data-engineer` | Data pipelines, warehouses, streaming architectures |
| `data-scientist` | Statistical modeling, ML, business insights |
| `ml-engineer` | Production ML systems, model serving |
| `mlops-engineer` | ML pipelines, experiment tracking, model registries |

### domain-security (7 agents)

Security scanning, incidents, forensics:

| Agent | Description |
|-------|-------------|
| `security-scanning` | SAST, DAST, dependency scanning |
| `penetration-tester` | Penetration testing and ethical hacking |
| `incident-responder` | Production incident handling |
| `incidents-response` | Structured incident triage |
| `incidents-postmortems` | Blameless postmortem creation |
| `error-detective` | Log analysis and error detection |
| `history-explorer` | Mine git history for patterns |

### domain-documentation (8 agents)

Documentation creation and optimization:

| Agent | Description |
|-------|-------------|
| `documentation-create-docs` | Create clear documentation |
| `documentation-optimize-docs` | Optimize to Knowledge OS standards |
| `documentation-search-docs` | Fast parallel doc search |
| `documentation-diataxis-auditor` | Audit for Diataxis compliance |
| `documentation-add-diagrams` | Add Mermaid diagrams |
| `doc-explorer` | Find and synthesize documentation |
| `document-structure-analyzer` | Analyze document layouts |
| `customer-support` | Support and documentation |

### domain-mobile (2 agents) [NEW]

iOS, Android, and cross-platform:

| Agent | Description |
|-------|-------------|
| `ios-developer` | Swift/SwiftUI, iOS patterns, App Store |
| `mobile-developer` | React Native, Flutter, cross-platform |

### domain-innovation (12 agents)

Architecture, automation, planning:

| Agent | Description |
|-------|-------------|
| `innovation-architecture-review` | Architecture improvement opportunities |
| `innovation-automation-opportunities` | Identify automation opportunities |
| `innovation-brainstorm-solutions` | Creative innovation brainstorming |
| `innovation-capability-explorer` | Explore untapped capabilities |
| `spec-architect` | Design specifications with precision |
| `validation-planner` | Comprehensive validation strategies |
| `assumption-validator` | Validate against target environment |
| `risk-assessor` | Identify risks and failure modes |
| `change-executor` | Execute changes with validation |
| `tracer-bullet-deployer` | Deploy minimal test resources |
| `connection-agent` | Obsidian vault connections |
| `archive-researcher` | Repository archival analysis |

---

## Specialist Tier (8 plugins)

### specialist-golang (1 agent)

| Agent | Description |
|-------|-------------|
| `golang-pro` | Idiomatic Go with goroutines, channels, interfaces |

### specialist-python (2 agents)

| Agent | Description |
|-------|-------------|
| `python-pro` | Advanced Python with decorators, generators, async |
| `python-uv-migration` | Migrate to unified Python uv standard |

### specialist-typescript (1 agent)

| Agent | Description |
|-------|-------------|
| `typescript-pro` | Advanced TypeScript type system, generics |

### specialist-shell (1 agent)

| Agent | Description |
|-------|-------------|
| `shell-scripting-pro` | Robust shell scripts with POSIX compliance |

### specialist-rust (1 agent) [NEW]

| Agent | Description |
|-------|-------------|
| `rust-pro` | Rust systems programming, ownership, async |

### specialist-java (1 agent) [NEW]

| Agent | Description |
|-------|-------------|
| `java-pro` | Java/Spring development, JVM, enterprise patterns |

### specialist-performance (1 agent) [NEW]

| Agent | Description |
|-------|-------------|
| `performance-engineer` | Load testing, profiling, optimization |

### specialist-accessibility (1 agent) [NEW]

| Agent | Description |
|-------|-------------|
| `accessibility-specialist` | WCAG compliance, ARIA, inclusive design |

---

## Installation Profiles

| Profile | Plugins | Best For |
|---------|---------|----------|
| `minimal` | foundation-core, foundation-bundles | Quick start, minimal overhead |
| `developer` | foundation-*, domain-software-engineer, specialist-typescript, specialist-python | Full-stack developers |
| `devops` | foundation-*, domain-devops, domain-platform, specialist-shell | DevOps/Platform engineers |
| `data` | foundation-*, domain-data, specialist-python | Data engineers/scientists |
| `full` | All plugins | Maximum capability |

---

## Core Concepts

### RPI Workflow

Research -> Plan -> Implement with fresh sessions between phases.

### Bundles

Compressed markdown (~1k tokens) preserving context across sessions.

### Vibe Levels

Trust calibration (0-5):

| Level | Trust | Use For |
|-------|-------|---------|
| 5 | 95% | Format, lint |
| 4 | 80% | Boilerplate |
| 3 | 60% | CRUD, tests |
| 2 | 40% | Features |
| 1 | 20% | Architecture |
| 0 | 0% | Novel research |

### Dependencies

Plugins declare dependencies. Install foundation tier first:

```
foundation-core (no dependencies)
├── foundation-bundles
├── foundation-quality
├── foundation-learning
├── foundation-meta
└── foundation-skills

domain-* (depends on foundation-core)
specialist-* (depends on foundation-core)
```

---

## Compatibility

- **Claude Code CLI:** v1.0.0+
- **Plugin Format:** Anthropic 2025 Schema v2
- **License:** MIT

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 2.0.0 | 2025-12-06 | 3-tier modular architecture, 10 new agents |
| 1.0.0 | 2025-12-06 | Initial release with categorized agents |
