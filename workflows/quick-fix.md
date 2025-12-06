---
name: quick-fix
description: Fast implementation for simple, low-risk changes
estimated_time: 10-30 minutes
phases: 2
---

# Quick Fix Workflow

**Purpose:** Rapid implementation of straightforward changes

**When to use:**
- Simple changes (1-2 files, obvious solution)
- Low risk (non-critical systems, reversible)
- Well-known patterns (done many times before)
- Time-critical (production hotfix)

**Token budget:** 20-40k tokens (single session)

---

## Workflow Phases

```
Phase 1: Orient & Implement (15-30% context)
   ↓
Phase 2: Validate & Commit (5-10% context)
```

**Key principle:** Skip research/planning for simple changes

---

## Phase 1: Orient & Implement

**Commands:**
```bash
Read CLAUDE.md-simple                     # Quick orientation
# Describe simple change
# Agent implements directly
```

**What happens:**
1. Load constitutional foundation (2k tokens)
2. Understand change request
3. Make change immediately
4. Validate inline

**Examples of quick fixes:**
- Fix typo in documentation
- Update dependency version
- Add environment variable
- Adjust configuration value
- Fix linting error

**Token budget:** 15-30k tokens

---

## Phase 2: Validate & Commit

**Commands:**
```bash
/validate --quick                 # Fast syntax + test check
# Commit with message
```

**What happens:**
1. Run quick validation (syntax, build, unit tests)
2. If pass: Commit with context
3. If fail: Fix and retry

**Token budget:** 5-10k tokens

---

## When to Use vs Complete Cycle

### Use Quick Fix when:
✅ Single file or small change
✅ Solution is obvious
✅ Risk is low
✅ Time is limited

### Use Complete Cycle when:
❌ Multiple files with dependencies
❌ Solution is unclear
❌ Risk is high (critical system, production)
❌ Pattern should be extracted (learning opportunity)

---

## Example: Fix Typo

```bash
Read CLAUDE.md-simple
# User: "Fix typo in README.md line 45, 'recieve' → 'receive'"

# Agent:
# ✅ Reads README.md
# ✅ Edits line 45
# ✅ Validates markdown syntax
# ✅ Commits: "docs(readme): Fix typo - recieve → receive"

# Total time: 2 minutes
# Token budget: 5k tokens
```

## Example: Update Dependency

```bash
Read CLAUDE.md-simple
# User: "Update golang-jwt/jwt from v4.5.0 to v4.5.1"

# Agent:
# ✅ Edits go.mod:12
# ✅ Runs: go mod tidy
# ✅ Runs: go test ./...
# ✅ Commits: "chore(deps): Update golang-jwt/jwt to v4.5.1"

# Total time: 5 minutes
# Token budget: 10k tokens
```

## Example: Add Environment Variable

```bash
Read CLAUDE.md-simple
# User: "Add LOG_LEVEL=info to config/app.yaml"

# Agent:
# ✅ Edits config/app.yaml:8
# ✅ Validates YAML syntax
# ✅ Verifies app reads LOG_LEVEL
# ✅ Commits: "feat(config): Add LOG_LEVEL environment variable"

# Total time: 8 minutes
# Token budget: 12k tokens
```

---

**Start quick fix with:** `Read CLAUDE.md-simple` → describe simple change
