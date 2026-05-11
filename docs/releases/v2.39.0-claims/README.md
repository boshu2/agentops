# v2.39.0 README claim evidence manifest

> Closes PG4 (bead soc-dec2.4). Cross-link: A1 audit
> `.agents/research/2026-05-11-aop-claim-evidence-map.md`.
>
> Every `AOP-CLAIM-README-*` marker in README.md has a paired evidence
> file in this directory. The file names the surface in the repo that
> demonstrates the claim and the verification gate that protects it
> from drift.

## Claim → evidence map

| Claim ID | Evidence file | Verified by |
|---|---|---|
| AOP-CLAIM-README-FACTORY-CONTEXT | [factory-context.md](factory-context.md) | `scripts/check-factory-claim-ledger.sh` |
| AOP-CLAIM-README-COMPETITIVE-MEMORY | [competitive-memory.md](competitive-memory.md) | `scripts/check-competitive-freshness.sh` |
| AOP-CLAIM-README-FIRST-VALIDATED | [first-validated.md](first-validated.md) | `tests/install/test-five-minute-journey.sh` |
| AOP-CLAIM-README-EVOLVE-AUTONOMOUS | [evolve-autonomous.md](evolve-autonomous.md) | `.agents/evolve/cycle-history.jsonl` (durable session record) |
| AOP-CLAIM-README-AUTONOMOUS-FLYWHEEL | [autonomous-flywheel.md](autonomous-flywheel.md) | `scripts/check-flywheel-compounding-snapshot.sh` |

## Other claim scopes

Non-README scope claims (PRODUCT, GOALS, BRIEF, ASSURANCE, COMP,
CONTRACT, DOCS, WIKI, SOFTWARE-FACTORY, TRUST-FACTORY) — 25 markers —
are mapped in [all-claims-evidence-map.md](all-claims-evidence-map.md).

## Why this pattern

Local `.agents/` notes are gitignored — they are not visible to anyone
reading the public repo. Public claims need public proof. This manifest
is the bridge: each README marker can be traced to a file in
`docs/releases/<version>/` that names the repo surface backing it.
