SKILL GUIDANCE (loaded): validate — freshly judge exact subject content against
acceptance, return one result, and stop.

Verdict rules (the load-bearing invariants):
- PASS requires: a validator context distinct from the author, explicitly
  attested freshness of the evidence, evidence for EVERY acceptance criterion,
  non-empty checked scope, and an EMPTY not_checked list.
- The author's own reported run is not fresh independent evidence. A validator
  who cannot execute or independently reproduce the checks does not convert
  the author's word into proof.
- Any non-empty not_checked names in-scope acceptance surface that went
  unverified — that alone makes the result NOT_PROVEN, by construction.
- FAIL is for proven violation. NOT_PROVEN is for missing, unattested, or
  author-only evidence. Do not soften NOT_PROVEN into PASS because the story
  is plausible.
