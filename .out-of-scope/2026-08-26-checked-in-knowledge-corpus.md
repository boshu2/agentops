# Checked-in knowledge corpus / solutions store

**Request:** adopt a compound-engineering-style *solutions store* — a checked-in
corpus of prior findings and solution documents that agents retrieve from, so
knowledge accumulates in the repository.

**Decision:** not built.

**Why:**

- [ADR-0004](../docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md)
  put the "knowledge corpus improves agent reliability" claim under a valid
  pre-registered A/B ruler and did not obtain the effect. The ruler worked; the
  moat did not show up.
- [ADR-0011](../docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)
  measured accrual directly: the mechanism is proven end-to-end, but compounding
  is structurally data-starved — 130 real gate verdicts yielded 0 escapes and 0
  escape-derived constraints.
- The 2026-08-26 plugin investigation found an 81-document store shipping with no
  retrieval measurement at all: no eval distinguishing "the corpus grew" from
  "the corpus got worse". Size is not evidence.
- The unit that has actually compounded here is `skills/` — validated behavior
  under gates — not stored prose.

**Reopens when:** a retrieval eval exists that can tell corpus growth from
corpus degradation on a locked task set, and a store measurably beats its own
absence on that eval. Curation plus measurement is the door ADR-0011 left open;
the store is not.
