---
name: vibe-docs
description: >
  Validate documentation claims against deployment reality. Use when checking
  if docs accurately describe what's deployed, finding false claims, or auditing
  doc accuracy. Triggers: "verify docs", "doc audit", "check doc claims",
  "docs match reality", "validate doc status".
version: 1.0.0
author: "AI Platform Team"
context: fork
allowed-tools: "Read,Glob,Grep,Bash,Task"
skills:
  - standards
---

# Vibe-Docs Skill

Semantic validation for documentation - verifies claims match deployment reality.

## Overview

**Structure vs Semantic:**
- `/doc` validates STRUCTURE (links, coverage, sections)
- `/vibe-docs` validates TRUTH (do claims match reality?)

**When to Use:**
- After deployment changes
- Before releases
- When docs feel "stale"
- Periodic audits (monthly)

---

## Commands

| Command | Action |
|---------|--------|
| `status` | Check status claims against deployment |
| `agents` | Verify agent catalog matches cluster |
| `images` | Verify IMAGE-LIST matches registry |
| `full` | Run all checks |

---

## Phase 1: Gather Claims

Extract claims from documentation:

```bash
# Status claims
grep -r "Current Status:" docs/code-map/ | grep -E "RUNNING|FAILED|DEPLOYED"

# Agent claims
grep -E "✅ Deployed|✅ Ready|DEPLOYED" docs/

# Image claims
grep -E "image:|Image:" charts/*/IMAGE-LIST.md
```

---

## Phase 2: Gather Reality

Query deployment state:

### For Agents (KAgent)

```bash
# Get all agents and their status
oc get agents.kagent.dev -n ai-platform -o json | \
  jq -r '.items[] | "\(.metadata.name): \(.status.conditions[0].status)"'
```

### For Pods/Services

```bash
# Get running pods
oc get pods -n ai-platform -o json | \
  jq -r '.items[] | select(.status.phase=="Running") | .metadata.name'
```

### For Images

```bash
# Get deployed images
oc get pods -n ai-platform -o json | \
  jq -r '.items[].spec.containers[].image' | sort -u
```

---

## Phase 3: Compare

Cross-reference claims vs reality:

### Mismatch Categories

| Category | Severity | Example |
|----------|----------|---------|
| **False Positive** | CRITICAL | Doc says RUNNING, pod doesn't exist |
| **False Negative** | HIGH | Pod running, doc says PLANNED |
| **Stale Date** | MEDIUM | Validation date > 30 days old |
| **Missing Validation** | LOW | Status without date |

### Output Format

```
===================================================================
              DOCUMENTATION REALITY CHECK
===================================================================
Repository: ai-platform
Cluster: ocppoc
Generated: 2026-01-04

CRITICAL: False Claims (3)
-------------------------------------------------------------------
| Document | Claims | Reality |
|----------|--------|---------|
| knowledge-assistant.md | RUNNING | Accepted=False |
| faq.md | "Knowledge Assistant deployed" | Not in agent list |

HIGH: Undocumented Reality (1)
-------------------------------------------------------------------
| Reality | Missing From |
|---------|--------------|
| relay-agent running | docs/agents/catalog.md |

MEDIUM: Stale Validation (5)
-------------------------------------------------------------------
| Document | Last Validated | Age |
|----------|----------------|-----|
| etl-service.md | 2025-11-15 | 50 days |

SUMMARY: 9 issues (3 critical, 1 high, 5 medium)
===================================================================
```

---

## Phase 4: Generate Fixes

For each mismatch, propose fix:

```markdown
### Fix: knowledge-assistant.md

**Claim:** `## Current Status: ✅ RUNNING`
**Reality:** Accepted=False (pod failed)

```diff
- ## Current Status: ✅ RUNNING
+ ## Current Status: ❌ FAILED
+ Status: Accepted=False (CRD exists but agent not running)
+ Validated: 2026-01-04 against ocppoc cluster
```
```

---

## Ground Truth Files

These files are authoritative - other docs MUST reference them:

| Domain | Ground Truth | Check Command |
|--------|--------------|---------------|
| Agents | `docs/agents/catalog.md` | Compare with `oc get agents` |
| Images | `charts/*/IMAGE-LIST.md` | Compare with pod images |
| Config | `values.yaml` | Compare with deployed configmaps |

---

## Integration with /doc

```bash
# Structural validation (existing)
/doc coverage

# Semantic validation (this skill)
/vibe-docs full

# Combined audit
/doc coverage && /vibe-docs full
```

---

## Automation

### Pre-commit Hook (Optional)

```bash
# .git/hooks/pre-commit
if git diff --cached --name-only | grep -q "docs/"; then
  echo "Docs changed - running vibe-docs status check"
  # Run lightweight status check
fi
```

### CI Integration

```yaml
# .gitlab-ci.yml
doc-audit:
  stage: validate
  script:
    - /vibe-docs status
  rules:
    - changes:
        - docs/**/*
```

---

## Anti-Patterns

| DON'T | DO INSTEAD |
|-------|------------|
| Skip reality check for "known good" docs | Always verify claims |
| Trust old validation dates | Re-validate if >30 days |
| Assume catalog is current | Cross-check with cluster |
| Fix docs without updating validation date | Always update date |

---

## References

- **Validation Rules**: `doc/references/validation-rules.md`
- **Code-Map Standard**: `code-map-standard/SKILL.md`
- **Learning**: `.agents/learnings/2026-01-04-doc-audit-gap.md`

---

## Related Skills

| Skill | Purpose |
|-------|---------|
| `doc` | Structural validation |
| `code-map-standard` | Doc format standard |
| `vibe` | Code semantic validation |
