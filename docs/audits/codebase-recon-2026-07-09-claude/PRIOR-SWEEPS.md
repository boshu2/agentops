# Prior Sweeps — Merged Context for the 2026-07-09 Claude Recon

> Merge of five prior codebase-recon sweeps of `/Users/bo/dev/agentops`: four historical runs
> (2026-06-14, 2026-06-24, 2026-07-01, 2026-07-02) plus the same-day Codex-native run of
> 2026-07-09. This document exists so the fresh Claude sweep does not re-derive known findings,
> knows which items recur, and can verify — rather than re-discover — the open watch-list.

## 1. Sweep lineage and how the verdict evolved

The five sweeps trace a codebase that has stayed structurally healthy while its risk profile
migrated from "debt and doctrine" to "fail-open seams in its own enforcement machinery." The
2026-06-14 sweep read a disciplined consolidation-and-hardening release (net −72k LOC of tooled
skill-prune, fail-closed test-heavy security machinery) whose residual risk concentrated in one
deliberate doctrine reversal (quorum family-floor → context-floor) and whose harsher parallel
FACTORY-SHAPE panel called the self-running factory avoidance — primitives built, never wired,
~1 provenance line of external value. The 2026-06-24 sweep found a mature, hexagonal,
self-honest codebase with 0 Critical / 0 High, its worst engineering debt a DRY atomic-write
violation and an in-progress gate-orchestration migration. The 2026-07-01 sweep confirmed strong
health (1.45x test-to-product ratio, zero secrets, drift gates green) but crystallized the
repo's governing second-order law: **the only lever that changes behavior here is a gate, and
everything ungated decays** — extractions get packaged and never adopted, policies documented
and never enforced. The 2026-07-02 sweep then found exactly what that law predicts: two High
fail-open seams left by the hookless migration, one of them in the release gate itself
(`ao gate check` failing OPEN when a blocking check's script is missing), plus ~60 docs teaching
removed commands as canonical. Finally, the same-day Codex-native run of 2026-07-09 sharpened
the lens onto the live trunk: the pinned origin/main tip is itself UNVERIFIED (no bound verdict,
backstop report-only) and GATE-RED (a skill-schema failure), meaning the membrane's own
no-verdict-no-done contract is currently violated by the trunk that ships the membrane. Across
five runs the verdict arc is: architecture and honesty consistently excellent; the persistent,
recurring danger is the gap between what the repo *says* it enforces and what actually fails
closed.

## 2. Per-sweep summaries

### 2026-06-14 — consolidation + hardening, one doctrine reversal, factory-shape critique

**Verdict:** Healthy, disciplined release window (v3.1.0..HEAD). Removes more risk than it
introduces; no blocking findings. Residual risk concentrated in the quorum doctrine reversal.
The parallel FACTORY-SHAPE panel was harsher: the factory built the router's primitives but
never wired them and shipped ~1 provenance line of external value.

Top findings:

- **Quorum floor weakened (High):** significant-action floor changed from ≥2 distinct model
  FAMILIES to ≥2 distinct non-author CONTEXTS; cross-family demoted to opt-in
  `RequireCrossFamily` (default OFF). Two fresh contexts of the same model can now satisfy a
  one-way door. "Context = independence" asserted in godoc, never validated. Unresolved at
  sweep time.
- **`check-doc-skill-refs.sh` built but not wired to CI (Medium):** correctly flags 7 dangling
  slash-refs in `operating-loop.md` (the declared primary navigation) but is not a gate.
- **`cli/cmd/ao` main-package concentration worsened (Medium):** ~633 `.go` files then;
  converge, provenance_verify, skills_retire, codex all landed in the one namespace prior recon
  flagged as a coupling monolith. (Confirmed grown to 669 by 2026-07-09.)
- **`sh -c` packet dispatch in codex.go (Medium):** safe while packets are operator-trusted;
  becomes arbitrary command execution if packet provenance widens. Trust boundary undocumented.
- **Provenance chain tamper-EVIDENT not tamper-PROOF (Medium):** unkeyed SHA-256; a writer with
  file access can recompute a clean chain. By design; do not market otherwise.
- **`ao skills retire` multi-surface transactionality unconfirmed (Medium):** mutates ~6
  correlated surfaces in load-bearing order; mid-run failure can leave ledger flipped with
  stale inventories.
- **Carried bd→br migration debt (Medium):** ~16 bd-named scripts invoking retired verbs;
  legacy `.beads/` Dolt config preserved byte-for-byte; the 2210-line bash pre-push gate grew
  to ~2252 rather than shrinking.
- **FACTORY-SHAPE (High):** the reversibility router existed as two orphaned primitives
  (`liveness.IsSignificantAction`, `rpi.ShouldForceEscalation`) with ZERO production callers;
  loop had no decision point consulting them and no global budget (`--max-cycles` default 0 =
  unlimited).
- **Release hygiene (watch):** breaking + doctrinal change sat on main with no CHANGELOG entry,
  no 3.2 tag, and cross-repo consumers (olympusd, fleet memory) still saying "≥2 families."

### 2026-06-24 — mature and self-honest; DRY atomic-write debt; gate migration in flight

**Verdict:** Genuinely hexagonal Go CLI + markdown skill corpus practicing what it preaches;
0 Critical / 0 High in a read-only static review (build-green snapshot, not a release
certification — full test suite and deep security scan were not run).

Top findings:

- **DRY atomic-write violation (Medium, headline):** 8 divergent temp→fsync→rename
  implementations, 2 exported with the same name `AtomicWriteFile` and incompatible
  signatures; `pool.atomicMove` rename-only, omitting fsync (crash data-loss gap). Partially
  actioned in-window: `storage.AtomicWriteFile` canonical, 5 of 8 delegated; `inject.go`,
  `vendorimage/codexruntime/runtime.go`, `pool.go atomicMove` still open. No parent-dir fsync
  anywhere.
- **`sh -c` on operator-configured verifier (Medium, M-1):** `cli/internal/canon/verifier.go:57`
  runs `AGENTOPS_CANON_VERIFIER_CMD`; not a live injection vuln but the trust boundary is
  undocumented and unguarded by test.
- **Path-containment maintenance surface (Medium advisory, M-2):** 6,371 `filepath.Join` calls;
  containment must be enforced by gate (extend `check-paths-resolver-coverage.sh`), not
  convention. Not built.
- **Gate triple-orchestration (Medium debt):** Go registry (primary) vs `validate.yml` (CI
  backstop) vs `scripts/pre-push-gate.sh` (legacy bash escape via `AGENTOPS_GATE_BASH=1`);
  ~11 deferred bash scripts remained to fold into the Go registry.
- **Flywheel/escape-corpus claims data-starved (Low, structural):** 0 escapes across 130
  production verdicts; already honestly demoted in ADR-0004 / ADR-0011.
- **Narrative doc drift (Low):** ARCHITECTURE.md / ports-and-adapters.md still teaching
  hooks / bd / PR-per-change; ~34 skills marked update/refactor.
- **Polish (Low):** curl|bash install tag-pinning under-documented; `eval` indirect assignment
  in `seed-evolution-roadmap-beads.sh:47`; discard ratio healthy (1,075 `_ =` vs 12,843
  `err != nil`).

### 2026-07-01 — strong health; the "ungated policy drifts" law named

**Verdict:** Mature, unusually self-aware codebase in strong health (156K LOC Go / 226K test =
1.45x, zero secrets, zero known-vuln deps, 0 shellcheck errors, drift gates green, zero
critical/high across four lenses). The real weakness is second-order: policies and extractions
that exist but aren't mechanically enforced drift, because the only behavior-changing lever in
this repo is a gate.

Top findings:

- **M-1 cwd-relative hook scripts, contextual RCE (Medium):** `hooks/finding-compiler.sh` (via
  `bestEffortRefreshFindingCompiler`, findings.go:380) and `scripts/prune-agents.sh` (via
  `bestEffortPruneAgents`, session_end_maintenance_helpers.go:92) execute WITHOUT the pawl
  `aoBinaryInside` trust guard (which IS applied in pawl.go:115-140), stderr discarded.
- **M-2 `make lint` RED (Medium):** 13 issues (3 funcs over gocyclo-25, 4 dropped errors)
  against the repo's own documented budget — lint not wired into the blocking gate.
- **P2 preamble non-adoption (Medium, headline pattern):** `scripts/lib/preamble.sh` exists
  but 0 of 317 scripts source it; ALL 13 scripts added after the 2026-06-26 adoption decision
  re-hand-rolled it (CWD-hijackable REPO_ROOT, macOS stat/find portability re-exposed).
- **P3 JSONL scanner sprawl (Medium):** 64 hand-rolled readers, ~20 O_APPEND writers, 45
  buffer bumps in 10+ size variants; un-bumped `bufio.Scanner` silently truncates >64KB lines —
  a LIVE class given the checked-in 2.4MB-line fixture. Canonical helpers unexported, 0 adopters.
- **P5 codex-exec defenses siloed (Medium):** timeout / stall-kill / echo-as-verdict /
  NO-VERDICT≠REFUTED logic lives only in `pawl-review.sh`; ≥8 scripts re-solve subsets.
- **P1/P4 consolidation escape (Low):** `forge_curator_id.go:17 writeJSONAtomic` re-rolled
  tmp+rename fsync-less — proving consolidation without a guard accretes new copies.
- **M-3 predictable /tmp paths (Medium-Low):** `check-three-gap-supergate.sh`, `pawl.sh` cron
  log — symlink/clobber pattern; one-line mktemp fix.
- **Build-tag spine/satellite footgun (Low, recurring):** 17 tag-gated files behind
  `flywheel`/`legacy`; spine-only validators broke twice (age-sydq, age-zei7). Tracked
  `.githooks/pre-commit` mis-derives REPO_ROOT and falsely blocks cli commits.
- **Count drifts (Low):** documented ~88 vs generated 79 commands; ~77 vs ~91 gate-check
  figures.

### 2026-07-02 — fail-open seams in the repo's own enforcement

**Verdict:** Disciplined, honest, self-verifying (0 Critical; the ledger is the most-churned
file) — but its two most serious issues are fail-open seams left by its own hookless migration,
exactly the escape class the product exists to compile into permanent checks.

Top findings:

- **A1 release gate fails OPEN (High/P0):** `ao gate check` maps a missing/unlaunchable
  blocking script to `GateStatusUnknown`, which `ExitCode()` excludes from `isBlockingFail` —
  a blocking gate silently passes. Empirically proven (22 blocking checks → UNKNOWN → exit 0).
  Latent since 2026-06-07.
- **A2 local pre-push gate orphaned (High/P1):** `core.hooksPath` → `.beads/hooks` whose
  pre-push runs only the retired `bd hooks run pre-push` shim; `install-pre-push-gate.sh`
  installs into `.git/hooks`, which git ignores under hooksPath. Both mechanisms fail toward
  "allowed"; the `.githooks/pre-push:7-13` comment is factually wrong.
- **A4/A5 removed-command doc drift + gate hole (High):** ~69 docs teach removed
  `ao rpi`/`orchestrate`/`evolve` as live canonical commands (incl. how-it-works.md,
  ARCHITECTURE.md); only 9 bannered. `check-docs-no-retired-tech.sh:43` regex omits those verbs
  so the drift stays green. Gate-fix + doc sweep must land together.
- **A3 bash-gate parity gap (High):** `scripts/pre-push-gate.sh` (2263 lines, 55 scripts,
  0 refs to `ao gate`, reachable via `AGENTOPS_GATE_BASH=1`) has no parity mechanism against
  the Go registry; `workflow_coverage.go` reconciles only against `validate.yml`.
- **A6 safety false-green (Medium):** `safety/doc.go` + tests describe deleted-hook enforcement
  as active; tests re-implement removed-hook logic in Go (`simulateRunRestricted`) — false-green
  testing no shipping code; `sandbox.go` funcs dead. The 06-24 run drew the opposite "Security
  STRONG" conclusion from these same green tests.
- **A7 exit-code contract incomplete (Medium):** `ao capabilities` lists {0,1,2}; CLI actually
  emits 3–10 across pawl review, plan-pawl decide, governor budget, tick.
- **A8 buildtag guard unwired (Medium):** `scripts/verify-buildtags.sh` (ADR-0012) wired to
  nothing automated; 47 archived .go files can silently stop compiling.
- **A10/A11 recon-authored docs stale (Low):** codebase-overview.md "Open debt" bullets stale
  despite same-day edit; disposition-triage counts drifted — the suite drifting against its own
  prior output.

### codex-2026-07-09 — the same-day Codex-native sweep (cross-family baseline)

**Verdict:** The verification architecture remains unusually credible, but the current trunk
itself (pinned fbba8af5) is UNVERIFIED (no bound verdict; the hosted backstop is report-only)
and GATE-RED (fails skill.schema on goal-design). The run also separated live escapes from
real-but-legacy-only defects. Full enumeration in §5.

## 3. Cross-sweep recurrence table

Findings/themes appearing in 2+ sweeps, with apparent status as of 2026-07-09:

| Recurring finding / theme | Sweeps | Apparent status today |
|---|---|---|
| Fail-open / degrade-rather-than-hold seams in enforcement (gate Unknown→pass; orphaned hook; verifycfg fallback; changelog SKIP; council overflow) | 2026-07-02 (A1/A2/A6), codex-2026-07-09 (F-01/F-02/F-03/F-07/F-11) | LIVE and worsening in visibility: trunk itself shipped unverified + gate-red per the Codex run. The single highest-priority verification target for the fresh sweep. |
| Doc / contract drift outrunning executable truth (removed commands taught as live; capabilities/schema prose lag; scale counts stale) | 2026-06-24, 2026-07-01, 2026-07-02 (A4/A5/A7/A10), codex-2026-07-09 (F-08/F-12/F-13) | OPEN, persistent. Capabilities partially fixed since 07-02; residual drift confirmed by Codex (F-12). Retired-tech regex hole status needs re-check. |
| Consolidation without a guard accretes new copies (atomic-write, JSONL scanners, codex-exec defenses, preamble.sh 0-adoption) | 2026-06-24, 2026-07-01 | Atomic-write mostly closed by 07-01 with one fresh escape (`writeJSONAtomic`); preamble/JSONL/codex-exec adoption status unverified since 07-01. |
| Ungated policy drifts — the gate is the only behavior-changing lever; detectors built but not wired (check-doc-skill-refs, verify-buildtags, lint) | 2026-06-14, 2026-07-01, 2026-07-02 (A8) | OPEN as a law of the repo. check-doc-skill-refs confirmed still not in `.github/` as of 2026-07-09. |
| `cli/cmd/ao` main-package concentration | 2026-06-14 (and its prior), implicit 2026-07-01 | WORSE: ~633 → 669 files confirmed 2026-07-09. No peeling into subpackages observed. |
| `sh -c` on operator-trusted inputs / ungated script exec (codex.go packets; canon verifier; finding-compiler/prune-agents without aoBinaryInside) | 2026-06-14, 2026-06-24 (M-1), 2026-07-01 (M-1), codex-2026-07-09 (F-06 legacy) | OPEN; codex.go trio now demoted to legacy-build-only per the Codex run, but the 07-01 aoBinaryInside asymmetry is unverified since. |
| Bash pre-push monolith / triple gate orchestration without parity | 2026-06-14, 2026-06-24, 2026-07-02 (A3) | OPEN at last check (2263 lines, no parity check). Verify existence + any drift check now. |
| Archived-but-executable retired surfaces (legacy/flywheel tags, bd-named scripts, bin/ralph, grandfathered baselines) | 2026-06-14, 2026-07-01, codex-2026-07-09 (F-04/F-05/F-08) | OPEN; baseline whole-file grandfathering keeps drift gates green (F-08). bin/ralph still carries `claude -p` per Codex. |
| Unbounded `bufio.Scanner` parsing (64KB truncation class) | 2026-07-01 (P3), codex-2026-07-09 (F-03) | LIVE: Codex directly reproduced a hidden trailing FAIL via a 70000-byte council line in tick.go. The 07-01 sprawl (64 readers) presumably still unconsolidated. |
| Quorum family→context weakening / cross-family opt-in | 2026-06-14 | Unresolved as a doctrine question; operationally superseded in practice by the cross-family pawl flow, but the code default (`RequireCrossFamily` OFF) was never re-verified. |
| Recon suite drifting against its own prior output; single-lens overconfidence (06-24 "Security STRONG" vs 07-02 false-green discovery) | 2026-07-02 (A6/A10/A11), codex-2026-07-09 (F-13) | Standing methodological warning: rotate the lens, re-check the numbers this sweep writes down. |
| Provenance chain tamper-evident-not-proof; ledger health | 2026-06-14, 2026-06-24 | Ledger grew 1 → 425 lines (confirmed 2026-07-09), so it is live; the unkeyed-SHA-256 caveat stands by design. The 0-escapes-across-130-verdicts figure needs refresh against the current ledger. |

## 4. Watch-list for the fresh sweep (merged, deduplicated, verifiable)

Gate / enforcement fail-open (highest priority):

1. **A1 gate fail-open:** inspect `cli/internal/gates/scriptrunner.go` (GateStatusUnknown
   mapping, ~44-71) and `cli/internal/gates/report.go` `isBlockingFail` (~28-34) — does
   Unknown-on-Blocking now fail closed in `ExitCode()`? Is `MissingBlockingCount>0` in the
   default exit path rather than behind `--require-workflow-parity` (gate_check.go:164)?
2. **A2 orphaned hook:** run `git config core.hooksPath` — must not be `.beads/hooks`; confirm
   the effective pre-push invokes `ao gate`; check `.githooks/pre-push:7-13` for the false
   comment and `install-pre-push-gate.sh` for hijack detection.
3. **Trunk verification (Codex F-01/F-02):** re-run `ao gate check --fast --scope head` and
   `bash scripts/validate-skill-schema.sh` — does `skills/goal-design/SKILL.md` (~20-24) still
   declare `context.intent.mode: explicit` against a schema admitting only
   questions/task/none? Grep `docs/provenance/ledger.jsonl` for a bound verdict on current
   origin/main HEAD; check `.github/workflows/verdict-backstop.yml` (~18-24, 50-64) — is
   enforce still false/report-only?
4. **Council scanner overflow (Codex F-03):** `cli/cmd/ao/tick.go` ~739-945 — do the three
   council `bufio.Scanner` passes now set `Buffer()` and check `scanner.Err()`? Regression
   test with a 70000-byte line + trailing FAIL?
5. **Changelog SKIP (Codex F-07):** `cli/internal/gates/checks/native_inline.go` ~56-67 — does
   a missing/unreadable changelog now FAIL instead of SKIP on a blocking check?
6. **verifycfg fail-open (Codex F-11):** `cli/internal/verifycfg/verifycfg.go` ~252-266 — does
   malformed `.aoverify.yaml` now fail closed for safety keys instead of warning into weaker
   defaults (strict=false, autobind=true)?

Gate parity / unwired detectors:

7. **A3 bash gate:** does `scripts/pre-push-gate.sh` still exist (~2263 lines) and is there now
   a drift check asserting its script set ⊆ Go registry backings
   (`cli/internal/gates/workflow_coverage.go` reconciled only `validate.yml`)?
8. **A8 buildtags:** grep `.github/workflows/*.yml`, `scripts/ci-local-release.sh`, and the
   gate registry for `verify-buildtags` — wired into anything automated yet?
9. **check-doc-skill-refs.sh:** confirmed still absent from `.github/` on 2026-07-09 — are the
   7 dangling slash-refs in `docs/architecture/operating-loop.md` repointed/retired?
10. **M-2 lint:** `cd cli && make lint` — still red (~13 issues) and still outside the
    blocking gate?
11. **A4/A5 retired-tech gate:** does `scripts/check-docs-no-retired-tech.sh:43` now flag
    `ao (rpi|orchestrate|evolve)` (only those verbs — NOT flywheel/corpus/loop/tick), and is
    `grep -rE '\bao (rpi|orchestrate|evolve)\b' docs/` near-zero or bannered
    (how-it-works.md, software-factory.md, ARCHITECTURE.md, agentops-system-map.md,
    first-value-path.md)?

Security / trust boundaries:

12. **M-1 (07-01) ungated script exec:** do `bestEffortRefreshFindingCompiler` (findings.go
    ~:380) and `bestEffortPruneAgents` (session_end_maintenance_helpers.go ~:92,:155) now
    apply the `aoBinaryInside` guard (pawl.go:115-140), and is stderr still discarded?
13. **Canon verifier `sh -c`:** `cli/internal/canon/verifier.go:57` — doc-comment trust
    invariant on `cv.Command` + guard test asserting operator-config-only provenance?
14. **bin/ralph (Codex F-04):** grep `bin/ralph` for `claude -p`/`bypassPermissions`
    (was ~165-179) — deleted or ported? (LAW 0 surface.)
15. **MCP stale binary (Codex F-14):** `cli/internal/adapters/mcpsurface/surface.go` ~114-137 —
    does it resolve `os.Executable()`/injected binary instead of shelling literal `ao` from
    PATH?
16. **M-3 /tmp paths:** `check-three-gap-supergate.sh` and `pawl.sh` — mktemp'd yet?
17. **Legacy codex dispatcher (Codex F-05/F-06/F-09):** does `cli/cmd/ao/codex.go`
    (`//go:build legacy`) still carry lexical path containment, `sh -c` required-commands with
    presence-only receipts, unbounded buffers? Confirm still absent from the default binary.

Consolidation adoption (the 07-01 pattern set):

18. **P2 preamble:** `grep -rl 'preamble.sh' scripts/ | wc -l` (was 1 = itself); did a
    new-file-scoped drift gate (`check-new-scripts-use-preamble.sh`) ship; do post-2026-06-26
    scripts source it?
19. **P3 JSONL:** are `storage.ScanJSONL`/`AppendJSONL` exported and adopted; grep
    `bufio.NewScanner` over `.jsonl` readers outside internal/storage; is
    `cli/testdata/transcripts/real-2.4mb.jsonl` still exercised?
20. **P1/P4 escape:** does `forge_curator_id.go` `writeJSONAtomic` (was :17) now delegate to
    `storage.AtomicWriteFile`? Also re-check the 06-24 leftovers: `inject.go` (~L452),
    `adapters/vendorimage/codexruntime/runtime.go` (~L741), `pool/pool.go atomicMove`
    (~L1157, fsync); parent-dir fsync in `storage/atomicfile.go`.
21. **P5 codex-exec:** does `scripts/lib/codex-exec.sh` exist and does `pawl-review.sh`
    delegate, or is stall/echo/NO-VERDICT logic still duplicated across ≥8 scripts?

Doctrine / structure / hygiene:

22. **RequireCrossFamily wiring (2026-06-14):** grep binding significant-action callers
    (`liveness.CheckSignificantAction` / `IsSignificantAction` /
    `AdmitInboundWorkMessage`) — does any one-way-door caller set `RequireCrossFamily:true`,
    or was same-model fresh-context quorum consciously accepted / the surface retired with
    rpi?
23. **cli/cmd/ao concentration:** `ls cli/cmd/ao/*.go | wc -l` — 669 confirmed 2026-07-09;
    any lanes peeled into subpackages since?
24. **Runaway-budget migration:** `ao rpi` removed (f61c5f0e7) — verify the unbounded-loop /
    finite-budget concern migrated to the replacement substrate (NTM / `ao agent` /
    governor budget) with a persistent per-bead fail-counter.
25. **bd→br debt:** `grep -rl 'bd ' scripts/` for bd-named scripts invoking retired verbs;
    legacy `.beads/` Dolt config physically retired or still byte-for-byte?
26. **Retired-command residue (Codex F-08):** do `scripts/install.sh` (~252-264
    `hooks install --force`) and `scripts/nightly-evolution.sh` (~593-690 daemon jobs) still
    call removed commands while `.scripts-ao-invocations-baseline` whole-file-grandfathers
    them?
27. **make docs-check (Codex F-15):** still exit 127 on the deleted
    `scripts/validate-hook-preflight.sh` (Makefile ~29-32)?
28. **A6 safety false-green:** does `cli/internal/safety/doc.go` still describe deleted hooks
    as active; `safety_test.go` `simulateRunRestricted`; dead
    `sandbox.go ValidateTeamLifecycle`/`ValidateMessageSize`?
29. **A7/F-12 machine contracts:** `ao capabilities` — per-command exit codes 3–10 present
    now (strict HOLD exit 5, verify entry, env inputs)? pawl-verdict schema prose vs
    `scripts/pawl-verdict.sh` REBOUND-authorizes divergence?
30. **Doc scale counts (Codex F-13, A10/A11):** `docs/architecture/codebase-overview.md`
    (~51-62, 122-151) and `cli/README.md` — do counts still lag (expect ~59 skills / 72
    default commands / 112 checks / 363 scripts) and still label `ao corpus`/`codex`/RPI
    active? Refresh worktree count (`git worktree list`) and skill-disposition counts against
    `docs/contracts/skill-dispositions.yaml`; do NOT re-copy stale numbers.
31. **Eval fixtures (Codex F-10):** `ao eval chaos` council fixtures (tick.go ~845-881)
    missing required `context_id` — fixed?
32. **Ledger + escapes refresh:** `wc -l docs/provenance/ledger.jsonl` (425 on 2026-07-09) —
    is it audited or just accreting; re-verify the 0-escapes figure against current verdicts.

## 5. The same-day Codex run — cross-family baseline

The Codex-native sweep of 2026-07-09 (pinned base fbba8af5) is the cross-family baseline the
fresh Claude sweep will be compared against. Its promoted findings, 3 High / 9 Medium / 3 Low:

**High (3):**

- **F-01** — origin/main tip has no bound verdict in `docs/provenance/ledger.jsonl`; the hosted
  `verdict-backstop.yml` warned (GH run 29033274027) but passed because enforce=false. The
  no-verdict-no-done contract is violated by the trunk itself.
- **F-02** — trunk fails the canonical fast gate: `skills/goal-design/SKILL.md:20-24` declares
  `context.intent.mode: explicit`; schema admits only questions/task/none.
  `ao gate check --fast --scope head` = 66 PASS / 1 FAIL / 1 SKIP.
- **F-03** — oversized council verdict line hides a trailing FAIL: three default
  `bufio.Scanner` passes in `cli/cmd/ao/tick.go:739-945` never raise the token limit nor
  check `scanner.Err()`; reproduced COUNCIL PASS + exit 0 with a 70000-byte line.

**Medium (9):**

- **F-04** — tracked `bin/ralph:165-179` invokes forbidden `claude -p` with bypassPermissions,
  outside the Door 9 scan boundary (adjusted DOWN from High: no current caller).
- **F-07** — missing/unreadable changelog clears a blocking fast gate as SKIP
  (`native_inline.go:56-67`).
- **F-08** — supported scripts call removed CLI commands (`install.sh:252-264`,
  `nightly-evolution.sh:593-690`) while `.scripts-ao-invocations-baseline` whole-file-
  grandfathers them.
- **F-11** — malformed committed `.aoverify.yaml` warns and falls back to weaker defaults
  instead of failing closed (`verifycfg.go:252-266`).
- **F-12** — machine contracts disagree with executable truth (pawl-verdict REBOUND prose vs
  script; capabilities omits HOLD exit 5, verify entry, env inputs). Partial fix since 07-02.
- **F-14** — MCP surface shells literal `ao` from PATH (`mcpsurface/surface.go:114-137`) — can
  execute a stale installed binary.
- **F-15** — `make docs-check` (Makefile:29-32) calls the deleted
  `scripts/validate-hook-preflight.sh`, exit 127.
- **F-05/F-06/F-09** (counted as the legacy dispatcher trio) — `cli/cmd/ao/codex.go`
  (`//go:build legacy`): lexical symlink-ancestor path containment; host `sh -c` with
  presence-only receipt validation ignoring non-zero exit; unbounded output buffers, no
  process-tree kill. Adjusted DOWN: legacy build profile only, absent from the default binary;
  the producer's untagged proving test was refuted.

**Low (3):**

- **F-10** — `ao eval chaos` ships stale council fixtures omitting required `context_id`
  (tick.go:845-881) → deterministic false council-matrix failure.
- **F-13** — canonical docs advertise obsolete scale (should be ~59 skills / 72 cmds /
  112 checks / 363 scripts) and label archived `ao corpus`/`codex`/RPI as active — persistent
  and broader than 07-02's A10/A11.
- (The third Low slot is the residual doc-count/labeling split of F-13's two halves as
  originally promoted; treat F-10 + both F-13 components as the Low set.)

The fresh Claude sweep should treat every F-* above as a claim to independently confirm or
refute (cross-family verification, not deference): agreement strengthens the finding;
divergence is itself signal — either a Codex false-positive or a Claude blind spot — and must
be recorded explicitly rather than silently merged.
