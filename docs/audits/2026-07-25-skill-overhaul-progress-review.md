# Skill and CLI Overhaul — Progress Review

- Date: 2026-07-25
- Scope: the SOL-orchestrated skill-system overhaul (`docs/plans/2026-07-24-skill-system-overhaul.md`)
  and its tranche fleet, plus the parallel Go CLI release program
- Mode: read-only review; every claim below was re-derived from the live tree,
  not taken from the plan or duel prose
- Reviewer: independent context, cross-family (Claude) against Codex-authored work

## Verdict in one line

The diagnosis is real and the proof discipline is working. The delivery is not:
two of nine tranches are proven, nothing is pushed, and the wave now in flight
violates a canonized architecture decision (ADR-0016) at scale because no gate
enforces it.

## What is actually built

The seed branch (`codex/skill-overhaul-seed`) is documentation only — 7,439
lines, zero code — and is 73 commits behind `origin/main`. The execution lives
on a fleet:

| Branch | Own commits vs integration | State |
|---|---:|---|
| `codex/skill-overhaul-20260724` | — (integration, 55 ahead of main) | T0 proven, T1 proven, T2 in repair |
| `codex/t1-repair-20260724` | 1 | folded into integration |
| `codex/t2-repair-publisher-20260724` | 12 | unconverged |
| `codex/t3-product-campaign-20260724` | 15 | code, no verdict |
| `codex/t4-evidence-judgment-20260724` | 7 | code, no verdict |
| `codex/t5-specialists-20260724` | 4 | code, no verdict |
| `codex/t6-evolution-20260724` | 4 | code, no verdict |
| `codex/t7a-w1-repair2-20260725` | 8 | code, no verdict |
| `codex/t7b-support-20260724` | 4 | code, no verdict |
| `codex/t8-cutover-20260724` | 1 | frozen intent only |
| `codex/go-g0-g2-fast-exit-repair-20260724` | 6 | unconverged |

`git ls-remote --heads origin` returns **no** tranche branch. There are **zero**
pull requests for T1 through T8 or for G0 through G2. The entire overhaul —
roughly 30k lines across the fleet — exists on one machine, unreplicated.

## What is genuinely good

Both load-bearing diagnostic claims were independently reproduced in this
review rather than accepted from the duel report.

### The exact-identity defect is real

- `skills/rpi/scripts/run_once.py:18` digests a canonical JSON serialization of
  the resolved intent mapping.
- `skills/validate/scripts/validate.py:210` digests the raw intent bytes.
- `skills/rpi/scripts/run_once.py:77` hard-compares the two and raises.
- `skills/rpi/tests/test_run_once.py:31` mocks Validate with **RPI's own digest
  function**, so both unit suites pass green over a broken composed contract.

This is a genuine hole in the exact-identity property the whole product rests
on, found by analysis rather than asserted.

### The drift gate is dead

`make regen-check` exits 126: `scripts/regen-all.sh` is mode `100644` and is
invoked as `./scripts/regen-all.sh --check`. A load-bearing pre-push
derived-artifact drift gate has been silently non-functional. Confirmed live.

### The membrane refutes

Ten durable verdicts exist across the proof epochs:

| Epoch | PASS | FAIL | NOT_PROVEN |
|---|---:|---:|---:|
| epoch-0 | 0 | 1 | 0 |
| epoch-0b | 1 | 3 | 1 |
| epoch-1 | 1 | 2 | 1 |

Six FAIL against two PASS. T0 took four rejection-repair rounds; T1 was
rejected, repaired, re-frozen, and only then activated as epoch 1. Judgment is
not rubber-stamping the candidate, which is the property that matters most.

## Finding 1 — Only two of nine tranches are complete

The plan's own completion rule is that every migration tranche must carry a
durable verdict under an activated proof contract. Measured against that rule:

- T0 and T1 are done.
- T2 through T8 carry substantial code and **zero** new verdicts. Every tranche
  branch carries the same inherited ten verdict records as the integration
  branch — no tranche after T1 has produced its own.
- T3 adds 8,634 lines. T7a adds 18,305 lines. Neither has been judged.
- T8 is a frozen intent document and nothing else.

Meanwhile T2 is on its fifth repair branch (`t2-repair-compiler`,
`t2-repair-go`, `t2-repair-publisher`, `t2-mode-repair`,
`t2-compiler-reader-canonical-ref-repair`). The front of the queue has not
converged and six tranches behind it are already writing code against its
unsettled contract.

This is the failure mode where opening fronts substitutes for closing them.

## Finding 2 — ADR-0016 is being violated at scale, silently

ADR-0016 states the rule without hedging:

- line 97: *"Python never ships in skills."*
- line 123: *"Shipping Python inside a skill is a gate failure, not a style
  nit."*

Measured against the live tree:

| Location | `origin/main` | `codex/t7a-w1-repair2-20260725` | Delta |
|---|---:|---:|---:|
| `skills/*/scripts/**.py` (user execution path) | 24 | 45 | +21 |
| `skills/*/tests/**.py` | 3 | 31 | +28 |
| other `skills/**.py` (fixtures, fakes) | 0 | 13 | +13 |
| **total** | **27** | **89** | **+62** |

Three compounding problems:

1. **The plan never mentions it.** The 798-line overhaul plan contains the word
   Python exactly three times, all as incidental filenames. The language rule
   was never considered, accepted, or waived — it was invisible.
2. **No gate enforces it.** A sweep of `scripts/` and `cli/internal/gates`
   finds no check for shipped Python in skills. ADR-0016 is inert prose, which
   is the same disease as the dead `regen-check`: a rule nobody executes.
3. **Duplication has already started.** `strict_json.py` — 130 lines — is
   vendored into four separate skills on the T3 branch. That is precisely the
   shared-mechanism code ADR-0016 routes into the `ao` binary, and it is
   already copy-pasting.

This is a one-way door. Once 89 Python files ship to strangers' machines,
"rewrite it in Go" never happens. The determinism pitch — deterministic
verification from a single static binary — degrades into "deterministic, if
your interpreter, venv, and PATH cooperate."

## Finding 3 — Merge debt is compounding against a moving main

The integration branch is built on a `main` that has advanced 73 commits since
the seed was cut. Thirty thousand unproven, unpushed lines are accumulating
against it while six downstream tranches build on an unconverged T2. Every day
this sits unlanded, the eventual reconciliation gets more expensive and less
reviewable.

## How this should proceed

### P0 — Push and land the proven kernel

T0 and T1 are proven under an activated proof contract. They are the exact
change that fixes the digest defect and they unblock every tranche behind them.
They should be a pull request today. The current state — the entire overhaul
resident on one disk with no remote copy — is an unforced risk that has nothing
to do with the engineering.

Land T0 and T1 first, separately from everything else.

### P1 — Enforce ADR-0016 before another line of Python lands

Order matters: ship the gate, then migrate. Reversing that order means the gate
never ships.

1. **Add the check now, grandfathered.** A gate that fails on any
   `skills/*/scripts/**.py` not present in a pinned allowlist of the 24 files
   currently on `main`. New Python in a skill's execution path becomes a hard
   failure immediately. Print the grandfathered count in the gate output so the
   number is visible and can only ratchet down.
2. **Decide the tests question explicitly, in writing.** 31 of the 62 new files
   are `skills/*/tests/**.py` and never execute on a user's machine — the
   interpreter-state argument does not reach them. That is a legitimate
   refinement, not a loophole, but it must be a recorded amendment to ADR-0016
   with its rationale, not an unstated exception. Same for the 13 fixture and
   fake files. If tests are exempted, say so in the ADR and scope the gate to
   `scripts/` accordingly.
3. **Classify the 21 new execution-path files before T3 or T7a land.** Shared
   mechanism (`strict_json.py` and its three siblings) goes into `ao` as one
   owner. Skill-specific parsing and contract logic becomes `ao` subcommands
   the skill invokes through `sh` glue. Anything that survives that
   classification as genuinely un-promotable is the evidence that ADR-0016
   needs a real amendment — but that case has to be made per file, not assumed
   wholesale.
4. **Treat this as a tranche, not a cleanup.** It gets frozen intent, a RED
   witness, and a durable verdict like everything else. A language-rule
   migration proven by "looks done" would be the same category error as the
   inert ADR.

### P2 — Freeze new tranche branches until T2 carries a verdict

No new tranche work starts until T2 converges and lands. Six tranches building
against a contract that has been through five repair rounds is how this becomes
an unmergeable pile. The tranche fleet's value is parallelism across
*independent* surfaces; T3 through T8 all depend on the T2 compiler, so they are
not independent.

### P3 — Fix the inert-check class, not just the two instances

`regen-check` exits 126 and nobody noticed. ADR-0016 has no gate and nobody
noticed. These are the same defect: a rule that exists only as text. The plan's
T0 already includes check-liveness classification with seeded negative
witnesses — that work should be generalized into a standing requirement that
every load-bearing check demonstrably detects its intended negative, and every
ADR with a mechanical claim names the check that enforces it or is explicitly
marked advisory.

## What was checked

- Branch topology, ahead/behind counts, and remote presence for all 47 local
  `codex/*` branches.
- Verdict tally by status across all proof epochs on the integration branch and
  on all eight downstream tranche branches.
- Direct read of `run_once.py`, `validate.py`, and `test_run_once.py` to
  reproduce the digest mismatch.
- Live execution of `make regen-check` and the file mode of
  `scripts/regen-all.sh`.
- Python file counts by path class on `origin/main` and on the most advanced
  tranche branch, split into scripts, tests, and other.
- Grep sweep of `scripts/` and `cli/internal/gates` for any ADR-0016
  enforcement.
- `docs/adr/ADR-0016-state-tiers.md` lines 91 through 123 for the exact rule
  text.

## What was not checked

- No tranche branch was checked out, built, or tested; assessment of T2 through
  T8 code quality is by diff statistics and file inventory only.
- No verdict record was validated for schema conformance or freshness
  attestation.
- The duel's wizard artifacts were read for their rulings, not audited for
  faithfulness to the sealed proposals.
- No claim is made about whether the 21 new execution-path Python files are
  individually promotable to Go; that classification is the P1.3 work.
