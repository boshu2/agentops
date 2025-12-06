---
description: Garbage collection with knowledge extraction - extract patterns before archiving/deleting bundles
---

# /bundle-prune - GC-as-Learning

**Purpose:** Extract wisdom from bundles before archiving or deleting them.

**Philosophy:** Garbage collection IS knowledge extraction. Never delete without learning.

**When to use:**
- Bundle is stale (not accessed in 30+ days)
- Bundle superseded by newer version
- Consolidating related bundles
- Cleaning up after project completion

**Token budget:** 5-15k tokens (2.5-7.5% of context window)

**Output:** Extracted patterns + archived/deleted bundle

---

## The GC-as-Learning Philosophy

**"Every bundle surrenders its wisdom before deletion."**

Traditional GC: Delete old files → Knowledge lost forever

GC-as-Learning: Analyze → Extract patterns → Archive/Delete → Knowledge preserved

**Four questions before any deletion:**
1. What patterns are in this bundle?
2. What decisions were documented?
3. What gotchas were discovered?
4. What evidence was captured?

---

## Quick Start

```bash
# Prune a specific bundle
/bundle-prune redis-caching-research

# Prune all stale bundles (>30 days)
/bundle-prune --stale

# Interactive mode (review each bundle)
/bundle-prune --interactive

# Dry run (show what would happen)
/bundle-prune --dry-run --stale
```

---

## Implementation (Guided Automation)

### Step 1: Identify Prune Candidates

**I will find bundles to prune:**

```
Scanning bundles...

Candidates for pruning:

STALE (not accessed >30 days):
  1. redis-caching-research.md (45 days, 1.2k tokens)
  2. k8s-migration-plan.md (38 days, 1.8k tokens)
  3. auth-debugging-notes.md (31 days, 0.9k tokens)

SUPERSEDED (newer version exists):
  4. bundle-v1.md → superseded by bundle-v2.md

CONSOLIDATION CANDIDATES (related bundles):
  5. auth-research.md + auth-plan.md → could merge

Total: 5 candidates, 6.2k tokens to review
```

### Step 2: Extract Patterns (Per Bundle)

**For each bundle, I will analyze:**

```
Analyzing: redis-caching-research.md

Pattern indicators found:
├─ Problem/Solution structure ✓
├─ Evidence with metrics ✓
├─ Implementation steps ✓
└─ Gotchas documented ✓

Extractable content:
1. ✨ Pattern: Connection Pooling Strategy
   Problem: Redis exhaustion under burst
   Evidence: 10x latency improvement
   → Extract to: patterns/implementation/

2. ✨ Pattern: Cache Invalidation via Pub/Sub
   Problem: Stale cache after writes
   Evidence: <100ms propagation
   → Extract to: patterns/implementation/

3. 📝 Decision: Why Redis over Memcached
   Context: Needed pub/sub + persistence
   → Already in bundle, no separate pattern needed

4. ⚠️  Gotcha: Pool size vs memory trade-off
   → Include in Connection Pooling pattern

Extract 2 patterns from this bundle? [Y/n]
```

### Step 3: Generate Patterns

**For approved extractions, I will:**

1. Create pattern file from extracted content
2. Link to source bundle (for provenance)
3. Add to `.agentops/patterns/{category}/`
4. Update INDEX.md

```
✅ Created: patterns/implementation/redis-connection-pooling.md
✅ Created: patterns/implementation/cache-invalidation-pubsub.md
✅ Updated: patterns/INDEX.md

Patterns extracted. Ready to archive bundle.
```

### Step 4: Archive or Delete

**After extraction, I will prompt:**

```
Patterns extracted from redis-caching-research.md

Actions:
  [A]rchive - Move to archive/2025-11/ (recoverable)
  [D]elete - Remove permanently (not recoverable)
  [K]eep - Cancel pruning, keep bundle active
  [S]kip - Skip this bundle, continue with others

Choice: [A/D/K/S]
```

**Archive action:**
```bash
# Move to archive with timestamp
mv .agents/bundles/work/redis-caching-research.md \
   .agents/bundles/archive/2025-11/redis-caching-research.md
```

**Delete action:**
```bash
# Remove permanently (patterns already extracted)
rm .agents/bundles/work/redis-caching-research.md
```

### Step 5: Update Indexes

**After archive/delete, I will:**

1. Remove from MCP Memory (if indexed)
2. Update metadata.yaml
3. Update graph.yml (if referenced)
4. Log pruning action

```
✅ Removed from MCP Memory index
✅ Updated metadata.yaml
✅ Updated graph.yml (removed relationship)
✅ Logged: "Pruned redis-caching-research.md, extracted 2 patterns"
```

---

## Command Options

### Prune Specific Bundle
```bash
/bundle-prune [bundle-name]
# Analyzes single bundle, extracts patterns, archives/deletes
```

### Prune Stale Bundles
```bash
/bundle-prune --stale
# Finds all bundles not accessed in 30+ days
# Processes each with extraction

/bundle-prune --stale --days 60
# Custom threshold (60 days instead of 30)
```

### Interactive Mode
```bash
/bundle-prune --interactive
# Reviews each candidate one-by-one
# Prompts for each extraction and action
```

### Batch Mode
```bash
/bundle-prune --batch --stale
# Processes all stale bundles
# Extracts all patterns automatically
# Archives all (doesn't delete without confirmation)
```

### Dry Run
```bash
/bundle-prune --dry-run --stale
# Shows what WOULD be pruned
# Shows what patterns WOULD be extracted
# Makes no changes
```

### Archive Only (No Delete)
```bash
/bundle-prune --archive-only [bundle-name]
# Extracts patterns, archives bundle
# Never deletes (safe mode)
```

### Force Delete (Skip Extraction)
```bash
/bundle-prune --force --no-extract [bundle-name]
# Deletes without extracting patterns
# Use with caution - knowledge may be lost
# Requires explicit confirmation
```

---

## Extraction Rules

### What Gets Extracted

**Patterns (→ `.agentops/patterns/`):**
- Problem/Solution pairs with evidence
- Implementation steps that are reusable
- Debugging approaches that can be generalized
- Architecture decisions with rationale

**Not Extracted (stays in bundle or is lost):**
- One-time fixes (not reusable)
- Context-specific details
- Working notes without conclusions
- Duplicate of existing patterns

### Extraction Decision Tree

```
Is this content in the bundle?
│
├─ Problem/Solution with evidence?
│  ├─ Generalizable? → Extract as Pattern
│  └─ One-time fix? → Skip (not reusable)
│
├─ Decision with rationale?
│  ├─ Reusable decision framework? → Extract as Pattern
│  └─ Context-specific? → Skip (stays in archive)
│
├─ Gotcha/Warning?
│  ├─ Related to extractable pattern? → Include in pattern
│  └─ Standalone? → Extract as Anti-Pattern
│
└─ Implementation steps?
   ├─ Generic approach? → Extract as Pattern
   └─ Specific to one case? → Skip
```

---

## Prune Triggers

### Automatic Detection (via `/maintain`)

```
Weekly maintenance report:

PRUNE CANDIDATES:
  3 stale bundles (>30 days)
  1 superseded bundle
  2 consolidation opportunities

Run /bundle-prune --stale? [Y/n]
```

### Manual Triggers

```bash
# After project completion
/bundle-prune project-x-*

# After bundle consolidation
/bundle-prune old-bundle --archive-only

# Quarterly cleanup
/bundle-prune --stale --days 90
```

---

## Archive Structure

```
.agents/bundles/archive/
├── 2025-11/
│   ├── redis-caching-research.md
│   ├── k8s-migration-plan.md
│   └── MANIFEST.md  # What was pruned and why
├── 2025-10/
│   └── ...
└── README.md  # Archive policy
```

### Archive Manifest

Each month's archive includes a manifest:

```markdown
# Archive Manifest: 2025-11

## Pruned Bundles

### redis-caching-research.md
- Pruned: 2025-11-23
- Reason: Stale (45 days)
- Patterns extracted:
  - patterns/implementation/redis-connection-pooling.md
  - patterns/implementation/cache-invalidation-pubsub.md
- Original location: work/infrastructure/

### k8s-migration-plan.md
- Pruned: 2025-11-23
- Reason: Superseded by k8s-migration-v2.md
- Patterns extracted: None (already in v2)
- Original location: work/infrastructure/
```

---

## Safety Features

### Prevent Accidental Deletion

1. **Dry run first:** Always show what would happen
2. **Extract before delete:** Patterns extracted before any deletion
3. **Archive by default:** Delete requires explicit `--delete` flag
4. **Confirmation prompts:** Interactive confirmation for destructive actions
5. **Manifest logging:** All actions logged in archive manifest

### Recovery

**From archive:**
```bash
# Restore archived bundle
mv .agents/bundles/archive/2025-11/redis-caching-research.md \
   .agents/bundles/work/infrastructure/

# Re-index to MCP Memory
/bundle-index-all
```

**From git:**
```bash
# If accidentally deleted and committed
git checkout HEAD~1 -- .agents/bundles/redis-caching-research.md
```

---

## Examples

### Example 1: Prune Single Stale Bundle

```bash
/bundle-prune redis-caching-research

# Output:
Analyzing: redis-caching-research.md
Last accessed: 45 days ago
Size: 1.2k tokens

Found 2 extractable patterns:
1. ✨ Connection Pooling Strategy
2. ✨ Cache Invalidation via Pub/Sub

Extract patterns? [Y/n] y

✅ Created: patterns/implementation/redis-connection-pooling.md
✅ Created: patterns/implementation/cache-invalidation-pubsub.md

Archive or delete? [A/D/K] a

✅ Archived to: archive/2025-11/redis-caching-research.md
✅ Updated: archive/2025-11/MANIFEST.md
✅ Removed from MCP Memory index

Prune complete. 2 patterns preserved, bundle archived.
```

### Example 2: Batch Prune Stale Bundles

```bash
/bundle-prune --stale --batch

# Output:
Found 5 stale bundles (>30 days):

Processing 1/5: redis-caching-research.md
  → Extracted 2 patterns
  → Archived

Processing 2/5: k8s-migration-plan.md
  → Extracted 1 pattern
  → Archived

Processing 3/5: auth-debugging-notes.md
  → No extractable patterns (already documented)
  → Archived

Processing 4/5: temp-testing-notes.md
  → No extractable patterns (working notes only)
  → Archived

Processing 5/5: old-bundle-v1.md
  → Superseded by v2
  → Extracted 0 patterns (in v2 already)
  → Archived

Summary:
  Bundles processed: 5
  Patterns extracted: 3
  Bundles archived: 5
  Bundles deleted: 0
  Tokens freed: 6.2k
```

### Example 3: Dry Run

```bash
/bundle-prune --dry-run --stale

# Output:
DRY RUN - No changes will be made

Would prune 5 bundles:
  1. redis-caching-research.md (45 days)
     Would extract: 2 patterns
     Would archive to: archive/2025-11/

  2. k8s-migration-plan.md (38 days)
     Would extract: 1 pattern
     Would archive to: archive/2025-11/

  ...

Total impact:
  Bundles: 5 would be archived
  Patterns: 3 would be extracted
  Tokens freed: 6.2k

Run without --dry-run to execute.
```

---

## Integration with Other Commands

| Command | Integration |
|---------|-------------|
| `/maintain` | Suggests prune candidates weekly |
| `/learn` | Extracts patterns (prune uses same logic) |
| `/bundle-save` | Creates bundles that may later be pruned |
| `/bundle-search` | Finds bundles including archived ones |
| `/kg-sync` | Updates graph after pruning |

---

## Metrics

Track pruning effectiveness:

```
Pruning Stats (Last 30 Days):
  Bundles pruned: 12
  Patterns extracted: 8
  Tokens archived: 15.2k
  Tokens deleted: 0
  Recovery rate: 0 (no accidental deletions)

Pattern Extraction Rate: 67%
  (8 patterns from 12 bundles)

Knowledge Preservation: 100%
  (All extractable content preserved)
```

---

## Related Commands

- `/learn` - Extract patterns from current work (prune does this for old bundles)
- `/maintain` - Weekly report includes prune candidates
- `/bundle-save` - Creates bundles
- `/bundle-load` - Can load archived bundles
- `/bundle-search` - Searches including archives

---

**Ready to prune? Run `/bundle-prune --dry-run --stale` to see candidates.**
