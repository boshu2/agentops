---
name: rust-port-validation-gauntlet
description: |-
  Use when running a Rust port through build, test, clippy, fmt, miri, fuzz, and bench gates.
  Triggers:
skill_api_version: 1
practices:
- evidence-over-confidence
- iterate-to-green
- negative-evidence-ledger
hexagonal_role: supporting
consumes: []
produces:
- gauntlet-ledger
context:
  window: isolated
  intent:
    mode: none
metadata:
  tier: library
  stability: experimental
  dependencies: []
output_contract: 'file: GAUNTLET-LEDGER.md (gate matrix + negative-evidence log); stdout: per-gate PASS/FAIL'
user-invocable: false
---
# rust-port-validation-gauntlet — drive a crate to all-green through every quality gate

> Take a Rust port/crate and run it through the full gate battery as a loop that
> iterates until everything is green, recording every dead end so it is never retried.

## ⚠️ Critical Constraints

- **One gate, one fix, one re-run — never batch.** Run a gate, fix exactly what it
  reports, re-run that same gate before advancing.
  **Why:** batched edits make it impossible to attribute a regression; a "fix" for
  clippy can silently break a test, and you lose the causal chain.
- **Every failed attempt goes in the ledger before the next attempt.** No fix is
  tried twice.
  **Why:** the loop's whole value is monotonic progress; without negative evidence
  an autonomous agent re-discovers the same dead end on every pass and never converges.
- **Never weaken a gate to make it pass.** Do not add `#[allow(...)]`, `--no-default-features`,
  `#[ignore]`, or lower the lint level to get green.
  **Why:** that converts a real defect into hidden debt and defeats the gauntlet.
  WRONG: `#[allow(clippy::all)]` over the module. CORRECT: fix the lint, or record
  in the ledger why the lint is provably inapplicable with a scoped, justified allow.
- **Gate order is fixed and fail-fast.** fmt → build → clippy → test → miri → fuzz → bench.
  **Why:** cheap, deterministic gates first; a fmt or build failure invalidates every
  downstream result, so running them first saves the whole battery.

## Why This Exists

A port (C/C++/Python → Rust, or a crate-split) is exactly where latent undefined
behavior, lint debt, and missing test coverage hide. Running gates ad-hoc and by
hand means an agent forgets what it already tried, oscillates between two broken
states, and declares victory after the cheap gates pass. This skill makes the
gauntlet **autonomous and monotonic**: a fixed gate order, a verification checkpoint
between every gate, and a persisted negative-evidence ledger so the loop converges
instead of looping forever.

## Quick Start

```bash
# from the crate root (the dir with Cargo.toml)
fmt:   cargo fmt --all -- --check
build: cargo build --all-targets --all-features
clippy:cargo clippy --all-targets --all-features -- -D warnings
test:  cargo test --all-features
miri:  cargo +nightly miri test            # UB detector (needs rustup component add miri)
fuzz:  cargo +nightly fuzz run <target> -- -max_total_time=60
bench: cargo bench
```

Initialize the ledger once, then enter the loop:

```bash
bash {baseDir}/scripts/init-ledger.sh GAUNTLET-LEDGER.md
```

## The Gauntlet Loop

1. **Snapshot.** Record the crate name, toolchain (`rustc -Vv`), and enabled features
   into the ledger header. This pins what "green" means.
2. **Pick the next red gate** in fixed order: fmt → build → clippy → test → miri →
   fuzz → bench. Run only that gate.
3. **If green:** mark the gate PASS in the matrix, advance to the next gate.
4. **If red:** read the failure. **Before editing, check the ledger** — if this exact
   approach is already a logged dead end, pick a different one. Apply ONE targeted fix.
5. **Verification checkpoint:** re-run the same gate. Also re-run fmt + build + clippy +
   test (the fast quartet) to catch a fix that regressed an earlier gate.
6. **Log the outcome.** If the fix worked, note it. If it failed, append a
   negative-evidence row (gate, approach, why it failed) so it is never retried.
7. **Repeat** until all gates in the matrix are PASS, or until a gate is logged as
   blocked with evidence (e.g. miri unsupported for an FFI call) and explicitly waived.

Per-gate failure patterns, miri/fuzz setup, and the ledger schema:
see [references/gates.md](references/gates.md).

## Output Specification

- **File:** `GAUNTLET-LEDGER.md` at the crate root.
  - A **gate matrix** table: `| Gate | Status | Last run | Notes |`.
  - A **negative-evidence log** table: `| Gate | Approach tried | Why it failed | Date |`.
- **Stdout:** one PASS/FAIL line per gate per pass.
- Done = every row in the gate matrix is PASS or an evidence-backed WAIVED.

## Quality Rubric

- Every gate in the matrix is PASS or WAIVED-with-evidence; no gate left UNKNOWN.
- No gate was made to pass by weakening it (no blanket `allow`, no `--no-default-features`
  shortcut, no `#[ignore]`) — verifiable by diffing the crate against the start snapshot.
- The negative-evidence log has no duplicate (gate, approach) rows — the loop never
  retried a dead end.

## Examples

- **Clippy red on a ported module:** `cargo clippy ... -D warnings` flags
  `needless_lifetimes`. Fix the signature, re-run clippy + the fast quartet, mark PASS.
- **Miri finds UB in unsafe FFI shim:** miri reports a use-after-free through a raw
  pointer. Fix the lifetime; if miri cannot model the foreign call at all, log it
  WAIVED with the exact unsupported-operation message as evidence.

## Troubleshooting

| Symptom | Likely cause | Move |
| --- | --- | --- |
| Loop oscillates between two failures | A fix for gate X regresses gate Y | Re-run the fast quartet after every fix (step 5); log both as a paired dead end |
| `cargo miri` not found | miri component/toolchain missing | `rustup +nightly component add miri`; see references/gates.md |
| Fuzz never finds a crash but never returns | No time bound | Always pass `-- -max_total_time=N`; treat clean run as PASS |
| Same fix attempted twice | Ledger not consulted before editing | Enforce step 4: read the negative-evidence log first |

## See Also

| I need to… | Reference |
| --- | --- |
| Per-gate failure patterns, miri/fuzz/bench setup, ledger schema | [references/gates.md](references/gates.md) |
