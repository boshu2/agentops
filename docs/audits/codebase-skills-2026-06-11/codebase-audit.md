# Multi-Domain Audit Report: agentops (skills corpus + `ao` Go CLI)

- **Date:** 2026-06-11
- **Method:** `skills/codebase-audit/SKILL.md` (domain-parameterized; multi-domain sweep per the skill's "Quick Multi-Domain Sweep", deep on **security** and **cli**, moderate on **performance**, spot-checks on **copy/docs**)
- **Scope:** `cli/` (1,282 Go files, 648 scanned by gosec, 145,199 LOC), `scripts/` + `lib/` + `tests/` (256+ shell scripts), `skills/` (169 skills), `.github/workflows/`
- **Tooling run (read-only):** `go vet`, `go build`, `go test ./...`, `golangci-lint`, `gosec 2.27.1`, `gitleaks`, `shellcheck`, targeted `rg` per the skill's CHECKLISTS.md, live CLI ergonomics tests against the built `ao`

## Summary

- **Total:** 11 findings
- **Critical:** 0 | **High:** 0 | **Medium:** 4 | **Low:** 7

**Bottom line:** the codebase is in unusually good shape for its size and velocity. All first-party gates are green from a cold run: `go vet` clean, `golangci-lint` clean (errcheck + gocyclo@25 + staticcheck + misspell), **12,155 tests passing across 94 packages**, gitleaks clean across **4,617 commits**, shellcheck error-severity clean across `scripts/` and `lib/`. The real findings are process-shaped (an untriaged 655-finding gosec backlog, a 2,020-line CI monolith, stale bead references in config) plus one genuine O(N²) perf hotspot.

---

## Critical Findings

None.

## High Findings

None. All 41 gosec HIGH-severity findings were manually triaged and are false positives or benign (see M2 and the triage appendix).

---

## Medium Findings

### M1 — `validate.yml` is a 2,020-line / 103.7 KB single-workflow monolith

- **Location:** `.github/workflows/validate.yml` (2,020 lines; ~160 named steps; vs. the next-largest workflow at 15.5 KB)
- **Issue:** One workflow file carries the entire gate wall: skill gates, Go tests, contract canaries, Codex parity, security toolchain, eval workbench, retrieval bench, provenance gates, AP#7 evidence verification. Any edit to any gate touches the same 100 KB file; step interdependencies are implicit; review diffs are unreadable; the free-plan 20-slot CI concurrency makes this the wall-clock bottleneck.
- **Root Cause:** Years of additive gate accretion ("add a step per lesson") without decomposition into reusable workflows or the Go-native gate.
- **Fix:** Already directionally in flight (ag-qidx push-to-main + `ao gate check` Go orchestrator, epic ag-3n71). Accelerate: split into `workflow_call` reusable workflows per job family (skill-gates / go-tests / security / evals), or fold T0/T1 checks into the Go gate and shrink the YAML to invocation shims. Treat any *new* gate added directly to validate.yml as a review smell.

### M2 — Untriaged gosec backlog: 655 medium+ findings, only 17 `#nosec` annotations, no suppression policy

- **Location:** repo-wide `cli/`; breakdown: 358× G304 (file inclusion via variable), 121× G204 (subprocess with variable), 135× G301/G302/G306 (permissions), 34× G703 (path traversal, HIGH), 4× G115 (int overflow, HIGH), 2× G702 + 1× G704 (HIGH)
- **Issue:** Not that the findings are exploitable — spot-checked HIGHs are false positives for a local operator CLI (fixed-argv `git`/`tmux` exec at `cli/cmd/ao/skills_edit.go:346` and `cli/cmd/ao/handoff.go:301`; the G704 "SSRF" at `cli/cmd/ao/retrieval_search_backends.go:170` is a POST to an operator-configured local rerank endpoint; G115s are masked/`&0o777` conversions). The issue is **signal burial**: a future *real* injection or traversal bug will land inside a 655-finding pile nobody reads. CI has a "security toolchain gate" job, which implies a baseline file is absorbing these.
- **Root Cause:** Findings are accepted in bulk (baseline/config) rather than per-site (`// #nosec G<NN> -- reason`), so each finding's justification is undocumented and unreviewable.
- **Fix:** (1) Write a short suppression policy (the repo already has the two-scanner annotation discipline documented in `.claude/rules/go.md` — extend it with "when to #nosec G304/G204 for operator-supplied paths"). (2) Burn down the 41 HIGHs with per-site annotations + one-line reasons (a few hours; this audit's triage in the appendix is a starting point). (3) Ratchet: fail CI on *new* un-annotated HIGHs.

### M3 — O(N²) regex recompilation in `ao beads audit-cluster` pairwise scoring

- **Location:** `cli/cmd/ao/beads_audit_cluster.go:790` (nested i/j loop) → `scoreBeadOverlap` at `:844` → `tokenSet` at `:865` (inline `regexp.MustCompile(`[^a-z0-9/]+`)`) and `pathSet` at `:875` (per-call `MustCompile`)
- **Issue:** Clustering scores every bead pair; each `scoreBeadOverlap` call invokes `tokenSet` twice and `pathSet` twice, and each of those compiles its constant regex from scratch. For N beads that is ~2·N² regex compilations (plus re-tokenizing the same bead text N times). On a tracker with 1,000+ beads this turns a sub-second command into many seconds of pure recompilation.
- **Root Cause:** Constant patterns declared inside hot functions instead of package-level `var ... = regexp.MustCompile(...)`. (166 in-function `MustCompile` sites exist repo-wide; the rest are once-per-invocation and harmless for a CLI — this is the only pairwise-loop case found.)
- **Fix:** Hoist both patterns to package-level vars; additionally memoize `tokenSet`/`pathSet` per bead ID before the pairwise loop (computes each bead's sets once: O(N) tokenization + O(N²) cheap map intersections).

### M4 — Stale/dead bead references baked into lint config (deferred-linter debt is aging invisibly)

- **Location:** `cli/.golangci.yml:13-18` — `errorlint -> agentops-bs8`, `unconvert -> agentops-0l9`, `unused -> agentops-0l9 (~37 backlog)`
- **Issue:** Three linters are disabled "pending fix beads," but the cited bead IDs use the retired `agentops-` prefix (the tracker now uses `ag-`/`cp-`). If those beads no longer resolve, the deferral has no owner and no expiry — ~37 known `unused` findings plus the `%w`/`errors.Is` backlog sit permanently un-linted. The comment also pins the rationale to `rpi_c2_events.go` being "deprecated," while `CLAUDE.md` now declares that lane "load-bearing, live (tested)" — the two statements have drifted.
- **Root Cause:** Config comments referencing tracker IDs with no gate verifying the IDs still exist.
- **Fix:** Re-home the deferrals to live beads (or close them: enable `unconvert`+`errorlint` with path exclusions, burn the ~37 `unused` findings down in one sweep), and update the `rpi_c2_events.go` rationale to match the current "legacy but live" doctrine.

---

## Low Findings

### L1 — Shellcheck warnings confined to `tests/` (traps + unquoted command substitution)

- **Location:** `tests/skills/test-allowlist-negative.sh:59,77,93` (SC2064 — `trap "rm -rf $TMPDIR_2" EXIT` expands at trap-set time, not signal time); `tests/claude-code/test-rpi-e2e.sh:196,267,383,595,701,755` (SC2046 — unquoted `$(date +%Y-%m-%d)` in redirect targets); plus 5× SC2155, 5× SC2034 nearby
- **Issue:** `scripts/` and `lib/` are warning-clean; all 18 warning sites live in test harnesses. The SC2064 traps work today only because the vars are set before the trap and never change; a reorder silently breaks cleanup.
- **Fix:** Single-quote the trap commands; quote the `$(date ...)` substitutions. Mechanical, ~15 minutes. Consider extending the CI ShellCheck step's warning severity to `tests/`.

### L2 — Usage errors exit 1, not 2

- **Location:** `ao` root command behavior (`ao --bad-flag` → exit 1; `ao nonexistent-cmd` → exit 1)
- **Issue:** The skill's CLI checklist (and POSIX convention) distinguishes runtime failure (1) from usage error (2). Cobra defaults to 1 for both; scripted callers can't tell "command failed" from "I called it wrong."
- **Fix:** Optional polish — set a custom `FlagErrorFunc`/arg-validation wrapper returning exit 2 for usage errors. Low value unless robot callers ask for it; if declined, document "all errors exit 1" in `ao capabilities`.

### L3 — File/dir permission inconsistency (0644/0755 writes; 135 gosec G301/G302/G306 findings)

- **Location:** e.g. `cli/cmd/ao/agent.go:86`, `autodev.go:77`, `beads.go:799`, `canon.go:359`, `defrag.go:182,185`, `eval_outcomes_ingest.go:172` + ~129 more
- **Issue:** Repo artifacts (`.agents/`, reports, ledgers) are written 0644/0755 — defensible for a collaborative repo tool, and *not* a vulnerability — but there is no stated policy, so gosec re-flags every new write site and reviewers can't tell intended from accidental.
- **Fix:** One-paragraph policy ("repo-visible artifacts 0644; anything under `~/.config`/secrets 0600") + a shared `writeRepoFile` helper, then a single `#nosec G306` inside the helper instead of 49 scattered findings.

### L4 — gosec HIGH false positives left un-annotated

- **Location:** `cli/cmd/ao/skills_edit.go:346` (G702), `cli/cmd/ao/handoff.go:301` (G702 — `tmux respawn-pane` executing `restartCmd` is the feature), `cli/cmd/ao/retrieval_search_backends.go:170` (G704), 4× G115 (`quality/doctor.go:189`, `corpus_snapshot.go:388,398`, `inject_context_paths.go:44`)
- **Issue:** Each is correct-by-construction but re-surfaces in every scan; the repo's own two-scanner suppression discipline (`.claude/rules/go.md`) isn't applied here.
- **Fix:** Per-site `// #nosec G<NN> -- <reason>` annotations (subset of M2's burn-down).

### L5 — 34 G703 path-traversal taint findings need a one-pass triage

- **Location:** spread across `cli/internal/` (`skillshealth/audit.go:244`, `ratchet/maturity.go:337`, `llm/redactor.go:93`, `lifecycle/*`, `goals/commands.go:469`, …) and `cli/cmd/ao/` (`seed.go:432`, `rpi_serve.go:575,591`, `session_bootstrap.go:270`, …)
- **Issue:** Spot-checks show operator-supplied paths flowing to `os.Open`/`os.ReadFile` — expected for a local CLI and not attacker-reachable in the standard threat model. But `rpi_serve.go` serves HTTP; any path that originates from a *request* rather than operator config deserves an explicit join-and-contain check. Notably, the one place that *does* face hostile input — tar extraction in `corpus_snapshot.go:extractSnapshot` — already has a traversal guard, a byte-budget cap, and skips symlink entries. That's the pattern to hold the other 34 sites to.
- **Fix:** Triage the 34 sites once; for any HTTP-or-archive-reachable path, apply the `filepath.Clean` + prefix-containment idiom already used in `extractSnapshot`; annotate the rest.

### L6 — Shared-checkout working-tree hygiene drift

- **Location:** `~/dev/agentops` (detached HEAD on the *serving* checkout that `~/.claude/skills` symlinks into); untracked `wt-ag-qidx/` worktree dir **inside** the shared checkout; untracked `evidence/cp-16vb.impl.md`, `evidence/skill-corpus-token-audit.md`, `evidence/skill-prune-recon.md`; modified `.gitignore`; 4+ prunable worktrees under `/private/tmp` (`ao-9zls`, `ao-gofmt-gate`, `ao-isolation-wave`, …)
- **Issue:** The repo's own multi-agent discipline says foreign uncommitted files are quarantined and the skills SSOT checkout should sit on `main` — a detached-HEAD serving checkout means skill edits/serves may not reflect main.
- **Fix:** `git worktree prune`; re-home `wt-ag-qidx/` outside the checkout (per the `bd worktree create` convention); attach the `evidence/` strays to their beads; return the serving checkout to `main`.

### L7 — `tests/` SC2034 unused variables and SC2155 masked return values

- **Location:** 10 sites across the same test files as L1 (e.g. `AUDIT_PATH` unused)
- **Issue:** Dead test scaffolding and `local x=$(cmd)` patterns that swallow exit codes — in tests, swallowed exit codes can convert a failing assertion into a silent pass.
- **Fix:** Bundle with L1's mechanical sweep; check each SC2155 site for an assertion that depends on the masked exit code.

---

## What's Healthy (verified, not assumed)

| Check | Result |
|---|---|
| `go build ./...` / `go vet ./...` | clean (648 files / 145K LOC) |
| `go test ./...` | **12,155 pass, 94 packages, 0 fail** |
| `golangci-lint` (errcheck, gocyclo@25, staticcheck, misspell, copyloopvar, usestdlibvars) | 0 issues — complexity budget actually holds |
| `gitleaks detect` (full history) | 0 leaks in 4,617 commits / 111 MB |
| `shellcheck -S error` on `scripts/` + `lib/` | 0 findings; warning-level clean outside `tests/` |
| Hardcoded secrets grep (sk-/ghp_/AKIA + key=value patterns) | none |
| CLI ergonomics | `--help` rich and agent-aware (`ao capabilities` JSON contract, `ao robot-docs`), `ao version` + `--json` valid, non-zero exits on bad flag/command, no ANSI escapes in piped help |
| Untrusted-input handling | tar extraction (`corpus_snapshot.go`) guards traversal, caps extract bytes, skips symlinks |
| Build artifact hygiene | `cli/ao` (23.9 MB) correctly gitignored, not tracked |
| TODO debt | 3 genuine TODO/FIXME lines in non-test Go |
| CI defense-in-depth | shellcheck, secret scan, dangerous-pattern scan, race detector, coverage floor, test-count ratchet all wired in `validate.yml` |

## Appendix — gosec HIGH triage detail (41 findings)

- **34× G703 path traversal:** operator-supplied local paths → see L5 (triage-and-annotate; pattern exists in-tree).
- **2× G702 command injection:** `skills_edit.go:346` fixed-argv `git -C`; `handoff.go:301` `tmux respawn-pane` running an intentional restart command. False positives → L4.
- **1× G704 SSRF:** `retrieval_search_backends.go:170` POST to operator-configured rerank endpoint with 5s timeout. False positive → L4.
- **4× G115 integer overflow:** rune→byte ASCII filter (`quality/doctor.go:189`), tar `hdr.Mode & 0o777` (`corpus_snapshot.go:388,398`), nanosecond→uint16 ID suffix (`inject_context_paths.go:44`). All benign → L4.

## Recommended next actions (per the skill's "report-only is an anti-pattern" rule)

File beads (operator action — this audit ran read-only, no tracker mutations):

1. `[ci] Decompose validate.yml monolith / continue Go-gate migration` — Medium (M1, rides ag-3n71)
2. `[security] gosec HIGH burn-down + suppression policy + new-HIGH ratchet` — Medium (M2, L4, L5)
3. `[perf] Hoist + memoize tokenSet/pathSet in beads audit-cluster` — Medium (M3)
4. `[chore] Re-home dead agentops-* bead refs in .golangci.yml; decide deferred linters` — Medium (M4)
5. `[chore] tests/ shellcheck sweep + 0644-policy helper + worktree hygiene` — Low (L1, L3, L6, L7)
