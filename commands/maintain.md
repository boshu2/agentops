---
description: Weekly knowledge maintenance - stale bundles, consolidation opportunities, orphan detection
---

# /maintain - Weekly Knowledge Hygiene

**Purpose:** Keep the knowledge system healthy with periodic maintenance checks.

**Philosophy:** Knowledge systems need hygiene like code needs refactoring.

**When to use:**
- First session of the week (auto-suggested)
- Before major project starts (clean slate)
- After project completion (cleanup)
- Monthly retrospectives

**Token budget:** 3-8k tokens (1.5-4% of context window)

**Output:** Maintenance report + suggested actions

---

## Quick Start

```bash
# Full maintenance report
/maintain

# Check specific area
/maintain --bundles     # Bundle health only
/maintain --patterns    # Pattern catalog only
/maintain --graph       # Knowledge graph only

# Auto-fix safe issues
/maintain --auto-fix
```

---

## What Gets Checked

### 1. Bundle Health

```
BUNDLE HEALTH CHECK
═══════════════════

📦 Total bundles: 44
├─ Active: 37
├─ Archived: 7
└─ Total tokens: 52.3k

STALE BUNDLES (>30 days, no access):
⚠️  3 bundles need attention
  1. redis-caching-research.md (45 days)
     Size: 1.2k tokens
     Action: /bundle-prune or access to refresh

  2. k8s-migration-plan.md (38 days)
     Size: 1.8k tokens
     Action: /bundle-prune or access to refresh

  3. auth-debugging-notes.md (31 days)
     Size: 0.9k tokens
     Action: /bundle-prune or access to refresh

OVERSIZED BUNDLES (>2k tokens):
⚠️  2 bundles exceed size limit
  1. massive-research-dump.md (4.5k tokens)
     Action: Split into smaller bundles

  2. everything-about-k8s.md (3.2k tokens)
     Action: Split into focused bundles

MISSING METADATA:
⚠️  1 bundle lacks proper frontmatter
  1. old-notes.md
     Action: Add YAML frontmatter

DUPLICATE DETECTION:
⚠️  1 potential duplicate pair
  1. redis-cache-v1.md ↔ redis-caching-research.md (82% similar)
     Action: Consolidate or differentiate
```

### 2. Pattern Catalog Health

```
PATTERN CATALOG HEALTH
══════════════════════

📚 Total patterns: 12
├─ Implementation: 5
├─ Debugging: 3
├─ Architecture: 2
├─ Automation: 2
└─ Anti-patterns: 0

ORPHANED PATTERNS (no references in 60 days):
⚠️  1 pattern may be unused
  1. patterns/implementation/old-api-pattern.md
     Last referenced: 65 days ago
     Action: Validate still relevant or archive

INCOMPLETE PATTERNS (missing sections):
⚠️  2 patterns need completion
  1. patterns/debugging/memory-leak.md
     Missing: Evidence section
     Action: Add evidence or mark as draft

  2. patterns/architecture/microservice-boundary.md
     Missing: When NOT to Use section
     Action: Complete pattern

INDEX OUT OF SYNC:
⚠️  INDEX.md doesn't match actual patterns
  - Listed but missing: 0
  - Exists but not listed: 1 (new-pattern.md)
  Action: /maintain --fix-index
```

### 3. Knowledge Graph Health

```
KNOWLEDGE GRAPH HEALTH
══════════════════════

📊 Graph stats:
├─ Repositories: 8
├─ Topics: 12
├─ Documents: 23
├─ Bundles: 45
└─ Relationships: 67

BROKEN REFERENCES:
⚠️  2 references point to missing files
  1. documents.agentops-guide → docs/old-path/guide.md (NOT FOUND)
     Action: Update path or remove reference

  2. bundles.old-research → .agents/bundles/deleted.md (NOT FOUND)
     Action: Remove from graph.yml

ORPHANED ENTITIES:
⚠️  1 entity has no relationships
  1. topic: deprecated-feature
     Action: Add relationships or remove

SYNC STATUS:
✅ Last synced: 2025-11-23 (today)
   MCP Memory: In sync
```

### 4. MCP Memory Health

```
MCP MEMORY HEALTH
═════════════════

📡 Memory status: Connected
├─ Entities: 156
├─ Relations: 89
└─ Last sync: 2025-11-23 10:45

UNINDEXED BUNDLES:
⚠️  3 bundles not in MCP Memory
  1. new-bundle-today.md
  2. manually-created.md
  3. imported-from-team.md
  Action: /bundle-index-all

STALE ENTITIES:
⚠️  2 entities reference deleted files
  1. bundle-old-research-2025-10-01
  2. bundle-deprecated-plan
  Action: Will be cleaned on next /kg-sync
```

---

## Maintenance Report

Running `/maintain` produces a summary:

```
╔════════════════════════════════════════════════════════════════╗
║              WEEKLY KNOWLEDGE MAINTENANCE REPORT                ║
║                      2025-11-23                                 ║
╠════════════════════════════════════════════════════════════════╣
║                                                                 ║
║  BUNDLES          PATTERNS         GRAPH          MEMORY        ║
║  ────────         ────────         ─────          ──────        ║
║  Total: 44        Total: 12        Entities: 88   Status: ✅    ║
║  Stale: 3 ⚠️      Orphaned: 1 ⚠️   Broken: 2 ⚠️   Unindexed: 3  ║
║  Oversized: 2 ⚠️  Incomplete: 2 ⚠️  Orphaned: 1 ⚠️              ║
║  Duplicates: 1 ⚠️                                               ║
║                                                                 ║
╠════════════════════════════════════════════════════════════════╣
║  HEALTH SCORE: 78/100  (Good, needs attention)                  ║
╠════════════════════════════════════════════════════════════════╣
║                                                                 ║
║  RECOMMENDED ACTIONS:                                           ║
║                                                                 ║
║  HIGH PRIORITY:                                                 ║
║  1. Fix 2 broken graph references                               ║
║     → /maintain --fix-graph                                     ║
║                                                                 ║
║  2. Index 3 unindexed bundles                                   ║
║     → /bundle-index-all                                         ║
║                                                                 ║
║  MEDIUM PRIORITY:                                               ║
║  3. Review 3 stale bundles for pruning                          ║
║     → /bundle-prune --stale                                     ║
║                                                                 ║
║  4. Complete 2 incomplete patterns                              ║
║     → Edit patterns manually                                    ║
║                                                                 ║
║  LOW PRIORITY:                                                  ║
║  5. Split 2 oversized bundles                                   ║
║  6. Consolidate 1 duplicate pair                                ║
║  7. Review 1 orphaned pattern                                   ║
║                                                                 ║
╚════════════════════════════════════════════════════════════════╝

Run actions? [1-7, all, none]
```

---

## Command Options

### Full Report
```bash
/maintain
# Complete health check across all areas
```

### Specific Area
```bash
/maintain --bundles     # Bundle health only
/maintain --patterns    # Pattern catalog only
/maintain --graph       # Knowledge graph only
/maintain --memory      # MCP Memory only
```

### Auto-Fix Safe Issues
```bash
/maintain --auto-fix
# Automatically fixes:
# - Updates INDEX.md to match actual patterns
# - Removes broken references from graph.yml
# - Indexes unindexed bundles to MCP Memory
# Does NOT:
# - Delete or archive bundles
# - Remove patterns
# - Make destructive changes
```

### Fix Specific Issues
```bash
/maintain --fix-index   # Sync INDEX.md with actual patterns
/maintain --fix-graph   # Remove broken references from graph.yml
/maintain --fix-memory  # Re-sync MCP Memory with filesystem
```

### Dry Run
```bash
/maintain --dry-run
# Shows what would be reported/fixed
# Makes no changes
```

### Quiet Mode
```bash
/maintain --quiet
# Only shows issues, not healthy stats
# Good for: CI/CD integration
```

---

## Health Score Calculation

```
HEALTH SCORE = 100 - (issues * weight)

Weights:
├─ Broken references: -5 per issue
├─ Stale bundles: -3 per issue
├─ Oversized bundles: -3 per issue
├─ Orphaned patterns: -3 per issue
├─ Incomplete patterns: -2 per issue
├─ Missing metadata: -2 per issue
├─ Duplicate bundles: -2 per issue
├─ Unindexed bundles: -1 per issue
└─ Out-of-sync index: -1

Score interpretation:
├─ 90-100: Excellent (green)
├─ 80-89: Good (green)
├─ 70-79: Needs attention (yellow)
├─ 60-69: Degraded (orange)
└─ <60: Critical (red)
```

---

## Automation Integration

### Session Start Hook

```
# Suggested on first session of week (Monday)
Welcome back! It's Monday.

📊 Quick health check:
  - 3 stale bundles found
  - Last maintenance: 7 days ago

Run /maintain for full report? [Y/n]
```

### Scheduled Maintenance

For automated environments:

```bash
# Weekly cron job (conceptual)
0 9 * * 1 claude-code --run "/maintain --quiet" >> maintenance.log
```

### CI/CD Integration

```yaml
# GitHub Action example
- name: Knowledge Health Check
  run: |
    claude-code --run "/maintain --quiet" > health.txt
    if grep -q "CRITICAL" health.txt; then
      exit 1
    fi
```

---

## Examples

### Example 1: Weekly Maintenance

```bash
/maintain

# Output:
Running weekly maintenance check...

✅ Bundles: 41 healthy, 3 need attention
✅ Patterns: 11 healthy, 1 needs completion
⚠️  Graph: 2 broken references
✅ Memory: In sync

HEALTH SCORE: 82/100 (Good)

Recommended actions:
1. [HIGH] Fix broken graph references
2. [MED] Review stale bundles
3. [LOW] Complete pattern documentation

Run /maintain --fix-graph to fix high priority issues.
```

### Example 2: Auto-Fix Safe Issues

```bash
/maintain --auto-fix

# Output:
Running maintenance with auto-fix...

AUTO-FIXED:
✅ Updated INDEX.md (added 1 missing pattern)
✅ Removed 2 broken references from graph.yml
✅ Indexed 3 bundles to MCP Memory

REQUIRES MANUAL ACTION:
⚠️  3 stale bundles → /bundle-prune --stale
⚠️  1 incomplete pattern → Edit manually
⚠️  2 oversized bundles → Split manually

Health score improved: 72 → 85
```

### Example 3: Bundles Only

```bash
/maintain --bundles

# Output:
BUNDLE HEALTH CHECK
═══════════════════

📦 Total: 44 bundles (52.3k tokens)

Issues found:
  STALE (3): redis-research, k8s-plan, auth-notes
  OVERSIZED (2): massive-dump, everything-k8s
  DUPLICATES (1): redis-v1 ↔ redis-research

Actions:
  /bundle-prune --stale     # Handle stale
  /bundle-save --split ...  # Split oversized
```

---

## Maintenance Cadence

| Frequency | Action | Command |
|-----------|--------|---------|
| Weekly | Full report | `/maintain` |
| After project | Cleanup | `/maintain` + `/bundle-prune` |
| After git pull | Re-index | `/bundle-index-all` |
| Monthly | Deep clean | `/maintain` + prune all stale |
| Quarterly | Archive review | Review `archive/` directory |

---

## Related Commands

| Command | Relationship |
|---------|--------------|
| `/bundle-prune` | Execute stale bundle recommendations |
| `/bundle-index-all` | Execute unindexed bundle recommendations |
| `/kg-sync` | Execute graph sync recommendations |
| `/learn` | Add patterns (affects pattern health) |
| `/bundle-save` | Create bundles (affects bundle health) |

---

## Troubleshooting

### Health Score Dropping
```
Score dropped from 85 to 65 this week.

Check:
- New bundles created without metadata?
- Patterns added without INDEX update?
- Bundles not accessed (becoming stale)?

Fix:
/maintain --auto-fix  # Safe fixes
/bundle-prune --stale # Handle stale
```

### MCP Memory Out of Sync
```
Memory shows 120 entities, filesystem has 150 bundles.

Fix:
/bundle-index-all  # Re-index all bundles
/kg-sync          # Re-sync knowledge graph
```

### Many Broken References
```
Graph has 10+ broken references.

Likely cause: Files moved/renamed without graph update.

Fix:
/maintain --fix-graph  # Auto-remove broken refs
# Then manually add correct paths to graph.yml
```

---

**Keep your knowledge healthy! Run `/maintain` weekly.**
