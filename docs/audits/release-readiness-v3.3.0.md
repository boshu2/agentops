# Release readiness — v3.3.0

Date: 2026-07-24. Tag: `v3.3.0` at `523dcb6635f24afa860c728e9215ba1348a6f9e1` (retagged from `d9eeee9cb` after removing the retired `sync-hooks` goreleaser pre-hook, PR #990; content delta between the two trees is that one-line release-plumbing fix only).

## Verdict: PASS — official mode, score 10.0 / threshold 8

| Dimension | Status | Points |
|---|---|---|
| SIL (software-in-loop: full test suites) | pass | 2 |
| VIL (validation-in-loop: gates, regen, docs) | pass | 2 |
| HIL (released artifact on real target) | pass | 2 |
| Artifacts | pass | 1.5 |
| Security | pass | 1.5 |
| Evals | pass | 1 |

## HIL evidence (no waiver used)

Real target, strong workflow: the published `ao-darwin-arm64` asset from the v3.3.0 GitHub release ran `ao version` (output: `ao version 3.3.0`, version_verified=true) and a full `ao init` (scaffolded `.agents/ao/{intents,verdicts,sessions,index,provenance}` in a scratch repo) on Bo-Mac (Darwin arm64, kernel 25.5.0).

- `hil-evidence.json` sha256: `1cb7568051ce6c648f15ed39666f5ff9e2dfb17938a0fd7e45773a625f585322`
- `release-readiness.json` sha256: `d2c1e61e9bd36f727fca5b99fcd02273da945018bcc4a7f40850cbf9d9e53676`
- Release Publisher run: 30055518194 (success). Tag Validate: success.

## Factory canary: deferred by operator decision, not waived silently

The GC pack ships as **preview** pinned to GC v1.3.5, whose three documented upstream defects (claim-store identity; gastownhall/gascity#3985; gastownhall/gascity#4586) preclude an autonomous factory-canary lap by design. Five stopped 3.3 canary attempts are on record. The operator's decision: the full mixed-provider factory canary is **mandatory and non-waivable at the pin-bump gate** (bead ag-agentops-33-gc-refinery-4km81.16) that promotes the pack preview → supported. It does not gate this release, whose notes disclose exactly this state.

## Process lesson (binding for next release)

The v3.3.0 tag was pushed before this readiness artifact existed; cross-family review (Codex) caught the gap post-tag. Rule going forward: **the official-mode readiness artifact is produced and recorded BEFORE the tag is pushed.** Additionally, the release path itself needs a between-releases smoke (`goreleaser release --snapshot` in CI) — the retired-hook failure sat undetected for 20 days because only a real tag exercises it.
