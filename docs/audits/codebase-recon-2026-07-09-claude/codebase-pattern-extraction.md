# AgentOps — Codebase Pattern Extraction (2026-07-09)

> **Skill:** `codebase-pattern-extraction` · **Run:** 2026-07-09 (Claude recon swarm)
> **Scope:** intra-repo mining — patterns recurring **≥3×** across AgentOps sub-corpora (Go packages, `scripts/`, `tests/scripts/`, `skills/`), each backed by grep-verified instances with file anchors, per the skill's "3+ instances, never abstract from 1-2" rule. CASS session mining out of scope (read-only worker).
> **Relation to prior run:** the 2026-07-02 sweep (`docs/audits/codebase-recon-2026-07-02/codebase-pattern-extraction.md`) extracted 10 patterns and a 6-item backlog. This run is **delta-first**: Part 1 verifies what happened to that backlog in the week since (~50 commits, 858 files changed); Part 2 extracts patterns that are new or newly visible.

**The through-line this run:** the repo's recurring weakness is not *extraction* — helpers get built promptly — it is **adoption**. The one mechanism that demonstrably converges adoption is the shrink-only grandfather ratchet, and that mechanism is itself the highest-value extraction candidate: it is re-implemented per-gate today.

---

## Part 1 — Prior-backlog delta (each item re-verified against HEAD)

| 07-02 backlog item | Status 07-09 | Evidence |
|---|---|---|
| **#1 Hash-chained ledger → `cli/internal/hashchain`** (very high, security-critical) | **NOT DONE** — duplication unchanged | no `cli/internal/hashchain` package; 48 `payload_hash` references across 8 non-test files (`drwitness/drwitness.go`, `turnstate/turnstate.go`, `rpi/ledger.go`, `provenancegraph/{edge,store,chain,verify}.go`, `drrebuild/drrebuild.go`) |
| **#7 Atomic-write adoption** (migrate private writers onto `storage.AtomicWriteFile`) | **STALLED** — and it is the only prior adoption gap with **no ratchet gate** | 18 files import/use `AtomicWriteFile` (`cli/internal/storage/atomicfile.go:26`); 26 non-test files still carry private `os.Rename` writers (`doctor/{engine,mutate,fix_skills,fix_knowledge,runartifact}.go`, `wiki/{index,pipeline,source}.go`, `search/{constraint,util,findings_ops}.go`, `pool/pool.go`, `llm/{review,session_writer}.go`, …); no `check-atomic-write-*.sh` exists |
| **#4 Absence → unsafe-verdict gate (audit A1: `GateStatusUnknown` excluded from failure)** | **FIXED** ✅ | `cli/internal/gates/report.go:26-41` — `ExitCode` now documents and enforces: "A blocking check fails on FAIL, **UNKNOWN**, an empty/unrecognized status, or an evaluation error (fail-closed — see isBlockingFail)" |
| **#2 Shared `ExitError` interface in `root.go`** | **REGRESSED** — the duplication *grew* | `cli/cmd/ao/root.go:85-172` now unwraps **12** concrete typed errors via `errors.As` (was 9 on 07-02; `verifyPrePushErr` and `landErr` added since); still no shared `ExitCode() int` interface |
| **#3 / #6 Decider contract, reflect-over-live-tree** (doc-only recommendations) | Not independently re-verified this run | — |

**Lesson the delta teaches (feeds Pattern N2):** the one prior gap that had a mechanical gate (A1 — caught by the gate suite's own consistency argument) got fixed within a week. The gaps enforced only by audit-report memory (hashchain, atomic-write, ExitError) did not move or got worse. In this repo, *a recommendation without a ratchet is a wish.*

---

## Part 2 — Newly extracted patterns

### Pattern N1 — Shrink-only grandfather ratchet (the debt-freezing gate)

**Source instances (8):**
- `scripts/.jsonl-scanner-grandfather` + `scripts/check-jsonl-scanner-ratchet.sh` (15.9K) — raw `bufio.NewScanner` over JSONL vs blessed `storage.ScanJSONL`
- `scripts/.preamble-grandfather` (11.4K, ~334 lines) + `scripts/check-new-scripts-use-preamble.sh` (12.5K) — new scripts must source `lib/preamble.sh`
- `scripts/.docs-cli-snippets-baseline`
- `scripts/.docs-demoted-claims-baseline`
- `scripts/.docs-skill-refs-baseline`
- `scripts/.scripts-ao-invocations-baseline`
- `scripts/.scenario-linkage-allow`
- `scripts/check-retrieval-quality-ratchet.sh` (metric-threshold variant of the same shape)

**Invariant core:** a heuristic detector runs over a scope; every *current* violator is pinned by filename in a checked-in exemption list; **new** violations fail the gate; the list may **only shrink** (an entry that no longer trips the heuristic must be pruned); a `--regenerate` mode rebuilds the list, run at land time after the final rebase. Header names the blessed replacement and the bead that introduced the ratchet (e.g. `age-storage-hardening-roxg.3`, `age-gate-the-ungated-egwt.10`).

**Variance points:** the detector (grep heuristic vs metric threshold) → a command/function parameter; the scope glob; the blessed-replacement name in the header; whether stale entries hard-fail or warn.

**Why it recurs:** it is the only observed mechanism that lets a gate land with *zero churn* of the existing tree while still guaranteeing convergence — the exact tension every retrofit gate in a hot repo faces.

**Packaged as (proposed):**
- [ ] Library: `scripts/lib/ratchet.sh` — `ratchet_check <detector-cmd> <pinned-file>`, `ratchet_assert_shrink_only`, `ratchet_regenerate`. Verified gap: `check-jsonl-scanner-ratchet.sh` and `check-retrieval-quality-ratchet.sh` share **zero** list-handling code today; each new ratchet is a 12-16K bespoke script.
- [ ] Optional Go twin: `ao lint ratchet --config <yaml>` once ≥3 gates migrate onto the bash lib.

**Validation:** applying the lib back to the two ratchet scripts and the five baseline gates is the acceptance test (skill rule: "apply back to all source projects").

---

### Pattern N2 — Extract-then-migrate stall: adoption, not extraction, is the failure mode

**Source instances (4 helper extractions, three observed enforcement levels):**

| Helper (extracted, tested, documented) | Adopters | Hold-outs | Enforcement | Debt trajectory |
|---|---|---|---|---|
| `storage.AtomicWriteFile` (`cli/internal/storage/atomicfile.go:26`) | 18 files | 26 private `os.Rename` writers | **none** | flagged 06-24 → 07-02 → 07-09, unchanged |
| `lib/bats-common.bash` (shared bats fixtures, soc-jhq6) | 4 `.bats` files | 244 of 248 `.bats` files hand-roll setup | **none** | growing |
| `scripts/lib/preamble.sh` (strict mode + `REPO_ROOT` + portable stat/find) | 9 sourcing scripts | 75 scripts hand-roll `git rev-parse --show-toplevel` | **new-files-only ratchet** (`check-new-scripts-use-preamble.sh`; grandfather froze existing tree "with ZERO churn" by design) | frozen |
| `storage.ScanJSONL` (`cli/internal/storage/file.go:251`, loud `ErrLineTooLong`) | growing | 44 grandfathered files | **shrink-only ratchet** (`check-jsonl-scanner-ratchet.sh`) | melting — list "only SHRINKS", pruned at land |

**Invariant core:** the helper is extracted promptly and well (docs, tests, portability notes — `preamble.sh` even stages the work explicitly as "P6a extract / P6b migrate"); the migration half is deferred to a later bead and stalls unless a gate carries it.

**Variance → the parameter that matters:** the enforcement level. Three levels observed, with three distinct outcomes: none → debt grows silently; new-only → debt frozen; shrink-only → debt converges to zero.

**Packaged as (proposed):**
- [ ] **Policy rule** (one line in `skills/standards/` or `AGENTS-WORKFLOW.md`): *every helper extraction ships with a shrink-only ratchet gate in the same arc* — using the N1 lib, so the marginal cost is a detector one-liner plus a pinned file.
- [ ] Immediate applications: `.atomic-write-grandfather` + check (closes the 3-sweep-old crash-safety gap; also fixes `pool.atomicMove` no-fsync flagged 06-24), `.bats-common-grandfather` + check.

**Watch item (2 shapes, below the 3-instance bar — not extracted):** 86 files in `cli/cmd/ao/` hand-roll `json.MarshalIndent`/`json.NewEncoder(os.Stdout)` next to 79 `--json` flag registrations, while `cli/internal/formatter/` (jsonl/markdown/table) exists. If a third emission shape appears, this becomes instance #5 of this pattern.

---

### Pattern N3 — Bash prototype → Go authority promotion, with env-var escape hatch

**Source instances (3 promotions, one in flight):**
- `scripts/pre-push-gate.sh` → `ao gate check` (Go release authority); legacy bash reachable via `AGENTOPS_GATE_BASH=1` — `scripts/ship.sh:154-217` implements the hatch and the fallback warning
- `land.sh` → `ao land` (Go) — commit `e0138bf82`
- `emit_pawl_catch` bash → `ao membrane catch --evidence` (Go reason-extraction) — commit `bfa21dd52` (age-ulab), bash replaced outright
- (lineage: the RPI bash lane → Go before its retirement)

**Invariant core:** authority-bearing logic is prototyped as `scripts/*.sh`; once it becomes *release authority* it is promoted into the `ao` binary (typed exit-code error, Go tests, generated command docs); during transition the bash twin stays reachable behind an env hatch; narrative docs are updated to name the Go path as authority (`CLAUDE.md` "legacy bash only via `AGENTOPS_GATE_BASH=1`").

**Variance points:** hatch retention (gate keeps its hatch long-term; membrane catch dropped bash immediately) → a per-promotion risk call; whether the bash script is deleted or parked.

**Packaged as (proposed):**
- [ ] Skill/standards reference: a **promotion playbook checklist** — typed exit error + `root.go` unwrap (ideally via the still-missing `ExitError` interface, see Part 1), bats/Go parity test against the bash behavior, env hatch + a retirement bead for it, docs flip. Each promotion so far re-derived this list.

---

### Pattern N4 — Script-backed gate check as a four-part unit (script + contract header + bats twin + registry row)

**Source instances:** 121 `scripts/check-*.sh`; **46** carry an explicit exit-code contract header (`0 = clean, 1 = violation, 2 = usage/environment error` — e.g. `check-adr-registry.sh:14`); ~95 are wired into the Go gate registry (`cli/internal/gates/checks/seed.go`, 30.7K, self-registering per the 07-02 run's Pattern 5); near-1:1 bats twins in `tests/scripts/` (248 `.bats` files); headers carry bead provenance ("Introduced by age-gate-the-ungated-egwt.11 after two ADRs shipped as ADR-0004").

**Invariant core:** one check = one script with `set -euo pipefail`, a header stating the contract it guards + why it exists (bead id) + exit vocabulary, a same-named bats twin, and a registry row so the Go gate owns scheduling/blocking semantics.

**Variance points:** the guarded contract; blocking vs advisory; fast-scope inclusion.

**Already partially packaged:** `scripts/add-validate-job.sh` scaffolds the workflow job + pre-push section + **bats stub** + AGENTS row — the unit *is* recognized as a unit.

**Gap (the extraction):** the scaffold does not emit the exit-code contract header or the `preamble.sh` source line, and 75 of 121 check scripts lack the documented exit vocabulary.
- [ ] Extend `add-validate-job.sh` to emit the canonical header (contract, bead id, exit codes) + preamble line; add an exit-contract check to `audit-skill-metadata.sh`-style lint or the preamble ratchet.

---

### Pattern N5 — Classed failure register → digest with per-class recurrence deltas

**Source instances (3):**
- **Producer-defect register**: `ao membrane catch --evidence` appends classed catches (`bfa21dd52`); `ao membrane digest --deltas` computes per-class recurrence before/after (`d91e01043`, age-de5t) — `cli/cmd/ao/membrane.go` (44K), `membrane_digest.go` (23K)
- **Escape register / EM spine**: escape → classified by domain → compiled into a derived check; `ao membrane recall --domain` surfaces escapes (and optionally the "abundant memory" of catches) per class — `membrane.go:216-218`
- **Verdict ledger → verification-economics ruler**: per-verdict records aggregated into the economics report — `tests/scripts/verification-economics-report.bats` (new this week)

**Invariant core:** an append-only register in which every entry carries a **class** from a shared taxonomy; a digest computes per-class recurrence (the before/after delta is the fitness signal); an **UNCLASSIFIED entry renders loud as visible debt** — never silently dropped or folded into a real class (`membrane.go:770-773`: "an UNCLASSIFIED escape is visible debt, never a real domain"). This is the same fail-loud-on-absence family as the 07-02 run's Pattern 4, now applied to taxonomy membership.

**Variance points:** the register's domain (producer defects / escapes / verdicts) and the consumer (derived check vs digest vs economics ruler).

**Packaged as:** this *is* the product doctrine (EM spine) — the extractable asset is the convention, not code: any **new** register must (a) join on the shared class taxonomy and (b) render UNCLASSIFIED loudly. Worth one paragraph in the membrane contract doc so register #4 doesn't re-derive it.

---

## Part 3 — Prioritized extraction backlog (2026-07-09)

| # | Pattern | Extract into | Value | Effort | Notes |
|---|---------|--------------|-------|--------|-------|
| 1 | N1 ratchet mechanics | `scripts/lib/ratchet.sh` | **High** | Low | Makes every subsequent row cheaper; 8 in-tree consumers on day one |
| 2 | N2 atomic-write ratchet | `.atomic-write-grandfather` + check (via #1) | **High** (crash-safety; 3rd consecutive sweep flag) | Low | Also covers `pool.atomicMove` no-fsync |
| 3 | Hash-chained ledger (carried from 07-02 #1) | `cli/internal/hashchain` | **Very high** (security-critical, 8-file dup) | Medium | Still the single largest code extraction; unmoved for a week |
| 4 | `ExitError` interface (carried, **regressed** 9→12) | shared interface in `cli/cmd/ao` | Medium | Low | Each new promoted command (N3) adds another concrete unwrap until this lands |
| 5 | N2 bats-common ratchet | `.bats-common-grandfather` + check (via #1) | Medium | Low | 4/248 adoption |
| 6 | N3 promotion playbook | standards reference | Medium | Low | Checklist, not code |
| 7 | N4 scaffold completion | extend `add-validate-job.sh` | Low-Med | Low | Header + preamble emission |

## Method notes (anti-pattern compliance per the skill)

- **3+ instances**: every extracted pattern above has ≥3 grep-verified in-tree instances with file anchors; the JSON-emission observation was explicitly *left unextracted* at 2 distinct shapes (watch item under N2).
- **Apply-back validation**: each proposed artifact names its acceptance test (N1: migrate the two existing ratchet scripts onto the lib; N2: the pinned lists themselves).
- **Context preserved**: each pattern records *why it recurs*, and the ratchet headers' bead-id provenance convention is itself part of what's being extracted.
- **Escape hatches**: N1 keeps `--regenerate`; N3's env hatch is a first-class variance point, not an afterthought.
- Read-only run: no source files were modified; this report and the swarm result record are the only writes.

## Related artifacts

- Prior run: `docs/audits/codebase-recon-2026-07-02/codebase-pattern-extraction.md` (10 patterns; this run verifies its backlog in Part 1)
- Siblings (this run): `codebase-archaeology.md` · `codebase-audit.md` · `codebase-report.md` · `SYNTHESIS.md` in this directory
