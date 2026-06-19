# Harvest Governance Model

## Sweep Frequency

| Cadence | Trigger | Scope |
|---------|---------|-------|
| Weekly | Manual or scheduled | Full sweep of all rigs |
| Post-burst | After 3+ rigs active in one day | Targeted sweep of active rigs |
| Pre-evolve | Before `/evolve` cycle | Full sweep for fresh knowledge base |

## Size Budgets

| Location | Budget | Action When Exceeded |
|----------|--------|---------------------|
| Per-rig `.agents/learnings/` | 500 files | Run `ao defrag --prune` on the rig |
| Global `~/.agents/learnings/` | 2,000 files | Run `ao dedup --merge` on global hub |
| Per-rig `.agents/research/` | 200 files | Archive files older than 90 days |
| Harvest catalog `.agents/harvest/` | 10 dated files | Delete catalogs older than 30 days |

## Staleness Thresholds

| Artifact Type | Stale After | Action |
|---------------|-------------|--------|
| Learning | 90 days unreferenced | Flag for review, decay confidence by 0.1 |
| Pattern | 180 days unreferenced | Flag for review |
| Research | 60 days unreferenced | Archive |
| Promoted artifact | 90 days uncited | Decay confidence, consider demotion |

## Cross-Rig Synthesis Triggers

Promote a learning from `project:<name>` to `global` scope when:
1. Same pattern (by content hash similarity >80%) appears in 2+ rigs
2. Confidence >= 0.7 in at least one rig
3. Not contradicted by any rig's learnings

## Deduplication Policy

- **Within rig:** `ao defrag --dedup` (trigram similarity >80%)
- **Cross-rig:** `ao harvest` (SHA256 content hash after normalization)
- **Resolution:** Keep highest-confidence artifact, archive duplicates
- **Tie-breaking:** Most recent date, then alphabetical ID

## Monitoring

Check harvest health with:
```bash
ao metrics flywheel status    # Overall flywheel health
ls ~/.agents/learnings/ | wc -l  # Global hub size
ls .agents/harvest/ | wc -l     # Catalog history
```
