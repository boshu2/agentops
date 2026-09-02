# Goals

Negative fixture for tests/goals/validate-goals.sh: a Gates table whose row
names a check script that does not exist in this repository. This is gate rot
— the row reads as executable and measures as a permanent failure.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| ghost-check | `bash scripts/check-this-script-does-not-exist.sh` | 5 | Names a script that is not in the tree. |
