# Dueling Idea Wizards — Phase 2: blind cross-score

The sealed proposal phase is complete. Score the other wizard's five finalists;
do not revise your own proposal in this phase.

## Identity and write boundary

- FABLE reads
  `raw/WIZARD_IDEAS_SOL.md` and writes only
  `raw/WIZARD_SCORES_FABLE_ON_SOL.md`.
- SOL reads
  `raw/WIZARD_IDEAS_FABLE.md` and writes only
  `raw/WIZARD_SCORES_SOL_ON_FABLE.md`.
- Paths are relative to
  `docs/audits/skill-system-overhaul-duel-2026-07-24/`.
- Do not read the opponent's score of your work; that remains sealed until the
  reveal.
- Do not edit any other file and do not delegate.

## Scoring

Give every finalist a 0–1000 score with this architecture rubric:

- **Structural soundness (0–250):** Does it clarify authority, separation of
  concerns, lifecycle ownership, and exact evidence identity without creating a
  second lifecycle?
- **System coherence (0–250):** Does it give all 49 skills and the Go CLI a
  coherent place in the product/campaign/experiment/evolution system?
- **Maintainability and proof (0–250):** Does it reduce cognitive and policy
  drift, and does it create executable, falsifiable acceptance rather than
  metadata theater?
- **Migration feasibility (0–250):** Can it land incrementally in the dirty,
  generated multi-surface repository with bounded risk and an honest rollback?

For every score:

1. show the four-component breakdown and total;
2. identify the strongest source-backed contribution;
3. identify the strongest objection or hidden cost;
4. state whether the idea should be adopted as written, adopted with a precise
   modification, subsumed into another idea, deferred, or rejected;
5. state what evidence would change the score by at least 150 points.

Then add:

- the opponent's strongest overall architecture;
- the opponent's weakest overall assumption;
- semantic matches between their finalists and yours;
- genuine disagreements that must not be averaged away;
- any item in their 49-skill appendix that changes your own disposition.

Use candid, discriminating scores. A flat cluster is not useful. When the
artifact is complete and verified, reply only `SCORES_WRITTEN`.
