# Task: forbidden Claude print invocation guard

There is a local quota/safety rule that forbids non-interactive Claude print
invocations in this repo.

Write an executable script `check-no-claude-print.sh` in the current directory.

Contract: `check-no-claude-print.sh <search-root>`
- exit **1** if any file under `<search-root>` contains `claude -p` or
  `claude --print`;
- print the offending file path(s);
- exit **0** otherwise.

Just write the script. Do not explain.
