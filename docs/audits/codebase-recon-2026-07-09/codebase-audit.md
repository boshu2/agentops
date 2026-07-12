# AgentOps codebase audit — 2026-07-09

> **Lead verification note:** this is the raw specialist report. Independent
> review confirmed the source facts but adjusted A-02, A-03, A-04, and A-07 for
> reachability/severity. In particular, `cli/cmd/ao/codex.go` and its tests are
> `legacy`-tagged, so the reported untagged focused command ran zero Codex
> tests. Use [VERIFICATION.md](VERIFICATION.md) and
> [findings.json](findings.json) for promoted dispositions.

## Audit metadata

- Repository: `boshu2/agentops`
- Pinned commit: `fbba8af5ace635104775ef18f34fef362ba368ce`
- Worktree: `/Users/bo/dev/agentops-wt/age-fresh-model-codebase-recon-toqd`
- Auditor: GPT-5 / Codex runtime; exact deployment ID unavailable
- Scope: security, correctness, reliability, performance, CLI behavior, gate integrity, and executable/docs drift
- Method: repository startup/bootstrap; architecture and contract review; independent source scan; targeted adversarial reproductions; focused tests and static analysis; only then comparison with the 2026-07-02 audit
- Constraints: read-only source audit. No source edits, commits, staging, pushes, issue creation, full release gate, or full test suite. Findings below are therefore either directly reproduced or backed by a deterministic source trace; unexecuted checks are called out explicitly.

## Executive summary

The current tree contains eight confirmed findings: four High, three Medium, and one Low. The most important escape is in the council verdict parser: an oversized line makes three independent `bufio.Scanner` passes stop silently, so a trailing contradictory `VERDICT: FAIL` is invisible and a council can return exit 0. The Codex dispatcher also has two separate trust-boundary failures: dispatcher-managed paths are only lexically contained and therefore follow planted symlink ancestors outside the declared roots, while `evidence.required_commands` run outside the requested Codex sandbox and a non-zero required command still permits a successful dispatch.

The July 2 audit drove meaningful repairs. Blocking `UNKNOWN` gate verdicts now fail closed, the retired-technology detector covers removed orchestration commands, the safety package documents the hookless migration, and build-tag verification is wired into the local release path. Strong current controls include subscription-auth checks, Git binary trust resolution, provenance validation, default CLI rejection of removed commands, and a clean gitleaks scan. Those controls do not cover the confirmed paths below.

| Severity | Count | IDs |
|---|---:|---|
| Critical | 0 | — |
| High | 4 | A-01, A-02, A-03, A-04 |
| Medium | 3 | A-05, A-06, A-07 |
| Low | 1 | A-08 |

## Confirmed findings

### A-01 — Oversized verdict line hides a trailing FAIL and produces a council PASS

- **Severity:** High
- **Location:** `cli/cmd/ao/tick.go:739-760`, `cli/cmd/ao/tick.go:763-829`, `cli/cmd/ao/tick.go:884-905`, `cli/cmd/ao/tick.go:932-945`
- **Root cause:** Council validation parses the same untrusted verdict text with three default `bufio.Scanner` instances. The default token limit is 64 KiB; none raises the limit or checks `scanner.Err()`. A long line after a valid PASS causes scanning to stop before later content. `tickCouncilGate` then counts the visible PASS and accepts unanimity.
- **Impact:** A crafted or accidentally verbose judge artifact can conceal a contradictory trailing `VERDICT: FAIL`. This is a direct verification-membrane escape: the command prints `COUNCIL PASS` and exits 0 even though the complete artifact is contradictory.
- **Deterministic reproduction:** At the pinned commit, a normal contradictory verdict exits 6, while inserting one 70,000-byte line before the same trailing FAIL exits 0:

  ```text
  FAIL-CLOSED: 1/2 verdict(s) unverified (no COMMANDS RUN / identity gap)
  Error:
  COUNCIL PASS: 2/2 judges unanimous across 2 distinct contexts (2 model families)
  short_contradiction_exit=6
  oversized_hidden_contradiction_exit=0
  ```

  The exact command is recorded in [Evidence commands and output](#evidence-commands-and-output).
- **Confidence:** High — directly reproduced with the binary built from the pinned SHA, and the stopped-scan behavior maps exactly to the unchecked scanner loops.
- **Recommended fix:** Parse each verdict once with an explicitly bounded reader whose overflow/error is returned as an unverified verdict. If `bufio.Scanner` remains, call `Buffer` with a documented maximum and fail on every non-nil `Err()`. Add an adversarial regression containing a valid PASS, a line larger than the limit, and a trailing FAIL; require non-zero exit.

### A-02 — A tracked executable still invokes forbidden `claude -p`; the blocking detector cannot see it

- **Severity:** High
- **Location:** `bin/ralph:1-13`, `bin/ralph:165-179`, `scripts/check-door9-no-claude-p.sh:2-7`, `scripts/check-door9-no-claude-p.sh:13-32`
- **Root cause:** `bin/ralph` remains an executable, usage-promoted workflow that directly runs `claude -p` with `--permission-mode bypassPermissions`. The Door 9 gate scans only production Go files beneath `cli/internal/rpi` and `cli/cmd/ao`, so tracked shell executables are structurally outside its search space.
- **Impact:** Running the advertised script can burn the Claude API/quota path that repository LAW 0 forbids, while also bypassing permissions. The dedicated blocking detector reports PASS, creating false assurance. The lack of current callers reduces immediate reachability, but the executable itself advertises direct use.
- **Deterministic reproduction/source trace:** `bin/ralph:173-179` executes the forbidden command. At the pinned commit, `bash scripts/check-door9-no-claude-p.sh` returned:

  ```text
  check-door9-no-claude-p: PASS — no Claude print defaults or direct production exec paths in phased RPI
  door9_gate_exit=0
  ```

  Separately, `/tmp/ao-audit-fbba8af eval chaos` detected the tracked script surface and reported `FAIL  6 found claude in -p/--print mode on a tracked script surface`.
- **Confidence:** High — exact executable call and detector exclusion are both source-visible; the detector and broader chaos probe were executed.
- **Recommended fix:** Delete or permanently retire `bin/ralph`, or port it to the approved Codex/local-shell runtime. Expand the always-run Door 9 detector to all tracked production executable/script surfaces, using syntax-aware exclusions for fixtures and documentation. Add a regression fixture proving a shell-script invocation fails the gate.

### A-03 — Codex dispatcher path containment is bypassable through symlink ancestors

- **Severity:** High
- **Location:** `cli/cmd/ao/codex.go:1246-1280`, `cli/cmd/ao/codex.go:1322-1332`, `cli/cmd/ao/codex.go:1601-1646`, `cli/cmd/ao/codex.go:1649-1675`, `cli/cmd/ao/codex_dispatch_test.go:147-203`, `docs/contracts/codex-task-packet.md:44-54`
- **Root cause:** `resolveCodexDispatchPath` cleans paths and performs a string-prefix comparison, but never resolves symlinks. A path such as `cwd/out/receipt.json` is accepted lexically even if `cwd/out` is a symlink to a directory outside `cwd` and every `allowed_paths` root. Reads use `os.ReadFile`; writes create directories or use the accepted path, so the operating system follows the symlink. Existing escape tests cover absolute paths and `..`, but not symlink ancestry.
- **Impact:** A repository or task preparation step that can plant a symlink can redirect dispatcher-managed prompt reads, JSONL writes, and receipt writes outside the declared capability boundary. This contradicts the task-packet contract's claim that these paths are enforced.
- **Deterministic source trace:** The accepted candidate is returned at `cli/cmd/ao/codex.go:1641-1644` after only `filepath.Clean`; it feeds the prompt read at `cli/cmd/ao/codex.go:1272-1278`, JSONL write at `cli/cmd/ao/codex.go:1322-1329`, and receipt write at `cli/cmd/ao/codex.go:1601-1611`. The repository already contains a correct longest-existing-prefix resolver in `cli/cmd/ao/path_containment.go:10-33` and a relative-path containment predicate at `cli/cmd/ao/path_containment.go:36-42`, demonstrating the missing control is available locally.
- **Confidence:** High — the complete read/write path is deterministic from source. No live exploit wrote outside the worktree because the assignment prohibited source-adjacent mutation.
- **Recommended fix:** Resolve `cwd`, allowed roots, and candidates through `realpathOrSelf` before containment checks, and reject symlink ancestry for dispatcher writes; stronger implementations should use directory-relative no-follow/openat semantics to avoid check/use races. Add separate planted-symlink tests for prompt reads and receipt/JSONL writes.

### A-04 — Required acceptance commands bypass the declared sandbox and failures still yield success

- **Severity:** High
- **Location:** `cli/cmd/ao/codex.go:1095-1122`, `cli/cmd/ao/codex.go:1158-1185`, `cli/cmd/ao/codex.go:1501-1538`, `cli/cmd/ao/codex.go:1541-1567`, `cli/cmd/ao/codex_dispatch_test.go:373-432`, `docs/contracts/codex-task-packet.md:56-58`, `docs/contracts/codex-task-packet.md:82-91`
- **Root cause:** The Codex worker receives the packet's sandbox through its command vector, but post-run `evidence.required_commands` are launched by the dispatcher as unrestricted host `sh -c` processes. Receipt validation checks only that each command string was recorded, not that its exit code is zero. The existing test explicitly expects `exit 7` to be recorded while dispatch returns no error.
- **Impact:** A packet described as `read-only` can mutate the repository or host through its required commands, expanding the effective blast radius beyond the audited sandbox. More importantly for the verification membrane, a required acceptance check can exit non-zero and the dispatch still returns success with a PASS verdict.
- **Deterministic reproduction/source trace:** `runCodexRequiredCommand` uses host `exec.CommandContext(ctx, "sh", "-c", command)` at `cli/cmd/ao/codex.go:1519-1527`; `validateCodexReceiptRequiredCommands` checks presence only at `cli/cmd/ao/codex.go:1545-1566`. `TestCodexDispatchRecordsFailingRequiredCommandExitCode` at `cli/cmd/ao/codex_dispatch_test.go:409-432` requires dispatch success for an `exit 7` command, pinning the fail-open behavior as the current contract.
- **Confidence:** High — direct source trace plus a focused green test that encodes non-zero acceptance as successful dispatch.
- **Recommended fix:** Run required commands through an isolation adapter at least as restrictive as the declared sandbox and under one overall dispatch budget. Unless the schema gains explicit expected-exit semantics, require every required command to exit 0; derive the receipt verdict and command return error from that result. Add regressions proving a read-only packet cannot create a file and `exit 7` fails dispatch.

### A-05 — Deleting either changelog clears a blocking fast gate as SKIP

- **Severity:** Medium
- **Location:** `cli/internal/gates/checks/native_inline.go:17-20`, `cli/internal/gates/checks/native_inline.go:56-67`, `cli/internal/gates/orchestrator.go:191-209`, `cli/internal/gates/report.go:26-40`, `tests/docs/validate-doc-release.sh:67-74`
- **Root cause:** The blocking `changelog.sync` evaluator returns `SKIP` for any read error, including a missing `CHANGELOG.md` or `docs/CHANGELOG.md`. Blocking SKIP is deliberately treated as non-failing by `isBlockingFail`, and `Report.ExitCode` therefore returns 0. The stricter release-doc script fails missing files, but it is not the native fast-gate implementation.
- **Impact:** A change deleting either required changelog can pass the dedicated routine release gate even though the invariant cannot be evaluated and downstream release tooling expects both files.
- **Deterministic source trace:** The deletion paths match the gate's routing glob at `cli/internal/gates/checks/native_inline.go:19`; missing/read errors become SKIP at lines 59-63; blocking SKIP clears at `cli/internal/gates/orchestrator.go:200-208` and therefore does not affect `Report.ExitCode` at `cli/internal/gates/report.go:31-40`.
- **Confidence:** High — deterministic native-gate control flow. A destructive deletion reproduction was not performed in the shared worktree.
- **Recommended fix:** Return FAIL for missing files and all read errors; reserve SKIP for a proven not-applicable condition, which does not exist for this always-required pair. Add table tests for deletion of each file and an unreadable/read-error case.

### A-06 — Supported scripts call removed CLI commands while the drift gate grandfathers them

- **Severity:** Medium
- **Location:** `scripts/install.sh:252-264`, `scripts/nightly-evolution.sh:14-18`, `scripts/nightly-evolution.sh:593-610`, `scripts/nightly-evolution.sh:623-690`, `scripts/.scripts-ao-invocations-baseline:1-17`
- **Root cause:** Two executable scripts retain removed command surfaces: `ao hooks install --force` and `ao daemon jobs ...`. The invocation gate uses a filename-level baseline that explicitly labels both files “REAL current offender[s]”; because they are baselined, the check reports PASS rather than forcing retirement or repair.
- **Impact:** `scripts/install.sh --with-hooks` fails after a partial install, and the nightly script's advertised execute paths cannot submit or wait for work with the current CLI. Users receive a green drift gate despite semantically dead automation.
- **Deterministic reproduction:** The pinned binary returned exit 1 for both `/tmp/ao-audit-fbba8af hooks install --force` and `/tmp/ao-audit-fbba8af daemon jobs list --json`. The drift check returned:

  ```text
  check-scripts-ao-invocations: PASS — no un-baselined script invokes a removed ao command (2 file(s) with findings, all baselined; 2 baseline entr(ies) still active).
  ```

- **Confidence:** High — dead calls are explicit, current CLI rejection was executed, and the baseline documents the intentional grandfathering.
- **Recommended fix:** Remove the dead hook branch/messages or route them to a supported opt-in installer. Retire or rewrite nightly execution against an operator-chosen supported substrate. Delete both baseline entries. Replace whole-file offender grandfathering with narrow false-positive pragmas only; real executable command invocations should remain blocking.

### A-07 — Codex dispatch buffers unbounded child output and does not terminate process trees

- **Severity:** Medium
- **Location:** `cli/cmd/ao/codex.go:1095-1113`, `cli/cmd/ao/codex.go:1519-1538`, `cli/cmd/ao/codex.go:1701-1712`, `cli/internal/goals/measure_unix.go:10-20`, `cli/internal/agentworker/process_unix.go:10-21`
- **Root cause:** Both the main Codex child and every required shell command attach unrestricted `bytes.Buffer` instances to stdout/stderr. The 500-byte excerpt limit is applied only after the processes exit. `exec.CommandContext` cancels the direct process but no process group, `WaitDelay`, or descendant cleanup is configured. Other repository packages already implement process-group termination, but the dispatcher does not reuse it.
- **Impact:** A noisy or stuck child can grow dispatcher memory until OOM. A timed-out shell command can leave descendants running and holding resources. Each required command also receives a fresh full timeout, so the packet timeout is not an overall wall-clock budget.
- **Deterministic source trace:** Unbounded buffers are at `cli/cmd/ao/codex.go:1105-1107` and `cli/cmd/ao/codex.go:1525-1527`; truncation occurs later at `cli/cmd/ao/codex.go:1701-1712`. Hardened process-group patterns exist at `cli/internal/goals/measure_unix.go:10-20` and `cli/internal/agentworker/process_unix.go:10-21`.
- **Confidence:** High for the resource-control absence; no OOM or orphan process was intentionally induced on the shared host.
- **Recommended fix:** Stream durable output to a bounded file/tee and retain only a fixed-size ring buffer for excerpts. Use a shared cross-platform subprocess adapter that creates and kills process groups, sets `WaitDelay`, caps output, and enforces one overall deadline. Add noisy-output and child-descendant timeout fixtures.

### A-08 — `ao eval chaos` ships stale council fixtures and always fails its council matrix

- **Severity:** Low
- **Location:** `cli/cmd/ao/tick.go:845-881`, `cli/cmd/ao/tick.go:1015-1035`
- **Root cause:** Council identity validation now requires `context_id`, but the built-in smoke fixtures `pass1`, `pass2`, and `fail1` omit it. The smoke matrix therefore cannot satisfy its first expected successful council case.
- **Impact:** The diagnostic emits a false failure on every normal run, obscuring real failures in the same smoke command and reducing operator trust in the diagnostic.
- **Deterministic reproduction:** `/tmp/ao-audit-fbba8af eval chaos` returned exit 1 and included:

  ```text
  FAIL  3 council-gate matrix failed
  ```

  The identity guard appends `missing judge.context_id` at `cli/cmd/ao/tick.go:869-870`, while the fixtures at `cli/cmd/ao/tick.go:1021-1023` contain no `context_id`.
- **Confidence:** High — reproduced and directly explained by source.
- **Recommended fix:** Construct smoke verdicts through the same valid fixture builder used by council tests, or add distinct `context_id` values explicitly. Add a controlled test of the production `tickSmoke` matrix so future identity-contract changes update both test and diagnostic fixtures.

## Refuted suspected issues and negative evidence

| Suspected issue | Result | Evidence |
|---|---|---|
| Blocking UNKNOWN verdicts still pass | Refuted | `cli/internal/gates/orchestrator.go:191-209` now fails FAIL, UNKNOWN, empty, and unrecognized blocking verdicts; `cli/internal/gates/report.go:26-40` propagates the failure. This closes July 2 A1. |
| Default binary still exposes removed `rpi`, `evolve`, or `orchestrate` commands | Refuted | All three commands returned exit 1 in the binary built from the pinned SHA; `cli/cmd/ao/orchestrate.go:1` is legacy-build-tagged. |
| Retired-tech docs detector omits removed orchestration verbs | Refuted | `scripts/check-docs-no-retired-tech.sh:48-58` includes `ao (rpi|orchestrate|evolve)`. This closes July 2 A5's regex gap. |
| Build-tag verification is entirely unwired | Refuted/qualified | `scripts/ci-local-release.sh:1100-1101` invokes the build-tag verifier. It is present in the local release path, closing the specific July 2 claim; this audit did not prove every CI route invokes it. |
| Production hardcoded credentials | Not found | `gitleaks detect --source . --no-git --no-banner --redact --config .gitleaks.toml` scanned about 51.27 MB and exited 0 with no leaks. Targeted secret patterns only hit fixtures/examples. |
| Rerank endpoint is an attacker-controlled SSRF sink | Not confirmed | Raw gosec flagged `cli/cmd/ao/retrieval_search_backends.go:170`, but the endpoint is read from the operator-controlled `AGENTOPS_RETRIEVAL_RERANK_ENDPOINT` configuration. No untrusted request-to-endpoint flow was found. |
| Git executable resolution trusts a repository-planted binary | Refuted | `cli/internal/adapters/worktreeconfig/worktree_config.go:87-176` excludes repo-internal candidates; `cli/cmd/ao/path_containment.go:10-52` resolves symlinks for that trust decision. |

## Strong controls observed

- The routine gate is now fail-closed for blocking UNKNOWN/error states (`cli/internal/gates/orchestrator.go:191-209`, `cli/internal/gates/report.go:26-40`).
- Codex dispatch rejects API-key worker authentication and records explicit auth/sandbox intent; the contract makes the subscription trust boundary visible (`docs/contracts/codex-task-packet.md:56-63`, `skills/codex-exec/SKILL.md:46-56`).
- Dispatcher tests cover ordinary absolute and `..` path escapes (`cli/cmd/ao/codex_dispatch_test.go:147-203`), even though symlink ancestry is missing.
- The repository has reusable symlink-aware containment and process-group controls (`cli/cmd/ao/path_containment.go:10-52`, `cli/internal/goals/measure_unix.go:10-20`).
- Focused Go tests for council parsing, Codex dispatch, gates, and storage passed. Shellcheck reported no warning-level syntax issues in the three reviewed scripts.
- The current checkout's `core.hooksPath` resolves to `/Users/bo/dev/agentops/.git/hooks`, and bootstrap reported `gate=active`. This is useful host-local evidence, not a property of the pinned commit.

## Delta from the 2026-07-02 audit

This section was written only after the independent current-tree scan.

| July 2 ID | 2026-07-09 status | Evidence/delta |
|---|---|---|
| A1 blocking UNKNOWN fail-open | Fixed | Blocking UNKNOWN/default now fails in `cli/internal/gates/orchestrator.go:191-209`; report propagation is at `cli/internal/gates/report.go:26-40`. Current A-05 is a different evaluator bug: it deliberately emits SKIP for a required missing file. |
| A2 orphaned hook | Fixed in this checkout / host-local | Bootstrap reports an active gate and the configured hooks path exists. Git config is not tracked, so this is not a commit-level guarantee. |
| A3 bash/Go gate parity | Residual, explicitly legacy | The bash escape hatch still exists, but current doctrine marks it legacy. No new deterministic parity escape was established in this audit. |
| A4 removed CLI commands in docs | Substantially improved, residual history | Flagship/getting-started wording now describes removal. Historical and architecture references remain; only executable breakage with a current user path is reported as current A-06. |
| A5 retired-tech regex gap | Fixed | The detector covers `rpi`, `orchestrate`, and `evolve` at `scripts/check-docs-no-retired-tech.sh:48-58`. |
| A6 stale safety threat model/tests | Fixed/reconciled | `cli/internal/safety/doc.go:4-20` explicitly describes hookless migration and removed enforcement. |
| A7 capabilities exit-code semantics | Residual/unverified in this pass | No current exploit or regression was independently established; retain as follow-up rather than restating it without new proof. |
| A8 buildtags checker unwired | Fixed in local release path | `scripts/ci-local-release.sh:1100-1101` calls the verifier. Full CI route coverage was not rerun. |
| A9 root help advertises legacy commands | Fixed | Default help and invalid-command diagnostics no longer advertise the removed orchestration commands in the checked binary. |
| A10/A11 stale narrative/counts | Improved but not exhaustively closed | Generated and canonical surfaces are more consistent; this audit did not perform a complete prose/count census. |
| A12 `sh -c` narrative | Superseded by stronger current proof | Current A-04 identifies the concrete host-shell trust-boundary and false-success behavior for required commands, with code and test evidence. |

## Evidence commands and output

All commands ran in `/Users/bo/dev/agentops-wt/age-fresh-model-codebase-recon-toqd` against `fbba8af5ace635104775ef18f34fef362ba368ce` unless stated otherwise.

### Build and identity

```sh
git rev-parse HEAD
(cd cli && go build -o /tmp/ao-audit-fbba8af ./cmd/ao)
```

```text
fbba8af5ace635104775ef18f34fef362ba368ce
```

### Oversized verdict contradiction

```sh
valid2='author: worker-b\njudge: judge-b\njudge_program: codex\njudge_model_family: codex\ncontext_id: ctx-b\nVERDICT: PASS\nCOMMANDS RUN:\n  go test ./...\n'
/tmp/ao-audit-fbba8af council-gate \
  <(printf 'author: worker-a\njudge: judge-a\njudge_program: claude\njudge_model_family: claude\ncontext_id: ctx-a\nVERDICT: PASS\nCOMMANDS RUN:\n  go test ./...\nVERDICT: FAIL\n') \
  <(printf "$valid2")
short_status=$?
/tmp/ao-audit-fbba8af council-gate \
  <(perl -e 'print "author: worker-a\njudge: judge-a\njudge_program: claude\njudge_model_family: claude\ncontext_id: ctx-a\nVERDICT: PASS\nCOMMANDS RUN:\n  go test ./...\n", ("X" x 70000), "\nVERDICT: FAIL\n"') \
  <(printf "$valid2")
oversized_status=$?
printf 'short_contradiction_exit=%s\noversized_hidden_contradiction_exit=%s\n' "$short_status" "$oversized_status"
```

```text
FAIL-CLOSED: 1/2 verdict(s) unverified (no COMMANDS RUN / identity gap)
Error:
COUNCIL PASS: 2/2 judges unanimous across 2 distinct contexts (2 model families)
short_contradiction_exit=6
oversized_hidden_contradiction_exit=0
```

### Forbidden executor and dead CLI surface

```sh
bash scripts/check-door9-no-claude-p.sh; printf 'door9_gate_exit=%s\n' "$?"
bash scripts/check-scripts-ao-invocations.sh
/tmp/ao-audit-fbba8af hooks install --force
/tmp/ao-audit-fbba8af daemon jobs list --json
```

```text
check-door9-no-claude-p: PASS — no Claude print defaults or direct production exec paths in phased RPI
door9_gate_exit=0
check-scripts-ao-invocations: PASS — no un-baselined script invokes a removed ao command (2 file(s) with findings, all baselined; 2 baseline entr(ies) still active).
Error: unknown command "hooks" for "ao"
Error: command removed in AgentOps 3.0: ao daemon jobs list
```

### Chaos smoke

```sh
/tmp/ao-audit-fbba8af eval chaos; printf 'eval_chaos_exit=%s\n' "$?"
```

```text
FAIL  1 guard-status NOT active
PASS  2 verdict-gate rejects empty / accepts cited
FAIL  3 council-gate matrix failed
PASS  4 chaos bare-verdict rejected
PASS  5 close aborts before git commit when br close fails
FAIL  6 found claude in -p/--print mode on a tracked script surface
PASS  7 br ready + git rev-parse HEAD resolve
SMOKE FAIL (3 check(s) failed)
eval_chaos_exit=1
```

Step 1 is environment-sensitive and is not reported as a code finding. Step 3 is A-08; step 6 independently corroborates A-02.

### Focused tests

```sh
cd cli
go test ./cmd/ao -run 'TestTick(CouncilGateMatrix|VerdictHasCommandsRun|VerdictTokenCounts)|TestCodexDispatch(ExecutesRequiredCommandsIntoReceipt|RecordsFailingRequiredCommandExitCode|RejectsPathEscapes)' -count=1
go test ./internal/gates/... -run 'Test.*(Skip|Blocking|ScriptRunner|Changelog)' -count=1
go test ./internal/storage -count=1
```

```text
ok  github.com/boshu2/agentops/cli/cmd/ao                 0.382s
ok  github.com/boshu2/agentops/cli/internal/gates        0.190s
ok  github.com/boshu2/agentops/cli/internal/gates/checks 0.254s
ok  github.com/boshu2/agentops/cli/internal/storage      0.622s
```

These green tests validate existing ordinary cases. They do not refute A-01 or A-03 because they omit oversized-token and symlink-ancestor adversarial cases. The green failing-command test is affirmative evidence for A-04 because it expects dispatch success after `exit 7`.

### Static/security tools

```sh
gitleaks detect --source . --no-git --no-banner --redact --config .gitleaks.toml
shellcheck -S warning bin/ralph scripts/install.sh scripts/nightly-evolution.sh
bash scripts/security-gate.sh --mode quick --require-tools
```

```text
gitleaks: 51.27 MB scanned, no leaks, exit 0
shellcheck: exit 0
security gate: gate_status=WARN_QUALITY, missing_tool_count=1 (radon not installed), exit 4
```

The quick security gate reported semgrep, trivy, and gosec pass, but skipped gitleaks, shellcheck, pytest, and Go tests in that mode; direct commands above cover gitleaks and the targeted shell scripts. A separate raw `gosec -exclude-generated -quiet ./...` run produced 602 mostly broad/noisy findings and was used only for candidate generation, not as a pass/fail claim.

## Unverified checks and remaining uncertainty

- The full `ao gate check --full`, full Go suite, full shell test suite, release workflow, and CI matrix were not run. Their status is unverified.
- No destructive changelog deletion, symlink escape write, OOM, or orphan-process reproduction was performed in the shared worktree. Those findings use complete deterministic source traces and should receive isolated regression tests during remediation.
- Raw static-analysis counts are not treated as confirmed defects without a source-to-impact trace.
- Historical docs were sampled after the independent scan for delta analysis; this is not a comprehensive documentation census.

## Suggested remediation order

1. Fix A-01 and add the oversized contradictory-verdict regression before relying on council output.
2. Remove/port `bin/ralph` and broaden Door 9 coverage (A-02).
3. Fix Codex dispatcher containment and acceptance semantics together (A-03, A-04), then harden subprocess resource control (A-07).
4. Make missing changelogs fail the native gate (A-05), burn down the executable-command baseline (A-06), and repair the chaos fixture (A-08).
