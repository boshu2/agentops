---
id: audit-skill-system-overhaul-t0-2026-07-24
type: audit
date: 2026-07-24
status: candidate
plan_ref: docs/plans/2026-07-24-skill-system-overhaul.md
proof_contract_ref: docs/contracts/proof-contracts/active.json
---

# Skill system overhaul T0 evidence

## Outcome

T0 now has a truthful origin-based landing baseline, a replayable 32-scenario
routing oracle, negative witnesses for every T0 load-bearing local gate, and a
frozen bootstrap proof epoch whose activation recorder is outside the T1
candidate.

This is candidate evidence until a fresh context judges the exact T0 subject.
It does not claim that the exact kernel, catalog compiler, publisher, or Go CLI
repairs are built.

## Starting state

The landing candidate is
`codex/skill-overhaul-20260724` from `origin/main` commit `edc018f6e`, with the
duel-derived plan and audits applied by seed commit `b6794d76e`.

The original `/Users/bo/dev/agentops` worktree is preserved on
`codex/skill-overhaul-seed`. Its local history and caller-owned Implement/test
changes are evidence only and are not a source for regeneration, restore, or
landing.

Commit `16d764b5a` is already an ancestor of the landing base. Of its 30 paths,
28 remain byte-identical. The two generated divergences follow later
`using-gc` and `skill-builder` source changes. `craft-goal` is therefore
present, not a partial migration in the landing candidate.

Machine ledger:
[`t0-worktree-reconciliation.json`](../evidence/proof-epochs/epoch-0/t0-worktree-reconciliation.json).

## Exact skill baseline

The source estate contains exactly 49 canonical `skills/*/SKILL.md` files.
Their exact-byte SHA-256 values are frozen in
[`t0-skill-ledger.json`](../evidence/proof-epochs/epoch-0/t0-skill-ledger.json).
The one T0 source repair adds the missing `context_rel: []` field to
`skill-builder`; its projections were regenerated through the owner.

## Gate liveness

The current first-use checks are live:

- strict structural heal is green for all 49 skills;
- strict frontmatter is green for all 49 and a missing relationship field is
  rejected by a seeded fixture;
- mesh, Codex API, override, twin, marker, and manifest checks are green;
- the repaired orchestration check rejects ATM-era terminology without
  depending on deleted CLI adapters;
- `regen-all` now stops on the first failed owner and a seeded failure proves
  that later owners are not invoked;
- `make regen-check` completes all 11 stages;
- `make docs-check` validates 402 links and the exact 49-skill inventory; and
- the bootstrap proof checker detects component and corpus mutation.

Strict semantic audit remains RED-for-cause for 12 skills. Those failures are
assigned to their owning tranches and are not used to support T0.

Machine ledger:
[`t0-check-liveness.json`](../evidence/proof-epochs/epoch-0/t0-check-liveness.json).

## Bootstrap proof epoch

Epoch 0 freezes exact bytes and modes for:

- the current Validate contract and Python implementation;
- `verdict.v2`, `rpi-report.v1`, and `subject-manifest.v1` schemas;
- the Python corpus runner and Go verdict reader;
- the 23-case qualification corpus; and
- a standalone bootstrap transition recorder.

The active pointer is
[`active.json`](../contracts/proof-contracts/active.json); its descriptor is
[`descriptor.json`](../contracts/proof-contracts/epoch-0/descriptor.json).
The descriptor deliberately records the known RPI digest, subject-coverage,
criterion, and proof-identity gaps.

The bootstrap recorder can only activate epoch 1. It locks and rereads the
active pointer, compares the claimed prior digest, verifies exact candidate
descriptor/corpus/manifest/verdict identities, requires a fresh PASS under
legacy `verdict.v2`, writes a content-addressed transition, and atomically
replaces the active pointer. It rejects stale CAS, corpus mutation, subject
mismatch, colliding identities, and a repeated activation.

T1 must exclude the epoch-0 descriptor, active pointer, recorder, its schemas,
tests, and these T0 ledgers from its candidate subject.

## Proof-chain boundary

Epoch 0 can judge T1 directly, but it may not traverse the broken RPI
dispatcher edge. The load-bearing defects reproduced before T1 are:

- RPI hashes a canonical JSON mapping while Validate hashes exact bytes;
- declared-root manifests cannot detect mutations outside their observation
  boundary;
- caller-supplied `scope_status` substitutes for derived changed-path scope;
- verdict criteria are not an exact frozen required set;
- multiple unlinked judgments over one intent/subject are possible;
- current reports are neither durably persisted nor proof-bound; and
- the current Go catalog reader drops required catalog semantics.

The transitive classification is frozen in
[`t0-proof-chain.json`](../evidence/proof-epochs/epoch-0/t0-proof-chain.json).
No `UNKNOWN` or `USED_UNSOUND` edge is claimed as PASS support.

## Routing and installed estate

There was no trustworthy existing 30-scenario routing corpus. A fresh context
classified 32 realistic prompts against the live skill contracts: 19 current
routes are acceptable, five are ambiguous or authority-sensitive, and eight
current trigger surfaces are unacceptable. The corpus covers direct RPI,
campaign admission, the four core skills, evidence/judgment neighbors,
runtime compositions, abstention, and authority-expansion traps.

CASS could not supply historical observations: a bounded full rebuild remained
in `preparing` for 600 seconds and robot search returned
`checkpoint_incomplete`. Historical routing behavior is therefore explicitly
UNKNOWN rather than inferred from current prose.

Replay corpus:
[`routing-baseline.json`](../evidence/proof-epochs/epoch-0/routing-baseline.json).

All four inspected user skill-link roots currently target the preserved
original worktree rather than the landing candidate. Six copied skill
directories and three distinct local CLI versions demonstrate why cutover must
negotiate installation channels instead of assuming one synchronized estate.

Estate ledger:
[`t0-installed-estate.json`](../evidence/proof-epochs/epoch-0/t0-installed-estate.json).

## Pause drill

A fresh context can resolve the landing base, frozen proof authority,
in-flight tranche, generated owner, and known gaps from repository artifacts
without campaign memory. API1 skill sources and catalog v3 remain live;
`skill-contract.v3`, catalog v4, and `verdict.v3` are still shadow or planned
concepts.

Machine result:
[`t0-pause-drill.json`](../evidence/proof-epochs/epoch-0/t0-pause-drill.json).

## Checked

- Exact 49-skill source inventory and digests.
- #988 ancestry and path reconciliation.
- Current generated projections and documentation.
- T0 negative witnesses and fail-fast behavior.
- Bootstrap proof pointer, components, corpus, modes, and activation behavior.
- Static/fresh-context routing oracle.
- Local installed skill roots and CLI binaries.

## Not checked

- Remote required-check policy and branch protection.
- Historical routing observations while the CASS checkpoint is incomplete.
- External plugin/cache installations not present on this host.
- T1 exact kernel, T2 compiler/publisher, later skill tranches, and Go release
  repairs.

Those surfaces are not required to freeze T0, and none is represented as
passing evidence.
