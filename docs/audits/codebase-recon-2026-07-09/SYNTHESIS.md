# AgentOps — Fresh-Model Codebase Recon Synthesis (2026-07-09)

> **Pinned repository state:** `fbba8af5ace635104775ef18f34fef362ba368ce` (`main` and `origin/main` at audit start)
> **Runtime:** GPT-5 / Codex; exact deployment identifier unavailable
> **Method:** one fresh Codex-native subagent per applicable `codebase-*` skill, followed by an independent evidence refuter and lead verification
> **Raw reports:** [archaeology](codebase-archaeology.md) · [audit](codebase-audit.md) · [patterns](codebase-pattern-extraction.md) · [architecture report](codebase-report.md)

## Outcome

AgentOps' verification center remains technically strong, but the pinned
`origin/main` tip is outside the repository's own definition of done in two
independent ways:

1. It has no bound verdict in `docs/provenance/ledger.jsonl`. The remote
   verdict backstop detected this, emitted a warning, and still succeeded
   because enforcement is report-only (`.github/workflows/verdict-backstop.yml:10-24`,
   `.github/workflows/verdict-backstop.yml:50-64`).
2. The same tip fails the canonical fast gate. The sole failure is
   `skill.schema`: `skills/goal-design/SKILL.md:23` declares
   `context.intent.mode: explicit`, while the schema admits only `questions`,
   `task`, or `none`.

This is the run's strongest result. It is not a theoretical architecture
critique: the current trunk escaped both proof binding and the local release
authority, while the hosted backstop recorded the escape without blocking it.

## What the repository is now

AgentOps is a local, file-and-subprocess-oriented verification control plane:
skills shape intent; the default `ao` binary exposes 72 top-level commands;
112 registered checks judge changed work; pawl supplies independent review;
and a locked, hash-chained provenance ledger binds proof to Git history. The
default build is deliberately narrower than the source tree: `legacy` and
`flywheel` tags retain archived command families that are not shipped in the
spine binary (`cli/Makefile:11-20`, `cli/Makefile:38-50`).

Fresh measurements found 5,163 tracked files, 1,456 Go files, 59 source skills,
109 Go packages, 363 scripts under `scripts/`, and 256 Bats files under
`tests/`. The detailed measurement boundaries are in
[codebase-report.md](codebase-report.md#measured-scale).

## Normalized risk findings

The independent refuter changed the raw audit materially. The table below is
the promoted result; producer severities are not repeated when reachability did
not justify them.

| Priority | Finding | Disposition | Why it matters |
|---|---|---|---|
| High | Current trunk lacks a bound verdict | Confirmed | The core `no verdict = not done` contract did not hold for `origin/main`; the hosted check warned but was intentionally report-only. |
| High | Current trunk fails `skill.schema` | Confirmed | A newly added skill makes the canonical local gate red at the pinned base. |
| High | Oversized council lines can hide a trailing FAIL | Confirmed, default build | Three unchecked default `bufio.Scanner` passes in `cli/cmd/ao/tick.go:739-945` stop at the token limit; the producer directly reproduced `COUNCIL PASS` and exit 0 for a contradictory artifact. |
| Medium | Tracked `bin/ralph` still invokes forbidden `claude -p` | Adjusted from High | The direct call at `bin/ralph:165-179` violates LAW 0 and is outside Door 9's scan boundary, but it has no current callers and is not a default `ao` path. |
| Medium | Changelog read errors clear a blocking fast gate as SKIP | Confirmed, default build | `cli/internal/gates/checks/native_inline.go:56-67` maps missing/read errors to SKIP; blocking SKIP remains non-failing. |
| Medium | Supported scripts call removed `ao` commands | Confirmed | `scripts/install.sh` and `scripts/nightly-evolution.sh` retain dead commands, while a whole-file baseline makes the invocation-drift check green. |
| Medium | `make docs-check` calls a deleted validator | Confirmed, current target | `Makefile:29-32` unconditionally invokes the intentionally removed `scripts/validate-hook-preflight.sh`, so the target exits 127 before doc-release validation. |
| Medium | Malformed committed verify policy weakens to defaults | Confirmed, default build | `cli/internal/verifycfg/verifycfg.go:252-266` warns and falls through; `strict` defaults false and `autobind` true. |
| Medium | Machine-readable contracts disagree with executable truth | Confirmed | Capabilities omit strict exit 5 and most environment inputs; the REBOUND schema description contradicts the authorizing checker. |
| Medium | MCP execution can select a stale PATH `ao` | Confirmed | `cli/internal/adapters/mcpsurface/surface.go:114-137` shells the string `ao` instead of the running or injected binary. |
| Low | `ao eval chaos` has stale council fixtures | Confirmed, default build | Fixtures omit required `context_id`, so the diagnostic's council matrix false-fails. |
| Low | Canonical narrative is stale about scale and shipped commands | Confirmed | The first-read overview advertises obsolete counts and labels build-tag-only `ao corpus`/`ao codex` surfaces active. |

Three real source defects are deliberately separated from default-runtime risk:

| Archived surface | Normalized severity | Verification correction |
|---|---:|---|
| Codex dispatcher symlink containment | Medium | Source-confirmed, but `cli/cmd/ao/codex.go:1` is `//go:build legacy`. |
| Required Codex commands bypass sandbox and ignore non-zero exit | Medium | Source-confirmed in the legacy profile. The raw audit's untagged test command ran zero Codex tests; `go test -tags legacy` executes the cited test. |
| Codex child-output/process-tree controls | Low | Resource-control gap is source-confirmed but legacy-only. |

See [VERIFICATION.md](VERIFICATION.md) for the complete A-01 through A-08
disposition table and [findings.json](findings.json) for machine-readable
evidence and prior-run status.

## Architectural conclusions

The strongest mechanisms are cohesive and real:

- blocking UNKNOWN and malformed gate states now fail closed;
- the provenance store validates records, locks append operations, and verifies
  every hash link;
- generated command and registry surfaces are built from the live default
  binary rather than filenames;
- test Go LOC exceeds production Go LOC; and
- `ao land` fresh-builds and re-execs the trusted binary before review and
  landing.

The recurring weakness is **surface reconciliation**. The repository moves
faster than its handwritten contracts, archived code remains executable under
tags or direct scripts, and some safety-strengthening policies degrade rather
than hold when malformed. The current unverified, gate-red trunk is the most
concrete expression of that gap.

## Pattern extraction result

The highest-value extraction candidate is a small, versioned hash-chain kernel.
Four packages independently implement the same two-stage shape, but RPI includes
`prev_hash` inside its payload hash while provenance and turn-state exclude it.
Any consolidation must therefore preserve legacy codecs and prove historical
bytes with golden chains; a generic record schema would be unsafe.

The next adoption targets are frontmatter decoding, JSONL scanning/line-size
policy, repository-directory resolution, atomic replacement, typed exit
unwrapping, and boolean normalization. The complete three-or-more-instance
evidence is in [codebase-pattern-extraction.md](codebase-pattern-extraction.md).

## Movement since 2026-07-02

The previous audit produced meaningful repairs: blocking UNKNOWN is fixed;
retired-command doc detection now covers the removed orchestration verbs; the
safety narrative reflects the hookless design; the build-tag verifier is wired
into local release; and default help stopped advertising several removed
commands.

The important regressions/new discoveries are:

- trunk proof binding and gate health both failed on the current tip;
- council parsing contains a new directly reproduced fail-open;
- legacy-only Codex defects were initially overstated until build tags were
  checked;
- capabilities improved but remain incomplete; and
- canonical architecture and disposition snapshots continue to lag executable
  state.

The complete prior-finding disposition is in
[DIFF-vs-2026-07-02.md](DIFF-vs-2026-07-02.md).

## Recommended actions

| Rank | Action | Class |
|---:|---|---|
| 1 | Restore trunk integrity: fix `goal-design` frontmatter, require a bound verdict for the escaped tip, and ratchet the hosted backstop from report-only when operationally ready. | Release authority |
| 2 | Parse council verdicts once with an explicit size bound and fail on overflow/read error; add the reproduced PASS/oversized-line/trailing-FAIL regression. | Default-path correctness |
| 3 | Make an existing-but-malformed `.aoverify.yaml` a HOLD/error for safety-strengthening policy instead of falling back to weaker defaults. | Verification policy |
| 4 | Make missing/unreadable changelogs FAIL the blocking native gate; reserve SKIP for proven non-applicability. | Gate integrity |
| 5 | Remove or port `bin/ralph`, eliminate dead script callers/baselines, and repair the stale `docs-check` target. | Runtime hygiene |
| 6 | Either retire the legacy Codex dispatcher or fix its containment, command-exit, sandbox, output-bound, and process-tree contracts under tagged tests. | Archived surface |
| 7 | Generate capabilities' exit/environment contract from executable declarations and reconcile REBOUND schema prose. | Contract governance |
| 8 | Extract only the cryptographic hash-chain kernel with versioned codecs and dual-compute/golden-chain validation. | Refactor |

The one-sentence takeaway: **AgentOps still has a unusually credible
verification architecture, but this model found that the current trunk itself
is unverified and gate-red—and independently separated that live escape from
several real but legacy-only defects.**
