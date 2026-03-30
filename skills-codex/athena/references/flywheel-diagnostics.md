# Flywheel Escape Velocity Diagnostics

> From systems-theory analysis of knowledge flywheel dynamics across 9 rigs, 132 learnings, and 24 patterns.

## The Flywheel Health Formula

```
σ × ρ > δ
```

Where:
- **σ (sigma)** = Consistency score — percentage of learnings with valid references, proper formatting, and non-stale content (0.0–1.0)
- **ρ (rho)** = Production velocity — learnings produced per session, weighted by citation count (higher citations = higher weight)
- **δ (delta)** = Decay rate — rate at which uncited learnings lose relevance (function of time since last citation)

**Escape velocity** is achieved when `σ × ρ > δ` — knowledge compounds faster than it decays.

**Graveyard state** occurs when `σ × ρ < δ` — knowledge rots faster than it's produced.

## Diagnostic Indicators

### Healthy Flywheel (σ × ρ > δ)
- Learnings are cited in plans, pre-mortems, and code reviews
- Patterns are extracted from clusters of related learnings
- Stale learnings are pruned or updated regularly
- New sessions benefit from prior session knowledge

**Example:** Shikamaru (gitops) — proper plan → pattern → retro → archive lifecycle. Knowledge compounds across epics.

### Decaying Flywheel (σ × ρ ≈ δ)
- Learnings produced but rarely cited
- No pattern extraction despite 10+ learnings in a domain
- Research files accumulate without synthesis
- `utility_score` stays at 0 (no feedback loop)

**Example:** Platform-lab — σ=0.02, ρ=1, producing artifacts nobody consumes.

### Graveyard (σ × ρ << δ)
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
grep -r "\[.*\](.*\.md)" .agents/learnings/ | while read line; do
  ref=$(echo "$line" | grep -oP '\(([^)]+\.md)\)' | tr -d '()')
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
| σ (consistency) | valid_refs / total_refs | > 0.8 | 0.5–0.8 | < 0.5 |
| ρ (velocity) | weighted_learnings / sessions | > 0.1 | 0.05–0.1 | < 0.05 |
| δ (decay) | uncited_30d / total | < 0.3 | 0.3–0.6 | > 0.6 |
| Escape velocity | σ × ρ > δ | YES | MARGINAL | NO |
| Production:extraction | learnings / patterns | < 10:1 | 10:1–20:1 | > 20:1 |

## Remediation Actions

### For Low σ (consistency)
- Run `ao defrag --prune --dedup` to fix broken references
- Validate learning frontmatter against schema
- Remove hallucinated learnings (coherence check)

### For Low ρ (velocity)
- Enable automatic extraction from sessions (forge hook)
- Lower confidence threshold for initial capture (0.5 instead of 0.7)
- Add extraction prompts to session wrap-up

### For High δ (decay)
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
