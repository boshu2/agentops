# Task: per-bead worktree preflight

AgentOps lanes should not edit from the shared root checkout. They start from a
per-bead worktree whose path names the bead.

Write an executable script `check-worktree-per-bead.sh` in the current directory.

Contract: `check-worktree-per-bead.sh <bead-id> <shell-script>`
- exit **1** if the script does not add or use a git worktree path containing
  the bead id;
- exit **0** when it does.

Just write the script. Do not explain.
