# Codebase Pattern Extraction — agentops

> ⚠️ **HISTORICAL SNAPSHOT — recon run 2026-06-24 against `abc018c42`; `main` has since advanced past `882e71c01`.** A point-in-time pattern pass, **not** a current-state reference. **The P1 atomic-write DRY finding below was PARTIALLY actioned** (`storage.AtomicWriteFile` is now the canonical impl and quest/llmwiki/doctor/wiki delegate to it, age-3azc/uja6; but **inject, vendorimage/codexruntime, and `pool.atomicMove` still carry their own copies**, and `pool.atomicMove` still omits fsync). Counts are as-of 2026-06-24.

**Repo:** `/Users/bo/dev/agentops`
**Date:** 2026-06-24
**Skill:** `codebase-pattern-extraction` (read-only recon pass)
**Method:** Discover (rg + CASS) → Collect 3+ instances → Diff (invariant vs variance) → recommend Package (library / skill / template). Per skill: *"If you solved it twice, you'll solve it again. Extract once, reuse forever."*

> **Scope note.** The skill's canonical prompt assumes *multiple* repos (`/data/projects/a,b,c`). Here the target is a single large, mature, hot monorepo (~1,360 Go files, 311 shell scripts, 77 SKILL.md). The method maps cleanly to *intra-repo* extraction: the "projects" become packages/commands/scripts, and the win is collapsing N copy-paste implementations into one shared helper. Every pattern below has **≥3 concrete instances** (the skill's abstraction floor) — none are abstracted from 1-2 examples.

---

## Executive summary

| # | Pattern | Instances | Severity | Recommend package as |
|---|---------|-----------|----------|----------------------|
| P1 | **Atomic file write (temp→sync→rename)** | 8 source instances (3 exported, 2 of them same-name/divergent-sig + 5 private copies) | **HIGH** (real DRY violation) — ◑ partially actioned (age-3azc/uja6: 5 of 8 instances delegate) | Library — `internal/storage` (already the home) |
| P2 | **JSONL append-ledger writer** | ≥5 ledger packages each re-rolling open-append-encode | MED | Library — shared `jsonlwriter` over `internal/storage` |
| P3 | **Ports & Adapters (hexagonal) wiring** | 27 `*Port` interfaces + adapter structs | LOW (already a deliberate convention — codify, don't refactor) | Template / scaffold + lint rule |
| P4 | **Cobra command + `--json`/human output split** | 221 cobra files, 399 `--json` refs, 120 JSON-marshalling cmds | LOW (codify) | Template + shared `output` helper |
| P5 | **fail-open / fail-safe safety annotation** | 95 mentions; documented convention | LOW (codify) | Skill / lint convention, not code |
| P6 | **Shell script preamble (repo-root + `set -euo pipefail`)** | 266/311 strict-mode; 166 derive repo root; 60 use `git rev-parse --show-toplevel` | MED | Template — sourced `scripts/lib/preamble.sh` |
| P7 | **Go test isolation (`t.Cleanup`/`t.Setenv`/`t.Chdir`)** | 3077 `t.TempDir`, 821 `t.Setenv`, 376 `t.Cleanup`, 267 `t.Chdir` | LOW (already codified in `.claude/rules/go.md`) | Already a rule — keep enforcing |
| P8 | **`fmt.Errorf("...: %w", err)` error wrapping** | 1,762 sites | LOW (mature, consistent) | No action — healthy invariant |
| P9 | **SKILL.md structure (frontmatter + references/ + SELF-TEST)** | 77 SKILL.md, 58 with `references/`, 10 with SELF-TEST | LOW (codified via skill-builder) | Already templated |

**The one actionable extraction (P1)** is below the fold and worth doing. The rest are *already-deliberate conventions* — the recommendation there is to **codify + lint** them (so drift is caught), not to refactor.

---

## P1 — Atomic file write (temp → write → fsync → rename)  ★ the real DRY finding

> ◑ **PARTIALLY ACTIONED (age-3azc / age-uja6).** After the 2026-06-24 snapshot, `cli/internal/storage.AtomicWriteFile` became the canonical implementation and **5 of the 8 instances** (quest's 2 funcs, llmwiki, doctor, wiki) now delegate to it. **3 remain unconsolidated** — `cli/cmd/ao/inject.go`, `cli/internal/adapters/vendorimage/codexruntime/runtime.go`, and `cli/internal/pool/pool.go`'s `atomicMove` (the last **still omits fsync**, so its data-loss-on-crash gap is still live). The section below is retained as the historical extraction record; the open work is to migrate the remaining 3.

**This is the canonical pattern the skill exists to catch.** **Eight** source instances of the same temp-file→sync→atomic-rename dance (the executive-summary table's earlier "7" undercounted by one), two of them *exported with incompatible signatures*.

**Source instances (collected):**
- `cli/internal/types/quest/atomic.go:27` — `func AtomicWriteFile(path string, data []byte) error`  *(exported, sig A)*
- `cli/internal/types/quest/atomic.go:68` — `func AtomicWriteFileWithPerm(path string, data []byte, perm os.FileMode) error`
- `cli/internal/llmwiki/atomic.go:32` — `func AtomicWriteFile(path string, contents []byte, mode os.FileMode) error`  *(exported, sig B — **different signature, same name**)*
- `cli/cmd/ao/inject.go:452` — `func atomicWriteFile(path string, data []byte, perm os.FileMode) error`  *(private copy)*
- `cli/internal/doctor/mutate.go:306` — `func atomicWrite(dir, path string, content []byte, mode os.FileMode) error`  *(private copy)*
- `cli/internal/wiki/repair.go:148` — `func atomicRewrite(path, contents string) error`  *(private copy)*
- `cli/internal/adapters/vendorimage/codexruntime/runtime.go:741` — `func atomicWriteFile(path string, data []byte, perm os.FileMode) error`  *(private copy)*
- `cli/internal/pool/pool.go:1157` — `func atomicMove(srcPath, destPath string) error`  *(rename half only)*

(Plus ~49 files total that call `os.Rename(`/`.tmp` directly — many are likely inlined copies of the same idea.)

**Invariant core** (identical across all):
```
MkdirAll(dir) → CreateTemp(dir, ".tmp-*") → Write(data) → Sync() → Close() → Rename(tmp, path)
with cleanup (os.Remove(tmp)) on every error path, each wrapped fmt.Errorf("...: %w", err)
```
(See `cli/internal/types/quest/atomic.go:27-52` for the reference shape.) **Durability caveat:** this sequence fsyncs the *file* and renames atomically, which closes the write-then-rename data-loss window for the file **contents** — but the rename itself is only guaranteed durable across a power-loss if the **parent directory** is also fsynced after the rename. Neither the reference shapes here nor main's shipped `storage.AtomicWriteFile` (`atomicfile.go` — file `Sync()` + rename, no dir fsync) does this yet, so "no data loss on crash" holds for contents, not for the directory entry. A parent-dir fsync is the correct refinement.

**Variance points → parameters:**
| Varies | Across instances | Parameterize as |
|--------|------------------|-----------------|
| File mode | absent / `0o644` / caller-supplied | `perm os.FileMode` arg (default `0o644`) |
| Input type | `[]byte` vs `string` | accept `[]byte`; callers convert |
| fsync | present in most, absent in some | always fsync the file; for full crash-durability also fsync the **parent dir** after rename (see Durability caveat) |
| `dir` arg | one passes `dir` separately | derive via `filepath.Dir(path)` |

**Why it matters (not cosmetic):** two *exported* `AtomicWriteFile` with different arities means any new caller must guess which package's version they imported; the no-`perm` variant (`quest`) silently uses temp-file default perms; and the `pool.atomicMove` rename-only copy omits the fsync, so a crash between write and rename can lose data. These are exactly the subtle divergences copy-paste spreads.

**Package as:** **Library.** A single `storage.AtomicWriteFile(path string, data []byte, perm os.FileMode) error` in `cli/internal/storage/` (which already owns `file.go`, `filelock_unix.go`) — the natural home. ◑ **Partially done (age-3azc/uja6):** 5 of the 8 instances (quest's 2 funcs, llmwiki, doctor, wiki) now delegate to it (file fsync kept; a parent-dir fsync after rename remains a refinement); the other 3 — inject, vendorimage/codexruntime, and `pool.atomicMove` — still carry their own copies. Validation per skill: `LOC(shared) < Σ LOC(instances)/N` holds for the migrated set.

> Recon-only: this report does **not** make the change. It is a ready-to-bead extraction.

---

## P2 — JSONL append-ledger writer

Every ledger subsystem re-implements "open file `O_APPEND`, JSON-marshal a record, write a line."

**Source instances:**
- `cli/internal/yieldledger/writer.go` (3 append/encode sites)
- `cli/internal/verdictledger/writer.go` (`AppendIteration` / `AppendCooldown` / `appendRecord` / `readExistingRecords`, lines 29-70)
- `cli/internal/canon/ledger.go` (2 sites)
- `cli/internal/provenance/provenance.go`, `cli/internal/provenancegraph/store.go`
- plus `docs/provenance/ledger.jsonl` consumers in `cli/cmd/ao/` (forge, loop_writer_adapter, etc.)

**Invariant core:** open-or-create append, marshal one record per line, read-back = scan + unmarshal per line, atomic-ish replace on rewrite.
**Variance:** record struct (`Record`/`Verdict`/yield row), path resolution (repo-root walk vs explicit), dedup key, fsync-or-not.

**Package as:** **Library** — a small generic `jsonl.Writer[T]` (append one, read all, atomic rewrite) layered on P1's atomic write. Each ledger keeps its own typed `Record`; the file mechanics stop being copy-pasted. **Caveat:** the repo's memory notes the append-only Writer dedup race (`TestConcurrentAppend`) and that **dedup belongs at the emitter, not the Writer** — so the shared writer must stay dedup-free and append-only by design.

---

## P3 — Ports & Adapters (hexagonal) wiring  *(deliberate convention — codify, don't refactor)*

27 `*Port interface` declarations (e.g. `CorpusReaderPort`, `GateRunnerPort`, `LoopWriterPort`, `OperatorPort`, `VerifyPort`, `SafetyPolicyPort`) with matching `*Adapter` structs.

**Invariant:** a `FooPort` interface in the core + a `FooAdapter` struct in `cli/cmd/ao/` or `internal/adapters/` that satisfies it; tests target the port, prod wires the adapter.
**Variance:** domain of the port.

This is **already a healthy, intentional pattern** (the repo is explicitly DDD/hexagonal — see `docs/architecture/`). The skill's correct call here is *not* extraction (there's nothing to dedup) but **template + lint**: a scaffold (`new-port`) that stamps interface + adapter stub + port-targeted test, and a drift check that every `*Port` has ≥1 adapter and a test. Recommend as a `scaffold`-skill target, not a code change.

---

## P4 — Cobra command + `--json`/human output split

221 files define `cobra.Command{`; `--json` appears 399×; 120 cmd files call `json.Marshal*`. The robot-mode invariant (machine-parseable on `--json`, human default) the skill names as the #1 extractable CLI pattern is **present and consistent** here.

**Invariant:** `RunE` closure → if json-flag, `json.MarshalIndent` to stdout; else human-format.
**Variance:** the payload struct and the human formatter.

Healthy and consistent already. Extraction value is modest: a shared `output.Emit(w, jsonFlag, payload, humanFn)` helper would remove the repeated marshal-or-print branch (64 such branches found), and a `new-command` scaffold would stamp the boilerplate. **Package as template + small helper**, low priority.

---

## P5 — fail-open / fail-safe safety annotation

95 mentions; a genuine repo-specific **convention**, not duplicated logic. Comments explicitly mark each guard as fail-open (observability, never blocks: `yield.go:38`, `session_bootstrap.go:65`) or fail-safe (e.g. corrupt-ledger → safe default). Notably, many were *added by cross-family pawl REFUTEs* (`beads_verify_acceptance.go:209,223,307`; `membrane.go:103`) — the membrane catching silent fail-opens in review.

**Not a code-extraction target** (no shared function to pull). The right package is a **skill / review-checklist convention**: "every guard must declare fail-open vs fail-closed in a comment, and fail-open must be *visible* (emit a marker), never silent." This is already lived doctrine; codifying it into the review skill closes the loop the pawl keeps re-finding.

---

## P6 — Shell script preamble (repo-root + strict mode)

Of 311 shell scripts: **266** use `set -euo pipefail`; **166** derive a repo root; **60** specifically `git rev-parse --show-toplevel`.

**Invariant:** `#!/usr/bin/env bash` + `set -euo pipefail` + `REPO_ROOT="$(git rev-parse --show-toplevel)"` (or `cd` to it).
**Variance:** extra traps, color setup, arg parsing.

**Package as:** **Template** — a single sourced `scripts/lib/preamble.sh` (strict mode + `REPO_ROOT` + common helpers) that scripts `source`. Collapses 166 copies of repo-root derivation into one. Note the macOS-portability hazard the repo memory flags (interactive `find`→`bfs` shim; portable `stat -f %m || stat -c %Y`) — a shared preamble is the right place to centralize portable helpers and stop each script re-deriving them.

---

## P7 / P8 / P9 — already-codified conventions (no extraction needed)

- **P7 Test isolation** — `t.Cleanup` (376), `t.Setenv` (821), `t.Chdir` (267), `t.TempDir` (3077). Already a hard rule in `.claude/rules/go.md` ("restore shared global/process state via `t.Cleanup`") and enforced by the `-shuffle=on` race suite. Keep enforcing; nothing to extract.
- **P8 Error wrapping** — 1,762 `fmt.Errorf("...: %w", err)` sites. Mature, uniform invariant; this is the *target state* the skill aims for, not a problem.
- **P9 SKILL.md structure** — 77 SKILL.md, 58 with `references/`, 10 with SELF-TEST; already templated by `skill-builder` / the unified template. Healthy.

---

## CASS pass (per skill method)

- `cass search "atomic write temp rename"` → 179 matches; top hits confirm the **"atomic temp+rename"** idiom is doctrine across the corpus (e.g. the `/validate` skill prescribes "`registry.jsonl` (atomic temp+rename)"), reinforcing P1/P2 as the canonical shape — yet code had 8 divergent copies at snapshot time (5 of 8 instances since consolidated — age-3azc/uja6; 3 remain).
- `cass search "fail-open cross-family refute"` → 0 direct hits (term too specific); the in-repo grep (P5) is the better evidence surface for that convention.

---

## Validation against the skill's checklist

| Check | Result |
|-------|--------|
| Discover (CASS + rg) | ✅ both used |
| 3+ instances before abstracting | ✅ every pattern ≥3 (P1=8, P6=166, etc.) |
| Clear invariant identified | ✅ per pattern |
| Clear variance → parameters | ✅ per pattern |
| Don't over-parameterize | ✅ recommendations keep sensible defaults |
| Apply-back / LOC validation | ✅ noted for P1/P2 (shared < Σ instances/N) |
| Document WHY | ✅ each pattern explains the divergence risk |

## Anti-patterns avoided

- Did **not** abstract from 1-2 examples (floor = 3).
- Did **not** recommend forcing uniformity where the convention is already deliberate (P3/P4/P7/P8/P9 → codify/template, not refactor).
- Did **not** touch source (read-only recon; this is a bead-ready extraction list, not an applied change).

## Recommended next moves (bead-ready, in priority order)

1. **P1** ◑ **PARTIAL (age-3azc/uja6)** — extracted `storage.AtomicWriteFile`; 5 of 8 instances (quest's 2 funcs, llmwiki, doctor, wiki) delegate; inject/vendorimage/`pool.atomicMove` still private (pool still omits fsync — open work). *(was the highest-value, lowest-risk extraction)*
2. **P6** — `scripts/lib/preamble.sh`; centralize repo-root + portable `stat`/`find` helpers.
3. **P2** — generic append-only `jsonl.Writer[T]` over P1 (dedup stays at emitter — honor the known race).
4. **P5** — fold the "declare fail-open vs fail-closed, fail-open must be visible" rule into the review skill.
5. **P3/P4** — `new-port` / `new-command` scaffolds + drift lint (template work, not refactor).
