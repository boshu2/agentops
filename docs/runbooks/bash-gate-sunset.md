# Bash-gate sunset criterion (`AGENTOPS_GATE_BASH`)

> **Bead:** cp-4jac (control-plane) records this criterion. **Execution** of the retirement is
> cp-v8m.6 (control-plane) — "PB2-tail: retire `scripts/pre-push-gate.sh` (2210 LOC) +
> `AGENTOPS_GATE_BASH`". This doc gives the bash gate a death date condition; it does not delete it.

## Current state (two gates, one default)

`scripts/hooks/pre-push.local` routes every push through:

| Route | Trigger | Status |
|---|---|---|
| **Go gate** (`ao gate check --fast`, built from source) | default | the live gate since PB2 (ag-3n71.2) flipped the default |
| **Bash gate** (`scripts/pre-push-gate.sh --fast`) | `AGENTOPS_GATE_BASH=1` | legacy escape hatch, kept for the transition |
| **Audited bypass** | `AGENTOPS_GATE_DISABLED=1` | logged exit for infra failures — see [`beads-failure-recovery.md`](beads-failure-recovery.md) |

The migration epic was ag-3n71 (agentops bd), superseded and re-homed to **cp-v8m** in control-plane
br (control-plane is the control plane; agentops bd was the wrong tracker for cross-repo
gate-architecture work).

## The sunset criterion

The bash gate (`scripts/pre-push-gate.sh` + the `AGENTOPS_GATE_BASH=1` branch in
`scripts/hooks/pre-push.local`) is deleted when **all** of the following hold:

1. **CI no longer needs its scripts (PB3, cp-v8m.2).** `validate.yml` runs `ao gate check
   --full --tier` instead of per-job bash orchestration. Until then the bash gate's backing
   scripts are load-bearing in CI even if no one sets the hatch.
2. **Coverage-equivalence stays green with the deferred list empty.**
   `TestRegistryCoversBashGateBackingScripts` (`cli/internal/gates/checks/coverage_test.go`) is the
   no-strangler net: every bash-gate backing script must be in the Go registry. The test currently
   allows an explicit `deferredBacking` exception list (`check-agents-hash-snapshot.sh`); sunset
   requires that list drained to empty — equivalence with exceptions is not equivalence.
3. **Bake-time confidence on real pushes.** The Go gate has been the default on real pushes long
   enough to trust (cp-v8m.6's "real-push bake-time confidence"). Benchmark: the council
   sequencing for the Olympus kernel uses **30 days of gating real closes**
   (cp-verdict-gate-family-author-enforcement-icb6 + cp-27gq before
   cp-olympus-kernel-v1-7kvz starts) — apply the same bar here: 30 days of the Go gate as default
   with no gate-miss incident (a defect the bash gate would have caught and the Go gate did not).
4. **The retirement is recorded in the retirement ledger.** Per the retirement invariant
   (cp-retirement-ledger-mjdm: every engine milestone retires bash control loops or the milestone
   stops), the deletion must show up as a decrease in the live bash-controller count — the bash
   gate's ~2210 LOC is a first-class deletion target, not silent cruft removal.

**When all four hold, the action is:** delete `scripts/pre-push-gate.sh`, remove the
`AGENTOPS_GATE_BASH` branch from `scripts/hooks/pre-push.local`, keep `AGENTS-WORKFLOW.md` aligned
with the Go-gate authority model, and record the deletion in the
retirement ledger. That work executes under **cp-v8m.6**; do not partially retire (a deleted script
with a live hatch is a broken hatch, and a removed hatch with a live script is dead code).

**Until then:** the hatch stays. Fail-closed — if the Go gate regresses, the documented fallback is
`AGENTOPS_GATE_BASH=1`, never `--no-verify`.

## Who watches this

- Tracking bead in this repo's bd: **ag-ltjq** ("bash gate sunset: retire scripts/pre-push-gate.sh
  and AGENTOPS_GATE_BASH hatch when criterion holds") — pointer bead filed by cp-4jac; execution
  stays in control-plane cp-v8m.6.
- Conformance test: `cd cli && go test ./internal/gates/checks -run TestRegistryCoversBashGateBackingScripts`

## Cited beads

| Bead | Tracker | Role |
|---|---|---|
| cp-4jac | control-plane br | this doc (criterion recorded) |
| cp-v8m / cp-v8m.2 / cp-v8m.6 | control-plane br | gate-collapse epic / PB3 CI cutover / the retirement itself |
| ag-3n71 (+ .2) | agentops bd | superseded migration epic; PB2 default-flip history |
| cp-retirement-ledger-mjdm | control-plane br | retirement invariant + live bash-loop count |
| cp-verdict-gate-family-author-enforcement-icb6, cp-27gq, cp-olympus-kernel-v1-7kvz | control-plane br | the 30-day "gated real closes" sequencing the bake-time bar mirrors |
