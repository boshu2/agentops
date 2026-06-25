# Codebase Audit Report: AgentOps (`ao` CLI + substrate)

> ⚠️ **HISTORICAL SNAPSHOT — recon run 2026-06-24 against `abc018c42`; `main` has since advanced past `882e71c01`.** A point-in-time audit, **not** a current-state reference. Some facts are already superseded — notably the **P1 atomic-write DRY finding was partially actioned** (`storage.AtomicWriteFile` is now canonical and quest/llmwiki/doctor/wiki delegate to it, age-3azc/uja6; inject, vendorimage/codexruntime, and `pool.atomicMove` still carry their own copies). The **`ao` command (89) and gate (98) counts are reproducible at the pinned `abc018c42`** (re-verified there 2026-06-25 — and still current on `main`; the draft's `~60`/`~95`/`87`/`~77` figures were simply wrong). `main` has since advanced, so narrative *findings* may be superseded (notably P1, partially actioned), but these architectural counts are stable; all other figures are as-of the 2026-06-24 snapshot. This was a READ-ONLY static review, not a full security scan or test run.

> **Skill:** `codebase-audit` (domain-parameterized: security, cli, performance, api, copy, shell).
> **Repo:** `/Users/bo/dev/agentops` @ `abc018c42` (branch `main`).
> **Date:** 2026-06-24. **Scope:** READ-ONLY recon. No source/docs/skill files were modified.
> **Method:** SKILL.md domain checklists + grep signal patterns + live build/test/CLI behavior probes.

## Surface profile

| Dimension | Measure |
|---|---|
| Go (`cli/`) | 377,583 LOC combined (~154k source + ~223k test), 606 `cmd/ao/*.go` files, 80+ `internal/` packages, 6 bounded contexts |
| Shell (`scripts/`, `bin/`, `lib/`) | 717 tracked `.sh` repo-wide; 303 top-level `scripts/*.sh` (311 recursive under `scripts/`) |
| Python (first-party) | 54 tracked `.py` (an untracked-inclusive `find` sees ~1,478, the rest vendored/generated under `.venv-docs/`, `.tmp/`, `.claude/worktrees/`) |
| JS | 154 (Claude workflow scripts + tooling) |
| Build | `go build ./...` ✅ clean; `go vet` ✅ clean on security-critical packages |
| Tests | security-critical **subset** ran live — `safety`, `gates`, `worktree`, `resolver`, `paths` packages all ✅ PASS (the **full** `go test ./...` suite was NOT run; see Limitations) |

This is a **mature, disciplined codebase** with strong engineering hygiene. The audit found **no Critical or High findings**. The headline is how *little* is wrong: the patterns this skill hunts for (hardcoded secrets, command injection, panics in library code, error swallowing, shell footguns) are almost entirely absent. Findings below are Medium/Low polish items and one doctrine-level note.

## Summary

- **Total:** 6 findings
- **Critical:** 0 | **High:** 0 | **Medium:** 3 | **Low:** 3

---

## Security domain — verdict: STRONG

Checklist run: secrets, injection (shell/SQL/code), path traversal, panics, dependency scanning, CI gates.

**What's good (evidence):**
- **No hardcoded secrets.** Grep for `(api_key|secret|password|token)=<16+ char literal>` and provider key shapes (`sk-…`, `ghp_…`, `AKIA…`) across `cli/` + `scripts/`: **zero real hits.**
- **No command-injection surface in production code.** 274 `exec.Command` calls across 109 files; only **one** non-test `sh -c` site (`cli/internal/canon/verifier.go:57`) and it runs an *operator-configured* verifier command (`AGENTOPS_CANON_VERIFIER_CMD`, e.g. `codex exec`) — a designed extension point, not user-tainted input. No `fmt.Sprintf` interpolation into a `-c` string anywhere.
- **No dangerous Python.** Zero `subprocess(..., shell=True)`; zero dynamic `eval()`/`exec()` on input across first-party Python.
- **Symlink-aware path containment.** `cli/internal/worktree/worktree.go` resolves with `filepath.EvalSymlinks` on both candidate and root before containment; `cli/internal/resolver/resolver.go` uses `filepath.Rel` for prefix checks — the correct pattern (matches the MEMORY note that path-containment was hardened across multiple security review rounds).
- **CI security gates wired.** `gosec`, `gitleaks`, `semgrep`, `trivy`, `govulncheck`-class tooling installed and run in `validate.yml`, `nightly.yml`, and `release.yml`. `.gitleaks.toml` present. Dual gosec+semgrep suppression discipline documented in `.claude/rules/go.md`.
- **Defensive panics only.** 5 `panic()` in non-test `internal/` code — all programmer-error invariants (duplicate registry IDs at init in `doctor/registry.go`, `gates/registry.go`; internal canonicalization failures in `drrebuild/drrebuild.go`). None are user-facing crash paths.

### M-1 (Medium): `sh -c` on operator-configured verifier command
- **Location:** `cli/internal/canon/verifier.go:57`
- **Issue:** `exec.Command("sh", "-c", cv.Command)` runs a string through a shell. The command is operator-set via env var, not end-user input, so this is **not** an injection vuln in the threat model. The residual risk is only if a future caller ever populates `cv.Command` from a less-trusted source (e.g. a corpus entry or remote config).
- **Root cause:** Shell indirection is needed because the configured value (`codex exec`) is a multi-token command line; `sh -c` is the simplest way to honor it.
- **Fix (defensive, optional):** Add a doc-comment invariant ("Command MUST come from operator config, never from corpus/remote data") right at the struct field, and a guard test asserting the constructor is only fed `AGENTOPS_CANON_VERIFIER_CMD`. No code change required today.

---

## CLI domain — verdict: STRONG

Live behavior probed against a freshly built `ao` binary.

**What's good (evidence):**
- `ao --help` is comprehensive (factory lane, flywheel, `ao capabilities` pointer for agents).
- `ao --version` → `ao version 3.1.0-rc` ✅.
- **Exit codes correct:** unknown flag → 1, unknown subcommand → 1, bare invocation → 0. (Matches the skill's 0=success / 1=error contract.)
- `NO_COLOR=1 ao --help` emits no ANSI escapes; 9 color-detection sites (`NO_COLOR`/`IsTerminal`) across the CLI.
- Machine-readable contract: `ao capabilities` emits valid JSON with `schema_version`, `exit_codes`, `env_vars`, `robot_surfaces`, `command_groups` — excellent agent-consumability (the "robot surface" the AGENTS.md doctrine promises).

No CLI findings. The CLI hygiene here is better than most production tools.

---

## Shell domain — verdict: STRONG

303 top-level `scripts/*.sh` audited.

**What's good (evidence):**
- **`shellcheck -S error` passes on ALL 303 top-level scripts** (0 failing / 303 checked), including the release-authority `pre-push-gate.sh`.
- Safety options ubiquitous: scripts that omit `set -e` deliberately use `set -uo pipefail` — the correct idiom for *validator* scripts that must collect all findings rather than abort on first failure (e.g. `check-goal-quality.sh`, `check-contracts-structural-floor.sh`, `audit-skill-metadata.sh`). This is intentional, not a defect.
- No dangerous `rm -rf $UNQUOTED` into unguarded variables. No real shell `eval` builtin abuse (one benign indirect-assignment in `seed-evolution-roadmap-beads.sh:47`, guarded with `|| true`).

### L-1 (Low): `curl … | bash` install pattern
- **Location:** `scripts/install-codex.sh`, `install-opencode.sh`, `install-agy.sh`, `install.sh` (documented invocation).
- **Issue:** The published install path is `curl -fsSL …/install-*.sh | bash`. This is the industry-standard convention and the repo is the trust root, so severity is Low — but it's worth a one-line note in install docs that pinning to a tag/ref (`| bash -s -- --ref vX.Y.Z`, already supported in `install-agy.sh`) is the safer default for cautious operators.
- **Fix:** Document the tag-pinned variant as the recommended path for security-conscious users.

### L-2 (Low): `eval "$2_ID='$id'"` indirect assignment
- **Location:** `scripts/seed-evolution-roadmap-beads.sh:47`
- **Issue:** Uses `eval` for dynamic variable naming. `$id` is internally generated (a bead id), so not tainted, but `eval` for indirection is fragile.
- **Fix:** Replace with a bash nameref (`declare -n`) or an associative array. Cosmetic; the `|| true` already prevents failure propagation.

---

## API / Contract domain — verdict: STRONG

- `ao capabilities` → valid JSON, fully populated contract (see CLI section). This is the API surface for agents, and it is consistent and self-describing.
- Generated projections (`registry.json`, `cli/docs/COMMANDS.md`, `docs/cli-surface.{json,md}`) are drift-gated via `make regen-check` — the contract cannot silently drift from the executable.

No API findings.

---

## Performance domain — verdict: NO HOT-PATH ISSUES FOUND

The skill's performance lens (N+1, blocking I/O in async, unbounded queries) is largely **N/A** to this codebase: there is no database query layer or web request hot path — the `ao` CLI is a batch/offline tool over the filesystem + git + occasional subprocess. Spot checks found no obvious O(n²) hot loops in the security-critical packages reviewed.

### M-2 (Medium, advisory): 6,371 `filepath.Join` calls — no perf issue, but a containment-surface reminder
- **Observation:** `filepath.Join` is used 6,371 times across `cli/`. This is normal for a filesystem-heavy CLI and is **not** a performance finding. The note is for *security maintenance*: any new path built from corpus/bead/remote-derived input must route through the same `EvalSymlinks` + `filepath.Rel` containment used in `worktree`/`resolver`, not a bare `Join`. Most `Join` calls operate on internally-derived paths and are fine.
- **Fix:** Consider a lint/grep gate (similar to `check-paths-resolver-coverage.sh`, which already exists) that flags new `filepath.Join` on externally-sourced path segments. Partly already covered.

---

## Copy domain — verdict: CLEAN

- No `lorem ipsum`, placeholder text, or stray `TBD` in `README.md`, `PRODUCT.md`, `GOALS.md`, `docs/newcomer-guide.md`.
- ~7 genuine `TODO`/`FIXME`/`HACK` markers in the entire 377K-LOC CLI (most grep hits were false matches on the literal words in `ParseVerdict`-style allowlists).

### L-3 (Low): error-discard discipline is good but high-volume
- **Observation:** 1,075 `_ = …` discard assignments across `cli/internal/` + `cli/cmd/`. Go idiom often requires discarding deferred `Close()`/`Write()` errors, so most are legitimate. Against 12,843 explicit `err != nil` checks, the ratio is healthy.
- **Fix:** No action required. If a future hardening pass wants to tighten this, `errcheck` with an exclude-list for known-safe defers would surface any genuinely-swallowed error. Low priority.

---

## What this audit did NOT cover (honest scope)

- **Deep dependency CVE scan** (`govulncheck`/`trivy` live run) — not executed locally; CI already runs these, so coverage exists, but a fresh local run was out of timebox.
- **Full `go vet`/`golangci-lint` across all 80+ packages** — only the security-critical subset was vetted live (clean). The push gate runs the full suite.
- **UX/accessibility domain** — N/A (no web UI; this is a CLI + library).
- **Per-package test coverage numbers** — only ran the security-critical package tests (all PASS).

## Bottom line

AgentOps practices what it preaches. For a 377K-LOC Go CLI plus 717 tracked shell scripts (303 canonical under `scripts/`), this **read-only static review surfaced no Critical/High findings**: no hardcoded secrets, no injection surface in production paths, symlink-aware path containment, defensive-only panics, all 303 top-level scripts passing `shellcheck -S error`, CI security gates (gosec/gitleaks/semgrep/trivy) wired across three workflows, and a clean machine-readable CLI contract. The six findings are polish (Medium/Low) — the most actionable being **M-1** (annotate the `canon` verifier `sh -c` trust invariant) and **L-1** (document the tag-pinned install variant). **Scope caveat (per the Limitations above):** a live deep dependency-CVE scan (`govulncheck`/`trivy`) and the full `go vet`/`golangci-lint` + full test suite were **not** run locally — CI covers them, but this review bounds *observed* static risk only, it does not certify release-readiness. Within that scope the security posture reads clean.

## Suggested follow-ups (if you want beads)

| Finding | Title | Priority |
|---|---|---|
| M-1 | `[security] Annotate canon CommandVerifier sh -c trust-boundary invariant + guard test` | P2 |
| M-2 | `[security] Extend path-resolver-coverage gate to flag filepath.Join on external input` | P2 |
| L-1 | `[docs] Recommend tag-pinned curl-install variant for security-conscious operators` | P3 |
| L-2 | `[shell] Replace eval indirect-assignment in seed-evolution-roadmap-beads.sh with nameref` | P3 |
