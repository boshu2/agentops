# RPI Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Discovery BLOCKED | Premortem returned unresolved evidence | Review the matching report and submit the next orchestrator decision through the governor |
| Crank returns repeated blockers | Epic has unresolved evidence | Inspect `ao beads exec show <epic-id>` and let the governor classify repair, re-plan, or a stuckness breaker |
| Validation repeatedly refutes | Critical defects remain | Preserve the verdict through Learn; another validation requires a new admitted action |
| Missing epic ID | Discovery didn't produce a parseable epic | `ao beads exec list --type epic --status open` |
