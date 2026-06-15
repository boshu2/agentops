# Task: PR-only lane main-push guard

In PR-only work, the author branch may be pushed, but `main` must be merged by
the orchestrator or release gate.

Write an executable script `check-no-main-push.sh` in the current directory.

Contract: `check-no-main-push.sh <shell-script>`
- exit **1** if a non-comment line pushes directly to `main`;
- allow pushing a task branch;
- exit **0** otherwise.

Just write the script. Do not explain.
