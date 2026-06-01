# Warmind: Team Knowledge Sharing

Warmind extends AgentOps learnings from individual sessions to team-wide shared knowledge. When one engineer discovers something useful, Warmind ensures the whole team benefits.

## Quick Start

```bash
# Initialize warmind in your repo (creates .warmind/ directory)
ao warmind init

# Sync local learnings to team pool (automatic at session end)
ao warmind sync

# Check warmind health
ao warmind status
```

## How It Works

### Knowledge Flow

```
Your Session          Team Pool              Team Canon
─────────────────────────────────────────────────────────
.agents/learnings/ → .warmind/pool/staged/ → .warmind/learnings/
    (local)              (scored)              (promoted)
```

1. **Your learnings** accumulate in `.agents/learnings/` during your sessions
2. **Sync** copies them to `.warmind/pool/staged/` with quality scores
3. **Promotion** moves high-value learnings to `.warmind/learnings/` for the team

### Automatic Sync

Warmind syncs automatically at session end (opt-out with `AGENTOPS_WARMIND_SYNC=0`). No manual action needed—just use AgentOps normally and learnings flow to the team.

## Commands

### `ao warmind init`

Initialize warmind in the current repository.

```bash
ao warmind init
```

Creates:
- `.warmind/pool/staged/` — staging area for scored candidates
- `.warmind/pool/rejected/` — learnings that didn't make the cut
- `.warmind/learnings/` — promoted team knowledge
- `.warmind/citations.jsonl` — citation tracking
- `.warmind/contradictions.jsonl` — detected conflicts

### `ao warmind sync`

Sync local learnings to the team pool.

```bash
ao warmind sync              # Sync new learnings
ao warmind sync --auto       # Called automatically at session end
ao warmind sync --quiet      # Suppress output
```

Each synced learning is scored on four dimensions:
- **Novelty** (35%): Is this new knowledge?
- **Specificity** (25%): Does it have concrete values/thresholds?
- **Actionability** (20%): Can you act on it?
- **Confidence** (20%): How certain is the claim?

### `ao warmind pool list`

View staged candidates waiting for promotion.

```bash
ao warmind pool list
ao warmind pool list --pending   # Show only pending
ao warmind pool list --staged    # Show only staged
```

### `ao warmind status`

Check warmind health and statistics.

```bash
ao warmind status
```

Shows:
- Number of local learnings
- Staged candidates in pool
- Promoted team learnings
- Citation statistics

### `ao warmind promote <id>`

Manually promote a learning to team canon.

```bash
ao warmind promote wm-abc123def456       # Promote by ID
ao warmind promote wm-abc123def456 --force   # Skip citation check
```

### `ao warmind close-loop`

Run the full knowledge maintenance cycle.

```bash
ao warmind close-loop
ao warmind close-loop --quiet
```

This:
1. Auto-promotes high-scoring learnings
2. Applies citation-based maturity transitions
3. Detects contradictions between learnings
4. Archives expired knowledge

## Promotion Rules

Learnings are promoted based on quality score and citations:

| Tier | Score | Requirement |
|------|-------|-------------|
| Gold | ≥0.85 | Auto-promote after 24h |
| Silver | 0.70-0.84 | 1 citation from another engineer |
| Bronze | 0.50-0.69 | 3 citations from other engineers |
| Discard | <0.50 | Not promoted |

**Key rule**: Self-citations don't count. Knowledge gains value when others find it useful.

## Maturity Lifecycle

Promoted learnings start as **provisional** and mature through use:

```
provisional → established → archived
```

- **Provisional**: New learning, needs validation (expires in 30 days without citations)
- **Established**: 3+ citations from different engineers (expires after 90 days without activity)
- **Archived**: No longer relevant, kept for history

## Contradiction Detection

Warmind automatically scans for conflicting advice between learnings.

```bash
ao warmind contradict              # Scan for contradictions
ao warmind contradict resolve <id> # Resolve a contradiction
ao warmind contradict dismiss <id> # Dismiss a false positive
```

When detected:
1. Both learnings are flagged for review
2. The conflict appears in `ao warmind status`
3. A human resolves which advice is correct

## Configuration

Warmind uses defaults that work for most teams. Override in `.warmind/config.yaml`:

```yaml
# Promotion tier thresholds
promotion:
  gold_threshold: 0.85
  silver_threshold: 0.70
  bronze_threshold: 0.50

# Citation requirements
citations:
  gold_auto_promote_hours: 24
  silver_citations: 1
  bronze_citations: 3

# Maturity lifecycle
maturity:
  provisional_expire_days: 30
  established_expire_days: 90
  min_corpus_size: 5

# Contradiction detection
contradict:
  min_signals: 2
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENTOPS_WARMIND_SYNC` | `1` | Auto-sync at session end (`0` to disable) |
| `AGENTOPS_WARMIND_DIR` | `.warmind` | Override warmind directory |

## Best Practices

1. **Let it run automatically** — Warmind works best when you don't think about it
2. **Cite useful learnings** — When `ao inject` returns something helpful, the citation is recorded
3. **Review contradictions** — Conflicts indicate evolving understanding; resolve promptly
4. **Trust the scoring** — Don't force-promote everything; let quality emerge naturally

## Troubleshooting

### "No learnings to sync"

Your `.agents/learnings/` directory is empty. Learnings are extracted from sessions by:
- `ao forge transcript --last-session` (automatic at session end)
- `ao retro` skill during work

### "Citation not recorded"

Citations require:
1. A warmind-initialized repo (run `ao warmind init`)
2. An active session ID
3. The learning must exist in the pool or learnings directory

### "Promotion stuck on citations"

Self-citations don't count toward promotion. The learning needs other team members to find it useful. For testing, use `--force`:

```bash
ao warmind promote <id> --force
```
