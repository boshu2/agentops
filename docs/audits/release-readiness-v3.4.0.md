# Release readiness — v3.4.0

Date: 2026-07-30. Subject: `main` at `520911899b10fa9459305dc30488fbf040508041`
(PR #1013 release prep + PR #1014 gate-self-test hermeticity fix). Tag `v3.4.0`
is **not yet cut**; per the binding process lesson in the v3.3.0 record, this
artifact exists BEFORE the tag.

## Verdict: PASS — official mode, score 9.0 / threshold 8

Local-ci run `20260730T022029Z` (`scripts/ci-local-release.sh --release-version
3.4.0 --readiness-mode official --security-mode full`, real HIL target, gate
range `v3.3.0..HEAD`).

| Dimension | Status | Points |
|---|---|---|
| SIL (software-in-loop: full test suites) | pass | 2 |
| VIL (validation-in-loop: gates, regen, digital twin) | pass | 2 |
| HIL (real target) | pass | 2 |
| Artifacts (SBOM, manifest) | pass | 1.5 |
| Security (full mode) | pass | 1.5 |
| Evals | not_applicable this run | 0 |

## HIL evidence (no waiver used)

Real target, strong workflow (`ao-version` + `ao-init` checks): `ao` built from
the release SHA on Bo-Mac (Darwin arm64, kernel 25.5.0) reported
`ao version 3.4.0` (version_verified=true) and ran a full `ao init` scaffolding
`.agents/ao/{index,intents,provenance,sessions,verdicts}` in a scratch repo.
Published-asset HIL re-verification happens post-tag via Release Publisher and
Tag Validate, as it did for v3.3.0.

- `release-readiness.json` sha256: `5f8d6cc6ce17bcdd086e3c845fb444e04b891a3e25305526d4118831c7469cd9`
- `hil-evidence.json` sha256: `7f5f857cd8fa86833722d39a501b7dda8241b8e501c6b5929fcf708020e7d03f`

## Factory canary obligation: dissolved with the pack path, not waived

The v3.3.0 record bound a mandatory mixed-provider factory canary to the gate
promoting the in-repo Gas City pack from preview to supported. v3.4.0 retires
that product path entirely: the prototype under `deploy/gc/` is retired in
place and the supported factories are the upstream Gas City build pack and the
Agentic Coding Flywheel, with AgentOps as the skills-and-evidence layer either
executes. The promotion the canary gated can no longer occur; the obligation is
dissolved by retirement of its subject, not silently waived.

## Found and fixed during this readiness cycle

- `scripts/check-orchestration-skill-boundaries.sh` had been broken (exit 2)
  since the 3.3 single-pass refactor deleted the adapter files it probed; no CI
  runner invoked it, so nothing noticed. Repaired in PR #1013.
- The command/test pairing self-test inherited `AGENTOPS_GATE_RANGE` from the
  release run and failed on fixture repos; hermeticity fix in PR #1014.
- `TestBlockingGatesHaveProvenNegativeWitness` gives false verdicts in dirty
  checkouts (it reads gitignored files under `tests/`); a follow-up task exists
  to restrict the scan to tracked files. Clean-tree behavior is correct.

## Open commitment carried forward

The v3.3.0 lesson to add a between-releases `goreleaser release --snapshot`
smoke in CI remains unimplemented. Release plumbing (`.goreleaser.yml`,
`release.yml`) is unchanged since the last successful publish, so the risk is
low for this cut, but the commitment stands.
