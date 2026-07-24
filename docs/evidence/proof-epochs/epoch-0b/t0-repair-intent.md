# T0 repair invocation exact intent

Consume the immutable failed T0 verdict
`bf865e3233c1e19e6346d37403db775e9fb0fa6b252d14af88e4c9aaa081d804`
and perform one new, bounded experiment that repairs only its four findings.
The prior verdict and rejected bootstrap descriptor must remain byte-immutable.

## Required criteria

- **T0R-1 — Direct heal witness:** strict `heal.sh` has a seeded hostile fixture
  that invokes it directly and fails for the intended structural reason.
- **T0R-2 — Component-bound activation:** the active bootstrap recorder refuses
  a missing, byte-mismatched, mode-mismatched, or subject-unbound candidate
  component and future transition recorder before changing the active pointer.
- **T0R-3 — Honest lineage:** the rejected epoch-0 descriptor and FAIL verdict
  remain immutable; an explicit operator-authorized bootstrap-root replacement
  binds them to corrected epoch 0b because no PASS was minted under epoch 0.
- **T0R-4 — Accurate pause state:** proof-chain, liveness, audit, and pause
  artifacts identify the stable prior commit, immutable failure, active epoch
  0b, and the exact remaining work without calling any unfinished tranche
  complete.

## Declared exclusions

- Repairing any T1 exact-kernel behavior.
- Activating epoch 1.
- Changing the rejected epoch-0 descriptor or failed verdict.
- Go CLI G0 through G2 implementation.
- Installed skill links, external systems, credentials, or release channels.
