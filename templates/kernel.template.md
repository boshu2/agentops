# Project: {{PROJECT_NAME}}

## Behavioral Standards

<default_to_action>
Implement changes rather than suggesting. Infer intent and proceed.
</default_to_action>

<investigate_before_answering>
Read files before proposing changes. No speculation about unread code.
</investigate_before_answering>

<avoid_overengineering>
Only make requested changes. Keep solutions simple.
</avoid_overengineering>

## Intent Detection

| Intent | Keywords | Action |
|--------|----------|--------|
| Resume | "continue", "pick up", "back to" | Load bundles, read progress |
| End | "done", "stopping", "finished" | Save state, update progress |
| Status | "what's next", "where was I" | Show progress, next item |
| New Work | "add", "implement", "create" | Check bundles, start RPI |
| Bug Fix | "fix", "bug", "broken" | Debug directly |

## Session Protocol

On first interaction, check for progress files:

```bash
[ -f "claude-progress.json" ] && [ -f "feature-list.json" ]
```

If found, display current state and next work item.

## Vibe Levels

| Level | Trust | Verify | Use For |
|-------|-------|--------|---------|
| 5 | 95% | Final only | Format, lint |
| 4 | 80% | Spot check | Boilerplate |
| 3 | 60% | Key outputs | Features |
| 2 | 40% | Every change | Integrations |
| 1 | 20% | Every line | Architecture |
| 0 | 0% | N/A | Research |

## Constraints

- Use semantic commits (`feat:`, `fix:`, `docs:`)
- Keep context under 40% - compress and bundle when approaching limit
- {{ADDITIONAL_CONSTRAINTS}}

## Resources

| Resource | Location |
|----------|----------|
| Commands | `.claude/commands/` |
| Bundles | `.agents/bundles/` |
| Progress | `claude-progress.json` |
| Features | `feature-list.json` |

---

## Slash Commands

| Command | Action |
|---------|--------|
| `/session-start` | Initialize session |
| `/session-end` | Save state and end |
| `/research` | Deep exploration |
| `/plan` | Create implementation plan |
| `/implement` | Execute approved plan |
| `/bundle-save` | Save context bundle |
| `/bundle-load` | Load context bundle |
| `/vibe-check` | Measure session metrics |
| `/vibe-level` | Classify task trust level |
