# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work atomically
bd close <id>         # Complete work
```

## Non-Interactive Shell Commands

Shell commands may be aliased to `-i` (interactive) mode, causing agents to hang. Always use force flags:

```bash
cp -f source dest       # NOT: cp source dest
mv -f source dest       # NOT: mv source dest
rm -f file              # NOT: rm file
rm -rf directory        # NOT: rm -r directory
```

Also: `apt-get -y`, `scp -o BatchMode=yes`, `HOMEBREW_NO_AUTO_UPDATE=1 brew ...`

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

Use **bd** for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists. Run `bd prime` for full workflow context.

Work is NOT complete until `git push` succeeds. After finishing: `git pull --rebase && bd dolt push && git push`.
<!-- END BEADS INTEGRATION -->
