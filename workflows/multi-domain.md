---
name: multi-domain
description: Coordinate work spanning multiple domains simultaneously
estimated_time: Varies (multi-session)
phases: 3
---

# Multi-Domain Workflow

**Purpose:** Work across multiple domains (technical + personal, product + infrastructure) simultaneously

**When to use:**
- Cross-domain projects (backend + frontend + infrastructure)
- Technical + personal tracking (work + career development)
- Multi-flavor coordination (devops + product-dev + life)
- Complex organizational change (code + process + documentation)

**Token budget:** 60-120k tokens across 2-4 sessions (using bundles)

---

## Workflow Phases

```
Phase 1: Domain Identification (5-10% context)
   ↓
Phase 2: Parallel Execution (30-40% context per domain)
   ↓
Phase 3: Integration & Learning (10-15% context)
```

**Key principle:** Use context bundles to prevent collapse when spanning domains

---

## Phase 1: Domain Identification

**Goal:** Identify all domains involved

**Activities:**
- List technical domains (backend, frontend, infrastructure, data)
- List non-technical domains (documentation, process, personal)
- Map dependencies between domains
- Plan execution order (parallel where possible)

**Output:** Domain map with dependencies

---

## Phase 2: Parallel Execution

**Goal:** Execute work in each domain

**Pattern:**
```bash
# Domain 1: Technical work
Read CLAUDE.md
/research "[technical-topic]"
/bundle-save technical-research
# Continue technical workflow...

# Domain 2: Personal tracking (separate session or flavor)
# Load personal flavor bundle
/bundle-load life-career-2025
# Track accomplishment in Master Capability Inventory
# Update career metrics
```

**Key:** Use bundles to switch between domains without losing context

**Output:** Work complete in each domain

---

## Phase 3: Integration & Learning

**Goal:** Connect learnings across domains

**Activities:**
- Verify technical work completed
- Verify personal tracking updated
- Extract cross-domain patterns
- Document insights

**Output:** Integrated learning + cross-domain patterns

---

## Example: Build K8s App + Track Career Growth

**Session 1: Technical (DevOps domain)**
```bash
Read CLAUDE.md
/research "create Kubernetes application"
/plan k8s-app-research
/implement k8s-app-plan
/validate
# ✅ App deployed successfully
/bundle-save k8s-app-complete
```

**Session 2: Personal (Life domain)**
```bash
# Switch to life flavor
/bundle-load life-career-2025
/capability-auditor
# Add to Master Capability Inventory:
# - Skill: Kubernetes application deployment
# - Evidence: [commit SHA]
# - Impact: Deployed production app
# - Level: Intermediate → Advanced
/bundle-save life-career-updated
```

**Session 3: Integration**
```bash
# Both bundles loaded (technical + personal)
/learn k8s-deployment
# Pattern extracted:
# - Technical: K8s app deployment pattern
# - Personal: Career advancement evidence

# Result:
# - Technical accomplishment documented
# - Personal metrics updated
# - Pattern available for reuse
# - Cross-domain tracking maintained
```

---

## Multi-Flavor Coordination

**What are flavors?**
- **Technical flavors:** devops, product-dev, infrastructure-ops, data-eng
- **Personal flavors:** life, career, learning
- **Process flavors:** documentation, operations, governance

**Coordination pattern:**
```bash
# Work in technical flavor
cd workspace/gitops/
# Use devops workflows
Read CLAUDE.md
[do work]
/bundle-save technical-work

# Switch to personal flavor
cd workspace/life/
# Use personal workflows
/bundle-load life-career-2025
[track progress]
/bundle-save personal-tracking

# Both bundles available for future reference
# Technical work feeds personal metrics automatically
```

---

## When to Use Multi-Domain vs Single Domain

### Use Multi-Domain when:
✅ Work spans multiple areas (technical + docs + personal)
✅ Tracking across domains needed (work + career)
✅ Complex organizational change (code + process + culture)
✅ Integration points exist between domains

### Use Single Domain when:
- Work is purely technical (or purely personal)
- No cross-domain tracking needed
- Simple, focused task

---

## Related Documentation

- **Multi-flavor coordination:** `docs/reference/multi-flavor-coordination.md`
- **Context bundles:** `core/commands/bundle-save.md`, `core/commands/bundle-load.md`
- **40% rule:** `core/CONSTITUTION.md`

---

**Start multi-domain work with:** Domain mapping → Parallel execution → Integration
