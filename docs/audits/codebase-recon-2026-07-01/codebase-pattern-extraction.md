# Codebase Pattern Extraction — agentops (2026-07-01)

**Repo:** `/Users/bo/dev/agentops` (HEAD `e4ec22c46` at scan time)
**Date:** 2026-07-01
**Skill:** `codebase-pattern-extraction` (read-only recon pass)
**Method:** Discover (grep/rg sweeps, ≥3 instances per pattern) → Collect concrete sites → Diff/Align (invariant vs variance) → recommend Package (library / template / gate). Per the skill: *"If you solved it twice, you'll solve it again. Extract once, reuse forever."*

> **Scope notes.**
> 1. The skill's canonical prompt targets *multiple* repos. Applied to one large monorepo (~317 top-level `scripts/*.sh`, 715 Go source + 662 Go test files in `cli/`, 70 `skills/*/SKILL.md`), the "projects" become subsystems: `cli/internal/*` packages, `cli/cmd/ao` (one `package main`, 614 files), the script corpus, and the test corpora.
> 2. **CASS mining deliberately skipped** this pass: any `cass search` re-ingests the live session DB (known footgun, memory `cass-quality-semantic-build-blocked`), and the 2026-06-24 recon already ran the CASS lane. All discovery here is repo-grep-grounded.
> 3. A prior pattern pass exists: [`docs/audits/codebase-recon-2026-06-24/codebase-pattern-extraction.md`](../codebase-recon-2026-06-24/codebase-pattern-extraction.md). This report is a **delta + re-verification**, not a re-statement — §1 closes the loop on its findings before adding new ones.

---

## Executive summary

| # | Pattern | Instances (measured) | Status / Severity | Package as |
|---|---------|----------------------|-------------------|------------|
| P1 | Atomic file write (prior recon's headline) | consolidation ~complete; **1 new escape** (`writeJSONAtomic`) | ✅ mostly CLOSED · escape = LOW | fold escape into `storage.AtomicWriteFile` |
| P2 | **Shell preamble: packaged, zero adopters** | `scripts/lib/preamble.sh` exists; **0/317** scripts source it; **13/13** scripts added *after* the adoption decision hand-roll it again | **HIGH — the headline finding** | new-file-scoped drift gate (no churn of the 284) |
| P3 | **JSONL scan/append ledger plumbing** | helpers exist in `storage` but are **unexported**; 64 files hand-roll scanners, ~20 hand-roll `O_APPEND` writers, 45 buffer bumps in **10+ size variants** | **MED** | export `storage.ScanJSONL` / `storage.AppendJSONL`, adopt at touch-time |
| P4 | `cmd/ao` `--json` output emit | in-package `emitJSON`/`outputJSON` exist; **52 files / 88 sites** still hand-roll `MarshalIndent` | MED-LOW | touch-time adoption of existing helper |
| P5 | Cross-family `codex exec` invocation (timeout + stall/echo/wander defense) | ≥8 scripts each re-solve subsets; hardened logic lives only in `pawl-review.sh` | **MED** | `scripts/lib/codex-exec.sh` sourced lib (or Go port) |
| P6 | Drift-gate triple (check script + bats twin + Go registration) | 104 `check-*.sh`, ~50 bats twins, path-glob seeds in `gates/checks/seed.go`; output protocol only ~25% consistent | LOW (healthy; codify) | scaffold template for NEW gates |
| P7 | Go test micro-helpers (capture-stdout ×5, flag save/restore 86 inline vs 2 helper-shaped) | duplication inside `cli/cmd/ao` tests | LOW | tiny generic helpers in `testutil_test.go` |
| P8 | Script `trap`-cleanup tmpdir + dep checks | 79 trap-EXIT scripts (~6 naming variants); 62 hand-rolled `command -v jq` checks | LOW | fold `with_tmpdir` / `require_cmd` into `preamble.sh` (rides P2) |

**Meta-finding (the pattern under the patterns):** this repo is *good at the first four steps* of the extraction pipeline (collect → diff → abstract → package: `preamble.sh`, `storage.AtomicWriteFile`, the gates registry, and skill-builder templates all exist) and **systematically stops before the skill's final step — "apply back to the source projects."** Three separate extractions (P2, P3, P4) sit packaged-but-unadopted. The repo's own measured lesson applies: documentation-only adoption is inert (memory: the graphify A/B — a doc instruction to prefer a tool changed 0/2 behaviors). The binding constraint is not extraction skill; it is **adoption mechanics**, and in this codebase the only lever that demonstrably works is a gate.

---

## §1 — Re-verification of the 2026-06-24 findings (close the loop first)

The skill's validation step demands applying extractions back to sources. Status of the prior report's actionable items, verified against today's tree:

### P1 (prior) — Atomic write consolidation: **effectively DONE, with one new escape**

- `cli/internal/types/quest/atomic.go:34,45` — both `AtomicWriteFile` variants now **delegate to `storage.AtomicWriteFile`** ✅
- `cli/cmd/ao/inject.go:450` — delegates via `quest.AtomicWriteFileWithPerm` ✅
- `cli/internal/adapters/vendorimage/codexruntime/runtime.go:742` — same delegation ✅
- `cli/internal/pool/pool.go:1152` (`atomicMove`) — **legitimately distinct**: it is a *move* (read src → `writeTempFile` which `Sync()`s → rename), not a write; the prior report's "still omits fsync" claim was already refuted at verification time (memory `recon-findings-are-hypotheses`, age-3azc). No action.
- **NEW escape:** `cli/cmd/ao/forge_curator_id.go:17` `writeJSONAtomic` — re-rolls tmp+rename **without fsync** and with fixed `0o644` perms, bypassing the canonical `storage.AtomicWriteFile` that the consolidation established. It even marshals indented JSON first, i.e. it duplicates *both* P1 and P4 in one function. One-line fix at touch time: marshal, then `storage.AtomicWriteFile(path, data, 0o644)`.

**Lesson recorded:** a consolidation without a guard accretes new copies. If the atomic-write invariant matters, the durable close is a lint/gate (e.g. forbid `os.Rename` after `os.WriteFile`/`CreateTemp` outside `internal/storage`, allowlist the genuinely-different sites), not another sweep.

### P6 (prior) — Shell preamble: packaged (age-0dq9.1) ✅, adoption **pruned then measurably failed** → promoted to this pass's P2 headline (below).

### P3/P4/P5/P7-P9 (prior) — codify-don't-refactor conventions (hexagonal wiring, cobra `--json` split, fail-open annotations, error wrapping, SKILL.md template): unchanged and healthy; no re-litigation.

---

## §2 — P2 (headline): the shell preamble — an extraction that proves the adoption gap

**This is the purest instance of the skill's anti-pattern table in the wild: "Skip validation → apply back to source projects."**

### The history (bead-verified)

1. The 2026-06-24 recon recommended extracting strict-mode + repo-root + portable stat/find into a sourced lib.
2. **Packaged:** `scripts/lib/preamble.sh` was built (age-0dq9.1) — and it is a *good* extraction: hijack-proof `REPO_ROOT` (anchored `git -C` at the lib's own dir), `portable_mtime` (GNU-first `stat -c %Y` with load-bearing probe order), `portable_find` (defeats the interactive `bfs` shim), `newest_by_mtime` (global-sort, SIGPIPE-safe). Each helper encodes a *real bug class this repo has already paid for* (macOS bats sweep, age-0dq9 recon P6a).
3. **Adoption decision:** age-0dq9.2 (migrate ~60 scripts) was **PRUNED 2026-06-26**, deliberately and defensibly: *"big-bang migration with 'no functional diff' acceptance = churn of working code (anti-cathedral). Adopt preamble.sh opportunistically in new/touched scripts instead."*
4. **The measurement this pass adds:** the opportunistic policy has had **0% uptake**. Grep-verified today:
   - Scripts sourcing `preamble.sh`: **0 of 317** (the only match is the lib itself).
   - Scripts **added since the prune** (git `--diff-filter=A --since=2026-06-26`): **13** — `assert-no-actions.sh`, `capture-repo-metrics.sh`, `check-no-operator-skills.sh`, `check-skill-redirects.sh`, `check-spine-integrity.sh`, `check-staged-scope.sh`, `check-workflow-no-retired-tracker.sh`, `land-lane-flaky-retry.sh`, `land-lane-run.sh`, `land-queue-next.sh`, `land-queue-test.sh`, `land-submit.sh`, `verify-buildtags.sh`. **All 13 hand-roll the preamble again** (e.g. `check-spine-integrity.sh:18` re-derives `REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"`; `land-lane-run.sh:65` re-rolls the git-rev-parse fallback).

### Diff/Align (the variance that keeps re-appearing)

154 scripts derive `REPO_ROOT` in **12+ textual variants** (measured by uniq-count):

```
79× REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
23× REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
13× REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"   ← CWD-hijackable variant
10× REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
10× REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
 5× REPO_ROOT="$(pwd)"                                                ← wrong outside root
 … (6 more variants)
```

The variance is *accidental* (no script needs a different resolution semantics except the `$2`-parameterized validators) — exactly what the skill says should be collapsed. And two of the surviving variants are **latent bugs** the lib already fixes (`git rev-parse || pwd` falls back to CWD; bare `$(pwd)` breaks when invoked from subdirs).

### Recommendation (consistent with the anti-cathedral prune)

Do **not** reopen the big-bang migration. Package the missing piece — the *adoption mechanism*:

- **A new-file-scoped drift gate** (`check-new-scripts-use-preamble.sh` + bats twin + `seed.go` registration, i.e. an instance of P6's own triple): fail only for `scripts/*.sh` **added after a pinned cutoff** that define `REPO_ROOT=`/`set -euo pipefail` inline instead of sourcing the lib. Zero churn of the 284 existing scripts; stops the bleeding. This follows the repo's proven lever (gates change behavior; doc instructions measurably don't).
- While there (P8 rides along): add `with_tmpdir` (mktemp + trap-EXIT, currently re-rolled in 79 scripts across ~6 naming variants) and `require_cmd` (the `command -v jq || die` check re-rolled in 62 scripts) to `preamble.sh`.

**Validation criterion (skill §Validation):** LOC saved is modest (~8-15/script), but the payoff metric is the *bug class*: every helper in the lib corresponds to a shipped, paid-for defect (bfs shim, stat order, SIGPIPE, CDPATH hijack). Preventing one recurrence pays for the gate.

---

## §3 — P3: JSONL scan/append — extracted, then locked in a private room

The repo's dominant persistence idiom is append-only JSONL ledgers (provenance, yield, verdicts, ratchet chains, goals history, canon, pool, mine, doctor artifacts…). The plumbing recurs everywhere:

**Collected instances (invariant core = open/scan-lines/unmarshal-per-line + open-`O_APPEND`/encode/write-line):**

- Writers hand-rolling `os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, …)`: **20 non-test files**, incl. `cli/internal/yieldledger/writer.go:70`, `cli/internal/ratchet/chain.go`, `cli/internal/goals/history.go`, `cli/internal/canon/ledger.go`, `cli/internal/provenancegraph/store.go`, `cli/internal/mine/work_items.go`, `cli/cmd/ao/loop_next_work.go`, …
- Readers hand-rolling `bufio.NewScanner` line loops: **64 non-test files** (e.g. `cli/internal/provenance/provenance.go:266`, `cli/internal/yieldledger/loader.go:35`).
- **The tell that this is one pattern, not 64:** 45 sites re-decide the *same* "tolerate long lines" concern with a hand-tuned `scanner.Buffer(...)`, in **10+ distinct configurations**:

```
6× scanner.Buffer(make([]byte, 0, 64*1024),  1024*1024)
5× scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
4× scanner.Buffer(make([]byte, 64*1024),      1024*1024)
3× scanner.Buffer(make([]byte, 0, 64*1024),  4*1024*1024)
3× scanner.Buffer(make([]byte, 0, 256*1024),  256*1024)
… (5 more variants)
```

Every one of these is answering "how big can a JSONL line get?" independently — and any site that *didn't* bump (the other ~19 scanner files) silently truncates at 64KB on a fat line. That is a live bug class in a repo whose transcripts include a checked-in 2.4MB-line fixture (`cli/testdata/transcripts/real-2.4mb.jsonl`).

**The extraction already half-exists:** `cli/internal/storage/file.go` has `scanJSONL` (:225), `scanJSONLFile` (:233), `appendJSONL` (:312) — **all unexported**, usable only inside `storage`. External adoption measured: **0**.

**Recommendation (library — the skill's "Code logic → Library" row):**
- Export from `internal/storage`: `ScanJSONL(path, fn func(line []byte) error) error` and `AppendJSONL(path string, v any) error` (single `Write(marshal+"\n")` for the atomic-append property `yieldledger/writer.go` already documents), with the buffer policy (e.g. 64KB initial / 4MB max) **decided once inside the helper**.
- Adopt at touch-time only (anti-cathedral); optionally seed a grep-level gate for *new* `bufio.NewScanner` over `.jsonl` paths outside `storage`.
- Variance points that stay parameterized: strict-vs-skip on corrupt lines (lifecycle wants skip-with-sentinel, gates want fail-closed), fsync-on-append (ledgers yes, caches no).

---

## §4 — P4: `--json` emit in `cli/cmd/ao` — the helper is *in the same package* and still unused

`cli/cmd/ao` is one `package main` (614 files). It already contains the extracted helper pair:

- `beads.go:910` — `emitJSON(w *os.File, v any)` (json.Encoder, two-space indent)
- `lookup.go:502` — `outputJSON(v any)` → delegates to `emitJSON(os.Stdout, v)` ✅ (these two are consolidated)

Yet **52 files / 88 call sites** in the same package hand-roll `json.MarshalIndent(v, "", "  ")` + print (sample: `agent.go:81`, `codex.go:1759,2140`, `config.go:109,206,307`, `constraint.go:217`, `batch_forge.go:244` …). No import is even required to adopt — it is a same-package function call.

A third sibling, `forge_curator_id.go:17` `writeJSONAtomic`, re-rolls indented-marshal *plus* the P1 atomic-write dance (§1).

**Recommendation:** no sweep. Adopt `outputJSON`/`emitJSON` at touch-time; move `emitJSON`'s home to `testutil`-adjacent shared file (or an `output.go`) with a doc comment declaring it canonical, so discovery is not "whichever file you happen to have open." The measured risk of drift here is real but cosmetic (indent/encoder differences change `--json` byte output, which robot consumers may diff).

---

## §5 — P5: `codex exec` invocation — the highest-value *un*-extracted pattern

Shelling to `codex exec` as the cross-family membrane/producer is a repeated, failure-prone pattern with hard-won defensive logic — and that logic is **not shared**:

**Collected instances:** `scripts/pawl-review.sh` (13 refs; the hardened one — timeout array wrapper at :246, anti-WANDER/anti-ECHO verdict-format defense documented at :135, missing-dep precondition protocol at :204), `scripts/eval-membrane.sh` (7 refs; its own `timeout` wrapper + stall note "a hung codex exec froze a run for 22 min — age-9h3d" at :90), `scripts/eval-agent-harness.sh` (3), `scripts/smoke-test-codex-skills.sh` (4), `scripts/pawl.sh`, `scripts/run-rpi-phases.sh`, `scripts/corpus-delta-harness.sh`, `scripts/second-poll.sh`, `scripts/epic-d16-donetest.sh`.

**Diff/Align:**
- *Invariant:* build prompt → invoke `codex exec` under a timeout → detect the three known failure modes (stall/hang; ECHO — prompt reflected with no review, no `tokens used` marker; WANDER — greps the filesystem instead of reviewing) → retry-or-fail-safe → parse a constrained output format.
- *Variance:* prompt content, model (`-m gpt-5-mini` vs default), sandbox (`-s workspace-write` vs read-only), output contract (VERDICT format vs free text), retry budget.

The failure modes are institutional knowledge (age-a9iv, age-9h3d, multiple memory entries: "codex reap → evidence-based no-tool verdict", "codex stall can echo whole prompt"). Today that knowledge lives in one 37KB script; every *new* harness that shells to codex re-learns it.

**Recommendation (library — sourced shell lib, matching where the callers live):** extract `scripts/lib/codex-exec.sh` providing `codex_exec_guarded <timeout> <sandbox> <prompt-file> <out-file>` with stall-kill + echo-detection (+ exit-code contract distinguishing NO-VERDICT from REFUTED — a distinction `pawl-review.sh` already fought for). `pawl-review.sh` becomes the first delegator; eval harnesses adopt at touch-time. (A Go port under `cli/internal/` is the longer-term home, but the shell lib meets the callers where they are — all current call sites are bash.)

---

## §6 — P6: the drift-gate triple — a healthy packaged pattern; scaffold the remaining variance away

The repo's strongest *successful* extraction is the gate architecture, worth naming so it is preserved deliberately:

- **104** `scripts/check-*.sh` drift gates, **~50** with a bats twin in `tests/scripts/` (198 bats files there), each registered in `cli/internal/gates/checks/seed.go` with **path-glob change-class scoping** (so gates fire only when their surfaces change) and a self-reference glob (editing the gate re-runs it). Registration is `init()`-based (`gates/registry.go`) — "adding a check is one new registration… no central orchestrator switch" (the anti-monolith property, ag-qidx G2). Native Go ports happen opportunistically (`go_build.go` documents the pattern).

**Residual variance worth collapsing at scaffold-time (not by refactor):** the script output protocol is only partially consistent — 26/104 emit a `NAME: PASS` line, ~15 use the `failures=0` + `fail()` accumulator shape (`check-architecture-doc-drift.sh` is the clean exemplar), 42 use exit-2-for-usage. A new gate today is written by copying a nearby gate, inheriting whichever dialect it had.

**Recommendation (template — the skill's "Project structure → Template" row):** a `new-gate` scaffolder (script or skill) that emits the triple — `check-<name>.sh` (preamble-sourcing, `fail()` accumulator, `NAME: PASS/FAIL` protocol, 0/1/2 exit contract), `tests/scripts/check-<name>.bats` twin, and the `seed.go` registration stub with path globs. This simultaneously becomes the delivery vehicle for P2 (new scripts source the preamble because the scaffold writes them that way).

---

## §7 — P7: Go test micro-helpers (small, contained in `cli/cmd/ao`)

- **Capture-stdout family:** 5 near-duplicates in one package — `testutil_test.go:143 captureStdout`, `:183 captureJSONStdout`, `demo_test.go:23 captureDemoStdout`, `membrane_test.go:96 captureMembraneDerive`, `registry_test.go:112 captureRegistryOutput`. Invariant: pipe-swap `os.Stdout`, run fn, restore, return string. Fold the outliers into the two `testutil_test.go` canonicals at touch-time.
- **Flag save/restore:** the repo's own rule (`.claude/rules/go.md`) prescribes *"a self-cleaning `setFoo(t, v)` helper at every set-site is the durable shape"* — measured reality: **86** inline `old := …` save/restores vs **2** helper-shaped setters in `cli/cmd/ao` tests. A single generic `setGlobal[T any](t *testing.T, p *T, v T)` (assign + `t.Cleanup` restore) in `testutil_test.go` would make the documented rule the cheapest thing to type. This is the recurring-flake class the rule exists for (goals cobra-global `a9dab21c4`, ek8v, hvb).

---

## §8 — What NOT to extract (anti-pattern guardrails honored)

- **`pool.atomicMove`** — different semantics (move w/ copy), already durable; forcing it into `AtomicWriteFile` would be false uniformity ("Force uniformity → allow escape hatches").
- **The 284 existing scripts' preambles** — the age-0dq9.2 prune was correct; churning working scripts for textual uniformity is the cathedral. The gate scopes to *new* files only.
- **Error wrapping / hexagonal port-adapter wiring / `t.TempDir` isolation** — high-count but *healthy, consistent* conventions (prior recon P3/P7/P8); count ≠ problem. No action.
- **One-off `_oneoff/` scripts** — explicitly quarantined; not pattern sources.

---

## §9 — Extraction backlog (ranked, with the skill's checklist applied)

| Rank | Action | Type | 3+ instances ✓ | Invariant clear ✓ | Effort | Prevented bug class |
|------|--------|------|----------------|-------------------|--------|---------------------|
| 1 | New-file preamble gate (`check-new-scripts-use-preamble.sh` triple) + fold `with_tmpdir`/`require_cmd` into `preamble.sh` | Gate + lib | ✓ (13 post-prune re-rolls) | ✓ | S | macOS find/stat portability, CWD-hijacked REPO_ROOT |
| 2 | Export `storage.ScanJSONL`/`AppendJSONL` w/ baked-in buffer policy; touch-time adoption | Library | ✓ (64 readers / 20 writers / 45 buffer bumps) | ✓ | S-M | silent 64KB line truncation; non-atomic appends |
| 3 | `scripts/lib/codex-exec.sh` guarded invocation (stall/echo/wander defense) | Library | ✓ (≥8 scripts) | ✓ | M | 22-min hangs, echo-as-verdict false confidence |
| 4 | `new-gate` scaffolder emitting the check/bats/seed triple with protocol baked in | Template | ✓ (104 gates, 4 dialects) | ✓ | S | protocol drift in new gates; doubles as P2 delivery |
| 5 | Touch-time adoption of `outputJSON`/`emitJSON`; fix `writeJSONAtomic` → `storage.AtomicWriteFile` | Adoption | ✓ (52 files) | ✓ | XS/file | `--json` byte-format drift; fsync-less "atomic" write |
| 6 | `setGlobal[T]` + capture-helper fold in `cmd/ao` tests | Library (test) | ✓ (86 / 5) | ✓ | XS | shuffle-order test flakes |

Each proposed artifact must pass the skill's validation loop before being called done: applied back to ≥1 real call site per source subsystem, tested (the gate triples give this for free), and documented with the WHY (each helper's comment cites the paid-for defect, as `preamble.sh` already models).

---

## Appendix — Measurement commands (reproducible)

```bash
# P2: preamble adoption + variants
grep -rl 'preamble.sh' scripts/ | wc -l                      # 1 (itself)
grep -h 'REPO_ROOT=' scripts/*.sh | sort | uniq -c | sort -rn # 12+ variants, 154 files
git log --since=2026-06-26 --diff-filter=A --name-only --pretty=format: -- 'scripts/*.sh' | sort -u  # 13 new, 0 adopters

# P3: JSONL plumbing spread
grep -rln 'O_APPEND' cli/internal cli/cmd | grep -v _test | wc -l          # 20
grep -rln 'bufio.NewScanner' cli/{internal,cmd} --include='*.go' | grep -v _test | wc -l  # 64
grep -rh 'Buffer(make(\[\]byte' cli/{internal,cmd} --include='*.go' | grep -v _test | sort | uniq -c  # 10+ variants

# P4: hand-rolled JSON emit alongside the in-package helper
grep -rln 'json.MarshalIndent' cli/cmd/ao/*.go | grep -v _test | wc -l     # 52 files / 88 sites

# P5: codex exec call sites
grep -c 'codex exec' scripts/*.sh | grep -v ':0'                            # 8+ scripts

# P6: gate corpus + protocol dialects
ls scripts/check-*.sh | wc -l                                               # 104
grep -rl ': PASS' scripts/check-*.sh | wc -l                                # 26
```
