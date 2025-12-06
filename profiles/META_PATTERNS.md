# Meta-Patterns Extracted from Role-Based Taxonomy

**Analysis Date**: 2025-11-09
**Artifacts Analyzed**: 100+ (commands, agents, skills, workflows, settings)
**Roles Defined**: 5 (sre-devops, platform-engineer, web-developer, researcher, personal)
**Shared Infrastructure**: 3 profiles (foundational, orchestration, context)

---

## Pattern 1: Role Archetypes Map to Work Modes

**Discovery**: Each role represents a distinct *mode of thinking*, not just tooling.

| Role | Archetype | Primary Mode | Key Question |
|------|-----------|--------------|--------------|
| **sre-devops** | Firefighter/Guardian | Reactive + Proactive | "Is it working?" |
| **platform-engineer** | Builder | Creative | "How do I build this?" |
| **web-developer** | Presenter | Communicative | "How do I show this?" |
| **researcher** | Synthesizer | Analytical | "What pattern is this?" |
| **personal** | Strategist | Reflective | "Where am I going?" |

**Insight**: Switching roles = switching cognitive modes, not just tools.

**Application**: When stuck in one role, try viewing problem through different role's lens.

---

## Pattern 2: Token Budgets Reveal Complexity Hierarchy

**Discovery**: Role token budgets directly correlate with domain complexity.

```
sre-devops:        30k tokens (most complex - production systems)
researcher:        25k tokens (high complexity - meta-analysis)
platform-engineer: 20k tokens (medium complexity - application building)
web-developer:     15k tokens (moderate complexity - UI/frontend)
personal:          10k tokens (lowest complexity - planning/reflection)

Shared:
  foundational:     3k tokens (always loaded - constitutional baseline)
  orchestration:    6k tokens (JIT - workflow coordination)
  context:          4k tokens (on-demand - knowledge management)
```

**Insight**: Complexity isn't about "harder" - it's about surface area of concerns.
- SRE: Production, monitoring, incidents, deployment, infrastructure, security (6 domains)
- Personal: Planning, growth, strategy (3 domains)

**Application**: Use token budget as proxy for "how much do I need to know?"

---

## Pattern 3: Shared Infrastructure Enforces Consistency

**Discovery**: All roles inherit same foundational profile (Laws, standards, git hooks).

**Structure**:
```
foundational (3k tokens) → Always loaded, never disabled
    ↓
All 5 roles inherit this baseline
    ↓
Constitution enforced universally
```

**What this achieves**:
- Same Laws apply to SRE work AND personal planning
- Same commit format across all domains
- Same hook enforcement regardless of role
- Institutional memory captured uniformly

**Insight**: Shared foundation enables cross-domain learnings to transfer.

**Example**:
- ADHD patterns discovered in personal → Inform 40% rule in sre-devops
- AgentOps Laws proven in research → Apply to web-developer docs

---

## Pattern 4: Roles Compose via Multi-Flavor Loading

**Discovery**: Real work often requires 2-3 roles simultaneously.

**Observed Combinations**:

| Work Type | Roles Loaded | Token Budget | Use Case |
|-----------|--------------|--------------|----------|
| Deploy app to production | sre-devops + platform-engineer | ~50k (25%) | Create app + monitor deployment |
| Build showcase website | web-developer + researcher | ~40k (20%) | Frontend + document patterns |
| Career planning with proof | personal + researcher | ~35k (17.5%) | Strategy + extract accomplishments |
| Full-stack feature | platform-engineer + web-developer | ~35k (17.5%) | Backend + frontend |

**Insight**: Token budgets designed for composition (stay under 40% rule).

**Design Principle**:
- Single role: 10-30k tokens
- Two roles: 20-50k tokens (~25% avg)
- Three roles: 40-75k tokens (~37.5% max)
- Leaves headroom for context growth during session

---

## Pattern 5: Triggers Enable Auto-Detection

**Discovery**: Each role defines keywords, file patterns, git patterns for auto-suggestion.

**Example Auto-Detection Flow**:

```
User says: "There's a production incident with Redis"

Keywords matched: "production", "incident", "redis"
  → Suggests: sre-devops role

File context: work/gitops/apps/redis/
  → Confirms: sre-devops role

Loads:
  - foundational (3k)
  - orchestration (Read CLAUDE.md) (2k)
  - sre-devops (monitoring, incidents, deployment agents) (8k)

Total: ~13k tokens (6.5%)
```

**Insight**: Role detection is contextual, not manual.

---

## Pattern 6: Knowledge Sources Form Feedback Loops

**Discovery**: Each role both consumes AND produces knowledge.

**Feedback Loop Structure**:

```
researcher → Extracts patterns → Documentation
    ↓
sre-devops → Uses patterns → Production work
    ↓
Git history (commits, session logs)
    ↓
researcher → Analyzes production work → New patterns
    ↓
Cycle repeats...
```

**Concrete Example**:

1. **researcher**: Analyzes 204 sessions, extracts "harmonize pattern"
2. **researcher**: Documents in `docs/reference/workflows/harmonize.md`
3. **sre-devops**: Uses harmonize workflow in production
4. **sre-devops**: Commits with learnings ("learned X about Y")
5. **researcher**: Analyzes new commit, refines pattern
6. **Loop**: Pattern improves over time through production usage

**Insight**: Knowledge OS is self-improving through role interaction.

---

## Pattern 7: Skills vs Agents vs Workflows (Reusability Hierarchy)

**Discovery**: 3 levels of reusability emerged across roles.

| Level | Description | Example | Token Cost | Reusability |
|-------|-------------|---------|------------|-------------|
| **Skills** | Deterministic scripts | `validate.sh`, `sync.sh` | 100-300 tokens | Very high (used by all roles) |
| **Agents** | Specialized workflows | `applications-create-app.md` | 2000-3000 tokens | Medium (role-specific) |
| **Workflows** | Multi-phase processes | Research→Plan→Implement | 600-800 tokens | High (shared across roles) |

**Reuse Pattern**:

```
sre-devops uses:
  - Skills: validate.sh, sync.sh, harmonize.sh (shared with platform-engineer)
  - Agents: incidents-response.md, monitoring-alerts.md (SRE-specific)
  - Workflows: debug-cycle.md (shared with platform-engineer)

platform-engineer uses:
  - Skills: validate.sh, test.sh, rendering.sh (shared with sre-devops)
  - Agents: applications-create-app.md (platform-specific)
  - Workflows: application-creation.md (shared with web-developer)
```

**Insight**: Skills are most reusable (lowest token cost), agents are most specialized.

**Design Implication**: Extract common logic into skills, keep domain specifics in agents.

---

## Pattern 8: Role Boundaries Reveal Domain Separation

**Discovery**: Where one role ends and another begins shows natural domain boundaries.

**Boundary Analysis**:

| Boundary | Role A | Role B | Handoff Point |
|----------|--------|--------|---------------|
| **Build → Deploy** | platform-engineer creates app | sre-devops deploys to production | `git push` (ArgoCD takes over) |
| **Deploy → Monitor** | sre-devops deploys | sre-devops monitors | Deployment complete → Alert setup |
| **Work → Document** | (any role) does work | web-developer documents | Feature complete → Tutorial creation |
| **Production → Research** | sre-devops operates | researcher analyzes | Session ends → Learning extraction |
| **Technical → Personal** | (any role) accomplishes | personal tracks in MCI | Milestone reached → Capability update |

**Insight**: Handoff points are where *context* needs to transfer cleanly.

**Application**: Bundle system enables handoffs (bundle captures context for next role).

---

## Pattern 9: MCP Integration Patterns by Role

**Discovery**: Different roles use different MCP servers.

| Role | MCP Servers Used | Why |
|------|------------------|-----|
| **sre-devops** | memory (incident tracking) | Remember past incidents, pattern match |
| **platform-engineer** | context7 (K8s API docs), memory (app patterns) | Latest API schemas, app creation patterns |
| **web-developer** | context7 (React/Next.js docs), podman (containers) | Frontend frameworks, dev environments |
| **researcher** | memory (pattern tracking), context7 (research latest) | Cross-session pattern analysis |
| **personal** | memory (growth tracking) | Long-term capability evolution |

**Shared Across All**: `memory` (institutional memory capture)

**Insight**: MCP server usage reveals role's external dependencies.

---

## Pattern 10: Git Patterns as Role Signatures

**Discovery**: Commit prefixes reveal which role was active.

| Git Pattern | Role | What It Signals |
|-------------|------|-----------------|
| `fix(ops):`, `fix(monitoring):` | sre-devops | Operational fix |
| `feat(apps):`, `feat(charts):` | platform-engineer | New application/chart |
| `feat(ui):`, `docs(tutorial):` | web-developer | Frontend or documentation |
| `docs(explanation):`, `docs(research):` | researcher | Framework development |
| `docs(life):`, `feat(career):` | personal | Personal planning |

**Application**: Git history reveals role activity distribution over time.

**Example Query**:
```bash
# How much time in each role last quarter?
git log --since="3 months ago" --pretty=format:"%s" | \
  grep -E "(ops|monitoring)" | wc -l  # SRE work
git log --since="3 months ago" --pretty=format:"%s" | \
  grep -E "(apps|charts)" | wc -l    # Platform work
# etc...
```

**Insight**: Git commits are role activity telemetry.

---

## Pattern 11: Documentation Follows Role Perspective

**Discovery**: Same system documented differently by each role.

**Example: ArgoCD Documentation**

| Role | Documentation Focus | File Path |
|------|---------------------|-----------|
| **sre-devops** | Troubleshooting sync issues | `docs/how-to/troubleshooting/argocd-debug.md` |
| **platform-engineer** | Creating ArgoCD applications | `docs/how-to/guides/create-argocd-app.md` |
| **web-developer** | ArgoCD UI/dashboard usage | `docs/tutorials/argocd-ui-guide.md` |
| **researcher** | ArgoCD pattern analysis | `docs/explanation/patterns/argocd-gitops.md` |

**Insight**: Diátaxis format maps to roles (How-to=sre, Tutorial=web-dev, Explanation=researcher).

---

## Pattern 12: Token Budget Composition Mathematics

**Discovery**: Designed for 2-3 role composition while staying under 40% rule.

**Composition Math**:

```
Foundational (always loaded):           3k
Orchestration (JIT - if needed):       +6k
Role 1 (primary):                     +20k
Role 2 (secondary):                   +15k
─────────────────────────────────────────
Total:                                 44k (22% of 200k context window)

Leaves 156k (78%) for:
  - Session work
  - File reading
  - Git operations
  - Validation output
  - Learning extraction
```

**Maximum Safe Composition** (3 roles):
```
Foundational:                           3k
Orchestration:                         +6k
sre-devops:                           +30k
platform-engineer:                    +20k
web-developer:                        +15k
─────────────────────────────────────────
Total:                                 74k (37% of 200k window) ✅ Under 40%!
```

**Insight**: Can load ALL 5 roles if needed:
```
Foundational + Orchestration + Context:  13k
All 5 roles (30k+20k+15k+25k+10k):    +100k
─────────────────────────────────────────
Total:                                 113k (56.5%) ⚠️ Over 40%
```

**Design Principle**: 40% rule prevents loading all roles simultaneously → forces intentional role selection.

---

## Pattern 13: Evolution Path Visible in Git History

**Discovery**: Roles evolved over time, visible in git commits.

**Evolution Timeline** (inferred from workspace):

1. **Phase 1 (2023-2024)**: Monolithic (everything in gitops, no roles)
2. **Phase 2 (Early 2025)**: Separation (work/ vs personal/)
3. **Phase 3 (Mid 2025)**: Specialization (12-factor-agentops, life, agentops-showcase)
4. **Phase 4 (Nov 2025)**: Explicit taxonomy (this role system)

**Git Evidence**:
- 538 commits in 60 days → High activity in Phase 2
- 204 sessions logged → Institutional memory capture began Phase 2
- 52 agents created → Specialization in Phase 3
- Role taxonomy → Formalization in Phase 4

**Insight**: Role boundaries emerged organically, then formalized explicitly.

---

## Pattern 14: Cross-Flavor Feedback Loops

**Discovery**: Technical work (work/) feeds personal development (personal/).

**Feedback Loop**:

```
sre-devops (work/gitops/) → Build AgentOps framework
    ↓
Git metrics: 40x speedup, 95% success rate
    ↓
researcher (personal/12-factor-agentops/) → Extract patterns
    ↓
Document framework, compliance audits
    ↓
personal (personal/life/) → Track in MCI, update resume
    ↓
Career leverage: NVIDIA application with proof
    ↓
Visibility campaign: LinkedIn, YouTube
    ↓
Community adoption → New use cases
    ↓
researcher → Analyze new use cases → Improve framework
    ↓
Loop repeats...
```

**Insight**: Work and personal are not separate - they're symbiotic feedback loops.

---

## Pattern 15: Role-Specific Validation Strategies

**Discovery**: Each role validates work differently.

| Role | Validation Method | Tools | Success Criteria |
|------|-------------------|-------|------------------|
| **sre-devops** | Monitoring, alerts, production health | Prometheus, ArgoCD | Zero downtime, SLO met |
| **platform-engineer** | Tests, builds, manifests | `make test-app`, yamllint | CI passes, app deploys |
| **web-developer** | Visual testing, cross-browser | Browser DevTools, Lighthouse | Renders correctly, accessible |
| **researcher** | Peer review, production usage | Git history analysis | Pattern adopted in production |
| **personal** | Career outcomes, opportunities | Resume, interviews, offers | Goal achieved, growth measured |

**Insight**: Validation is role-contextual, not universal.

---

## Summary: The Taxonomy as Knowledge Architecture

**Key Discovery**: Roles aren't just organizational - they're architectural.

**What This Taxonomy Achieves**:

1. **Discoverability**: "I'm doing X" → Load role Y
2. **Composition**: Mix roles without exceeding 40% rule
3. **Consistency**: Shared foundation across all roles
4. **Evolution**: Roles can specialize independently
5. **Feedback**: Cross-role learnings transfer cleanly
6. **Measurement**: Token budgets reveal complexity
7. **Optimization**: Reusable skills reduce duplication
8. **Continuity**: Bundles enable multi-session work
9. **Visibility**: Git patterns show role distribution
10. **Self-awareness**: System knows which role is active

**Meta-Pattern**: Role-based taxonomy is a *knowledge architecture*, not just a filing system.

---

**Total Patterns Extracted**: 15

**Reusability**: These patterns apply beyond this workspace (generalizable).

**Next**: Document this taxonomy for others to adopt.
