# RPI Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Discovery BLOCKED | Premortem returned unresolved evidence | Review the matching report and record `REPAIR`, `REPLAN`, `HOLD`, or `ANDON` with evidence |
| Crank returns repeated blockers | Epic has unresolved evidence | Inspect `ao beads exec show <epic-id>` and classify the evidence; use `HOLD` only for real stuckness |
| Validation repeatedly refutes | Critical defects remain | Preserve the verdict through Learn; repair once or re-plan instead of repeating review |
| Missing epic ID | Discovery didn't produce a parseable epic | `ao beads exec list --type epic --status open` |
