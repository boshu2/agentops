# Codebase Audit Report: AgentOps

- **Skill:** codebase-audit (domain-parameterized; per the skill's "one domain, deep" rule the primary lens is **security**, with focused secondary passes on **cli**, **performance**, and **copy/docs** — the domains that match this repo's shape: a 403k-LOC Go CLI, 345 shell scripts, 62 skills)
- **Repo:** `/Users/bo/dev/agentops` @ `2c2bfc3fb` (main, 2026-07-09)
- **Method:** domain checklists + quick-grep patterns from the skill's references, plus live tooling: `go vet ./...` (clean), `govulncheck ./...` (2 hits), `shellcheck` across all `scripts/*.sh` (0 errors, 1 warning), live CLI behavior probes against the installed `ao` 3.2.0-rc.
- **Read-only audit:** no source changes, no tracker mutations. Bead creation for the criticals is recommended below but deliberately left to the orchestrator (this worker is under a no-tracker-writes constraint).

## Summary

- **Total:** 13 findings
- **Critical:** 0 | **High:** 1 | **Medium:** 3 | **Low:** 9

Overall posture: this is one of the most defensively hardened codebases this skill has been run against. Prior audit rounds are visible in the code itself (trust-boundary comments citing "recon 2026-06-24 audit M-1", "recon-2026-07-02 audit A2", fail-closed tar extraction with realpath containment, 49 justified `#nosec` annotations, structural tests pinning trust boundaries). The remaining findings are mostly at the edges the in-repo gates don't cover: the **Go toolchain itself** (two stdlib CVEs, one of which directly weakens the repo's `os.Root` containment primitive), **CI tooling supply chain** (unpinned `@latest` security scanners, no `govulncheck` job), and **contract/doc drift** (the machine-readable CLI contract documents 3 env vars while the binary honors ~77).

---

## High Findings

### H-1. Stdlib `os.Root` escape (GO-2026-4970) in the toolchain the release is built with — os.Root is this codebase's containment primitive

- **Location:** `cli/go.mod:3-4` (`go 1.26` / `toolchain go1.26.3`; local scan resolved `os@go1.26.4`); reached from `cli/internal/adapters/vendorimage/codexruntime/runtime.go:231` (`openCodexFile` → `os.Root.Open`), `cli/internal/goalstrace/artifacts.go:58` (`os.Root.ReadFile`), `cli/internal/lifecycle/seed.go:36`, `cli/internal/adapters/agentsreferences/references.go:27`
- **Issue:** `govulncheck` confirms the built binary is affected by **GO-2026-4970 — "Root escape via symlink plus trailing slash in os"**, fixed in `os@go1.26.5`. This is not an abstract dep bump: `os.Root` is used here precisely as a sandbox boundary (opening vendor/runtime files, RPI artifacts, seed templates, and walking `.agents` references *rooted* to a directory). A symlink planted inside a rooted tree — and `.agents/` plus vendored runtime dirs are exactly the kind of tree that agents and third-party tools write into — can escape the root under the vulnerable stdlib.
- **Root Cause:** Toolchain pinned below the fixed patch release; no `govulncheck` in CI to surface stdlib CVEs when they land (see M-2).
- **Fix:** Bump `toolchain go1.26.5` in `cli/go.mod`, rebuild, re-release binaries (homebrew tap + goreleaser artifacts inherit the fix only after re-tagging). Verify with `govulncheck ./...` going clean.

## Medium Findings

### M-1. Second stdlib CVE: crypto/tls ECH privacy leak (GO-2026-5856)

- **Location:** same toolchain pin; example traces `cli/internal/llm/ollama_client.go:247` (`http.Client.Post`), `cli/internal/quality/doctor.go:279`, `cli/internal/search/index.go:156`
- **Issue:** `crypto/tls@go1.26.4` leaks the true SNI when Encrypted Client Hello is used; fixed in go1.26.5. Practical impact is low today (the LLM client defaults to `http://localhost:11434`, `cli/internal/llm/ollama_client.go:285`), but the TLS paths are reachable and the fix rides the same toolchain bump as H-1.
- **Fix:** Same as H-1 — one toolchain bump clears both.

### M-2. No `govulncheck` in CI — the scanner suite has a stdlib/deps blind spot

- **Location:** `.github/workflows/validate.yml:813-815`, `.github/workflows/nightly.yml:127-130` (installs semgrep, gosec, gitleaks — no govulncheck); the only `govulncheck` reference in the repo is an eval fixture (`evals/agentops-core/optimization-dependency-governance.json`)
- **Issue:** CI runs gosec (own-code static analysis), gitleaks (secrets), and semgrep (patterns) — but nothing checks the module graph or the Go standard library against the vulnerability database. That is exactly why H-1/M-1 were sitting undetected in a repo that otherwise gates aggressively.
- **Root Cause:** Scanner suite was assembled around own-code risk classes; known-CVE reachability is a different class no current tool covers.
- **Fix:** Add a `govulncheck ./...` step (nightly at minimum; validate.yml ideally) and pin its version (see M-3).

### M-3. CI security tooling installed unpinned at `@latest` / bare pip

- **Location:** `.github/workflows/validate.yml:813-815` (`pip install semgrep ruff radon`, `go install .../gosec/v2/cmd/gosec@latest`, `go install .../gitleaks/v8@latest`); same pattern `.github/workflows/nightly.yml:127-130`
- **Issue:** The tools that *gate* the release are themselves fetched at whatever version upstream published that morning. A breaking release changes gate behavior silently (flaky red or, worse, silent green); a compromised release is arbitrary code execution inside the CI job with the repo checked out. The repo's own doctrine ("pin external contracts with ground truth") is applied to bd/br/atm version floors but not to the CI scanners.
- **Fix:** Pin exact versions (`gosec@v2.x.y`, `gitleaks@v8.x.y`, `semgrep==x.y.z`) and bump deliberately — the `library-updater` lane already exists for this.

## Low Findings

### L-1. `ao capabilities` contract documents 3 env vars; the binary honors ~77

- **Location:** `ao capabilities` → `env_vars` = `{AGENTOPS_CONFIG, AO_DOCTOR_LOG_LEVEL, NO_COLOR}`; meanwhile `cli/cmd/ao/**` alone has ~77 distinct `os.Getenv` names (`AGENTOPS_CANON_VERIFIER_CMD` — executed via `sh -c`; `AGENTOPS_COMPILE_RUNTIME`, `AGENTOPS_RPI_RUNTIME`, `AGENTOPS_AUTO_PRUNE`, `PAWL_NO_SERVICE`, session/vendor detection vars, …)
- **Issue:** The help text tells agents "run `ao capabilities` first — machine-readable CLI contract," but the contract under-documents the surface that actually changes behavior. Cross-checked against the skill's CLI checklist ("stdout=data" ✓, exit codes ✓, help ✓) this is the one discoverability gap that matters for the primary consumer (agents).
- **Fix:** Generate the env-var table into the capabilities payload (the repo already generates `cli/docs/COMMANDS.md` from source; same regen lane can enumerate `os.Getenv` sites or a registered-var table).

### L-2. Hardcoded fallback version string is stale: binaries report `3.2.0-rc` after v3.2.0 shipped

- **Location:** `cli/cmd/ao/main.go:9` (`var version = "3.2.0-rc"`); observed live: both `/Users/bo/.local/bin/ao --version` and `cli/bin/ao --version` → `ao version 3.2.0-rc`; latest tag is `v3.2.0`
- **Issue:** `make build` injects `git describe` via ldflags (`cli/Makefile:6-7`), but the documented verify command (`cd cli && go build ./...`) and any plain `go build` produce binaries claiming an RC of a release that already shipped. Support/diagnostic ambiguity — and this repo already has a recorded "stale ao binary" footgun class.
- **Fix:** Bump the fallback at tag time (release checklist line), or derive the fallback from `debug.ReadBuildInfo()` VCS data so plain builds self-describe.

### L-3. `eval "$gate_cmd"` in the pre-push pawl check — operator-trust env executed without the trust-boundary comment its siblings carry

- **Location:** `scripts/check-pawl-pre-push.sh:194,213` (`gate_cmd="${AGENTOPS_PREPUSH_GATE_CMD:-}"` … `cd "$tmpwt" && eval "$gate_cmd"`)
- **Issue:** Same trust class as `AGENTOPS_CANON_VERIFIER_CMD` (env var = operator trust, fine), but the canon verifier documents the boundary in code and pins it with a structural test (`cli/internal/canon/verifier.go:45-49`), while this `eval` has only "Tests inject…" as rationale. A future refactor could source `gate_cmd` from somewhere less trusted without tripping anything.
- **Fix:** Add the same TRUST BOUNDARY comment convention; optionally assert the var is ignored when the script detects a non-interactive untrusted context.

### L-4. Installers default to the moving `main` ref

- **Location:** `README.md:25-33`, `scripts/install-codex.sh:5-6`, `scripts/install-claude.sh` (`INSTALL_REF` defaults empty = marketplace default branch)
- **Issue:** `curl | bash` from `raw.githubusercontent.com/.../main/...` is TOFU on a moving ref. `--ref v3.2.0` pinning exists but is opt-in. Accepted-risk pattern for this distribution model; noting for completeness (no checksum/signature verification lane exists for script installs; brew + goreleaser artifacts are the verified path).
- **Fix (optional):** Make the README's copy-paste line pin the latest release tag by default.

### L-5. Doc/gate contradiction on tracked `.agents/`: the doc says "expect no output," the gate maintains a 13-file allowlist

- **Location:** `AGENTS-RUNTIME.md:26,30` ("run `git ls-files .agents` and expect no output") vs `scripts/check-no-tracked-agents.sh:22` (`ALLOWED_PATHS_REGEX` — nightly snapshots, `findings/registry.jsonl`, `rpi/next-work.jsonl`, reconcile decisions; 13 files currently tracked)
- **Issue:** An agent following AGENTS-RUNTIME.md's merge-resolution instruction verbatim would delete allowlisted audit-truth files and then be blocked (or worse, pass if the deletion looks like main's). Doc and executable gate disagree; per the repo's own precedence rule the gate wins, but the doc should say so.
- **Fix:** One sentence in AGENTS-RUNTIME.md: "expect only the audit-truth allowlist from check-no-tracked-agents.sh."

### L-6. Tracked nightly "audit truth" looks dormant

- **Location:** `.agents/nightly/2026-05-07/` (single day; last touch of the tracked set 2026-06-06)
- **Issue:** The allowlist's rationale is data that "compounds across nightly runs" — one snapshot from two months ago hasn't compounded. Either the nightly write-back lane is off or the allowlist entry outlived the experiment.
- **Fix:** Confirm the nightly lane's intent; drop the allowlist entries (and files) if the lane is retired.

### L-7. ADR-0013 describes a consumer that no longer exists, linked from the live docs index

- **Location:** `docs/documentation-index.md:72` ("…consumed by `ao rpi phased --domain`"); `ao rpi` was removed in 3.0 (`f61c5f0e7`)
- **Issue:** ADRs are historical records and most `ao rpi` references correctly say "removed" (checked `docs/first-value-path.md:176`, `docs/knowledge-flywheel.md:24`, `docs/MIGRATION.md`) — but the index line describes the removed command in the present tense as the manifest's consumer.
- **Fix:** Annotate the index line ("consumer removed in 3.0; manifest contract retained for X").

### L-8. ~19 of 90 non-test `exec.Command` sites run without a context/timeout

- **Location:** e.g. `cli/internal/doctor/engine.go:96` (`git rev-parse`), `cli/internal/doctor/fix_cliconfig.go:273,502`
- **Issue:** 71/90 subprocess sites use `CommandContext`; the remainder are mostly local `git`/self-invocations. Local git can still block indefinitely (index.lock contention under swarm load — a real condition in this repo — or a credential prompt), hanging `ao doctor` rather than failing.
- **Fix:** Sweep the stragglers onto `CommandContext` with a generous default; lowest priority of the set.

### L-9. `filepath.Walk` (23 sites) and one shellcheck warning

- **Location:** 23 non-test `filepath.Walk` uses (pre-`WalkDir` API: one extra `lstat` per entry — only matters on large trees like `.agents/` walks); `scripts/assert-no-actions.sh:112` (SC2124, array-to-string assignment — the sole shellcheck warning in 345 scripts, and a real semantic footgun if that arg parser ever sees multi-word args)
- **Fix:** Opportunistic `WalkDir` migration on hot walkers; one-line fix for SC2124.

---

## Domain scorecards (what was checked, not just what failed)

### Security — primary lens

| Checklist item | Verdict | Evidence |
|---|---|---|
| Injection via shell | **Strong** | No `fmt.Sprintf`-into-`sh -c` anywhere; the three shell lanes (trusted repo scripts via `repoScriptTrusted`, `SanitizedBashCommand` with `--noprofile --norc` + BASH_ENV/ENV stripped, operator-configured verifier commands) all carry documented trust boundaries; `cli/internal/safety/doc.go` states the doctrine |
| Path traversal | **Strong** (modulo H-1) | Snapshot tar extraction (`cli/cmd/ao/corpus_snapshot.go:387-443`) rejects absolute entries, `..`, realpath-escapes, and fails closed on non-Reg/Dir typeflags with a byte ceiling; `os.Root` used for runtime file access — which is why H-1 matters |
| Secrets | **Clean** | Pattern sweep found none; `.gitleaks.toml` present; gitleaks runs in CI |
| Dependency vulns | **Gap** | H-1/M-1/M-2: two stdlib CVEs live, no govulncheck lane; 10 modules with pending minor updates (testify 1.8→1.11 etc., all low-risk) |
| Error-message leakage | **OK** | Errors are actionable without leaking sensitive paths beyond the repo the operator owns |
| Suppression hygiene | **Strong** | 49 `#nosec` annotations, all with justifications; gosec+semgrep dual-annotation convention documented in `.claude/rules/go.md` |
| Shell script hygiene | **Exceptional** | shellcheck `-S error`: **0** across 345 scripts; `-S warning`: **1** (L-9) |

### CLI

| Checklist item | Verdict |
|---|---|
| `--help` / `--version` | ✓ comprehensive, grouped (7 command groups, 80 top-level commands); version works (but see L-2) |
| Exit codes | ✓ 0 success / 1 error (usage errors also 1, not 2 — cobra default; exit_codes are documented in the capabilities contract, so this is consistent-by-contract) |
| stdout=data, stderr=diagnostics | ✓ verified (`Error: unknown command…` on stderr only) |
| NO_COLOR | ✓ zero ANSI escapes emitted with or without it |
| Shell completion | ✓ `ao completion` |
| Machine contract | ✓ `ao capabilities` + `ao robot-docs`; env-var section under-documented (L-1) |
| Startup latency | ✓ excellent: `--version` 31ms, `capabilities` 17ms, `lookup --query` ~300ms |

### Performance (quick pass)

- Hot paths agents pay per-session are fast (above). The per-push cockpit gate (`ao gate check --fast --scope head`) measured **3m54s for 45 checks (45 pass, 67 gates not-run via routing)** on this audit's live timing run against HEAD — the dominant loop tax, paid on every push; routing already scopes it, so this is a measurement baseline, not a defect. If it creeps, the per-check timings in the gate report are the profiling surface.
- No async-in-loop, unbounded-query, or blocking-in-async classes apply (no server, no DB); no goroutine-leak surface of note (3 `go func()` sites in non-test code).
- `go vet ./...`: clean. Build tags (ADR-0012 archive lanes) verified present in Makefile.

### Copy/docs (quick pass)

- README: plain-language, claims match behavior (verdict-gated done, hookless install, no telemetry — consistent with code), install paths current. 6 TODO/FIXME across 403k LOC of Go. Findings L-5/L-6/L-7 are the drift residue.

---

## Continuity vs the 2026-07-01 audit (same skill, same repo)

The prior run's result record (`.agents/swarm/results/codebase-audit.json`, 07-01) was checked against today's tree:

- **07-01 M-1 (planted-script RCE gap in `bestEffortRefreshFindingCompiler` / `bestEffortPruneAgents`): FIXED.** Both now route through the trusted-script chokepoint with the `aoBinaryInside` trust boundary (`cli/cmd/ao/findings.go:380-384`, `cli/cmd/ao/session_end_maintenance_helpers.go:91-94`).
- **07-01 L-3 (tar extractor implicit absolute-path handling): FIXED.** Explicit `filepath.IsAbs` rejection + realpath containment now in `cli/cmd/ao/corpus_snapshot.go:388-405`.
- **07-01 "govulncheck no vulns" → today 2 stdlib CVEs.** Both CVEs published after 07-01. This is the strongest evidence for M-2: a one-off scan goes stale in a week; only a recurring CI lane holds the line.

## Recommended follow-ups (for the orchestrator to bead)

1. **P0 (H-1/M-1):** `toolchain go1.26.5` bump + rebuild + re-release; `govulncheck` clean as acceptance.
2. **P1 (M-2/M-3):** Add pinned `govulncheck` to CI; pin gosec/gitleaks/semgrep versions.
3. **P2 (L-1, L-2):** Env-var surface into the capabilities contract; version fallback discipline at tag time.
4. **P3 (L-3..L-9):** One-line doc reconciliations and hygiene sweeps; none block anything.

*Audit performed read-only per swarm constraints; no beads created, no files modified outside `docs/audits/codebase-recon-2026-07-09-claude/` and `.agents/swarm/results/`.*
