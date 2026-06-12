## Step 7: Report to User

Tell the user:
1. Council verdict on implementation
2. Key learnings
3. Any follow-up items
4. Location of post-mortem report
5. Knowledge flywheel status
6. **Suggested next `/rpi` command** from the harvested `## Next Work` section (ALWAYS — this is how the flywheel spins itself)
7. ALL proactive improvements, organized by priority (highlight one quick win)
8. Knowledge lifecycle summary (Phase 3-5 stats)

**The next `/rpi` suggestion is MANDATORY, not opt-in.** After every post-mortem, present the highest-severity harvested item as a ready-to-copy command:

```markdown
## Flywheel: Next Cycle

Based on this post-mortem, the highest-priority follow-up is:

> **<title>** (<type>, <severity>)
> <1-line description>

Ready to run:
```
/rpi "<title>"
```

Or see all N harvested items in `.agents/rpi/next-work.jsonl`.
```

If no items were harvested, write: "Flywheel stable — no follow-up items identified."

---

## Integration with Workflow

```
/plan epic-123
    |
    v
/pre-mortem (council on plan)
    |
    v
/implement
    |
    v
/validate (council on code)
    |
    v
Ship it
    |
    v
/post-mortem              <-- You are here
    |
    |-- Phase 1: Council validates implementation
    |-- Phase 2: Extract learnings (inline)
    |-- Phase 3: Process backlog (score, dedup, flag stale)
    |-- Phase 4: Activate (promote to MEMORY.md, compile constraints)
    |-- Phase 5: Retire stale learnings
    |-- Phase 6: Harvest next work
    |-- Suggest next /rpi --------------------+
                                              |
    +----------------------------------------+
    |  (flywheel: learnings become next work)
    v
/rpi "<highest-priority enhancement>"
```

---

## Examples

### Wrap Up Recent Work

**User says:** `/post-mortem`

**What happens:**
1. Agent scans recent commits.
2. Runs `/council --deep validate recent`.
3. Extracts learnings, processes backlog, and promotes items.
4. Harvests next-work to `.agents/rpi/next-work.jsonl`.

**Result:** Report with learnings, stats, and a suggested `/rpi` command.

### Other Modes

- **Epic-specific:** `/post-mortem ag-5k2` — review against the target plan
- **Quick capture:** `/post-mortem --quick "insight"` — write a learning without council
- **Process-only:** `/post-mortem --process-only` — run backlog processing only
- **Cross-vendor:** `/post-mortem --mixed ag-3b7` — broaden judgment coverage
