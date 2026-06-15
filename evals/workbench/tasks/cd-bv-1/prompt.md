# Task: robot-safe graph triage guard

The graph triage tool has an interactive mode that can block an unattended
agent lane. Automation must use robot-safe flags instead.

Write an executable script `check-bv-robot-mode.sh` in the current directory.

Contract: `check-bv-robot-mode.sh <shell-script>`
- exit **1** if a non-comment line runs `bv` without a `--robot-*` flag;
- exit **0** if every `bv` command uses a robot mode.

Just write the script. Do not explain.
