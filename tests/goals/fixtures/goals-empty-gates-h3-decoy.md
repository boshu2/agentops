# Goals

Negative fixture for tests/goals/validate-goals.sh: the Gates table is empty
and the decoy table sits under a `### Notes` SUBheading rather than a new
`## ` section. A block parser that ends only at `## ` keeps reading past the
subheading and counts the decoy's row as a gate.

The production parser (cli/internal/goals/markdown.go) ends the gates section
at any heading, so a validator that reads further is more permissive than the
thing it validates — it would report a healthy table that `ao goals measure`
sees as empty.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|

### Notes

Documentation below this subheading. Not gates.

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| decoy-row | `bash scripts/check-contract-compatibility.sh` | 5 | Prose, not a gate. |
