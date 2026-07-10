# SYNTHESIS — Codebase Recon 2026-07-09 (Claude sweep)

> Executive summary over the four sibling reports in this directory. Read this first, then
> drill into: [codebase-archaeology.md](codebase-archaeology.md) ·
> [codebase-audit.md](codebase-audit.md) ·
> [codebase-pattern-extraction.md](codebase-pattern-extraction.md) ·
> [codebase-report.md](codebase-report.md). Prior-sweep lineage and the merged watch-list:
> [PRIOR-SWEEPS.md](PRIOR-SWEEPS.md). Repo tip at recon time: `2c2bfc3fb` (main).

---

## Verdict

AgentOps is a structurally healthy, unusually self-honest codebase — a ~165k-LOC Go CLI +
59-60 skill corpus + evidence ledgers enforcing one invariant (*no verdict = not done*) — with
**zero critical findings** and a defensive posture the audit calls "one of the most hardened
this skill has been run against" ([codebase-audit.md](codebase-audit.md)). Its real risk has
migrated outward, to the edges its own gates don't cover: the **Go toolchain itself carries a
High CVE that weakens the repo's `os.Root` containment primitive**, CI has **no govulncheck
lane** and installs its security scanners unpinned, and the repo's own second-order law —
*"a recommendation without a ratchet is a wish"* — is re-confirmed by a week of unmoved
adoption debt (hashchain dup, atomic-write hold-outs, a regressed ExitError sprawl)
([codebase-pattern-extraction.md](codebase-pattern-extraction.md)). Architecture and honesty:
excellent. Enforcement coverage at the boundary: the work.

---

## Converging findings (flagged independently by ≥2 reports — the signal)

1. **Doc scale-table drift is real and measured twice.** Both
   [codebase-archaeology.md](codebase-archaeology.md) and
   [codebase-report.md](codebase-report.md) independently measured
   `docs/architecture/codebase-overview.md` against disk: **101 gate checks (doc says ~77),
   59-60 skills (doc says 73), 257 bats files (doc says ~139), 345 scripts (doc says ~280)**.
   Drift direction is doctrinally correct (more gates, fewer skills) but the map lags the
   territory; [PRIOR-SWEEPS.md](PRIOR-SWEEPS.md) shows doc-drift as a 4-sweep recurring theme.
2. **"Only a gate changes behavior; everything ungated decays."**
   [codebase-pattern-extraction.md](codebase-pattern-extraction.md) proves it with a
   one-week delta: the prior-backlog item that had a mechanical gate (fail-open on
   `GateStatusUnknown`) got FIXED; the three enforced only by audit memory did not move
   (hashchain, atomic-write) or **regressed** (ExitError unwraps 9→12).
   [codebase-audit.md](codebase-audit.md) confirms the same law from the security side: the
   07-01 findings that were code-fixable got fixed; the class no in-repo gate covers (stdlib
   CVEs) sat undetected because there is no recurring scanner lane (M-2).
3. **The verification membrane itself is credible and consistently described.** All four
   reports independently converge on the same spine: skills plane (stochastic) / Go CLI +
   gates (deterministic) / evidence ledgers (hash-chained provenance, 425 records), with the
   local pre-push cockpit — not CI — as release authority, and honest demotion of the
   unproven corpus/flywheel claims (ADR-0004/0011) visible in the code itself (help-group
   demotion in `root.go`).
4. **"The exit code IS the verdict"** — the repo's most distinctive code pattern, called out
   by both [codebase-archaeology.md](codebase-archaeology.md) and
   [codebase-report.md](codebase-report.md): ~12 typed exit errors unwrapped in `Execute()`
   map domain verdicts to process exit codes (pawl 0/3/4, plan-pawl 3/4, governor 3). Machine
   callers never parse prose. The missing shared `ExitCode() int` interface is the flip side
   (finding #2 above).
5. **Stale `3.2.0-rc` fallback version** — observed live by
   [codebase-audit.md](codebase-audit.md) (L-2, both installed binaries report it after
   v3.2.0 shipped) and measured by [codebase-report.md](codebase-report.md); this repo
   already has a recorded "stale ao binary" footgun class, so it's not cosmetic.
6. **`cli/cmd/ao` main-package concentration** — ~300 non-test command files
   ([codebase-report.md](codebase-report.md)); [PRIOR-SWEEPS.md](PRIOR-SWEEPS.md) tracks the
   package growing 633→669 files across sweeps with no peeling into subpackages.

---

## Top risks (deduplicated, severity-ordered)

| # | Risk | Severity | Source |
|---|------|----------|--------|
| 1 | **GO-2026-4970 — `os.Root` symlink escape in the pinned toolchain (go1.26.3/.4; fixed in .5).** Not abstract: `os.Root` is this codebase's sandbox boundary for `.agents/` walks, vendor/runtime files, seed templates — exactly the trees agents and third-party tools write into. | **High** | [codebase-audit.md](codebase-audit.md) H-1 |
| 2 | **No `govulncheck` in CI.** gosec/gitleaks/semgrep cover own-code classes; nothing checks the module graph or stdlib against the vuln DB — which is precisely why #1 sat undetected one week after a clean scan. | Medium | [codebase-audit.md](codebase-audit.md) M-2 |
| 3 | **CI security scanners installed unpinned (`@latest`, bare pip).** The tools that gate the release are TOFU on upstream's morning; violates the repo's own "pin external contracts" doctrine. | Medium | [codebase-audit.md](codebase-audit.md) M-3 |
| 4 | **crypto/tls ECH SNI leak (GO-2026-5856)** — low practical impact today (LLM client is localhost), rides the same toolchain bump as #1. | Medium | [codebase-audit.md](codebase-audit.md) M-1 |
| 5 | **Adoption debt with no ratchet, 3 sweeps old:** 26 private `os.Rename` writers vs `storage.AtomicWriteFile` (crash-safety), 8-file hash-chain duplication (security-critical ledger code), ExitError unwrap sprawl regressing. Trajectory data shows these only converge once a shrink-only gate exists. | Medium | [codebase-pattern-extraction.md](codebase-pattern-extraction.md) Part 1; corroborated by [PRIOR-SWEEPS.md](PRIOR-SWEEPS.md) §3 |
| 6 | **Machine contract under-documents the real env surface:** `ao capabilities` lists 3 env vars; the binary honors ~77 (incl. `AGENTOPS_CANON_VERIFIER_CMD`, executed via `sh -c`). The primary consumer is agents; this is the discoverability gap that matters. | Low | [codebase-audit.md](codebase-audit.md) L-1 |
| 7 | **Doc/gate contradictions an agent would obey:** AGENTS-RUNTIME.md says `git ls-files .agents` should be empty while the gate maintains a 13-file allowlist (L-5); the tracked nightly "audit truth" lane looks dormant since 2026-06-06 (L-6); docs index describes the removed `ao rpi` as a live consumer (L-7). | Low | [codebase-audit.md](codebase-audit.md) L-5/6/7 |
| 8 | **Watch (cross-family, not re-verified this sweep):** the same-day Codex sweep found the pinned trunk UNVERIFIED (no bound verdict) and GATE-RED at its base commit; the Claude audit's live gate run at `2c2bfc3fb` passed 45/45, so this may be resolved — verify rather than assume. | Watch | [PRIOR-SWEEPS.md](PRIOR-SWEEPS.md) §1, §5 |

---

## Strongest reusable patterns & architecture facts worth remembering

From [codebase-pattern-extraction.md](codebase-pattern-extraction.md):

- **Shrink-only grandfather ratchet (N1)** — the repo's highest-value extractable mechanism:
  detector + checked-in pinned violator list that may only shrink + `--regenerate` at land
  time. The only observed mechanism that lands a retrofit gate with zero churn AND guarantees
  convergence. 8 in-tree instances, zero shared code — extract `scripts/lib/ratchet.sh`.
- **Three enforcement levels, three destinies (N2):** no gate → debt grows; new-files-only
  ratchet → debt frozen; shrink-only ratchet → debt melts. The policy follow-on: *every
  helper extraction ships its ratchet in the same arc.*
- **Bash prototype → Go authority promotion with env escape hatch (N3)** (`pre-push-gate.sh`
  → `ao gate check` behind `AGENTOPS_GATE_BASH=1`; `land.sh` → `ao land`; `emit_pawl_catch`
  → `ao membrane catch`). Worth a promotion-playbook checklist — each promotion re-derives it.
- **Gate check as a four-part unit (N4):** script + exit-contract header (0/1/2) with bead
  provenance + same-named bats twin + Go registry row. `add-validate-job.sh` scaffolds most
  of it already.
- **Classed failure register + UNCLASSIFIED-renders-loud digest (N5)** — the EM-spine
  convention: append-only register, shared class taxonomy, per-class recurrence deltas as the
  fitness signal; an unclassified entry is visible debt, never silently folded.

From [codebase-archaeology.md](codebase-archaeology.md) / [codebase-report.md](codebase-report.md):

- **Three planes:** skills (`skills/` SSOT, markdown contracts = typed ports) · deterministic
  control plane (`cli/`, ~80 internal packages, 101 gate checks) · evidence plane
  (`docs/provenance/ledger.jsonl` hash-chained where `hash = sha256(payload_hash + "\n" +
  prev_hash)` — tamper-*evident*, not tamper-proof; `_beads/` private nested repo; `.agents/`
  gitignored).
- **Release authority is the local pre-push cockpit** (fresh `ao` build → full race suite →
  gate → bound pawl verdict), installed in `$(git-common-dir)/hooks`; the tracked
  `.githooks/pre-push` is a historical shim, and CI `validate.yml` is a backstop only.
- **Lean by design:** ~9 direct Go deps, no DB, no network service, no daemon, no telemetry;
  all state is repo-local JSONL/YAML. Out-of-session orchestration is a swappable external
  substrate (NTM + Agent Mail; Gas City via `packs/agentops-membrane/`).
- Six bounded contexts route everything; BC2 Validation is the proven product, BC1 Corpus is
  honestly demoted to experimental.

---

## Prioritized actions

1. **P0 — Bump `toolchain go1.26.5`, rebuild, re-tag/re-release; acceptance = `govulncheck
   ./...` clean.** Clears H-1 + M-1 in one move. ([codebase-audit.md](codebase-audit.md))
2. **P1 — Add a pinned `govulncheck` step to CI (validate.yml + nightly) and pin
   gosec/gitleaks/semgrep versions.** The 07-01→07-09 staleness proves one-off scans don't
   hold the line. ([codebase-audit.md](codebase-audit.md) M-2/M-3)
3. **P1 — Extract `scripts/lib/ratchet.sh`** (ratchet_check / assert_shrink_only /
   regenerate); acceptance = migrating the two existing ratchet scripts onto it. Multiplier
   for every following item. ([codebase-pattern-extraction.md](codebase-pattern-extraction.md) #1)
4. **P1 — Ship the atomic-write shrink-only ratchet** (`.atomic-write-grandfather` + check
   via #3). Third consecutive sweep flag; crash-safety; also covers `pool.atomicMove`
   no-fsync. ([codebase-pattern-extraction.md](codebase-pattern-extraction.md) #2)
5. **P2 — Extract `cli/internal/hashchain`** from the 8 files duplicating `payload_hash`
   chain logic — the single largest, security-critical code extraction, unmoved for a week.
   ([codebase-pattern-extraction.md](codebase-pattern-extraction.md) #3, carried from 07-02)
6. **P2 — Land the shared `ExitError` interface in `cli/cmd/ao`** before the next N3
   promotion adds unwrap #13. ([codebase-pattern-extraction.md](codebase-pattern-extraction.md) #4)
7. **P2 — Generate the env-var surface into `ao capabilities`** (same regen lane as
   COMMANDS.md). ([codebase-audit.md](codebase-audit.md) L-1)
8. **P3 — Version-fallback discipline at tag time** (or derive from
   `debug.ReadBuildInfo()`), killing the `3.2.0-rc` ghost. ([codebase-audit.md](codebase-audit.md) L-2)
9. **P3 — Refresh (or generate) the codebase-overview Scale table** — 4 counts materially
   stale, measured identically by two reports. ([codebase-archaeology.md](codebase-archaeology.md),
   [codebase-report.md](codebase-report.md))
10. **P3 — One-line doc reconciliations:** AGENTS-RUNTIME.md `.agents` allowlist sentence;
    decide the dormant nightly audit-truth lane; annotate the ADR-0013 index line.
    ([codebase-audit.md](codebase-audit.md) L-5/L-6/L-7)

---

## Missing reports

**None of the four expected reports is absent or stub-thin** — codebase-archaeology,
codebase-audit, codebase-pattern-extraction, and codebase-report are all present and
substantive (14-16 KB each), plus [PRIOR-SWEEPS.md](PRIOR-SWEEPS.md) as merged prior-run
context. Note the run was scoped to these four lenses: the broader recon suite's
**briefing-report** and **risk-audit** lenses did not run as separate workers this sweep —
risk coverage is carried by codebase-audit (security-primary, 4-domain) and the PRIOR-SWEEPS
recurrence/watch-list. If a dedicated risk-audit pass is wanted, the highest-value target is
PRIOR-SWEEPS §4 items 1-6 (gate/enforcement fail-open verification), which this sweep's
workers did not systematically re-verify.

---

*Synthesized 2026-07-09 from the four sibling reports at repo tip `2c2bfc3fb`; no findings
were re-derived — every claim above cites its source report.*
