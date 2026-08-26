# Out of scope — decisions not to build

One dated file per thing this repository decided **not** to build, so the same
proposal does not get re-litigated from scratch every time someone reads about
it somewhere.

A record belongs here when an outside source (a plugin, a tool, a paper, a
conference talk) actively prompts the request, and the answer is no. It does not
belong here for work that is merely unscheduled — that is a tracker item, not a
decision.

## Convention

- Filename: `YYYY-MM-DD-<slug>.md`, dated the day the decision was made.
- Keep each record under ~20 lines. A record that needs more is really an ADR;
  write the ADR in `docs/adr/` and link it from here.
- Every record states four things:
  1. **The request** — what was proposed, and what prompted it, named concretely.
  2. **The decision** — not built.
  3. **Why** — with citable evidence, not preference.
  4. **What would reopen it** — the observation that changes the answer.

## Relationship to other surfaces

An ADR in `docs/adr/` is an active constraint on how the repository is built. A
record here is narrower: it says one specific thing stays unbuilt, usually
*because* of an ADR. Records cite ADRs; they never override or replace one.

Delete a record when its reopening condition is met and the work is actually
picked up — the git history keeps the reasoning.
