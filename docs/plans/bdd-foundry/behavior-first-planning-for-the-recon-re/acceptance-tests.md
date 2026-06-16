# Acceptance Tests — Recon-Recommended Work (bdd-foundry Phase 2, ATDD)

> **Phase 2 of bdd-foundry — EXECUTABLE definition of done.** Every FROZEN
> scenario in [`behaviors.md`](behaviors.md) is turned into a runnable test that
> is **currently FAILING (red)** because the feature is not built yet. The test
> IS the executable acceptance contract: **no runnable acceptance test, no bead.**
>
> Tests live under [`acceptance-tests/`](acceptance-tests/). Two runners:
> **bats** for the workflow/script/content/git surfaces and a thin **bats→`go test`**
> driver for the Go-backed liveness + codex scenarios.

## Run the whole suite (one line)

```bash
bash docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/run-acceptance.sh
```

Exit 0 only once every feature ships. Today it is RED (the expected ATDD state).
Run an individual stream with `bats <file>`; filter one scenario with
`bats <file> --filter "<id>"`.

## Expected color today (RED-by-construction, with explicit guardrails)

Most scenarios are **RED** (feature unbuilt). A handful are **GREEN guardrails** —
they assert an invariant that holds now AND must keep holding after the work lands
(stable precondition / forbidden-revert / regression). They are listed as
`GUARDRAIL (green)` below.

## Scenario → test map

### Stream A — codebase-recon workflow hardening
File: `acceptance-tests/stream-a-codebase-recon.bats`

| Scenario | Test name (bats `@test`) | Today |
|---|---|---|
| A0-S1 | `A0-S1: codebase-recon.js is a governed repo citizen ...` | red |
| A0-S2 | `A0-S2: bijection + regen drift gates pass ...` | GUARDRAIL (green) |
| A0-S3 | `A0-S3: a tracked .js with NO ledger row fails governance loudly ...` | GUARDRAIL (green) |
| A0-S4 | `A0-S4: a stale ledger row (kind:workflow) with no .js fails the reverse direction` | GUARDRAIL (green) |
| A0-S5 | `A0-S5: vendoring does not perturb the four pre-existing workflows` | GUARDRAIL (green) |
| A0-G10 | `A0-G10: governance fails when the ledger path disagrees with the tracked .js` | red |
| A0-G14 | `A0-G14: identity is parsed from exported meta.name, not a leading comment ...` | red |
| A1-S1 | `A1-S1: no fable model default anywhere in the workflow source` | red |
| A1-S2 | `A1-S2: explicit args.model=opus is threaded to worker/repair dispatch` | red |
| A1-S3 | `A1-S3: regression — no 'fable' default pin that would total-fail the fan-out` | red |
| A1-G7 | `A1-G7: explicit args.model=fable is rejected before fan-out ...` | red |
| A2-S1 | `A2-S1: zero reports landed -> status:failed, reports_landed:0, NO synthesis` | red |
| A2-S2 | `A2-S2: at least one usable report -> synth proceeds` | red |
| A2-S3 | `A2-S3: empty != clean — failure is unambiguous in the tool result` | red |
| A2-G4 | `A2-G4: guard counts USABLE reports (non-empty + required marker) ...` | red |
| A2-G8 | `A2-G8: report-dir IO error is a hard failure before synth ...` | red |
| A3-S1 | `A3-S1: repair dispatches on a model tier != the worker tier` | red |
| A3-S2 | `A3-S2: model-unavailable class skips the same-model retry` | red |
| A3-S3 | `A3-S3: cross-family repair escalation is codex or agy, never fable` | red |
| A4-S1 | `A4-S1: args.scope string reaches every worker AND repair prompt` | red |
| A4-S2 | `A4-S2: args.since resolves REF..HEAD with diffstat + log range injected` | red |
| A4-S3 | `A4-S3: a bare string arg is treated as args.scope (not dropped)` | red |
| A4-S4 | `A4-S4: an unresolvable since ref fails loudly, no silent full-repo fallback` | red |
| A4-G5 | `A4-G5: a since ref with shell metacharacters is passed to git as argv/data ...` | red |
| A4-G6 | `A4-G6: scope text is bounded/fenced data, cannot inject worker instructions` | red |
| A5-S1 | `A5-S1: committed monitor guidance hard-requires task-state tools ...` (content-assertion) | red |
| A5-S2 | `A5-S2: documented contract says a tool-less monitor aborts, not verdict` (content-assertion) | red |
| A5-G12 | `A5-G12: guidance also aborts on malformed/timeout/partial task-state ...` (content-assertion) | red |

### Stream B — recon repo action items (B1, B2, B4)
File: `acceptance-tests/stream-b-recon-actions.bats`

| Scenario | Test name (bats `@test`) | Today |
|---|---|---|
| B1-S1 | `B1-S1: validate.yml invokes check-doc-skill-refs.sh --strict (occurrence >= 1)` | red |
| B1-S2 | `B1-S2: the detector passes --strict against HEAD after the sweep` | red |
| B1-S3 | `B1-S3: a fixture doc citing a retired skill on a non-exempt line is caught ...` | GUARDRAIL (green) |
| B1-S4 | `B1-S4: a retirement-note line is exempt and does not fail` | GUARDRAIL (green) |
| B1-S5 | `B1-S5: archival refs are removed OR allowlisted with an inline reason ...` | red |
| B1-G9 | `B1-G9: the checker is a REQUIRED, live, path-reaching CI job ...` | red |
| B1-G11 | `B1-G11: incidental retirement words do NOT exempt a live stale ref ...` | red |
| B2-S1 | `B2-S1: CHANGELOG has a v3.2.0 section listing the release-window items` | red |
| B2-S2 | `B2-S2: the quorum change is flagged BREAKING with the correct new default` | red |
| B2-S3 | `B2-S3: the v3.2.0 tag exists locally and points at the release-window HEAD` | red |
| B2-S4 | `B2-S4: B2's bead carries a blocks dep so B3 closes first (ordering)` | red |
| B2-G15 | `B2-G15: the v3.2.0 tag exists on origin at HEAD (remote release proof)` | red |
| B4-S1 | `B4-S1: build/vet/test stay green after extraction` | GUARDRAIL (green) |
| B4-S2 | `B4-S2: the command surface is unchanged after decomposition` | GUARDRAIL (green) |
| B4-S3 | `B4-S3: the cli/cmd/ao top-level .go file concentration is measurably reduced` | red |

### Stream B — Go-backed scenarios (B3, B5)
File: `acceptance-tests/stream-b-go.bats` (drives `go test` against the real packages).
Go test templates: `acceptance-tests/go-acceptance/*.go.txt` (copied into the target
package as `recon_acceptance_*_gen_test.go`, run, then removed in teardown — the
docs tree never holds compilable test files; the package stays clean).

| Scenario | Go test func | bats `@test` | Today |
|---|---|---|---|
| B3-S1 | `TestReconAcceptanceB3S1QuorumDefaultCrossFamilyOff` (liveness) | `B3-S1 + B5-G3: quorum default OFF + forged-ratification cannot execute (liveness)` | GUARDRAIL (green) — fails if anyone forces the family floor (the forbidden revert) |
| B5-G3 | `TestReconAcceptanceB5G3ForgedRatificationCannotExecute` (liveness) | (same bats line as B3-S1) | red |
| B5-S1/S2 | `TestReconAcceptanceB5S1S2UntrustedPacketRefused` (cmd/ao) | `B5-S1..S4 + B5-G1/G2/G13: codex sh -c trust boundary + symlink-safe paths (cmd/ao)` | red |
| B5-S3 | `TestReconAcceptanceB5S3ProvenanceKeyingDecided` (cmd/ao) | (same bats line) | red |
| B5-S4 | `TestReconAcceptanceB5S4TrustedPathPreserved` (cmd/ao) | (same bats line) | GUARDRAIL (green) — trusted path regression |
| B5-G1 | `TestReconAcceptanceB5G1DuplicateSandboxRejected` (cmd/ao) | (same bats line) | red |
| B5-G2 | `TestReconAcceptanceB5G2RequiredCommandsGuarded` (cmd/ao) | (same bats line) | red |
| B5-G13 | `TestReconAcceptanceB5G13SymlinkOutputPathRejected` (cmd/ao) | (same bats line) | red |

> The grouped Go bats lines pass only when EVERY scenario in their package run is
> green — i.e. when the whole B3/B5 feature set ships. That is the intended
> definition-of-done semantics: the green guardrails (B3-S1, B5-S4) ride along but
> the line stays red until the new behaviors land.

## Direct per-package Go invocations (for implementers)

```bash
# B3-S1 + B5-G3 (liveness) — copy template, run, clean:
cp docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/go-acceptance/recon_acceptance_liveness_test.go.txt cli/internal/liveness/recon_acceptance_gen_test.go
( cd cli && go test ./internal/liveness/ -run ReconAcceptance -v ); rm -f cli/internal/liveness/recon_acceptance_gen_test.go

# B5 (cmd/ao):
cp docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/go-acceptance/recon_acceptance_codex_test.go.txt cli/cmd/ao/recon_acceptance_codex_gen_test.go
cp docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/go-acceptance/recon_acceptance_codex_helpers_test.go.txt cli/cmd/ao/recon_acceptance_codex_helpers_gen_test.go
( cd cli && go test ./cmd/ao/ -run ReconAcceptanceB5 -v ); rm -f cli/cmd/ao/recon_acceptance_codex_gen_test.go cli/cmd/ao/recon_acceptance_codex_helpers_gen_test.go
```

## Notes for implementers

- **A1–A4** assert against the in-repo `.claude/workflows/codebase-recon.js` —
  they are red until **A0** vendors that file (the BLOCKER). The greps encode the
  required source shape (args threading, fail-closed guards, fenced scope data,
  argv-not-shell `since`); make the source satisfy them.
- **A5 / B2 / B3** are doc/process scenarios per their behaviors.md dispositions:
  content-assertions (grep over committed guidance/changelog/memories), git-tag
  assertions, and a code-assertion floor (B3-S1). A5 deliberately EXCLUDES the
  plan tree so it stays red until the guidance is written to a surface a monitor
  reads (`docs/memory/` or the workflow note).
- **B2-S3 / B2-G15** assert the local + remote `v3.2.0` tag. The push is
  conductor/operator-performed (this lane never pushes); the assertion is the gate.
- **B5** Go tests key to the NEW behavior's specific error signature
  (`duplicate sandbox`, `operator-trusted`/`trust boundary`, symlink escape) so
  they cannot be satisfied by an unrelated existing rejection
  (schema/auth/sandbox-mismatch). Implement the real guard, not a string.
