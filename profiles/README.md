# Role-Based Taxonomy for .claude/ Artifacts

**Purpose**: Organize 100+ .claude/ artifacts (commands, agents, skills, workflows) into 3 discoverable profiles

**Created**: 2025-11-09
**Version**: 2.0.0 (Consolidated from 5 roles → 3 profiles)
**Status**: Production-ready, 3 profiles defined, example sessions included

---

## Quick Start

**"What am I working on?"**

| You're doing... | Load this profile | Token Budget |
|-----------------|-------------------|--------------|
| Building apps (backend, frontend, full-stack) | **software-dev** | ~25k (12.5%) |
| Operations (monitoring, incidents, deployments) | **platform-ops** | ~28k (14%) |
| Writing (docs, tutorials, research, patterns) | **content-creation** | ~22k (11%) |

---

## The 3 Profiles

### 1. Software Development (`software-dev`)

**What you do**:
- Build applications (backend APIs, frontends, full-stack)
- Work with React, Next.js, Node.js, Go, Python
- Manage Kubernetes/Helm deployments
- Create CI/CD pipelines
- Implement features end-to-end

**Token budget**: ~25k tokens (12.5% of context window)

**Key artifacts** (from 100+ total):
- Backend: `applications-create-app`, `services-crossplane-dev`, `services-edb-databases`
- Frontend: `documentation-create-docs`, React component patterns
- Pipelines: `pipelines-gitlab-ci`, `pipelines-troubleshooting`
- Testing: `testing-integration-e2e`
- Skills: `test.sh`, `rendering.sh`, `audit.sh`

**Tools**: kubectl, helm, docker, podman, node, npm, Context7 MCP (API docs)

**Example session**: See [software-dev-session.md](examples/software-dev-session.md)
- **Scenario**: Build REST API + React UI feature
- **Time**: ~2.3 hours
- **Outcome**: Feature deployed to production with full tests

---

### 2. Platform Operations (`platform-ops`)

**What you do**:
- Respond to production incidents
- Monitor system health and SLOs
- Deploy applications safely (blue-green, canary)
- Harmonize site configurations
- Debug ArgoCD sync issues
- Maintain 99.9%+ uptime

**Token budget**: ~28k tokens (14% of context window)

**Key artifacts**:
- Monitoring: `monitoring-alerts-runbooks`, `monitoring-slo-dashboards`
- Incidents: `incidents-response`, `incidents-postmortems`
- Deployment: `deployments-progressive-delivery`, `argocd-debug-sync`
- Infrastructure: `sites-harmonize`, `harmonize-guide`
- Operations: `playbooks-gitops-operations`, `playbooks-platform-upgrades`
- Skills: `validate.sh`, `sync.sh`, `harmonize.sh`

**Tools**: kubectl, helm, argocd, prometheus, grafana, yamllint

**Example session**: See [platform-ops-session.md](examples/platform-ops-session.md)
- **Scenario**: P1 incident - Redis OOM causing API timeouts
- **Time**: ~45 minutes
- **Outcome**: Incident resolved, postmortem completed, permanent fix deployed

---

### 3. Content Creation (`content-creation`)

**What you do**:
- Write documentation (technical and non-technical)
- Extract patterns from production work
- Conduct meta-analysis and research
- Create tutorials and examples
- Develop frameworks
- Synthesize learnings into reusable knowledge

**Token budget**: ~22k tokens (11% of context window)

**Key artifacts**:
- Meta: `meta-retro-analyzer`, `meta-workflow-auditor`, `meta-memory-integration`
- Documentation: `documentation-create-docs`, `documentation-optimize-docs`, `documentation-diataxis-auditor`
- Innovation: `innovation-architecture-review`, `innovation-capability-explorer`
- Research: `/research`, `/research-multi`, `/learn`, `/ultra-think`
- Skills: `agent-refactoring.sh`, compliance auditing

**Tools**: git (history analysis), grep, find, Context7 MCP (research), Memory MCP

**Example session**: See [content-creation-session.md](examples/content-creation-session.md)
- **Scenario**: Extract context bundle pattern, write docs + tutorial + blog post
- **Time**: ~3.25 hours
- **Outcome**: Pattern documented, tutorial created, blog post drafted

---

## The Taxonomy Structure

```
.claude/profiles/
├── README.md                          ← This file
├── COMPARISON.md                      ← Comparison with 12-factor-agentops examples
├── META_PATTERNS.md                   ← 15 extracted meta-patterns
│
├── schema/
│   └── role-profile.yaml              ← Profile schema definition
│
├── shared/                            ← Common across all profiles
│   ├── foundational.yaml              ← Laws, standards, git hooks (3k tokens)
│   ├── orchestration.yaml             ← Workflow commands (6k tokens, JIT)
│   └── context.yaml                   ← Bundles, memory (4k tokens, on-demand)
│
├── roles/                             ← 3 profile definitions
│   ├── software-dev.yaml              ← Development (25k tokens)
│   ├── platform-ops.yaml              ← Operations (28k tokens)
│   └── content-creation.yaml          ← Writing/research (22k tokens)
│
└── examples/                          ← Example sessions
    ├── software-dev-session.md        ← 2.3 hour feature development
    ├── platform-ops-session.md        ← 45 min incident response
    └── content-creation-session.md    ← 3.25 hour pattern documentation
```

---

## Shared Infrastructure

All 3 profiles inherit these shared components:

### Foundational (3k tokens) - Always Loaded

**What it is**: Constitutional baseline (Laws, standards, git hooks)

**Artifacts**:
- `work/gitops/.claude/CONSTITUTION.md` - AgentOps Laws (5 laws)
- `.claude/AGENT_INTERACTION_STANDARDS.md` - Consistent prompting
- Git hooks (pre-commit, prepare-commit-msg, commit-msg, post-commit)
- `tools/scripts/post-push-law4-check.sh` - Hook loop prevention

**Why it matters**: Ensures all profiles follow same Laws, commit format, standards

---

### Orchestration (6k tokens) - Loaded JIT

**What it is**: Workflow coordination (loaded when invoked)

**Artifacts**:
- Commands: `Read CLAUDE.md`, `Read CLAUDE.md-task`, `Read CLAUDE.md-task`, `Read CLAUDE.md`
- Phase commands: `/research`, `/plan`, `/implement`, `/validate`, `/learn`
- Workflows: `complete-cycle.md`, `debug-cycle.md`, `quick-fix.md`

**When loaded**: User invokes orchestration command (e.g., `Read CLAUDE.md`, `/research`)

---

### Context (4k tokens) - Loaded On-Demand

**What it is**: Knowledge management (bundles, memory, continuity)

**Artifacts**:
- Bundle commands: `/bundle-load`, `/bundle-save`, `/bundle-list`, `/bundle-search`
- Memory: `/memory-prune`, MCP memory tools
- System docs: `COMMAND_HIERARCHY.md` (3-level command system)

**When loaded**: User invokes context command or loads bundle

---

## Token Budget Design

**Philosophy**: Designed for single or multi-profile loading while staying under 40% rule.

### Single Profile Example

```
Foundational (always):              3k
Orchestration (if needed):         +6k
Profile (software-dev):           +25k
──────────────────────────────────────
Total:                             34k (17% of 200k window) ✅
```

### Two Profile Composition

```
Foundational:                       3k
Orchestration:                     +6k
Profile 1 (software-dev):         +25k
Profile 2 (platform-ops):         +20k (partial load)
──────────────────────────────────────
Total:                             54k (27% of 200k window) ✅
```

### Maximum Safe (All 3 Profiles)

```
Foundational:                       3k
Orchestration:                     +6k
Context:                           +4k
software-dev:                     +25k
platform-ops:                     +28k
content-creation:                 +22k
──────────────────────────────────────
Total:                             88k (44% of 200k window) ⚠️ Over 40%
```

**Design Principle**: Can load 1-2 profiles comfortably. Loading all 3 exceeds 40% rule → Forces intentional profile selection.

---

## Profile Selection Guide

### By Task Keywords

| Keywords | Profile |
|----------|---------|
| "build", "implement", "feature", "api", "frontend" | software-dev |
| "incident", "outage", "deploy", "monitor", "slo" | platform-ops |
| "document", "write", "research", "pattern", "tutorial" | content-creation |

### By File Patterns

| File Pattern | Profile |
|--------------|---------|
| `*.tsx`, `*.jsx`, `*.go`, `*.py`, `apps/*/` | software-dev |
| `*.yaml`, `config.env`, `prometheus/`, `grafana/` | platform-ops |
| `docs/**/*.md`, `README.md`, `patterns/`, `*.bib` | content-creation |

### By Git Patterns

| Commit Prefix | Profile |
|---------------|---------|
| `feat(api):`, `feat(ui):`, `feat(backend):` | software-dev |
| `fix(ops):`, `fix(monitoring):`, `feat(deploy):` | platform-ops |
| `docs(explanation):`, `docs(tutorial):`, `feat(pattern):` | content-creation |

---

## Real-World Use Cases

### Use Case 1: Full-Stack Feature

**Scenario**: Build new API endpoint with React UI

**Profiles**: software-dev (primary)

**Loading**:
```
Foundational:                3k
software-dev:              +25k
──────────────────────────────
Total:                      28k (14%)
```

**Time**: ~2-3 hours
**Outcome**: Feature deployed with tests

---

### Use Case 2: Production Incident

**Scenario**: Redis OOM causing service degradation

**Profiles**: platform-ops (primary)

**Loading**:
```
Foundational:                3k
Orchestration (Read CLAUDE.md): 2k
platform-ops:               +8k (incident agents only)
──────────────────────────────
Total:                      13k (6.5%)
```

**Time**: ~45 minutes
**Outcome**: Incident resolved, postmortem done

---

### Use Case 3: Framework Documentation

**Scenario**: Extract pattern from production, document + tutorial

**Profiles**: content-creation (primary)

**Loading**:
```
Foundational:                3k
content-creation:          +22k
──────────────────────────────
Total:                      25k (12.5%)
```

**Time**: ~3-4 hours
**Outcome**: Pattern doc + tutorial + blog post

---

### Use Case 4: Platform Feature (Multi-Profile)

**Scenario**: Build monitoring dashboard (code + deploy + document)

**Profiles**: software-dev + platform-ops + content-creation (partial)

**Loading**:
```
Foundational:                3k
software-dev:              +15k (frontend only)
platform-ops:              +10k (monitoring only)
content-creation:           +8k (docs only)
──────────────────────────────
Total:                      36k (18%)
```

**Time**: ~4-5 hours
**Outcome**: Dashboard deployed + monitored + documented

---

## What Changed from v1.0.0

### Consolidation Rationale

**v1.0.0** (5 roles):
- sre-devops (30k)
- platform-engineer (20k)
- web-developer (15k)
- researcher (25k)
- personal (10k)

**v2.0.0** (3 profiles):
- **software-dev** (25k) = platform-engineer + web-developer
- **platform-ops** (28k) = sre-devops (streamlined)
- **content-creation** (22k) = researcher + documentation aspects

**Why consolidate?**
1. ✅ **Natural groupings**: Developers do both backend AND frontend in real work
2. ✅ **Reduced cognitive load**: 3 choices vs 5
3. ✅ **Better composition**: Clearer when to load multiple profiles
4. ✅ **Removed workspace-specific**: "personal" role was too specific to this workspace
5. ✅ **More generalizable**: 3 profiles apply to most software work

**What was removed**:
- ❌ `personal.yaml` - Too workspace-specific (career planning, philosophy)
- ❌ Split between backend/frontend - Merged into `software-dev`
- ❌ Split between SRE/DevOps - Combined in `platform-ops`

---

## Meta-Patterns (Extracted)

**See**: `META_PATTERNS.md` for complete analysis (15 patterns)

**Highlights**:

1. **Profile Archetypes**: Software-dev=Creator, Platform-ops=Guardian, Content-creation=Synthesizer
2. **Token Budgets Reveal Complexity**: Ops (28k) > Dev (25k) > Content (22k)
3. **Shared Foundation Enforces Consistency**: All profiles follow same Laws
4. **Multi-Profile Composition**: Real work often requires 2 profiles (~35k tokens)
5. **JIT Loading**: Load agents only when needed (not all at once)

**For full meta-patterns**: Read `META_PATTERNS.md`

---

## Integration with 12-Factor AgentOps

**See**: `COMPARISON.md` for complete comparison

**Key insight**: This workspace taxonomy and 12-factor-agentops examples are complementary:

- **12-factor examples**: Educational templates (learn patterns)
- **Workspace profiles**: Production inventory (organize reality)

**Together**: Learn → Apply → Extract → Improve (knowledge compounds)

**Cross-references**:
- Workspace → Framework: "Learn patterns from [12-factor-agentops/examples/](../../personal/12-factor-agentops/examples/)"
- Framework → Workspace: "See production validation in workspace profiles"

---

## How to Use This Taxonomy

### For Navigation

**Before**: "Where's the monitoring agent?"
**After**: "I'm doing ops work" → Load `platform-ops` → See all monitoring agents

### For Learning

**Before**: Overwhelmed by 100+ artifacts
**After**: "I'm a developer" → Focus on `software-dev` profile (25k tokens), progressive learning

### For Composition

**Before**: Manually search agents across repos
**After**: Load `software-dev + platform-ops` → Get both profiles (53k tokens = 26.5%)

---

## Future Evolution

### Potential Additions

**New profiles** (if patterns emerge):
- `data-engineering` - ML pipelines, data processing, analytics
- `security-ops` - Pen testing, compliance, vulnerability management

**Refinements**:
- More granular sub-profiles (e.g., `software-dev/frontend` vs `software-dev/backend`)
- Domain-specific overlays (healthcare, finance, government)

### Promotion Criteria (Artifact → Shared)

An artifact should move to shared when:
- ✅ Used by 2+ profiles
- ✅ No profile-specific dependencies
- ✅ Proven in production (10+ uses)
- ✅ Well-documented

**Example**: `validate.sh` started in platform-ops, now used by all profiles → Could move to shared

---

## Maintenance

### Adding New Artifacts

**To add a command, agent, skill, or workflow**:

1. Determine which profile it belongs to (use trigger patterns)
2. Update appropriate profile YAML file
3. Update token budget
4. Test loading (stays under 40%?)
5. Document in profile

**Example**:
```yaml
# In roles/platform-ops.yaml
new_agent:
  - path: work/gitops/.claude/agents/kubernetes-upgrade.md
    description: Automated Kubernetes version upgrades
    token_cost: 2200
    category: operations
```

### Quarterly Review

**Check**:
- Are profiles still balanced? (token budgets reasonable)
- New patterns emerging? (need new profile?)
- Artifacts in wrong profile? (move to correct one)
- Usage metrics? (which profiles most used)

---

## Example Sessions

**Included**: 3 complete example sessions showing realistic usage

1. **software-dev-session.md** - Build REST API + React UI (~2.3 hours)
2. **platform-ops-session.md** - P1 incident response (~45 minutes)
3. **content-creation-session.md** - Pattern extraction + docs (~3.25 hours)

**Each shows**:
- Context loading (token budgets)
- Agent usage (which agents loaded when)
- Time breakdown (phase by phase)
- Outcomes (what was accomplished)
- Comparison (with vs without profile)

---

## References

- **Schema**: `schema/role-profile.yaml` - Profile structure definition
- **Meta-Patterns**: `META_PATTERNS.md` - 15 extracted patterns
- **Comparison**: `COMPARISON.md` - Workspace vs 12-factor-agentops
- **Command Hierarchy**: `../.claude/COMMAND_HIERARCHY.md` - 3-level command system
- **Workspace Kernel**: `../../CLAUDE.md` - Workspace orchestration
- **AgentOps Laws**: `../../work/gitops/.claude/CONSTITUTION.md` - Constitutional baseline

---

## Version History

**v2.0.0 (2025-11-09)**:
- Consolidated from 5 roles → 3 profiles
- Added example sessions for each profile
- Removed workspace-specific "personal" role
- Merged platform-engineer + web-developer → software-dev
- Streamlined sre-devops → platform-ops
- Combined researcher + docs → content-creation
- Updated all documentation

**v1.0.0 (2025-11-09)**:
- Initial taxonomy creation
- 5 roles defined
- 3 shared profiles
- 100+ artifacts categorized
- 15 meta-patterns extracted

---

**The taxonomy is production-ready. Use it to discover artifacts, compose profiles, and build faster.**
