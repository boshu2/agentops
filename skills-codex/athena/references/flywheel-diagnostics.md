# Flywheel Escape Velocity Diagnostics

> From systems-theory analysis of knowledge flywheel dynamics across 9 rigs, 132 learnings, and 24 patterns.

## The Flywheel Health Formula

```
σ × ρ > δ/100
```

Where:
- **σ (sigma)** = Retrieval coverage — unique surfaced retrievable artifacts / total retrievable artifacts (last 10 sessions), 0.0–1.0
- **ρ (rho)** = Decision influence rate — surfaced artifacts later evidenced by `reference` or `applied` citations / surfaced artifacts, 0.0–1.0
- **δ (delta)** = Knowledge age — average age of active learnings in days (normalized by /100 for escape velocity check)

**Escape velocity** is achieved when `σ × ρ > δ/100` — knowledge compounds faster than it ages out.

**Graveyard state** occurs when `σ × ρ < δ/100` — knowledge ages out faster than it's reinforced.

## Diagnostic Indicators

### Healthy Flywheel (σ × ρ > δ/100)
- Learnings are cited in plans, pre-mortems, and code reviews
- Patterns are extracted from clusters of related learnings
- Stale learnings are pruned or updated regularly
- New sessions benefit from prior session knowledge

**Example:** Shikamaru (gitops) — proper plan → pattern → retro → archive lifecycle. Knowledge compounds across epics.

### Decaying Flywheel (σ × ρ ≈ δ/100)
- Learnings produced but rarely cited
- No pattern extraction despite 10+ learnings in a domain
- Research files accumulate without synthesis
- `utility_score` stays at 0 (no feedback loop)

**Example:** Platform-lab — σ=0.02, ρ=1, producing artifacts nobody consumes.

### Graveyard (σ × ρ << δ/100)
- Broken references in learning files
- Hallucinated learnings contaminating the pool
- No extraction automation
- Sessions start from scratch despite prior work

**Example:** Ichigo pre-cleanup — 261 broken references, 141 learnings but only 2 patterns extracted.

## Measurement Commands

### Quick Health Check
```bash
# Count learnings with citations
grep -rl "cited_by\|referenced_in" .agents/learnings/ | wc -l

# Count learnings without citations (decay candidates)
total=$(find .agents/learnings/ -name "*.md" | wc -l)
cited=$(grep -rl "cited_by\|referenced_in" .agents/learnings/ | wc -l)
echo "Uncited: $((total - cited)) / $total"

# Check for broken references
grep -rE '\[.+\]\(.+\.md\)' .agents/learnings/ | while read line; do
  ref=$(echo "$line" | grep -oE '\([^)]+\.md\)' | tr -d '()')
  [ ! -f "$ref" ] && echo "BROKEN: $line"
done

# Production:extraction ratio
learnings=$(find .agents/learnings/ -name "*.md" | wc -l)
patterns=$(find .agents/patterns/ -name "*.md" | wc -l)
echo "Ratio: $learnings learnings : $patterns patterns"
```

### Full Diagnostic
During Athena's Grow phase, compute and report:

| Metric | Formula | Healthy | Warning | Critical |
|--------|---------|---------|---------|----------|
| σ (retrieval) | surfaced / total retrievable | > 0.5 | 0.2–0.5 | < 0.2 |
| ρ (influence) | evidenced / surfaced | > 0.3 | 0.1–0.3 | < 0.1 |
| δ (age) | avg_age_days / 100 | < 0.3 | 0.3–0.6 | > 0.6 |
| Escape velocity | σ × ρ > δ/100 | YES | MARGINAL | NO |
| Production:extraction | learnings / patterns | < 10:1 | 10:1–20:1 | > 20:1 |

## Remediation Actions

### For Low σ (retrieval effectiveness)
- Run `ao defrag --prune --dedup` to fix broken references
- Validate learning frontmatter against schema
- Remove hallucinated learnings (coherence check)

### For Low ρ (citation rate)
- Enable automatic extraction from sessions (forge hook)
- Lower confidence threshold for initial capture (0.5 instead of 0.7)
- Add extraction prompts to session wrap-up

### For High δ (knowledge age)
- Promote frequently-cited learnings to patterns
- Wire learnings into planning rules (make them consumed)
- Add citation tracking to /plan and /pre-mortem
- Prune uncited learnings older than 90 days

### For Bad Production:Extraction Ratio (> 20:1)
- Pick top-10 most-related learnings and synthesize into patterns
- Run Athena Grow with explicit synthesis mode
- Archive learnings that don't cluster with others

## Four Closure Loops

The flywheel requires all four loops to be closed:

1. **Capture:** Automatic extraction from sessions (forge, retro, post-mortem)
2. **Promotion:** Citation-driven — used knowledge gets promoted (higher confidence)
3. **Decay:** Built-in — unused knowledge loses confidence over time
4. **Reinforcement:** Promoted knowledge surfaces more often in future sessions

If any loop is broken, the flywheel stalls. During Grow phase, check each loop's health.

## Normalization Defect Detection

During flywheel diagnostics, Athena should check for these common normalization defects that corrupt metric accuracy:

| Defect | Detection Rule | Impact |
|--------|---------------|--------|
| Placeholder patterns | Files with only frontmatter (no content after closing `---`) | Inflates learning count without adding knowledge |
| Stacked frontmatter | Multiple `---` delimiter pairs in a single file | Breaks YAML parsing, corrupts metadata extraction |
| Bundled multi-learning files | More than one `## Learning:` heading in a single file | Under-counts learnings, skews citation tracking |
| Duplicated heading artifacts | Identical `## ` headings within a file | Indicates copy-paste or merge errors |
| Stale contradiction reports | Contradiction findings older than the latest extraction pass | Pollutes defrag decisions with outdated signals |

### Remediation

- **Placeholder files:** Delete or populate with extracted content.
- **Stacked frontmatter:** Split into valid single-frontmatter files; re-run forge on source transcript.
- **Bundled learnings:** Split into one learning per file, preserving original timestamps.
- **Duplicated headings:** Deduplicate manually or via `ao dedup`.
- **Stale contradictions:** Re-run contradiction detection (`ao contradict`) after latest extraction pass; discard stale findings.
