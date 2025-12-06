---
description: Save compressed research/plan bundles to filesystem
---

# /bundle-save - Archive Context Bundle

**Purpose:** Save compressed research or plan bundles for reuse across sessions.

**Why this matters:** Bundles preserve discoveries and plans across context refreshes. Without bundles, each session starts from scratch. Small, focused bundles keep context budgets healthy.

**When to use:**
- After completing research phase - save findings for future reference
- After creating plan - archive for team coordination
- When discovering reusable patterns - capture for institutional memory

**Token budget:** 5-10k total (2-5% context window)

---

## Opus 4.5 Behavioral Standards

<compression_discipline>
Bundles should be <2k tokens. If larger, split into sub-bundles. Remove working notes, keep insights. Extract key findings as bullet points.
</compression_discipline>

<descriptive_naming>
Use descriptive, searchable names: `redis-caching-research` not `stuff`. Include type suffix: `-research`, `-plan`, `-pattern`.
</descriptive_naming>

---

## How It Works

**Filesystem (Source of Truth)**
- Saves to `.agents/bundles/{name}.md`
- Git-tracked, persistent, editable
- Works offline, survives restarts
- Discoverable via `/bundle-search` (searches filesystem)

---

## Step 1: Identify Bundle Content

You have one of these:
- **Research bundle** - From `/research` phase (500-1k tokens)
- **Plan bundle** - From `/plan` phase (1-2k tokens)
- **Other discoveries** - Patterns, solutions, findings from sessions

**Example research bundles:**
```
- redis-caching-architecture (technical discovery)
- argocd-sync-troubleshooting (debugging solution)
- kyverno-policy-pattern (infrastructure pattern)
```

**Example plan bundles:**
```
- applications-create-app (implementation strategy)
- database-migration (operational plan)
```

## Step 2: Define Bundle Name

Choose a descriptive name that helps future you find it:

```
Format: [topic]-[type]
Examples:
  redis-caching-research
  argocd-debugging-plan
  kyverno-policy-implementation
  architecture-review-findings
```

## Step 3: Save Bundle (Auto-Indexing)

**Provide:**
1. Bundle name (as described above)
2. Bundle content (paste research/plan output, or reference file)
3. Optional: Tags for filtering (redis, argocd, security, etc.)

**I will:**
1. Validate bundle is <2k tokens (size limit)
2. Extract key insights for index
3. **Save to `.agents/bundles/{name}.md`**
4. Store metadata: creation date, tags, UUID
5. Remind about progress files: "Don't forget to update `claude-progress.json` before ending session"

## Step 4: Confirmation

Output confirmation:
```
✅ Bundle saved successfully

📁 Location: .agents/bundles/redis-caching-research.md
   Size: 1.2k tokens
   Created: 2025-11-10 16:45
   Tags: redis, caching, architecture
   UUID: bundle-abc123def456

Access later:
  - Direct load: /bundle-load redis-caching-research
  - Search: /bundle-search "caching"
```

---

## Example Usage

### Research Bundle Workflow

```
1. Run: /research "How should we implement Redis caching layer?"
   Output: research.md (findings, patterns, constraints)

2. Run: /bundle-save
   Input:
     - Name: redis-caching-research
     - Content: [paste research findings]
     - Tags: redis, caching, architecture

   Output:
     ✅ Saved to filesystem

3. Later (different session):

   # Option A: Direct load (if you know the name)
   /bundle-load redis-caching-research

   # Option B: Semantic search (if you forgot the name)
   /bundle-search "caching performance"
   → Shows: redis-caching-research (95% relevance)
   /bundle-load redis-caching-research
```

### Plan Bundle Workflow

```
1. Run: /plan redis-caching-research.md
   Output: plan.md (file-by-file changes, test strategy)

2. Run: /bundle-save
   Input:
     - Name: redis-caching-implementation
     - Content: [paste plan]
     - Tags: redis, implementation, approved

   Output:
     ✅ Saved (1.8k tokens)

3. Team coordination:
   → Share: "Check /bundle-search 'redis implementation'"
   → Team finds bundle via semantic search
   → Multiple people can load same plan
```

---

## Writing Good Bundles

**What makes a bundle useful:**

- Captures key decisions with rationale
- Summarizes findings (not copy-paste of full research)
- Lists concrete next steps
- Is small enough to load quickly (~500-2000 tokens)

**What to include:**

1. **Key findings** - The important discoveries
2. **Decisions made** - What you chose and why
3. **Parameters** - Concrete values for next phase
4. **Next steps** - Clear, actionable items

**What to leave out:**

- Exploration paths that didn't work
- Verbose explanations (summarize instead)
- Intermediate debugging steps
- Full file contents (reference paths instead)

**Example bundle:**

```markdown
# Redis Caching Architecture

**Key Finding:** Use Redis with pub/sub pattern
**Why:** 3x faster than database cache, handles 10k msgs/sec
**Constraints:** Must implement circuit breaker, 500MB memory limit
**Next Steps:**
- Implement Redis client in src/cache/
- Add circuit breaker wrapper
- Write integration tests
```

---

## Bundle Storage

**Filesystem (Persistent)**
```
.agents/bundles/
├── redis-caching-research.md
├── argocd-debugging-plan.md
├── kyverno-policy-pattern.md
└── index.json (metadata for all bundles)
```

**Metadata (auto-generated):**
```json
{
  "name": "redis-caching-research",
  "created": "2025-11-10T16:45:00Z",
  "size_tokens": 1200,
  "tags": ["redis", "caching", "architecture"],
  "uuid": "bundle-abc123def456",
  "source": "research phase",
  "accessed": ["2025-11-10T16:45", "2025-11-11T09:15"],
  "size_tokens": 1200,
  "repository": "gitops"
}
```

---

## Bundle Lifecycle

### Creation
```
/research → /bundle-save → filesystem
```

### Discovery
```
# Option 1: Know the name
/bundle-load redis-caching-research

# Option 2: Semantic search
/bundle-search "caching optimization"
→ /bundle-load redis-caching-research
```

### Access
```
/bundle-load redis-caching-research → loads from filesystem
```

### Updates
```
# Option 1: Overwrite
/bundle-save redis-caching-research (new content)
→ replaces filesystem

# Option 2: Version
/bundle-save redis-caching-research-v2
→ creates new version
```

---

## Constraints & Best Practices

**Size limits:**
- Max 2k tokens per bundle (enforced)
- If larger: Split into sub-bundles
- Example: research-architecture (1.5k) + research-implementation (1.2k)

**Naming conventions:**
- Use hyphens: `redis-caching-research`
- Include type: `-research`, `-plan`, `-pattern`
- Searchable: `team-onboarding-plan` better than `stuff`

**Tags (important for search):**
- Tags enable filtering when searching bundles
- Keep it short (3-5 tags max)
- Be specific: `redis, caching, performance` better than `backend, stuff`
- Examples: redis, argocd, kubernetes, security, monitoring

---

## Integration with Other Commands

**Full workflow:**

```bash
# Day 1: Research
$ /research "How to implement caching?"
# → research.md output (5k tokens)

$ /bundle-save
# Name: caching-research
# Tags: redis, caching, performance
# → Saved to filesystem ✅

# Day 2: Planning (fresh context)
$ /bundle-search "caching"
# → Shows: caching-research

$ /bundle-load caching-research
# → Loads from filesystem (1k tokens)

$ /plan caching-research.md
# → plan.md output

$ /bundle-save
# Name: caching-implementation-plan
# → Saved ✅

# Day 3: Implementation
$ /bundle-load caching-implementation-plan
$ /implement caching-implementation-plan.md
# → execute approved plan

# Weeks later: Forgot the name
$ /bundle-search "redis"
# → Finds both bundles
```

---

## Related Commands

- `/bundle-search` - Search for bundles in filesystem
- `/bundle-load` - Load bundle from filesystem
- `/bundle-list` - List all bundles

---

## Next Steps

1. **Save this bundle** - Use /bundle-save with your research/plan
2. **Discover via search** - Use /bundle-search to find bundles
3. **Load efficiently** - Use /bundle-load to get compressed context

**Bundles are stored in git-tracked filesystem - persistent and shareable.**

---

## Progress File Reminder

**For long-running projects, don't forget to update progress files before ending your session:**

```bash
# Update claude-progress.json with:
# - What you completed this session
# - Current blockers
# - Next steps for future sessions

# Bundles = snapshots (read-only)
# Progress files = live state (read-write)
```

**Key insight:** Bundles capture the plan, progress files track execution state across sessions.
