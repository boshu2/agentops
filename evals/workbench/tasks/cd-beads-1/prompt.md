# Task: file-backed br invocation guard

AgentOps uses the file-backed `_beads` tracker database. A bare `br` command can
hit the wrong local store and make a claim or close vanish from the shared file
database.

Write an executable script `check-br-beads-dir.sh` in the current directory.

Contract: `check-br-beads-dir.sh <shell-script>`
- exit **1** if a non-comment line invokes `br` without a same-command
  `BEADS_DIR=.../_beads` environment assignment;
- exit **0** when every `br` invocation is scoped to `_beads`.

Just write the script. Do not explain.
