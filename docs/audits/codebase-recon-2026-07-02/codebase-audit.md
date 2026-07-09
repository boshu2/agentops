# AgentOps — Codebase Audit (2026-07-02)

> **Skill:** `codebase-audit` · **Run:** 2026-07-02 · Lenses: architecture-drift, security/safety, CLI-robustness, debt
> **Method:** two adversarial verification agents (drift/debt + security/CLI) over the mental model in `codebase-archaeology.md`. Every finding was verified by reading/grepping/building; refuted hypotheses and verified-sound guards are reported for honesty (matching the repo's own "don't market ahead of the ruler" posture).
> Repo @ `fd9f38e26` (main).

## Summary

| Severity | Count | Findings |
|----------|-------|----------|
| **High** | 5 | A1 (gate fails-open), A2 (orphaned pre-push hook), A3 (no bash↔Go gate parity), A4 (~60 docs teach removed CLI), A5 (retired-tech gate blind spot) |
| **Medium** | 3 | A6 (safety threat-model drift + false-green tests), A7 (capabilities exit-code contract gap), A8 (buildtag guard unwired) |
| **Low** | 4 | A9 (CLAUDE.md/help cite legacy cmds), A10 (overview stale), A11 (triage counts drifted), A12 (`sh -c` narrative mismatch) |
| **Refuted / Sound** | 7 | pawl RCE guard sound · gold sanitizer real · CLI robustness good · regen in sync · worktrees clean · 3.0-readiness present · path-traversal guard real |

**The two that matter most:** **A1** — a live fail-open in the release-authority gate, and it is *itself an uncaught membrane escape* (the product's whole thesis is that unproven work is rejected; here an unrunnable proof passes). **A2** — on this checkout the release-authority hook is not actually wired. Together they mean the local release gate has two independent holes that both fail toward "allowed."

---

## HIGH

### A1 — `ao gate check` fails OPEN when a blocking check's backing script is missing or won't launch
- **Location:** `cli/internal/gates/scriptrunner.go:44-45,66-71` · `cli/internal/gates/report.go:28-34` · `cli/internal/gates/orchestrator.go:193-194`
- **Issue:** `ScriptRunner.Run` maps a missing backing script (`os.Stat` fails) or a subprocess that fails to launch to `GateStatusUnknown` with a `nil` error. `ExitCode()` returns 1 only for `isBlockingFail` = `Blocking && Status == GateStatusFail`. `Unknown` is structurally excluded — so a **blocking check that cannot run silently passes.**
- **Root cause:** "cannot produce a verdict" was modeled as `Unknown` (advisory) rather than fail-closed for blocking checks. The only guard that treats a missing blocking script as failure (`report.Coverage.MissingBlockingCount`, `gate_check.go:164`) is gated behind the opt-in `--require-workflow-parity` flag, which the default/pre-push invocation does not set.
- **Proof:** In an isolated scratch repo with no `scripts/` dir, 22 blocking script-backed checks — incl. `always.agents-write-surfaces`, `corpus.path-guard`, `always.door9-no-claude-p`, `always.no-tracked-agents` — all returned `UNKNOWN` and contributed **zero** to the exit code.
- **Failure scenario:** a `Backing:` name typo in `seed.go`, a renamed/deleted script, or running `ao` against a root where `scripts/` doesn't resolve → that blocking gate reports `UNKNOWN` → `ao gate check` exits 0 → the change lands unguarded, silently.
- **Why it's the headline:** this is a **membrane escape the membrane doesn't catch** — exactly the class AgentOps exists to eliminate. "No verdict = not done" is violated: *no verdict currently reads as done.*
- **Fix:** treat `GateStatusUnknown` on a `Blocking` check as fail-closed in `ExitCode()` (or make a missing script / non-nil launch error return `GateStatusFail` for blocking checks). Fold `MissingBlockingCount > 0` into the default exit path, not behind `--require-workflow-parity`. This is a candidate for a new *constraint* in the escape corpus.

### A2 — Local pre-push gate is orphaned: `git push` runs the retired `bd` shim, not `ao gate check`
- **Location:** `git config core.hooksPath` → `.beads/hooks` · `.beads/hooks/pre-push` · `.githooks/pre-push:7-13` · `scripts/install-pre-push-gate.sh:25-58`
- **Issue:** `core.hooksPath` points at `.beads/hooks`, whose `pre-push` runs **only** `bd hooks run pre-push` (0 references to `ao gate`). `bd` is installed so the shim fires; `.git/hooks/pre-push` does not exist. `install-pre-push-gate.sh` installs the gate into `${git-common-dir}/hooks` (`.git/hooks`), which git **ignores** whenever `core.hooksPath` is set.
- **Root cause:** `bd`'s hook install hijacked `core.hooksPath` and the AgentOps installer doesn't detect it. Aggravating: `.githooks/pre-push:7-13` asserts "core.hooksPath points at `.git/hooks`" — factually wrong.
- **Impact:** the documented "local cockpit/pawl gate as release authority" is not invoked on push on this checkout; enforcement relies on manual `ao gate check --fast` + the CI backstop. Combined with A1, the release authority has two fail-toward-allowed holes.
- **Fix:** `git config --unset core.hooksPath` and re-run `scripts/install-pre-push-gate.sh`, or repoint `core.hooksPath` at the AgentOps hooks dir and chain the bd shim. Correct the `.githooks/pre-push` comment. Have the installer detect/refuse a hijacked `core.hooksPath`.

### A3 — No parity mechanism between the bash monolith `pre-push-gate.sh` and the Go gate registry
- **Location:** `scripts/pre-push-gate.sh` (2263 lines, ~40 sections, invokes 55 unique `scripts/*.sh`, **0** refs to `ao gate`) · `cli/internal/gates/checks/seed.go:196-360` · `cli/internal/gates/workflow_coverage.go` · `.github/workflows/validate.yml:210-215`
- **Issue:** three orchestrations of the same check corpus (bash monolith / Go registry / CI). Refinement of the premise: the Go registry mostly **shells the same `scripts/*.sh`** via `Backing:` (~99 shell-backed + ~11 native = ~110 checks), so the *logic* is shared but the *orchestration* is triplicated. `workflow_coverage.go` reconciles the Go registry against `validate.yml` only — nothing reconciles `pre-push-gate.sh`'s 55-script inventory against the registry, so the bash escape hatch can silently drift.
- **Root cause:** incomplete migration; the bash monolith is reachable via `AGENTOPS_GATE_BASH=1` and has no coverage assertion.
- **Fix:** retire `pre-push-gate.sh` (its own comment at `pre-push.local:92` plans this), or add a drift check asserting its script set ⊆ the Go registry's backings.

### A4 — ~60 docs teach the removed `ao rpi` / `ao orchestrate` / `ao evolve` command surface as live
- **Location:** `docs/how-it-works.md:181-194` · `docs/software-factory.md:43,67` · `docs/agentops-system-map.md:126-127` · `docs/first-value-path.md:175` · `docs/ARCHITECTURE.md:432,436` (+ ~64 more)
- **Issue:** `ao rpi` was removed in `f61c5f0e7` (2026-06-19); the default build has no `rpi`/`orchestrate`/`evolve`/`corpus`/`loop`/`tick`. Yet **69 docs** reference these; only **9** carry a retirement banner in their first 15 lines. Flagship "getting started" docs present them as canonical live commands.
- **Impact:** any user/agent following `how-it-works.md` hits "unknown command" — direct contradiction of CLAUDE.md.
- **Fix:** rewrite flagship docs to the operating-loop path; add RETIRED banners where historical context is intentional. (`docs/architecture/ports-and-adapters.md` is clean — good.)

### A5 — The retired-tech gate's regex omits the removed `ao` verbs, so A4 stays green
- **Location:** `scripts/check-docs-no-retired-tech.sh:43` (gate `docs.no-retired-tech`, `seed.go:330`)
- **Issue:** `PATTERN` matches `bd (ready|list|…)`, `gas[ -]?city`, `agentopsd`, `ao init --hooks`, `CI is the authoritative gate` — but **not** `ao rpi`, `ao orchestrate`, `ao evolve`. This coverage hole is exactly why unbannered docs teaching those removed verbs pass the gate.
- **Root cause:** the pattern wasn't extended when the RPI/orchestrate/evolve CLIs were removed.
- **Fix (highest leverage) — CORRECTED after build-tag verification (Stage-3 rpi, OP-2/OP-5):** add only **`\bao (rpi|orchestrate|evolve)\b`** to the pattern. My original draft `…(rpi|orchestrate|evolve|flywheel|corpus|loop|tick)…` was **over-broad and would break the build's own docs** — verified against the default `ao` build: `flywheel` is LIVE (`cmd/ao/metrics_flywheel.go`, no build tag), `corpus`/`inject` are real `//go:build flywheel` commands docs legitimately cite, and `tick`/`loop` are `//go:build legacy` (absent from default but not *removed*). Only `rpi` (no `Use:` — gone, `ao rpi`→exit 1), `orchestrate` (`//go:build legacy`), and `evolve` (gone) are safe to flag. **Scope note:** even the safe set trips **163 lines across 52 live docs** (`rpi|orchestrate|evolve|…` measured; the narrowed set is smaller but still ~dozens), so the gate change MUST land together with the doc sweep or the gate goes red — this is a sizeable, careful sweep, not a one-liner. Tracked as remaining work; the *correction to this finding* is the deliverable landed here.

---

## MEDIUM

### A6 — Safety threat model (`doc.go`) and its tests describe deleted hook enforcement — false-green tests
- **Location:** `cli/internal/safety/doc.go` (T1/T3/T4/T9) · `cli/internal/safety/safety_test.go:5-77` · `cli/internal/safety/sandbox.go`
- **Issue:** commit `e431339c4` (hookless) deleted every hook the threat model cites as mitigation (`task-validation-gate.sh` T1/T2, `dangerous-git-guard.sh` T3, `git-worker-guard.sh` T4, `stop-team-guard.sh` T9). Per-threat status now: **T2** (path traversal) is genuinely enforced in Go (`pool.validateCandidateID`, `ratchet.ValidateArtifactPath`) ✔; **T5** relocated to the Go pre-push gate + pawl verdict ✔ (modulo A1); **T1/T3/T4/T9** enforcement is **gone** or downgraded to advisory skills. `safety_test.go` states "the actual enforcement lives in hooks/…" then **re-implements the logic in Go** (`simulateRunRestricted`) — green tests exercising no shipping code (the "fixture fidelity" antipattern `.claude/rules/go.md` warns against). `sandbox.go`'s `ValidateTeamLifecycle`/`ValidateMessageSize` have **zero runtime callers** — dead code.
- **Impact:** an auditor reading `doc.go` believes command-injection allowlisting, destructive-git blocking, and worker-commit blocking are actively enforced. They are not.
- **Fix:** rewrite `doc.go` for the hookless architecture (what moved to Go, what is advisory-skill, what was removed); delete or re-point the mirror tests to assert real enforcement; delete the dead `sandbox.go` functions or wire them.

### A7 — Published `ao capabilities` exit-code contract omits the typed codes the CLI emits
- **Location:** `cli/cmd/ao/root.go:110-157` · `cli/cmd/ao/tick.go:21-27` · `cli/cmd/ao/governor.go:126` · `ao capabilities` output
- **Issue:** `ao capabilities` publishes a global `exit_codes` map of only `{0,1,2}` and no per-command codes. But `Execute()` propagates verbatim: `pawl review` 3/4, `plan-pawl decide` 3/4, `governor budget` 3 (HARDEN), `tick` 3/4/5/6/8/10. An agent mapping results via the machine contract cannot interpret 3–10.
- **Root cause:** the capabilities generator reflects the command *tree* but not the typed-error exit-code definitions.
- **Fix:** add per-command `exit_codes` to the capabilities output, generated from the typed error definitions. For a tool whose value prop is a machine-readable agent contract, this is a real inconsistency.

### A8 — ADR-0012 build-tag bitrot guard exists but is wired to nothing automated
- **Location:** `scripts/verify-buildtags.sh` · `Makefile:23-24` · `scripts/ci-local-release.sh` (no ref) · no `.github/workflows/*.yml` ref · not in the gate registry
- **Issue:** 47 non-test archived `.go` files (32 `legacy` + 15 `flywheel`; 62/33 incl. tests) are guarded only by `verify-buildtags.sh`, whose sole callers are a standalone Make target and `buildtags_test.go`. It is not in `local-ci`/`ci`/CI/gate. Archived satellite code (`ao orchestrate/loop/tick/codex/corpus/evolve`) can silently stop compiling between manual runs.
- **Note:** the legacy/flywheel code itself is **intentional** (ADR-0012 "archive satellites behind build tags"), not dead code to delete — the only real drift is the un-wired guard.
- **Fix:** add `verify-buildtags` to `ci-local-release.sh` and/or `nightly.yml`.

---

## LOW

### A9 — CLAUDE.md and `ao --help` cite flywheel/legacy commands as if default
- `cli/cmd/ao/corpus.go:1` is `//go:build flywheel`, so `ao corpus inject` (CLAUDE.md Quick Reference alt) is absent from the default build. Default `ao --help` still prints the example `… or ao orchestrate status` (archived). Fix: change the CLAUDE.md alt to `ao inject`; drop the `orchestrate` example from help.

### A10 — `docs/architecture/codebase-overview.md:330-332` stale despite a same-day edit
- File committed 2026-07-02, yet: :330 says `3.0-readiness.md` "checklist items still open" (the file exists but is a 2026-05-23 narrative with **zero** checklist items); :331 "~34 skills update/refactor" (actual: update=24/refactor=5=29); :332 "27 merge-eligible worktrees" (stale — 1 worktree now). Fix: refresh the "Open debt" section.

### A11 — Disposition triage doc counts drifted
- `docs/audits/2026-06-16-skill-disposition-triage.md` reports keep=38/update=27/refactor=7 (=34); live `skill-dispositions.yaml` is 35/24/5 (=29). Self-labeled "mechanical snapshot" but now wrong by 3/3/2. Fix: regenerate or add a "regenerate before trusting" note.

### A12 — `sh -c` execution surfaces have no allowlist (by-design, but contradicts the T1 narrative)
- `cli/internal/canon/verifier.go:63` (`AGENTOPS_CANON_VERIFIER_CMD`, documented TRUST BOUNDARY ✔), `cli/internal/eval/engine.go:293` (operator eval suites), `cli/cmd/ao/codex.go:1522` (task-packet `RequiredCommands`). These are legitimate dual-use (the agent/operator chose the commands) — **not vulnerabilities** — but no allowlist ever guarded them; the T1 allowlist only existed in the deleted hook. Reconcile the `doc.go` T1 narrative with reality (ties to A6).

---

## Refuted / Verified Sound (reported for honesty)

| Claim under scrutiny | Verdict | Evidence |
|----------------------|---------|----------|
| Pawl RCE guard bypassable | **Sound — no bypass** | `aoBinaryInside` uses `os.Executable` + symlink-resolved containment; stranger path sanitizes PATH, clears `BASH_ENV`/`GIT_EXTERNAL_DIFF`, resolves `bash` via `trustedLookPath`, pure-Go git-root discovery; `pawl-review.sh` hardens git (`--no-ext-diff --no-textconv`, `core.fsmonitor=`) |
| Gold-wiki sanitizer is theater | **Real** | `gold.go:174` → `llm.Redact` (16 secret patterns + `$HOME`), any-user home + session-UUID patterns, fail-closed leak scan on output. (Note: `.ao/wiki/` is untracked in this checkout, so the "tracked" premise doesn't hold here) |
| CLI robustness | **Good** | `ao --help`=0; unknown cmd/flag=1 with typo suggestions (`flagErrorWithSuggestion`); `--json` valid; `SanitizeGitProcessEnv` unsets `GIT_DIR`/`GIT_WORK_TREE`/`GIT_COMMON_DIR` every invocation |
| Path-traversal guard (T2) | **Real** | `pool.validateCandidateID` (pool.go:37 @ :319/:876); `ratchet.ValidateArtifactPath` (validate.go:706 @ :759) |
| regen drift (registry.json, COMMANDS.md) | **In sync** | `generate-registry.sh --check` = OK; `generate-cli-reference.sh --check` = OK. The MEMORY.md "154 missing commands" note is stale/resolved |
| Worktree bloat | **Resolved** | `git worktree list` = 1 worktree; the "27 merge-eligible" state is resolved |
| `docs/3.0-readiness.md` absent | **Refuted — present** | 5938 bytes, last touched 2026-05-24 |

---

## Recommended issue creation (top 3)

```bash
BEADS_DIR="$(ao beads dir)" br create --type=bug --priority=0 \
  --title="[gate] ao gate check fails open on missing/unlaunchable blocking backing script" \
  --description="scriptrunner maps missing script → Unknown, excluded from isBlockingFail; blocking check silently passes. See docs/audits/codebase-recon-2026-07-02/codebase-audit.md#A1"

BEADS_DIR="$(ao beads dir)" br create --type=bug --priority=1 \
  --title="[hooks] core.hooksPath hijacked by bd shim — ao gate not run on push" \
  --description="core.hooksPath=.beads/hooks runs bd shim only. See audit A2"

BEADS_DIR="$(ao beads dir)" br create --type=task --priority=1 \
  --title="[docs] add ao rpi/orchestrate/evolve to retired-tech gate + sweep ~60 docs" \
  --description="check-docs-no-retired-tech.sh:43 regex omits removed verbs. See audit A5/A4"
```

## Meta-observation

The two HIGH security findings (A1, A6) share a root cause: **the hookless migration (`e431339c4`) relocated enforcement from shell hooks into Go, but two seams were left fail-open** — the gate's `Unknown` handling (A1) and the safety threat-model narrative + tests (A6). Both are honest-to-goodness membrane holes in a product whose thesis is that the membrane has none. That is not a criticism of the thesis — it is exactly the escape-corpus feedstock the design predicts (`ADR-0011`). Compiling A1 into an active constraint check would be the system dogfooding its own self-improvement mechanism.
