# gc Pool Dispatch — DEPRECATED, historical reference only

> **gc tier removed (soc-2rtm0); retained for historical reference only — NOT selected. Top tier is NTM.** The Gas City (`gc`) CLI bridge was severed and deleted; `runtime=gc` is rejected by the CLI (see `agentops/CLAUDE.md`, "Gas City (gc) bridge — REMOVED"). The crank dispatch ladder is **NTM > runtime-native > beads floor** (with the `AGENTOPS_ORCHESTRATION=off` opt-out) — see `skills/shared/SKILL.md` "Selection policy" and crank `execution-preflight.md` Step 0.6. `GC_POOL_AVAILABLE` is never set true. The content below documents the old gc pool dispatch shape for archival purposes only — do not select or invoke it.

When `GC_POOL_AVAILABLE=true` (no longer reachable), `/swarm` invocation was replaced with gc pool dispatch:
- Workers are pre-started by gc pool (no spawn overhead)
- Assign work via `gc session nudge <worker> "<issue prompt>"`
- Poll completion via `gc status --json` + `bd show <id>` (check issue closed)
- gc handles crash recovery and session restart automatically

```bash
if [[ "$GC_POOL_AVAILABLE" == "true" ]]; then
    for issue in $READY_ISSUES; do
        ISSUE_DETAIL=$(bd show "$issue" 2>/dev/null)
        WORKER=$(gc status --json 2>/dev/null | jq -r '.pool.agents[] | select(.state == "idle") | .name' | head -1)
        if [[ -n "$WORKER" ]]; then
            gc session nudge "$WORKER" "Implement issue $issue: $ISSUE_DETAIL"
        else
            echo "No idle gc pool workers — waiting for pool auto-scale"
            gc pool wait --min-idle 1 --timeout 300
            WORKER=$(gc status --json 2>/dev/null | jq -r '.pool.agents[] | select(.state == "idle") | .name' | head -1)
            gc session nudge "$WORKER" "Implement issue $issue: $ISSUE_DETAIL"
        fi
    done
    # Poll until all wave issues are closed
    while true; do
        OPEN=$(bd ready 2>/dev/null | wc -l)
        [[ "$OPEN" -eq 0 ]] && break
        sleep 30
    done
else
    # Standard /swarm invocation (existing behavior)
    # Invoke /swarm with TaskCreate for each issue in the wave
fi
```

When `GC_POOL_AVAILABLE=false`, the existing `/swarm` path is used unchanged.
