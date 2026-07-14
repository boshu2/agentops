# Discovery Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Premortem returns FAIL | The exact plan has concrete blockers | Return the complete blocker set to the orchestrator for repair or replanning |
| No epic ID after plan | br unavailable and TaskList empty | Check tracking mode, verify `/plan` produced output |
| Brainstorm loops without advancing | Goal too vague for automated clarification | Use `--interactive` or provide a specific goal |
| ao search returns nothing | No prior sessions on this topic | Normal — proceed without history context |
| Idea challenge disagrees | Advisory perspectives remain contested | Preserve dissent and let Plan choose; Premortem still judges the final exact plan |
