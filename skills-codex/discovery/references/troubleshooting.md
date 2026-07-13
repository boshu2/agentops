# Discovery Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Premortem retries hit max | Plan has unresolvable risks | Review the matching Premortem report, refine the goal, and re-run `/discovery` |
| No epic ID after plan | br unavailable and TaskList empty | Check tracking mode, verify `/plan` produced output |
| Brainstorm loops without advancing | Goal too vague for automated clarification | Use `--interactive` or provide a specific goal |
| ao search returns nothing | No prior sessions on this topic | Normal — proceed without history context |
