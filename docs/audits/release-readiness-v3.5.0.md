# Release readiness — v3.5.0

Date: 2026-07-31. Subject: `main` at `1c1500ce871d0584b2ba1b2407fefdee557f38cb`
(release prep #1029 atop the post-3.4.0 delta: `ao gc` port #1016, factory
doctrine #1020–#1023, ponytail plan #1024, plan manifest mode #1025, the
fresh-install fix wave #1026–#1028). Tag `v3.5.0` is **not yet cut**; per the
binding process rule, this record exists BEFORE the tag.

## Verdict: PASS — official mode, score 9.0 / threshold 8

Local-ci run `20260731T140818Z` (`scripts/ci-local-release.sh
--release-version 3.5.0 --readiness-mode official --security-mode full`, real
HIL target, gate range `v3.4.0..HEAD`). First lap, no reruns: the v3.4.0
cycle's fixes (pairing-gate hermeticity, description budget, HIL command
markers) held.

| Dimension | Status | Points |
|---|---|---|
| SIL (full test suites) | pass | 2 |
| VIL (gates, regen, digital twin) | pass | 2 |
| HIL (real target) | pass | 2 |
| Artifacts (SBOM, manifest) | pass | 1.5 |
| Security (full mode) | pass | 1.5 |
| Evals | not_applicable this run | 0 |

## HIL evidence (no waiver used)

Real target, strong workflow: `ao` built from the release SHA on Bo-Mac
(Darwin arm64) reported `ao version 3.5.0` (version_verified=true) and ran a
full `ao init` scaffold in a scratch repo. Published-asset re-verification
happens post-tag via Release Publisher, as with 3.3.0 and 3.4.0.

- `release-readiness.json` sha256: `99d1080733a323d44cb5d2c93716cd84d508e66c1f5eace76158bb23745e8c2f`
- `hil-evidence.json` sha256: `374a134be1775e794048200e7e6e237b8f039ec6652ee5a495f4c714973ac02b`

## Disclosure

- The v3.5.0 delta includes the `ao init` gitignore behavior change and the
  `gc-maintainer-ops.sh` wrapper deprecation; both are in the upgrade notes.
- The Gas City substrate work this release documents (tending loop, run API,
  Mayor dispatch) was exercised against a live city whose defects are filed as
  rig beads; the factory remains an optional adapter and none of its runtime
  state gates this release.
- Open commitment carried a third time: the between-releases
  `goreleaser release --snapshot` CI smoke remains unimplemented. Release
  plumbing is unchanged since the last successful publish.
