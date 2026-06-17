# age-i52 — skills retire historical row placement

**Bead:** age-i52  
**Date:** 2026-06-17

## Fix

`flipDispositionsLedger` now inserts retired rows at the end of `historical:`,
immediately before `workflows:` (when present) or `dispositions:` — never inside
the workflows block (which schema validators parse as `workflow:<slug>`).

## Proof

```bash
cd cli && go test ./cmd/ao/ -run 'HistoricalInsertAt|InsertsBeforeWorkflows|SkillsRetireInto' -count=1
```
