# Knowledge Synthesis Patterns

## Mine Patterns

What `ao mine` detects and their significance:

| Pattern | Source | Significance |
|---------|--------|-------------|
| **Co-change clusters** | `git log --name-only` | Files that always change together indicate hidden coupling. Candidate for module extraction or explicit dependency documentation. |
| **Orphaned research** | `.agents/research/*.md` not referenced in learnings | Research was done but insights were never captured. High-value extraction targets. |
| **Complexity hotspots** | High cyclomatic complexity + recent edits | Frequently-changed complex code is the highest-risk area. Candidate for refactoring or additional tests. |
| **Commit burst patterns** | Multiple commits to same file in short window | Indicates iterative debugging or unclear requirements. Learning candidate about the root cause. |
| **Test gap signals** | Source files with no corresponding `*_test.*` | Missing test coverage in actively-changed code. Candidate for testing improvement cycle. |

**Fallback (no ao CLI):**
```bash
# Co-change clusters
git log --since="7 days ago" --name-only --pretty=format: | sort | uniq -c | sort -rn | head -20

# Orphaned research
for f in .agents/research/*.md; do
  basename="$(basename "$f")"
  if ! grep -rl "$basename" .agents/learnings/ >/dev/null 2>&1; then
    echo "Orphaned: $f"
  fi
done

# Complexity hotspots (requires gocyclo or radon)
# Go: gocyclo -over 15 ./... 2>/dev/null | head -10
# Python: radon cc -s -n C . 2>/dev/null | head -20
```

## Grow Synthesis Rules

### Validation Classification

For each learning being validated, classify into one of three states:

| State | Criteria | Action |
|-------|----------|--------|
| **Validated** | Referenced code/function still exists and behaves as described | Mark with `validated: YYYY-MM-DD` in frontmatter |
| **Stale** | Code changed significantly since learning was written | Propose update with current state, or archive |
| **Contradicted** | Current code does the opposite of what learning claims | Flag for immediate update or deletion |

### Cross-Domain Grouping Heuristics

Group mine findings into themes using these heuristics:

1. **File path clustering**: Findings touching the same directory belong to the same theme
2. **Keyword overlap**: Findings mentioning the same function names, types, or concepts
3. **Temporal proximity**: Findings from the same git commit range likely relate
4. **Category matching**: Group by auto-classified category (debugging, architecture, process, testing, security)

**Synthesis trigger**: When 2+ findings share a theme, write a synthesized pattern that captures the common principle. The synthesis should be more general than any individual finding.

Example:
- Finding 1: "TaskCreate must happen after TeamCreate"
- Finding 2: "Workers can't see tasks created before their team existed"
- Synthesis: "Claude Code task visibility is scoped to team creation time. Always create teams before tasks."

### Gap Identification

Compare mine output topics against existing learnings using this scoring:

| Signal | Weight | Description |
|--------|--------|-------------|
| Topic has no matching learning | 3 | Complete knowledge gap |
| Topic has learning but >30 days old | 2 | Potentially stale coverage |
| Topic has recent validated learning | 0 | Well-covered |
| Topic involves security or breaking change | +2 | High-impact gap bonus |

Gaps scoring 3+ are reported as high-priority. Gaps scoring 2 are medium.

## Defrag Strategies

### Retention Policies

| Artifact Type | Keep If | Archive If | Delete If |
|--------------|---------|-----------|-----------|
| Learning | Referenced in last 30 days OR validated | >60 days unreferenced AND not validated | Contradicted by current code |
| Research | Referenced by a learning or plan | >90 days unreferenced | Superseded by newer research on same topic |
| Council report | Part of active epic | >30 days AND epic closed | Duplicate of newer report |
| Mine output | Latest only | N/A | Previous runs (only latest matters) |

### Deduplication Rules

Two learnings are near-duplicates when:
- >80% content similarity (measured by shared significant words / total words)
- Same category classification
- Reference the same source files

**Resolution**: Keep the more recent one, merge any unique details from the older one, archive the older.

### Oscillation Sweep

Check `cycle-history.jsonl` for goals that alternate improved/fail:

```bash
# Count improved→fail transitions per goal
jq -r '.target' .agents/evolve/cycle-history.jsonl | \
  awk '{
    if (prev[$0] == "improved" && result[$0] == "fail") osc[$0]++
    prev[$0] = result[$0]
  } END {
    for (g in osc) if (osc[g] >= 3) print g, osc[g]
  }'
```

Oscillating goals (3+ transitions) indicate the fix approach is wrong, not just incomplete. Flag for human review rather than automated retry.

## Flywheel Health Signals

| Signal | Healthy | Degraded | Critical |
|--------|---------|----------|----------|
| **New learnings/week** | 3+ | 1-2 | 0 |
| **Duplicate rate** | <10% of new learnings | 10-30% | >30% |
| **Stale ratio** | <20% of total | 20-40% | >40% |
| **Orphaned research** | <3 files | 3-10 files | >10 files |
| **Oscillating goals** | 0 | 1-2 | 3+ |
| **Gap coverage** | <5 open gaps | 5-15 | >15 |
| **Validation recency** | >50% validated in 30 days | 20-50% | <20% |

**Interpretation:**
- All healthy: flywheel is compounding effectively
- 1-2 degraded: run `/athena` to address specific areas
- Any critical: prioritize flywheel maintenance before feature work
