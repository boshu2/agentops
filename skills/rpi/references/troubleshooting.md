# RPI Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Discovery BLOCKED | Premortem failed 3x | Review the matching Premortem report, refine the goal, and re-run `/rpi --from=discovery` |
| Crank retries hit max | Epic has blockers | `ao beads exec show <epic-id>`, fix blockers, re-run `/rpi --from=implementation` |
| Validation retries hit max | Vibe found critical defects repeatedly | Apply findings, re-run `/rpi --from=validation` |
| Missing epic ID | Discovery didn't produce a parseable epic | `ao beads exec list --type epic --status open` |
