# Gate reference — per-gate failure patterns, setup, and the ledger schema

Read this when a specific gate is red, when setting up miri/fuzz, or when defining
the ledger. The fixed gate order (cheap/deterministic first) is:
**fmt → build → clippy → test → miri → fuzz → bench**.

## 1. fmt

```bash
cargo fmt --all -- --check
```

- **Red:** any diff. **Fix:** `cargo fmt --all` (no `--check`), re-run.
- Never edit `rustfmt.toml` to silence a diff — that is weakening the gate.

## 2. build

```bash
cargo build --all-targets --all-features
```

- **Red:** compile error. Fix the real error.
- Common port pitfalls: missing feature gates, integer-width mismatches from the
  source language, `unsafe` blocks that don't actually compile under the new API.
- Do not reach for `--no-default-features` to dodge a build error — that hides the
  defect and is a logged anti-pattern.

## 3. clippy (deny warnings)

```bash
cargo clippy --all-targets --all-features -- -D warnings
```

- **Red:** any lint, because `-D warnings` promotes all to errors.
- Fix the lint. Scoped `#[allow(specific_lint)]` is permitted ONLY with a one-line
  justification in the ledger explaining why the lint is provably inapplicable.
- Blanket `#[allow(clippy::all)]` or module-wide allows are forbidden — they defeat
  the gate.

## 4. test

```bash
cargo test --all-features
```

- **Red:** failing or panicking test. Fix the code, not the assertion, unless the
  assertion is provably wrong (record why in the ledger).
- `#[ignore]` to make the suite green is forbidden. A genuinely environment-gated
  test must be WAIVED with evidence, not silently ignored.

## 5. miri (undefined behavior)

```bash
rustup +nightly component add miri    # one-time
cargo +nightly miri test
```

- Detects UB: out-of-bounds, use-after-free, invalid alignment, data races,
  uninitialized reads — the failure modes a port through `unsafe`/FFI introduces.
- **Red:** miri prints the exact UB and a backtrace. Fix the unsafe code.
- **Unsupported:** miri cannot model arbitrary foreign (non-Rust) calls. If the
  crate's UB surface is behind an FFI boundary miri refuses, log it WAIVED with the
  literal "unsupported operation" message as evidence — do not fake a pass.

## 6. fuzz

```bash
cargo install cargo-fuzz                       # one-time
cargo +nightly fuzz run <target> -- -max_total_time=60
```

- Always bound the run with `-max_total_time=N`; an unbounded fuzz never returns and
  stalls the loop.
- **Red:** a crash artifact under `fuzz/artifacts/`. Reproduce, fix, re-run.
- A clean bounded run is PASS for the loop's purposes; deeper campaigns are a
  separate, longer-running effort.

## 7. bench

```bash
cargo bench
```

- The gauntlet treats bench as **must-run-clean**, not must-beat-a-number, unless a
  perf budget is declared. A bench that fails to compile or panics is RED.
- If a regression threshold matters, record the baseline numbers in the ledger and
  compare against them.

## Ledger schema (GAUNTLET-LEDGER.md)

```markdown
# Gauntlet Ledger — <crate>

Toolchain: <rustc -Vv first line>
Features: <enabled features>
Snapshot taken: <date>

## Gate matrix
| Gate   | Status  | Last run | Notes |
| ------ | ------- | -------- | ----- |
| fmt    | PASS    | <date>   |       |
| build  | PASS    | <date>   |       |
| clippy | RED     | <date>   | needless_lifetimes in shim.rs |
| test   | UNKNOWN | -        |       |
| miri   | UNKNOWN | -        |       |
| fuzz   | UNKNOWN | -        |       |
| bench  | UNKNOWN | -        |       |

## Negative-evidence log (never retry these)
| Gate   | Approach tried | Why it failed | Date |
| ------ | -------------- | ------------- | ---- |
| build  | --no-default-features to dodge missing feature | hid the real defect; forbidden | <date> |
```

**The rule:** before any edit, scan the negative-evidence log for the same
(gate, approach). If present, choose a different approach. After any failed edit,
append a row. This is what makes the autonomous loop monotonic instead of cyclic.
