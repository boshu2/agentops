# Goals

Negative fixture for tests/goals/validate-goals.sh: the Gates table is present
but EMPTY — header and separator only — while an unrelated table with
lowercase IDs sits further down the document. A file-wide row selector counts
the decoy's rows and reports the fitness surface as healthy while it measures
nothing at all. Row checks must read only the `## Gates` block.

The Gates header here is deliberately lowercase (`| id |`), so this fixture
also proves the header row is skipped case-insensitively: if it were counted
as a data row, the zero-row failure below would not fire.

## Gates

| id | check | weight | description |
|----|-------|--------|-------------|

## Measured learning hypothesis

Repeated defect classes may deserve better context. The table below is
documentation, not an executable gate — it must not be mistaken for one.

| Class | Example | Weight | Notes |
|-------|---------|--------|-------|
| flaky-order | shuffle-order test isolation | 3 | Prose, not a gate. |
| stale-surface | a retired command left in a doc | 4 | Prose, not a gate. |
