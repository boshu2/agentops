# Task: runtime `.agents` artifact guard

The `.agents` tree contains runtime evidence as well as intentional planning and
spec artifacts. Runtime evidence should not be committed as product source.

Write an executable script `check-no-runtime-agents.sh` in the current directory.

Contract: `check-no-runtime-agents.sh <changed-files.txt>`
- exit **1** if a changed file is under `.agents/rpi/`, `.agents/yield/`,
  `.agents/swarm/`, `.agents/ao/`, or `.agents/handoff/`;
- allow `.agents/specs/`, `.agents/plans/`, and `.agents/research/`;
- exit **0** otherwise.

Just write the script. Do not explain.
