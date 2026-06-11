# Codebase Risk Audit — agentops (2026-06-11)

Produced by the `codebase-risk-audit` skill (skills/codebase-risk-audit/SKILL.md).
Audited checkout: `/Users/bo/dev/agentops` at `2bbc44e8e` (detached HEAD).

## Scope

Inspected:

- System map: `cli/` (Go `ao` CLI: `cli/cmd/ao` + 60+ `cli/internal/*` packages), `skills/` (167 skill dirs), `scripts/` (272 files), `.github/workflows/` (11 workflows), git hook wiring (`.git/hooks/pre-push`, `pre-push.local`), `.beads/` control-plane wiring, worktree topology.
- Commands run: `go build ./...`, `go vet ./...` (clean), `go test ./... -count=1` (**12,155 tests passed, 94 packages, exit 0**), `git worktree list`, `bd ping`, structure/size measurements, targeted greps for shell-out call sites, env-var surface, secret patterns, unpinned actions, `rm -rf` patterns, fail-open paths in the gate.
- Doctrine cross-check: CLAUDE.md workflow claims vs on-disk reality (referenced files, provenance ledger, skill SSOT).

Intentionally skipped (time-box): full read of `validate.yml` (103.7K, ~15 jobs — job list sampled), `nightly.yml`/`release.yml` deep review, skill content semantics, evals, dependency CVE scan, `golangci-lint`/gosec runs (only `go vet`), Dolt server-side config on bushido, runtime behavior of the `@claude` GitHub action's permission gate.

## Executive Risk Summary

| # | Pri | Risk |
|---|-----|------|
| 1 | P1 | The bead/Dolt control plane is a remote single-host SPOF and was **down at audit time** while doctrine requires a bead for every change — delivery blocks when bushido is unreachable. |
| 2 | P1 | The fleet-wide live skill SSOT checkout is on a **detached HEAD, diverged from main** (1 ahead / 3 behind) — Claude/Codex/Gemini on this host are silently serving non-main skills, contradicting the "on-main checkout serves it" contract. |
| 3 | P1 | With branch protection OFF, the entire pre-merge wall is one 2,210-line bash script with multiple fail-open edges (missing hook, beads timeout→continue, un-audited per-check skip env vars, `--no-verify`); the Go replacement is partial and opt-in. |
| 4 | P2 | `cli/cmd/ao` is a 620-file / 9.2MB single `main` package with documented symbol coupling that already blocks deletions — change risk concentrates in one namespace. |
| 5 | P2 | ~140 registered worktrees, many in volatile `/private/tmp` and several nested inside the repo root with hand-edited `.gitignore` entries; one nested worktree (`wt-ag-qidx/`) is untracked and **not** ignored. |
| 6 | P2 | `docs/provenance/ledger.jsonl` — declared in CLAUDE.md as the provenance source of truth ("ledger wins on disagreement") — **does not exist**. |
| 7 | P2 | Supply chain: `curl \| bash` install path with zero checksum/signature verification; third-party GitHub Actions tag-pinned (not SHA-pinned), one floating `@nightly`. |
| 8 | P3 | Ambient-config sprawl: 50 distinct `AGENTOPS_*` env vars in Go + 74 in scripts steer critical flows (gates, skips, runtimes) with no single registry. |
| 9 | P3 | `@claude` workflow grants `contents: write` on a comment trigger; safety depends on the action's built-in actor-permission gate (inference, not verified). |
| 10 | P3 | `canon.CommandVerifier` executes an env-configured string via `sh -c` — acceptable operator trust boundary, but undocumented as arbitrary-shell. |

Counterweight (what is demonstrably healthy): full test suite green (12,155 tests / 94 packages), `go vet` clean, tiny TODO density (3 in non-test Go), no hardcoded secrets found by pattern scan, a lean dependency tree (8 direct deps, all mainstream), and the gate's primary bypass (`AGENTOPS_GATE_DISABLED=1`) is deliberately audited.

## Findings

### F1 — Delivery is coupled to a remote single-host control plane that is down right now (P1, Operations)

- **Evidence:** `bd ping` at audit time: `[circuit-breaker] 100.105.194.61:.../bushido: open → open (active probe failed)` → `Error: failed to open database: dolt circuit breaker is open: server appears down`. `.beads/metadata.json` pins `"dolt_server_host": "100.105.194.61"` (bushido WSL tailnet node). CLAUDE.md workflow: "Every change to `main` cites a bead… `bd ready` → pick a bead → `bd update <id> --claim`. **No bead, no PR.**" Stale local state also accretes: `.beads/` carries `dolt.stale-pre-bushido-sync-20260429T222530Z/`, `dolt.local-cache-bak/`, `backup/`, `backup-sync/`.
- **Impact:** When bushido, its WSL Dolt server, or the tailnet path is down (this has happened before — the 2026-06-05 crash — and is happening now), claim/close/triage on this repo's tracker fails fast. Agents either stall or work bead-less, breaking the provenance contract either way.
- **Likelihood:** High — observed live during the audit; Wi-Fi-underlay tailnet to a single consumer box.
- **Remediation:** Smallest credible step: document and test a sanctioned offline lane (queue claims locally, reconcile on reconnect) so an open circuit breaker degrades instead of blocks. Real fix is the already-tracked HA control-plane work (ag-o2tc) and/or finishing the migration of tracking to the git-native control-plane (`br`/`cp-`) which has no server dependency. Also GC the stale `.beads/*bak*` dirs.
- **Owner boundary:** control-plane / fleet infra.

### F2 — The live skill SSOT is served from a detached, diverged commit (P1, Operations)

- **Evidence:** `git rev-parse HEAD` = `2bbc44e8e`, `HEAD detached`; `git log --oneline main..HEAD | wc -l` = 1; `HEAD..main` = 3 (HEAD is 1 ahead / 3 behind main). `~/.claude/skills/<name>` entries are symlinks into this checkout (verified: `~/.claude/skills/agent-mail -> /Users/bo/dev/agentops/skills/agent-mail`). Both CLAUDE.md files declare: "Deploy = edit canonical → merge to main → **the on-main checkout serves it**."
- **Impact:** Every Claude/Codex/Gemini session on this host is consuming skills from a commit that is neither main nor any branch. Skill fixes merged to main (3 commits) are not live; 1 unmerged commit *is* live. The drift is silent — nothing asserts the serving checkout's ref.
- **Likelihood:** High — it is the current state, and detached-HEAD operation is routine in this multi-worktree workflow, so recurrence is structural.
- **Remediation:** Add a mechanical assertion (doctor check, cron tick, or shell-prompt check) that the checkout backing `~/.claude/skills` is on `main` and not behind `origin/main`; alert/auto-fast-forward on violation. One-line immediate fix: `git checkout main && git pull` once in-flight work is dispositioned.
- **Owner boundary:** fleet-ops / dotfiles `link-skill` discipline.

### F3 — The pre-merge wall is a bash monolith with fail-open edges, and nothing behind it reviews (P1, Architecture/Delivery)

- **Evidence:**
  - Branch protection is OFF; push-to-main is the model (CLAUDE.md, ag-qidx). The wall is `scripts/pre-push-gate.sh`: **2,210 lines**, ~25 checks, single bash file (`set -euo pipefail`).
  - Fail-open edges observed in the wiring: (a) `.git/hooks/pre-push` runs the gate only `if [ -x pre-push.local ]` — a clone without `scripts/install-pre-push-gate.sh` run pushes ungated, silently; (b) the beads hook section converts timeout (exit 124/142) to `_bd_exit=0` — "continuing without beads"; (c) per-check `AGENTOPS_PREPUSH_SKIP_<NAME>=1` flags skip individual checks **without the audit logging** that the global `AGENTOPS_GATE_DISABLED=1` bypass gets (pre-push.local: "the ONLY bypass is the audited AGENTOPS_GATE_DISABLED=1" — not true given the skip flags and `git push --no-verify`); (d) checks #9/#10 are permanently "skipped — manually maintained" (script header, lines 19–20).
  - The Go-native replacement (`ao gate check`) is opt-in via `AGENTOPS_GATE_GO=1` at ~12/79 check parity (CLAUDE.md / ag-3n71); the bash script remains the default.
  - `validate.yml` (103.7K, ~15 jobs) is a **post-push** backstop on main; doctrine on red main is "fix forward".
- **Impact:** Broken or unreviewed work lands on main whenever the gate is absent, skipped, or bypassed — and because main is also the live skill SSOT (F2) and the install source (`curl … /main/scripts/install.sh`), a red main propagates to consumers, not just CI. The 2,210-line bash gate itself is hard to test and easy to regress (its own correctness is verified mostly by shellcheck and use).
- **Likelihood:** Medium-high — multi-host pushers (Mac, bushido, codex worktrees) each need correct hook installation; env-var skips travel invisibly in agent environments.
- **Remediation:** Smallest steps first: log skip-flag usage the same way `AGENTOPS_GATE_DISABLED` is logged; add a cheap CI assertion that pushes carry a gate receipt (e.g., commit trailer or note written by the gate) so ungated pushes are *detected* post-hoc; add hook-installed check to `ao doctor`. Continue the ag-3n71 Go-gate migration rather than growing the bash file.
- **Owner boundary:** gate/CI (ag-3n71 epic).

### F4 — `cli/cmd/ao` is a 620-file single `main` package (P2, Maintainability)

- **Evidence:** `find cli/cmd/ao -maxdepth 1 -name '*.go' | wc -l` = 620 (271 source + 349 test), 9.2MB. Largest test files exceed 3,400 lines (`rpi_loop_test.go` 3,421). CLAUDE.md itself documents the consequence: the legacy RPI lane cannot be deleted because symbols are cross-referenced from "13+ `rpi_phased*` files + `mine`… deleting any breaks the build; full removal needs a caller-migration refactor (soc-1gbpz)".
- **Impact:** One namespace couples every command: any symbol collision, dead-code removal, or refactor risks the whole binary; new contributors (and agents) cannot reason about ownership boundaries inside `package main`. The repo's hexagonal/BC architecture exists in `cli/internal/` (60+ packages) but the command layer concentrates risk.
- **Likelihood:** Medium — already manifesting (the documented can't-delete state), and the package grows with every feature.
- **Remediation:** Keep doing what the in-flight `cp-cd9-*-adapter` worktree slices show: extract cohesive command groups behind `cli/internal/` adapters. Track package-file-count as a ratchet so the number only goes down.
- **Owner boundary:** CLI (soc-1gbpz / cp-cd9).

### F5 — Worktree sprawl: volatile paths, prunable records, and unignored nested worktrees (P2, Operations/Repo hygiene)

- **Evidence:** `git worktree list` returns ~140 entries, including: 15+ under `/private/tmp/*` (volatile — cleared on reboot; several already marked `prunable`), ~12 under `~/.codex/worktrees/` mostly detached, replay worktrees under `~/.agents/research/…2026-05-02…` (5 weeks old), and three worktrees **nested inside the repo root** (`wt-ag-if7p/`, `wt-ag-pj51/`, `wt-ag-qidx/`). `.gitignore` carries an uncommitted hand-edit adding `wt-ag-pj51/` and `wt-ag-if7p/` individually; `wt-ag-qidx/` is untracked and **not ignored** (visible in `git status`).
- **Impact:** (a) An agent or bulk `git add` in the shared checkout can commit an entire nested worktree into main; (b) `/tmp` worktree loss leaves stale admin records and orphaned branches; (c) the per-name `.gitignore` editing pattern guarantees recurring near-misses.
- **Likelihood:** Medium — the unignored nested worktree exists right now in a repo where automated agents run `git add`.
- **Remediation:** Replace per-name ignores with a single `wt-*/` pattern (commit it); run `git worktree prune` and a sweep that removes worktrees for merged/dead branches; adopt a rule (or `bd worktree create` default) that worktrees live outside the repo root and outside `/tmp` for anything that must survive a reboot.
- **Owner boundary:** repo hygiene / `bd worktree` tooling.

### F6 — Declared provenance source of truth does not exist (P2, Maintainability/Doc-reality drift)

- **Evidence:** CLAUDE.md → Provenance: "Source of truth: append-only JSONL at `docs/provenance/ledger.jsonl` (schema `agentops-sdlc-provenance.v1`)… ledger wins on disagreement." On disk: `ls docs/provenance/` → "No such file or directory"; `find . -maxdepth 3 -name ledger.jsonl` → nothing. The repo's own router doctrine states: "Missing canonical context files are defects."
- **Impact:** Agents told to trust the ledger over `bd` metadata will either fail or silently fall back to the projection CLAUDE.md says to distrust. Every provenance-citing workflow step is unverifiable.
- **Likelihood:** High that the contradiction is hit — CLAUDE.md is auto-loaded into every session.
- **Remediation:** Decide which is true: restore/start the ledger (plus the writer that appends to it), or amend CLAUDE.md's Provenance section to the real surface. One-file fix either way.
- **Owner boundary:** docs/doctrine.
- **Related (same lens, smaller):** installed skills (275 entries in `~/.claude/skills`) vs repo skills (167) — the delta is unattributed in this audit (other sources legitimately exist) but there is no mechanical reconciliation of "what is installed vs what the SSOT serves". Existing `evidence/skill-prune-recon.md` suggests this is already being worked.

### F7 — Unverified install path and tag-pinned third-party actions (P2, Security-adjacent / supply chain)

- **Evidence:** README.md:69,74 instruct `curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash` (and install-agy.sh); `scripts/install.sh` contains zero occurrences of `sha256|checksum|gpg|verify`. Workflows pin actions by tag, not SHA: `actions/checkout@v6` (×27), `dorny/paths-filter@v4`, `codecov/codecov-action@v7`, `goreleaser/goreleaser-action@v7`, `DavidAnson/markdownlint-cli2-action@v23`, and a floating `dtolnay/rust-toolchain@nightly`.
- **Impact:** Consumers execute whatever is on `main` at fetch time — and per F3, main has no pre-merge review and a bypassable gate. A compromised or hijacked action tag executes in CI with repo token. Blast radius is consumer machines + release pipeline.
- **Likelihood:** Low per-event, but the standard mitigations are cheap and the repo publicly advertises the curl|bash path.
- **Remediation:** SHA-pin the non-`actions/*` third-party actions (5 lines); have goreleaser publish checksums and point install.sh at release artifacts (not raw main) with a checksum verify; at minimum state the trust model in README.
- **Owner boundary:** release/CI.

### F8 — Ambient env-var configuration steers critical flows without a registry (P3, Architecture)

- **Evidence:** 50 distinct `AGENTOPS_*` variables referenced in `cli/**/*.go` (non-test grep) and 74 in `scripts/` — gate skips (`AGENTOPS_PREPUSH_SKIP_*`), gate disable, Go-gate opt-in (`AGENTOPS_GATE_GO`), hooks disable (`AGENTOPS_HOOKS_DISABLED`), canon verifier command (`AGENTOPS_CANON_VERIFIER_CMD`), etc.
- **Impact:** Hidden ordering/ambient process configuration (a named skill-lens risk): two agents in the same repo can get different gate behavior from inherited shells, and there is no one place to see what a variable does or that it exists.
- **Likelihood:** Medium — multi-agent tmux environments inherit and export liberally.
- **Remediation:** Generate an env-var registry doc from grep (script it, gate drift in CI like the other generated contracts), and have `ao doctor` print non-default AGENTOPS_* vars in effect.
- **Owner boundary:** CLI/docs generation.

### F9 — `@claude` comment trigger runs with `contents: write` (P3, Security-adjacent — inference)

- **Evidence:** `.github/workflows/claude.yml` triggers on `issue_comment`/`pull_request_review_comment` containing `@claude`, with `permissions: contents: write, pull-requests: write, issues: write`; uses `anthropics/claude-code-action@v1` with `CLAUDE_CODE_OAUTH_TOKEN`.
- **Impact:** If the action did not enforce actor write-permission, any commenter on the public repo could drive a write-capable agent. The official action does gate on actor permissions by default (**inference — not verified in this audit**); timeout is bounded (30m).
- **Likelihood:** Low (assuming the action's default gate holds).
- **Remediation:** Verify the action's permission-gate behavior once and record it; consider an explicit `if: github.event.comment.author_association` guard in the workflow so the protection is visible in-repo rather than delegated.
- **Owner boundary:** CI.

### F10 — `canon.CommandVerifier` executes an env-configured string via `sh -c` (P3, Security-adjacent)

- **Evidence:** `cli/internal/canon/verifier.go:57` — `exec.Command("sh", "-c", cv.Command)` where `Command` comes from `AGENTOPS_CANON_VERIFIER_CMD`. This is the only `sh -c` shell-out in non-test CLI Go (171 total `exec.Command` lines, the rest argv-style).
- **Impact:** Whoever controls the env of an `ao` invocation controls a shell execution. For a local dev tool run by the operator this is an accepted boundary, but it is undocumented as such, and agents export env freely.
- **Likelihood:** Low.
- **Remediation:** One doc line ("this var is arbitrary shell; treat like PATH") and/or log the command being run at invocation.
- **Owner boundary:** CLI (canon).

## Remediation Plan

**Immediate (hours, this week):**

1. Re-point the serving checkout to main once in-flight work is dispositioned; add the on-main assertion to `ao doctor` or a cron tick (F2).
2. Commit a `wt-*/` ignore pattern; `git worktree prune`; sweep `/tmp` and prunable worktrees (F5).
3. Resolve the provenance-ledger contradiction in CLAUDE.md — restore the ledger or fix the doc (F6).
4. Log `AGENTOPS_PREPUSH_SKIP_*` usage like `AGENTOPS_GATE_DISABLED` (F3).
5. SHA-pin the five third-party actions; replace `@nightly` with a pinned toolchain (F7).

**Near-term (1–3 weeks):**

6. Sanctioned offline/degraded lane for bd when the Dolt circuit breaker is open; GC stale `.beads/` backups (F1).
7. Gate-receipt assertion in CI so ungated pushes to main are detected (F3).
8. Continue ag-3n71: move the highest-value bash gate checks to `ao gate` (F3).
9. Generated `AGENTOPS_*` env-var registry + drift gate (F8); document `AGENTOPS_CANON_VERIFIER_CMD` trust (F10).
10. Verify and make explicit the `@claude` actor-permission gate (F9).

**Later (architectural):**

11. HA control plane or full migration of this repo's tracking to the git-native `br` lane (F1; ag-o2tc decision).
12. `cli/cmd/ao` decomposition via adapter extraction with a file-count ratchet (F4; soc-1gbpz / cp-cd9).
13. Checksummed, release-artifact-based install path replacing raw-main curl|bash (F7).

## Validation Gaps

- No `golangci-lint`/gosec/CVE scan was run (only `go vet` + pattern greps); the lint config exists (`cli/.golangci.yml`) but was not executed.
- `validate.yml` (103.7K) was sampled (job inventory), not read — unknown whether its 15 jobs duplicate, contradict, or fully cover the local gate's 25 checks; the 79-check parity number is taken from doctrine, not re-derived.
- The `anthropics/claude-code-action` permission gate (F9) is an inference from upstream defaults, not verified.
- Skill *content* quality, evals, and `nightly.yml` autonomous-evolution behavior were out of scope.
- Bushido-side state (Dolt server health cause, its checkout/hook installation) was not inspected — the SPOF finding is from the Mac side only.

## Residual Risk

- **Push-to-main is a deliberate trade** (ag-qidx): even with every F3 fix, no human or second-agent reviews changes before they hit the branch that simultaneously serves live skills and the public install path. The structural mitigation is the gate-receipt + post-push backstop, not review; accept or revisit explicitly.
- **Single-operator fleet coupling:** Mac (orchestration) + bushido (compute/control-plane) over a consumer Wi-Fi tailnet remains a correlated failure domain until the HA decision (ag-o2tc) is made — no local fix removes it.
- **Bash gate longevity:** until ao-gate parity is real, every gate improvement deepens investment in a 2,210-line script the repo has already decided to replace.
