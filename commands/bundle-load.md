---
description: Load saved research/plan bundles from filesystem (search first with /bundle-search)
---

# /bundle-load - Restore Context Bundle

**Purpose:** Load previously saved bundles into your current context window from filesystem.

**Why this matters:** Loading bundles restores prior research and plans without re-discovering context. This enables multi-session work and team collaboration - anyone can load the same bundle.

**TIP:** Don't remember the bundle name? Use `/bundle-search` first for semantic discovery!

**Token budget:** 1-2k total (includes bundle + metadata)

---

## Opus 4.5 Behavioral Standards

<search_first_pattern>
If unsure of the bundle name, use `/bundle-search` first. Semantic search finds related bundles even if you don't remember the exact name.
</search_first_pattern>

<progress_file_awareness>
When loading plan bundles, check for associated progress files (`feature-list.json`, `claude-progress.json`). Display current state if found.
</progress_file_awareness>

---

## Quick Start

### If You Know the Bundle Name

```bash
/bundle-load redis-caching-research
```

### If You Don't Remember the Name

```bash
# Step 1: Search semantically
/bundle-search "redis caching performance"

# Step 2: Load the bundle you found
/bundle-load redis-caching-research
```

---

## How It Works

**Filesystem (Source of Truth)**
- Bundles stored in `.agents/bundles/{name}.md`
- Git-tracked, persistent, always available
- Use `/bundle-search` to find bundles by name, tag, or content

---

## Usage Patterns

### Pattern 1: Direct Load (You Know the Name)

```bash
/bundle-load redis-caching-research

# Output:
# ✅ Loaded: redis-caching-research.md (1.2k tokens)
# Created: 2025-11-10
# Tags: redis, caching, architecture
#
# [Bundle content loads into context]
```

### Pattern 2: Search + Load (You Forgot the Name)

```bash
# Step 1: Semantic search
/bundle-search "kubernetes deployment strategies"

# Output shows:
# 1. k8s-migration-research (95% relevance)
# 2. container-orchestration-plan (87% relevance)

# Step 2: Load what you need
/bundle-load k8s-migration-research
```

### Pattern 3: Multi-Repository Load

**Bundles automatically searched across ALL repositories:**
- `workspace/.agents/bundles/`
- `gitops/.agents/bundles/`
- `example-repo/.agents/bundles/`
- `12-factor-agentops/.agents/bundles/`
- `life/.agents/bundles/`
- All other repos with `.agents/bundles/`

```bash
/bundle-load agentops-roadmap

# Searches all repos, loads from whichever has it
# Shows: "Loaded from: gitops/.agents/bundles/agentops-roadmap.md"
```

### Pattern 4: Session Continuity Load

```bash
# Load bundle and initialize session
/bundle-load myproject-progress
/session-start

# Or combined (if in long-running project)
/bundle-load myproject-progress --session-start
```

This pattern:
1. Loads bundle context
2. Runs session-initializer skill
3. Reads progress files
4. Identifies next work item

---

## Progress File Detection

**When loading plan bundles, `/bundle-load` auto-displays progress state:**

```
✅ Loaded: myproject-plan.md (1.5k tokens)
📊 Progress files detected:
   - feature-list.json (8/12 features complete)
   - claude-progress.json (last session: 2025-11-26)

Current state:
   Working on: feature-009 (Add validation layer)
   Next steps: 3 items queued
   Blockers: None

💡 Run /session-start to continue where you left off
```

**Bundles = snapshots (read-only), Progress files = live state (read-write)**

---

## Example Workflows

### Research → Plan → Implement

```bash
# Day 1: Save research
/bundle-save caching-research

# Day 2: Load research, create plan
/bundle-load caching-research
/plan caching-research.md
/bundle-save caching-plan

# Day 3: Load plan, implement
/bundle-load caching-plan
/implement caching-plan.md
```

### Team Collaboration

```bash
# Teammate 1: Creates bundle
/bundle-save redis-implementation
# Shares: "I saved redis-implementation bundle"

# Teammate 2: Finds and loads
/bundle-search "redis"
# → Shows: redis-implementation (98% relevance)
/bundle-load redis-implementation
# → Loads same research/plan
```

### Multi-Session Work

```bash
# Week 1: Research
/bundle-save k8s-migration-research

# Week 2: Load + extend
/bundle-load k8s-migration-research
# [Do more research]
/bundle-save k8s-migration-research  # Overwrite with updated version

# Week 3: Create plan
/bundle-load k8s-migration-research
/plan k8s-migration-research.md
/bundle-save k8s-migration-plan
```

---

## Load Strategies

### Strategy 1: Exact Name Match

```bash
/bundle-load redis-caching-research
# Fastest if you know exact name
```

### Strategy 2: Partial Match

```bash
/bundle-load redis-caching
# Matches: redis-caching-research.md
# If multiple matches, shows list to choose from
```

### Strategy 3: UUID Match

```bash
/bundle-load bundle-abc123def456
# Loads by UUID (useful for team sharing)
```

### Strategy 4: Tag Filter + Search

```bash
# Use bundle-search with tags
/bundle-search "performance" --tags redis,database

# Then load
/bundle-load redis-caching-research
```

---

## Multi-Repository Loading

**Automatic search across all repositories:**

```bash
$ /bundle-load agentops-roadmap

# Searching repositories:
# - workspace/.agents/bundles/
# - gitops/.agents/bundles/
# - 12-factor-agentops/.agents/bundles/
# - example-repo/.agents/bundles/
# - life/.agents/bundles/
# - [all other repos]

# Found: gitops/.agents/bundles/agentops-roadmap-complete.md
# ✅ Loaded (1.5k tokens)
```

**Shows which repository the bundle came from** for transparency.

---

## Bundle Not Found?

### Option 1: Search Semantically

```bash
/bundle-search "your topic"
# Semantic search finds related bundles even if name doesn't match
```

### Option 2: List All Bundles

```bash
/bundle-list
# Shows all bundles across all repos
```

### Option 3: Check Repository

```bash
# Maybe bundle is in different repo
ls .agents/bundles/
ls ../example-repo/.agents/bundles/
ls ../12-factor-agentops/.agents/bundles/
```

---

## Loading Best Practices

**1. Search First (When Unsure)**
```bash
# Better UX - let semantic search find it
/bundle-search "your topic"
/bundle-load <result>
```

**2. Direct Load (When Certain)**
```bash
# Faster if you know exact name
/bundle-load exact-name
```

**3. Tag Your Bundles Well**
```bash
# When saving, use good tags
/bundle-save my-research
# Tags: redis, caching, performance  ← Makes it searchable
```

**4. Use Descriptive Names**
```bash
# Good names
redis-caching-research
k8s-migration-plan-approved
argocd-debugging-solution

# Bad names
research-1
stuff
mywork
```

---

## Troubleshooting

### Bundle Not Found

```bash
❌ Bundle not found: redis-research

Suggestions:
1. Search by keyword: /bundle-search "redis"
2. Check all bundles: /bundle-list
3. Check repository: ls .agents/bundles/
```

### Multiple Matches

```bash
⚠️  Multiple bundles match "redis":

1. redis-caching-research (gitops) - 2025-11-10
2. redis-deployment-plan (example-repo) - 2025-11-08
3. redis-performance-tuning (12-factor-agentops) - 2025-11-05

Which bundle? (enter number or full name)
```

### Bundle Too Large

```bash
⚠️  Bundle is 3.5k tokens (recommended max: 2k)

Options:
1. Load anyway (may impact context budget)
2. Request compressed version
3. Load specific sections only
```

---

## Related Commands

- `/bundle-save` - Save new bundles
- `/bundle-search` - Search bundles by name, tag, or content
- `/bundle-list` - List all available bundles
- `/session-start` - Initialize session with progress file detection
- `/plan` - Create implementation plan (generates progress files)

---

## Quick Reference

```bash
# Discovery (use first if unsure)
/bundle-search "topic or keyword"  # Semantic search

# Loading (direct)
/bundle-load <name>                # Load by name
/bundle-load <uuid>                # Load by UUID
/bundle-load <partial>             # Load by partial match

# Listing (browse all)
/bundle-list                       # All bundles across all repos
/bundle-list --tag redis           # Filter by tag
/bundle-list --recent 7d           # Last 7 days
```

---

## Next Steps

1. **Try searching first** - `/bundle-search "your topic"`
2. **Load what you find** - `/bundle-load <name>`
3. **Save new bundles** - `/bundle-save`

**Git-tracked filesystem ensures bundles are always available and shareable.**
