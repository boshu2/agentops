# Crank Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| "No ready issues found for this epic" | Epic has no child issues or all blocked | Run `/plan <epic-id>` first to decompose epic into issues. Check dependencies with `bd show <id>`. |
| Selected wave packet is invalid | Plan identity, scope, acceptance, rollback, or base is missing/mismatched | Return the mismatch; do not repair the packet or dispatch another wave. |
| Wave vibe gate fails repeatedly | Workers producing non-conforming code | Check `.agents/council/YYYY-MM-DD-vibe-wave-N.md` for specific findings. Add cross-cutting constraints to task metadata or refine worker prompts. |
| Workers report completion but files missing | Permission errors or workers writing to wrong paths | Check `.agents/swarm/<team>/worker-N-output.json` for file paths. Verify write permissions with `ls -ld`. |
| RED Gate passes (tests don't fail) | Test wave workers wrote implementation code | Return the unexpected-pass evidence to RPI. Another TEST action requires an explicit orchestrator decision. |
| TaskList mode can't find epic ID | bd CLI required for beads epic tracking | Provide plan file path (`.md`) or task description string instead of epic ID. Or install bd CLI with `brew install bd`. |
