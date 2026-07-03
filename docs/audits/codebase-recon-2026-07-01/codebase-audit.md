# Codebase Audit Report: AgentOps

> Skill: `codebase-audit` (domain-parameterized). Run: 2026-07-01. Scope: full repo, read-only.
> Method: security · CLI · performance · copy · code-quality lenses, each with its own checklist
> (references/CHECKLISTS.md) plus deterministic tooling (build, vet, `make lint`, gitleaks,
> govulncheck, grep sweeps, live CLI probing). Binary built from source at `cli/` and exercised.

## Summary

- **Total:** 8 findings
- **Critical:** 0 | **High:** 0 | **Medium:** 3 | **Low:** 5

**Headline:** This is a mature, security-conscious, well-instrumented codebase. The high-value
attack surfaces the skill's checklists target are largely *already defended, deliberately, with
tests and inline rationale* (tar path-traversal guard, `sh -c` verifier trust boundary, the pawl
RCE guard, sanitized bash env, secret scanning). Zero secrets (gitleaks clean over 930 MB), zero
known-vuln dependencies (govulncheck clean), a clean `go build`/`go vet`, all 12 derived-artifact
drift gates green, and excellent CLI hygiene (correct exit codes, `NO_COLOR` respected, stderr
discipline, ~20-50 ms startup). The findings below are real but narrow: one trust-boundary
*asymmetry* (some cwd-relative hook scripts run without the guard the pawl path enforces), a
lint/complexity-budget drift against the repo's own stated policy, and a handful of polish items.

**Tooling evidence:**
| Check | Result |
|---|---|
| `go build ./...` | Success |
| `go vet ./...` | No issues |
| `gitleaks detect` (930 MB, no-git) | No leaks found |
| `govulncheck ./...` | No vulnerabilities found |
| `make regen-check` (12 drift gates) | ALL GATES GREEN |
| `make lint` (repo-pinned golangci-lint v2) | **13 issues** (see M-2) |
| shellcheck `-S error` (326 scripts) | 0 errors |
| CLI: exit codes / `NO_COLOR` / completion / stderr | All correct |

---

## Medium Findings

### M-1 — cwd-relative hook/maintenance scripts execute without the pawl trust guard
- **Location:** `cli/cmd/ao/findings.go:380` (`bestEffortRefreshFindingCompiler`), invoked at `findings.go:219` and `findings.go:248`; `cli/cmd/ao/session_end_maintenance_helpers.go:92` (`bestEffortPruneAgents`, runs `scripts/prune-agents.sh`) and `:155`.
- **Domain:** security (trust boundary / contextual RCE)
- **Issue:** These helpers build a path from the current working directory — `filepath.Join(cwd, "hooks", "finding-compiler.sh")` and `filepath.Join(cwd, "scripts", "prune-agents.sh")` — and, if the file exists, execute it via `exec.Command("bash", script, ...)` with `Stdout`/`Stderr` set to `nil` (silent). They fire from ordinary commands: `ao findings pull`, `ao findings retire`, and hookless session-end maintenance.
- **Root Cause:** The repo already recognizes exactly this RCE class and defends against it in `cli/cmd/ao/pawl.go:115-140`: an installed `ao` must only run a repo-planted script when the running binary *physically lives inside* the resolved checkout (`aoBinaryInside`), precisely "because a repo under review can FORGE ... scripts/pawl-review.sh, which would make an installed `ao pawl review` execute the repo's PLANTED script (RCE)." That guard is **not** applied to `bestEffortRefreshFindingCompiler` / `bestEffortPruneAgents`. An operator who runs an installed `ao findings pull` (or triggers session-end maintenance) while `cwd` is some *other* repo tree that happens to contain `hooks/finding-compiler.sh` or `scripts/prune-agents.sh` would silently execute that repo's script.
- **Severity rationale:** Medium, not High — it requires running `ao` with cwd inside a hostile tree that carries those exact relative paths, and PRODUCT.md explicitly parks adversarial-multi-tenant as out of scope. But it is a genuine inconsistency with the project's own documented threat model, and the silent (`nil` stderr) execution makes it invisible.
- **Fix:** Gate both `bestEffort*` script executions on the same `aoBinaryInside(repoRoot)` (or "cwd is the trusted checkout") test used in `pawl.go`; or resolve the script through the trusted-repo-root resolver rather than raw `os.Getwd()`. At minimum, stop discarding stderr so an unexpected execution is observable.

### M-2 — `make lint` is red against the repo's own complexity budget and error-check policy
- **Location:** `cli/cmd/ao/skills_retire.go:384` (`flipDispositionsLedger`, gocyclo 26), `cli/internal/aostate/state.go:309` (`AdmitFinding`, gocyclo 26), `cli/internal/yieldledger/gauge.go:387` (`ComputeGauges`, gocyclo 28); non-test errcheck at `cli/cmd/ao/skills_retire.go:133`; plus `internal/claimproof/check.go:243`, `internal/gates/checks/claim_registry.go:309`, `internal/yieldledger/loader.go:32` (`defer f.Close()` unchecked).
- **Domain:** code quality
- **Issue:** The repo's pinned linter (`make lint` → `scripts/golangci-lint-v2.sh`, config `cli/.golangci.yml`) reports **13 issues**: 3 × gocyclo, 4 × errcheck, 5 × staticcheck (test-only quickfixes), 1 × copyloopvar (test). `cli/.claude/rules/go.md` states the complexity budget is "fail at 25," yet three non-test functions sit at 26/26/28. `skills_retire.go:133` drops the error return of `emitSkillsRetireReport` on the already-erroring path.
- **Root Cause:** `make lint` is evidently not wired as a blocking pre-push gate (the release authority is `ao gate check` + drift gates, which are all green), so complexity/errcheck drift accumulates unenforced against a policy the repo documents as hard limits.
- **Fix:** Either enforce `make lint` in the gate (making the budget real) or split the three over-budget functions and check the four dropped errors. `flipDispositionsLedger` and `ComputeGauges` are natural extract-method candidates. The `defer f.Close()` sites can use the `err = errors.Join(err, f.Close())` pattern on writers.

### M-3 — Predictable, non-unique `/tmp` paths in shared gate/service scripts
- **Location:** `scripts/check-three-gap-supergate.sh:40` (`>/tmp/sg-"${label}".out`), `scripts/pawl.sh:990` (documented cron `>> /tmp/pawl-reap.log`). (For contrast, `scripts/regen-all.sh:51` and `scripts/safe-pull.sh:132` correctly use `$$`-suffixed temp files.)
- **Domain:** security (local; predictable temp file) / reliability
- **Issue:** `check-three-gap-supergate.sh` writes gate output to a fixed `/tmp/sg-<label>.out` with no PID/`mktemp` randomization. On a multi-user host this is a classic predictable-temp-file pattern (symlink pre-creation / clobber / info-leak between users) and, more practically, causes cross-run collisions if two gate runs share a label.
- **Root Cause:** Convenience temp paths that skipped the `mktemp`/`$$` convention the sibling scripts already follow.
- **Fix:** Use `mktemp` (or at minimum a `$$`-suffixed name in `${TMPDIR:-/tmp}`) for the supergate output, matching `regen-all.sh`.

---

## Low Findings

### L-1 — Root `ao --help` carries operator-jargon on a user-facing surface
- **Location:** `cli/cmd/ao/root.go:34,41` ("Intelligence compounds.", "The Knowledge Flywheel underneath it: ... You get smarter every session.").
- **Domain:** copy
- **Issue:** The top-level help leads with brand/operator vocabulary ("Knowledge Flywheel," "Intelligence compounds," "software-factory control plane") before it says plainly what the tool does. The README, by contrast, opens with a clean plain-language value line ("Coding agents declare 'done' on code that is still wrong. AgentOps catches that."). The operator's own retro notes flag keeping doctrine vocabulary off front-door surfaces.
- **Fix:** Lead `--help` with the README's plain line; keep the Flywheel framing to a single trailing sentence or `ao demo`.

### L-2 — `TODO`/`FIXME` markers remain in shipped CLI source
- **Location:** 2 files under `cli/cmd/ao/*.go` contain `TODO`/`FIXME` (copy-domain checklist item: "No placeholder text").
- **Domain:** copy / hygiene
- **Issue:** Low volume and likely tracked, but the copy checklist flags any placeholder/TODO in shipped code. Worth confirming each has a bead.
- **Fix:** Convert to `br` beads or resolve; keep shipped source marker-free.

### L-3 — Tar extraction relies on `filepath.Join` semantics for absolute-path containment (implicit, not asserted)
- **Location:** `cli/cmd/ao/corpus_snapshot.go:381-385`.
- **Domain:** security (defense-in-depth)
- **Issue:** The guard cleans `hdr.Name` and rejects `..` prefixes/segments, and only extracts `TypeDir`/`TypeReg` (symlink/hardlink entries are correctly ignored, which closes the symlink-escape class). An *absolute* `hdr.Name` (e.g. `/etc/passwd`) is contained only because `filepath.Join(parentDir, "/etc/passwd")` treats the second arg as relative — correct, but implicit. A future refactor that swaps `Join` for string concatenation would silently reintroduce an escape.
- **Fix:** Add an explicit `if filepath.IsAbs(clean) { reject }` and a post-`Join` `strings.HasPrefix(target, parentDir)` containment assertion, so the invariant is stated rather than inferred from `Join` behavior. This is hardening, not a live bug.

### L-4 — `#nosec G204` subprocess sites depend on caller discipline
- **Location:** 9 `exec.Command("bash"|"sh", ...)` sites (e.g. `pawl.go:88`, `wiki_publish.go:48`, `agentsdoctor/doctor.go:128`, `agentslint/lint.go:42`, `canon/verifier.go:63`).
- **Domain:** security (review consistency)
- **Issue:** Each is annotated and reasoned (fixed in-repo script + operator args; the `sh -c` verifier at `canon/verifier.go` is explicitly documented as operator-config-only with a test, `TestCommandVerifier_CommandSourcedFromOperatorConfig`). No injection found. The residual risk is that the safety of these sites is *contextual* (arg provenance), so a future caller passing a less-trusted arg wouldn't trip a compiler check.
- **Fix:** No change required today. When touching any of these, re-verify arg provenance; keep the provenance assertion in a test where feasible (canon does this well — replicate the pattern).

### L-5 — 47 `filepath.Walk`/`WalkDir` traversals without a documented depth/size bound
- **Location:** repo-wide (e.g. `skills_retire.go:653` `globRecursive`, corpus/context scanners).
- **Domain:** performance (scalability, low)
- **Issue:** Directory walks over `skills/`, `evals/`, `cli/` are unbounded in depth. At current repo scale this is fine (CLI startup measured ~20-50 ms; `capabilities` ~22 ms), but these scanners grow O(tree) and several run inside interactive commands.
- **Fix:** None needed now; if a scan ever shows up in a latency budget, add early-exit pruning (skip `.git`, `node_modules`, `.venv-docs`) rather than walking everything.

---

## What was checked and found clean (negative results are signal too)

- **Secrets:** `gitleaks detect` over 930 MB / no-git → **no leaks**. No hardcoded `sk-`/`ghp_` tokens in source.
- **Dependencies:** `govulncheck ./...` → **no known vulns**; `go.mod` is lean (cobra, jsonschema, yaml, go-cmp, rapid, goleak).
- **Injection:** no `format!(...SELECT...)`/string-built SQL; the one `sh -c` (`canon/verifier.go`) is operator-config trust-bounded and unit-tested.
- **Path traversal:** tar extractor rejects `..` and skips symlinks; corpus snapshot cleans and byte-caps (`maxSnapshotExtractBytes`).
- **Shell-env hardening:** `shellutil.SanitizedBashCommand` runs workers with `--noprofile --norc` and strips `BASH_ENV`/`ENV` — user aliases can't hijack worker checks. Good defensive design.
- **Build/vet/drift:** `go build`, `go vet`, and all 12 `regen-check` drift gates green; JSON schemas (62 files) all parse.
- **CLI ergonomics (CLI domain, clean sweep):** `--version` works; unknown command / bad flag both exit **1**; error text goes to **stderr** with stdout empty; `NO_COLOR=1 --help` emits **zero** ANSI escapes; `completion bash` works; machine-readable `capabilities`/`--json` contract present; startup latency excellent; binary 25 MB (normal for Go).
- **Shell scripts:** 326 scripts, **0** shellcheck errors at `-S error`.

---

## Recommended next actions (ranked)

1. **M-1** — apply the `pawl.go` trust guard to `bestEffortRefreshFindingCompiler` + `bestEffortPruneAgents`, and stop discarding their stderr. Small, closes a self-identified threat-model gap.
2. **M-2** — decide the lint policy: either wire `make lint` into the gate (making the documented gocyclo=25 / errcheck budget real) or split the 3 over-budget funcs + fix the 4 dropped errors. Cheap, aligns code with stated rules.
3. **M-3** — `mktemp` the supergate temp file (one-line fix, matches sibling scripts).
4. **L-3 / L-1** — add the explicit absolute-path assertion to the tar extractor; front-load `--help` with the plain value line.

## Method notes / limitations

Read-only audit, time-boxed. Deterministic tooling did the heavy lifting (build/vet/lint/gitleaks/govulncheck/shellcheck/live-CLI probes); grep sweeps guided manual reads of the highest-signal files (`corpus_snapshot.go`, `canon/verifier.go`, `pawl.go`, `shellutil/sanitize.go`, `skills_retire.go`, `findings.go`, `session_end_maintenance_helpers.go`). Not exhaustively read: the full 1377-file Go tree, the 62 schemas beyond parse-validation, and the eval/membrane subsystems (surveyed, not line-audited). No runtime behavior was mutated; no issues were filed. Findings should be treated as hypotheses verified to the depth stated in each entry (M-1, M-2, M-3 confirmed against source + tool output; L-3/L-4/L-5 are hardening/consistency observations).
