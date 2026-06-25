# Codebase Recon Synthesis — AgentOps — 2026-06-24

> ⚠️ **HISTORICAL SNAPSHOT — recon run 2026-06-24 against `abc018c42`; `main` has since advanced past `882e71c01`.** A point-in-time synthesis, **not** a current-state reference. Some conclusions are already superseded — notably the **P1 atomic-write DRY finding was PARTIALLY actioned** (`storage.AtomicWriteFile` is now the canonical impl and quest/llmwiki/doctor/wiki delegate to it, age-3azc/uja6; inject, vendorimage/codexruntime, and `pool.atomicMove` still carry their own copies). The **`ao` command (89) and gate (98) counts are reproducible at the pinned `abc018c42`** (re-verified there 2026-06-25 — and still current on `main`; the draft's figures were simply wrong). `main` has since advanced, so narrative *findings* may be superseded (notably P1, partially actioned), but these architectural counts are stable; all other figures are as-of the 2026-06-24 snapshot. **Inter-report variance:** the four sub-reports were generated independently by different recon skills, so the same *secondary* metric (per-language file counts especially) can differ by counting method/scope between them — the reconciled headline figures (commands 89, gates 98, ~154k/~223k Go LOC, 77 skills, 717 `.sh`, 54 `.py`) are authoritative; treat divergent per-report file tallies as method artifacts, not contradictions.

> **Read this first.** Executive summary across four recon reports run against
> `/Users/bo/dev/agentops` @ `abc018c42` (branch `main`). Source reports:
> [`codebase-archaeology.md`](codebase-archaeology.md) ·
> [`codebase-audit.md`](codebase-audit.md) ·
> [`codebase-pattern-extraction.md`](codebase-pattern-extraction.md) ·
> [`codebase-report.md`](codebase-report.md).
> `go build ./...` → exit 0 this run; a *sample* of `go test` → green. The full test suite + a deep security scan were **NOT** run — this is a build-green snapshot, not a release-readiness verdict. Command/gate counts re-verified 2026-06-25 at the pinned `abc018c42` (still current on `main`); `main` has since advanced past the snapshot.

---

## Verdict

**AgentOps is a mature, disciplined, unusually self-honest codebase that practices what it preaches.** It is the operational layer *for* coding agents — a single Go CLI (`ao`, ~154k LOC source / ~223k LOC test) plus a markdown skill corpus and a local `.agents/` knowledge tree — that wraps an existing agent (Claude Code, Codex, Cursor, OpenCode) with a **validation membrane** (catch "done" when it isn't; *no verdict = not done*) and a durable evidence trail. The read-only audit surfaced **no Critical/High findings** in its static read-through (a manual code review, **not** a full security scan, dependency-CVE audit, or test run — so absence of findings is not a clean-bill-of-health proof). The architecture is *genuinely* hexagonal (not aspirationally), and the docs/ADRs explicitly demote the unproven parts (the corpus moat and escape-corpus compounding) rather than over-claiming. The one real engineering debt the reports converge on is small and well-scoped: a true-DRY violation (8 divergent atomic-write instances, **5 since consolidated** into `storage.AtomicWriteFile` — age-3azc/uja6; inject/vendorimage/`pool.atomicMove` still private) and an in-progress gate-orchestration migration. `go build ./...` is green, but with the full test suite + deep scan unrun, **release-readiness is unassessed** — this is a healthy build-green posture, not a release-ready claim.

---

## Converging findings (independently flagged by ≥2 reports — the signal)

These are the highest-confidence conclusions because multiple reports reached them separately:

1. **The product = the validation membrane; the flywheel/corpus is explicitly unproven.** All four reports lead with this. The proven asset is the cross-family review loop ("pawl") that writes a commit-bound verdict; the knowledge corpus and escape-corpus self-improvement are *demoted to unproven hypotheses in doctrine* (ADR-0004, ADR-0011) due to structural data starvation — a competent membrane catches at review, so escapes are rare (measured **0 escapes across 130 production verdicts**). [archaeology, report, audit-doctrine-note]

2. **Genuinely hexagonal, minimal dependencies, JSONL-not-DB persistence.** Real `*Port` interfaces with paired in-memory test doubles + real driven adapters (archaeology traces a port→adapter→double live; pattern-extraction counts 27 `*Port` interfaces). Lean dep tree (cobra/pflag, toml, yaml.v3, jsonschema + test-only rapid/goleak/go-cmp). **No SQL DB, no HTTP server, no network client** in core; persistence is append-only JSONL on local disk. [archaeology, report, pattern-extraction]

3. **Test code outweighs source (deliberate), with isolation enforced *by gates*.** ~223k test LOC > ~154k source LOC. Test-isolation discipline (`t.Cleanup`/`t.Setenv`/`t.Chdir`/`t.TempDir`, with a `-shuffle=on` race suite) is a first-class, gate-enforced bug class, not a convention. [archaeology, report, pattern-extraction P7, audit]

4. **Drift-gated generation is the contract-integrity mechanism.** `registry.json`, `cli/docs/COMMANDS.md`, domain maps are *generated* from sources via `make regen-all`; `make regen-check` is the drift gate — the executable contract cannot silently diverge from docs. Hand-editing generated artifacts is a named footgun. [archaeology, report, audit-API, pattern-extraction]

5. **`/rpi` and `ao rpi` are NOT the live orchestration path — the #1 navigation trap.** `ao rpi` is load-bearing *legacy* (heavily tested, compiled, not live); live navigation is the seven-move operating loop; out-of-session is NTM + MCP Agent Mail (ADR-0009 deleted the in-repo daemon). Flagged identically by archaeology and report. [archaeology, report]

6. **`bd`/Dolt is retired; `br` (beads_rust, JSONL) is live.** Resolve the ledger with `ao beads dir`; never `git add _beads`. Flagged by both archaeology and report as a footgun. [archaeology, report]

---

## Top risks (deduplicated, by severity)

Net security/quality risk reads **low** — the read-only audit found **0 Critical / 0 High** in a static review (not a full security scan / dependency-CVE audit / penetration test, so this bounds *observed* risk, not residual risk). The items below are the real, deduplicated exposure surface:

| Sev | Risk | Source |
|---|---|---|
| **Medium → ◑ partial** | **DRY violation: 8 divergent atomic-write instances** (5 since consolidated into `storage.AtomicWriteFile`, age-3azc/uja6; inject/vendorimage/pool remain), 2 *exported with the same name but incompatible signatures*; the `pool.atomicMove` rename-only copy **still omits fsync → can still lose data on a crash between write and rename** (unmigrated). The single highest-value fix in the run, now partly done. | pattern-extraction P1 |
| **Medium** | **`sh -c` on an operator-configured verifier** (`cli/internal/canon/verifier.go:57`). Not an injection vuln in the current threat model (value comes from `AGENTOPS_CANON_VERIFIER_CMD`, e.g. `codex exec`), but residual risk if a future caller ever feeds `cv.Command` from corpus/remote data. Wants a doc-invariant + guard test, not a code change today. | audit M-1 |
| **Medium** | **Path-containment maintenance surface:** 6,371 `filepath.Join` calls; new paths built from corpus/bead/remote input must route through the existing `EvalSymlinks`+`filepath.Rel` containment (as `worktree`/`resolver` already do), not a bare `Join`. Advisory — a lint gate would harden it. | audit M-2 |
| **Medium (debt, not risk)** | **Gate triple-orchestration migration in progress:** Go registry (primary) vs `validate.yml` (CI backstop) vs `scripts/pre-push-gate.sh` (legacy bash escape hatch via `AGENTOPS_GATE_BASH=1`); ~11 deferred scripts remain to fold into the Go registry. | archaeology open-debt |
| **Low/structural** | **Headline flywheel/escape-corpus claims are data-starved** (anti-correlated with membrane quality). Already honestly documented; the risk is *marketing ahead of the ruler*, which the repo deliberately refuses to do. | archaeology, report, ADR-0011 |
| **Low** | **Doc reconciliation drift** — older narrative docs (`ARCHITECTURE.md`, `ports-and-adapters.md`) still mention hooks/`bd`/PR-per-change; the source-of-truth precedence rule exists *because* of this drift. Plus ~34 skills marked update/refactor and pending worktree cleanup. | archaeology |
| **Low** | `curl … | bash` install path (industry-standard; tag-pinning supported, under-documented); `eval` indirect-assignment in `seed-evolution-roadmap-beads.sh:47`; high-but-healthy `_ = …` discard volume (1,075 vs 12,843 `err != nil`). | audit L-1/L-2/L-3 |

---

## Strongest reusable patterns + load-bearing architecture facts

**Patterns worth extracting / codifying** (pattern-extraction):
- **P1 (extract — highest value):** one `storage.AtomicWriteFile(path, data, perm)` to replace the 7 copies; keep the file fsync (and add a **parent-dir fsync after rename** for full crash-durability — main's current `atomicfile.go` lacks it); delete the private variants. Textbook DRY, low risk.
- **P6 (template):** `scripts/lib/preamble.sh` — `set -euo pipefail` + `REPO_ROOT` + **portable `stat`/`find` helpers** (centralizes the macOS `find`→`bfs` / `stat -f %m || stat -c %Y` portability hazard the repo memory keeps re-learning).
- **P2 (library):** generic append-only `jsonl.Writer[T]` over P1 — but **dedup stays at the emitter, never in the Writer** (honors the known `TestConcurrentAppend` race).
- **P5 (convention → review skill):** "every guard declares fail-open vs fail-closed, and fail-open must emit a visible marker, never be silent" — a rule the cross-family pawl keeps re-discovering; codifying it closes that loop.
- Already-healthy / codify-don't-refactor: P3 hexagonal ports, P4 cobra `--json`/human split, P7 test isolation, P8 `%w` error wrapping (1,762 uniform sites), P9 SKILL.md structure.

**Architecture facts to remember:**
- **Six bounded contexts** (DDD/hexagonal, ADR-0001): BC1 Corpus → BC2 **Validation (the proven product)** → BC3 Loop → BC4 Factory → BC5 Runtime → BC6 **Orchestration (substrate boundary)**.
- **Exit code = verdict.** `Execute()` maps typed errors to meaningful codes (gate FAIL=1, plan-pawl REDO=3/BLOCKED=4) so shell/CI read the decision without parsing stdout.
- **`App` DI container** injected via cobra context replaces former global mutable state.
- **Source-of-truth precedence:** executable+generated > contracts (SKILL.md, schemas) > narrative docs — stated inline so a lower doc can't redirect the rule.
- **Multi-runtime by design:** `.claude-plugin/`, `.codex-plugin/`, `.agy-plugin/`, `.opencode/` — deliberately not Claude-only.
- **LAW 0:** no `claude -p`/`--print` anywhere; cross-family review uses `codex exec` (live pawl scripts) / local bushido llama / an interactive NTM Claude pane — not `gemini -p`/AGY.

---

## Prioritized action list

1. ◑ **PARTIAL (age-3azc/uja6):** extracted `storage.AtomicWriteFile`; quest/llmwiki/doctor/wiki migrated onto it. **Still open:** migrate the remaining 3 (`inject.go`, `vendorimage/codexruntime`, `pool.atomicMove` — the last still omits fsync, the original data-loss gap), and add a **parent-dir fsync after rename** for full crash-durability (`atomicfile.go` fsyncs the file + renames but does not yet dir-fsync). (pattern-extraction P1)
2. **Annotate the `canon` `CommandVerifier` `sh -c` trust-boundary invariant + add a guard test** asserting `cv.Command` only comes from operator config. (audit M-1)
3. **Land `scripts/lib/preamble.sh`** centralizing strict-mode + repo-root + portable `stat`/`find` helpers across the 166 scripts that re-derive them. (pattern-extraction P6)
4. **Complete the gate-orchestration migration:** fold the ~11 deferred bash scripts into the Go registry; retire the `AGENTOPS_GATE_BASH=1` escape hatch once parity holds. (archaeology open-debt)
5. **Add a path-containment lint gate** flagging new `filepath.Join` on externally-sourced (corpus/bead/remote) segments, extending the existing `check-paths-resolver-coverage.sh`. (audit M-2)
6. **Fold the fail-open/fail-safe "declare + make visible" rule into the review skill** so the pawl stops re-finding silent fail-opens by hand. (pattern-extraction P5)
7. **Reconcile or label the drifted narrative docs** (`ARCHITECTURE.md`, `ports-and-adapters.md` — hooks/`bd`/PR-per-change wording) and triage the ~34 update/refactor skills. (archaeology)
8. **Document the tag-pinned `curl … | bash -s -- --ref vX.Y.Z` install variant** as the recommended path for cautious operators; replace the `eval` indirect-assignment with a bash nameref. (audit L-1/L-2)
9. **Add a fail-loud guard to the recon workflow itself:** if `< N` reports land, surface "RUN FAILED — k/N reports" instead of handing an empty dir to the synthesizer (the prior 2026-06-24 run silently produced zero reports). (process — from the stale SYNTHESIS this overwrote)

---

## Missing reports

This run landed four substantive reports — **archaeology, audit, pattern-extraction, report** — all present, none stub-thin (10.8K–16.5K each). Notes on coverage gaps:

- **No standalone risk report.** Risk findings are folded into [`codebase-audit.md`](codebase-audit.md) (severity-tagged Critical/High/Medium/Low) rather than a separate `*-risk.md`. Coverage is adequate; the audit *is* the risk surface.
- **No separate briefing report.** Orientation/briefing material is folded into [`codebase-archaeology.md`](codebase-archaeology.md) (recommended reading order, navigation traps) and [`codebase-report.md`](codebase-report.md).
- The four present reports are mutually corroborating and build-verified; no re-run is needed for this synthesis.
- *Prior-run note:* an earlier SYNTHESIS.md recorded a TOTAL FAILURE (0/6 reports) for this same date with a different expected naming scheme (`*-archaeology.md`, `*-briefing.md`, `*-risk.md`). That run's reports never landed; this synthesis supersedes it and is built on the four reports that did land.

---
*Synthesized 2026-06-24 from four read-only recon reports (counts re-verified 2026-06-25). Each section links its source. `go build ./...` → exit 0 this run (build only; full test suite + security scan NOT run).*
