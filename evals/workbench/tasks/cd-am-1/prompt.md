# Task: reservation conflict fail-closed guard

AgentOps agents reserve files before editing. A reservation response can grant
some paths while also reporting conflicts. In that case automation must stop,
not proceed with the granted subset.

Write an executable script `check-am-reservation-conflicts.sh` in the current
directory.

Contract: `check-am-reservation-conflicts.sh <reservation-response.json>`
- exit **1** if the JSON has any entries in `.conflicts`;
- print a short message naming the conflicted path if possible;
- exit **0** when `.conflicts` is absent or empty.

Just write the script. Do not explain.
