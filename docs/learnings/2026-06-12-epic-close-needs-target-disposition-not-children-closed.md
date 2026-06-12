---
title: Epic close requires target-by-target disposition, not children-all-closed
date: 2026-06-12
tags: [closeout, epics, beads, orchestration]
status: draft
source: ag-p273x harvest epic reconcile (2026-06-12 session)
---

# Epic close requires target-by-target disposition, not children-all-closed

## What happened

The ag-p273x harvest epic listed **7 operationalization targets** in its body but
only **5 child beads** were filed at creation. Both worker lanes finished, all 5
children closed green. "All children closed" looked like "epic done" — but
diffing the epic body's target list against the children at close time showed
target 5 (per-check timeouts in `ao codex image-health`) had silently never
become a bead. It was filed as ag-7ixm9 at reconcile, then shipped.

If the close gate had been "children all closed," a P2 finding from the source
review would have evaporated with a green epic on top of it.

## The rule

At epic close, reconcile the epic body's own enumerated scope (targets,
findings, acceptance bullets) against the child set — one disposition per item:
LANDED (sha), SCHEDULED (bead id), or EXPLICITLY DROPPED (reason on the epic).
Children-all-closed is necessary, not sufficient.

This is the sibling of the existing no-epic-close-with-open-child gate
(`scripts/check-epic-children-closed.sh`): that gate catches open children;
nothing catches **never-created** children. The decomposition step is where
items drop, and the close step is the last place to catch it.

## Evidence

- Epic ag-p273x body ("Operationalization targets" 1–7) vs children .1–.5.
- Recovered work: ag-7ixm9 → cf8084d22 (+ residual ag-1sibx → 25d9d1c85).
- Close comment on ag-p273x records the 7/7 disposition table.
