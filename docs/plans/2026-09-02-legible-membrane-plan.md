# Legible membrane, Train 1: fix what ships

> **Status:** EXECUTED 2026-09-02 — Train 1 landed as one PR (branch
> `legible/l2`; L1 ×5, L2 ×4, I-1 seam fix ×1, L3 ×1). Plan review history:
> r1/r2 (eight-lane) FAIL; r3/r4 (Train 1) FAIL with every finding folded into
> r5; execution proceeded on Bo's instruction. Fresh per-lane validators PASS;
> two cross-family train reviews, all findings closed on the landed tip.
> Train 2 (corpus reorganization) is a named successor,
> **Train state (09-02 evening):** branch `legible/l2` @ `7e9edc770`, 8 commits
> on `b0c81349d` (L1 ×4, L2 ×3, I-1 seam fix ×1); L3 excluded (second fresh
> validation FAIL, awaiting Bo). L1 PASS (fresh), L2 PASS (fresh), I-1 FAIL →
> one seam fix (stale literal) → re-validation pending; cross-family train
> review r1 FAIL (7) → all 7 closed on tip → r2 pending. Full gate 71/71 (HEAD
> binary), run-all green, Go bar green, security quick PASS, lint clean.
> not part of this intent. · **Intent source:** this document, once Bo
> accepts. **Provenance:** 2026-09-02 five-lane field audit, artifact
> <https://claude.ai/code/artifact/b7dfc5c2-e1a9-4ac0-a613-dcbdee5833b8>;
> Codex reviews r1 (16 findings) and r2 (8 closed / 8 open / 11 new), whose
> Train-1 residues (Codex budget metric, Go test bar, npx wording) are folded
> in here.
> **Retirement:** superseded when the three lanes and the integration subject
> are merged or explicitly dropped. **Consumer:** the orchestration run that
> executes it and the fresh per-lane validators that judge each lane against
> the acceptance written here.
> **Constraint floor honored:** no new `ao` root command
> (`cli/cmd/ao/default_spine_test.go` seals the spine); no new ADR; no new
> `skills/*/scripts/**/*.py`; no advisory gate promoted to blocking; the one
> acceptance change (the Codex catalog average) is named, measured, and
> justified in the commit body against its original intent.

## Caller-visible outcome (the one behavior)

A stranger who installs the Codex plugin gets a router that reads whole skill
descriptions instead of fragments like "Freshly judge whether a finished
change is Triggers: …". A stranger who reads the README learns before
installing which skills execute `python3` and which execute the `ao` binary,
from a table derived by reading each skill, with no behavior promised that
the skills do not implement. A contributor who runs the commands `AGENTS.md`
names (`scripts/regen-all.sh --check`, `tests/run-all.sh`) sees them run, and
run green, on `main`.

Countermetrics: `ao gate check --full` green on the integration subject;
`evals/skill-probes/LEDGER.md` measured count unchanged; the Claude catalog
(`skills/*` descriptions, 9,923 chars today) unchanged except validate's
line, which shrinks; no test or gate loosened except the one named below.

## Ground truth (Extend row)

- Codex projection: `scripts/codex-sync.sh` `codex_catalog_description`
  (lines 305-330) cuts prose at 44 chars on a word boundary; the enforced
  budget is `tests/skills/test-token-budgets.sh:33-40,227-234`, a **45-char
  average over `skills-codex/*/SKILL.md` frontmatter descriptions** (today:
  avg 40, total 2,241 over 56). `skills-codex-overrides/catalog.json` carries
  no descriptions. Contract: `docs/contracts/codex-skill-api.md`.
- CI runs `tests/scripts/*.bats` (`.github/workflows/validate.yml:293-304`)
  and routes on the path filter at `validate.yml:75-95`. `tests/run-all.sh`
  is not run by CI.
- Trailers: `Gate-Loosen-Reason:` governs `cli/internal/gates/**` and
  top-level `scripts/check-*.sh` (advisory); `Test-Removal-Reason:` governs
  net Go-test reductions under `cli/internal/`. Neither governs
  `tests/skills/*.sh`; a change there is justified in the commit body.
- Full gate: `always.regen-all` runs unconditionally in Full mode
  (`cli/internal/gates/checks/seed.go:422`), and the Fast changed-scope gate
  (`scripts/regen-changed-scope.sh:175-183,248-279`) detects withheld
  projections. Therefore every lane that changes a projection source
  **commits its own regenerated outputs**; I-1 re-runs regeneration after the
  sequential merge and expects a no-op (its acceptance is `--check` clean with
  an empty diff, or a diff that is itself the I-1 commit). Rebase corrupts the
  skills-codex manifest; on conflict, reset + regen.
- Go build bar (`AGENTS.md:32-36`):
  `cd cli && go build ./... && go vet ./... && go test ./...`.

Control experiment per lane: the simplest change satisfying acceptance, with
the insufficiency of doing less named in the lane report.

## Audit findings this plan answers

| # | Finding (verified 09-02) | Lane |
|---|---|---|
| F1 | 51/56 `skills-codex/*/SKILL.md` descriptions truncated mid-clause at the 44-char cap; the same generator rewrites `# /route` → `# $route` and blindly substitutes "Claude Code"→"Codex" (the source `skills/using-flywheel/SKILL.md:47,72` says "Claude Code, Codex CLI" twice; the twin says "Codex, Codex CLI" twice); carries dormant code that would emit `ao codex ensure-start` (no such subcommand; `codex-sync.sh:275-277,352-353`; 0 twins carry it today) | L1 |
| F2 | 23 shebang-bearing shell files under `scripts/` and `tests/` are mode 100644 (5 shebang-less `lib/` files are sourced and correctly 644), incl. `scripts/regen-all.sh` and 3 `check-*.sh` gates; `./scripts/regen-all.sh --check` → permission denied. The population is derived by command in L2, never asserted | L2 |
| F3 | `tests/run-all.sh` red on `main`: the GOALS lane invokes `tests/goals/validate-goals.sh`, whose North-Stars/directives assertions (`:41-62`) predate the 08-25 `GOALS.md` rewrite; the token-budget lane fails on validate's description (275 raw chars; 197 prose chars by the test's own metric; limit 180) | L2 |
| F4 | `README.md:25` "No other runtime is required" is false: the core path runs `validate.py` / `run_once.py` (python3); eight skills mention `ao` subcommands (bootstrap, cc-hooks, fitness, goals, human-only-skills, handoff, status, using-gc) but only some of them *require* the binary in their procedure — the lexical set is not the dependency set (`skills/bootstrap/SKILL.md:67-75` and `skills/human-only-skills/SKILL.md:22,32-33` invoke nothing). L3 publishes a hard/optional table derived by reading each skill, not by grep | L3 |

Deferred to the successor (not this intent): F5 process-artifact sweep, F6
promoted set / `skills-internal/`, F7 "It's working if" + prompt per skill,
F8 routing clusters, F9 doctrine diet, F10 PRODUCT.md vs CLI, F11 12/12
UNMEASURED, F12 front door. The successor's precondition, learned from r2: a
consumer-disposition table built from the tree with `rg` for every path to be
deleted or moved, before any lane is written.

## Ownership: one owner per path

| Path | Owner |
|---|---|
| `scripts/codex-sync.sh` · `tests/skills/test-token-budgets.sh` (the Codex-average limit and its comment block only) · `tests/scripts/legible-l1-codex-descriptions.bats` (new) · `tests/scripts/codex-desc-avg-budget.bats` · `tests/scripts/test-codex-sync-generator.sh` · the regenerated `skills-codex/**` twins and any other `scripts/regen-all.sh` output its change moves, committed in the same commit | L1 |
| file modes of the shebang-bearing entry points (derived by command) · `scripts/check-shell-exec-bits.sh` (new) · gate registration in `cli/internal/gates/checks/seed.go` + its `_test.go` · `tests/scripts/legible-l2-exec-bits.bats` (new) · `tests/run-all.sh` · `tests/goals/validate-goals.sh` + its fixtures under `tests/goals/` · `AGENTS.md` lines 38-41 · `skills/validate/SKILL.md` frontmatter `description:` line only, plus the regenerated validate twin and catalog rows that line moves, in the same commit (L2 is rebased on L1 first, so these are sequential edits of the same generated files, never concurrent) | L2 |
| `README.md` · `docs/install-day2-ops.md` · `tests/scripts/legible-l3-runtime-truth.bats` (new) | L3 |
| the post-merge `scripts/regen-all.sh` run; expected no-op | I-1 |

Each lane commits its own regenerated outputs (see Ground truth: both the
Full gate and the Fast changed-scope gate detect withheld projections). Each
lane's tip is validated with its acceptance commands, `scripts/regen-all.sh
--check` clean, and `ao gate check` at head scope. I-1 re-runs regeneration
after the sequential merge; its acceptance is `--check` clean and `ao gate
check --full` green.

## Lanes and dependency order

L1 and L3 in parallel. L2 last: its final `tests/run-all.sh` run needs L1's
generator fix present (the token-budget lane reads twins) and its own
validate-description line. I-1 after all three merge.

**L1 — Codex description projection** (S)
- Projection rule, frozen before implementation: twin description = the
  **first sentence** of the source prose (terminator: `.`, `!`, or `?`
  followed by a space or the end of the prose; semicolons and dashes are not
  terminators; with no terminator the whole prose is the sentence) + one
  space + the full `Triggers:` clause verbatim. Nothing else. The generator's
  44-char cap constant is deleted.
- Budget, chosen independently of the candidate's output and on the SAME
  metric the test enforces (prose only, `Triggers:` clause and quotes
  stripped): the Codex-catalog **average** limit in
  `tests/skills/test-token-budgets.sh` becomes the Claude catalog's prose-only
  average rounded up — measured by the test's own extraction as 5,357 / 56 =
  95.66 → **96** (the raw figure 9,796 incl. triggers is recorded but not
  used). Frozen expected projection on the 09-02 base: total **5,185**, avg
  **92** (pre-L2; validate's line then shrinks it). If the measured average
  exceeds 96 the lane stops and reports. The 180-char per-skill limit is
  untouched. Justification in the commit body: the budget existed to keep
  Codex's always-loaded catalog small; the 44-char cut defeated the catalog's
  purpose (routing); the new bound says Codex's catalog prose may not exceed
  Claude's per skill.
- RED first: `tests/scripts/legible-l1-codex-descriptions.bats` asserts, for
  **all 56** twins, (a) no description matches `[a-z] Triggers:`; (b) the
  description equals the frozen rule applied to the source description; (c)
  `skills-codex/using-flywheel/SKILL.md` contains "Codex CLI" exactly twice
  (the source says "Claude Code, Codex CLI" twice) and "Codex, Codex" zero
  times; (d) no twin title starts `# $`; (e) `rg -n ensure-start scripts
  skills-codex` is empty. On current `main`, (a) fails on exactly 51 files.
- Title rewrite `# /x` → `# $x`: removed. "Claude Code" → "Codex"
  substitution becomes phrase-aware (a fixed replacement table applied
  longest-match-first, first entry "Claude Code, Codex CLI" → "Codex CLI";
  the table lives in `codex-sync.sh` beside the function).
- Delete the dormant `ao codex ensure-start` emission
  (`codex-sync.sh:275-277,352-353`); it is prevention, not a live defect.
- Regenerate in the worktree (`bash scripts/regen-all.sh`) and commit the
  regenerated outputs with the three source files.
- Existing tests that encode the old behavior are in L1's scope and are
  updated to the frozen contract without removing their failure cases:
  `tests/scripts/codex-desc-avg-budget.bats` (its over-budget fixture stays
  over budget under 96) and `tests/scripts/test-codex-sync-generator.sh:122-129`
  (asserts first sentence kept, second sentence dropped, clause kept).
- Acceptance (all by command): the bats file green; `grep -lE
  '^description:.*[a-z] Triggers:' skills-codex/*/SKILL.md | wc -l` = 0;
  `bash scripts/regen-all.sh --check` clean; `bash
  tests/skills/test-token-budgets.sh` shows the Codex-catalog line PASS with
  the limit reading 96 and NO failure other than the two validate > 180
  entries (source and twin) that L2's description line removes — full green
  is L2's acceptance, not L1's; `bats tests/scripts/*.bats` green; `ao gate
  check` (head scope) green. Semantic judgment for the validator:
  is the commit-body justification the one written here (attestation, not
  command).

**L2 — documented entry points run green** (S, lands last)
- `git update-index --chmod=+x` on every tracked shell file under `scripts/`
  and `tests/` whose first line is a shebang and whose mode is 100644. The
  population is derived by this command and listed in the lane report
  (`git ls-files -s | awk '$1=="100644" && $4 ~ /^(scripts|tests)\/.*\.sh$/
  {print $4}' | while read f; do head -1 "$f" | grep -q '^#!' && echo "$f";
  done`; 23 files on the 09-02 base). Shebang-less `*.sh` files (5 on the
  base, all under `lib/`) are sourced and stay 644.
- New advisory gate `scripts/check-shell-exec-bits.sh`, registered in
  `cli/internal/gates/checks/seed.go` with `Blocking: false`, `Tiers:
  fast+full` (the same value the neighbouring script-backed fast gates use;
  the lane names the exact Go constant), matched on `scripts/**` and
  `tests/**`: every tracked `*.sh` under those roots whose first line is a
  shebang has mode 100755; every shebang-less `*.sh` lives under a `lib/`
  directory. RED against current `main` (23 hits).
  `tests/scripts/legible-l2-exec-bits.bats` exercises both branches on a temp
  repo; direct acceptance: `bash scripts/check-shell-exec-bits.sh` exits 0 on
  the lane tip and non-zero on `main`; routed acceptance: `ao gate check
  --scope head` lists the gate as run.
- `tests/goals/validate-goals.sh` (invoked by `tests/run-all.sh:120-123`):
  its North-Stars/directives assertions at `:41-62` are replaced by
  invariants of the current `GOALS.md` shape — a `## Gates` table with ≥ 1
  data row; when a Check cell cites a repository path (`scripts/…` or
  `*.sh`) that file must exist; bare gate ids (`GOALS.md:73-78` has
  `go-cli-tests`, `go-vet-clean`, `goals-denominator`, registered nowhere)
  are NOT required to resolve; existing weight / duplicate-id checks are
  preserved. Negative fixtures under `tests/goals/fixtures/` (missing table;
  row citing a nonexistent `scripts/check-*.sh`) must still be rejected. No
  lane is deleted from `tests/run-all.sh`.
- `skills/validate/SKILL.md` frontmatter description, verbatim target
  (measured ≤ 180 chars in-lane; wording adjusted, never the limit):
  `Freshly judge a finished change against its acceptance: PASS, FAIL, or NOT_PROVEN. Not for claim-vs-tree checks; that is reality-check. Triggers: "validate", "is this proven".`
- `AGENTS.md:38-41`: `tests/run-all.sh` is named as the local aggregate
  runner and must be green; the authoritative checks are CI's, quoted
  verbatim (bats: `tests/scripts/*.bats`). It is not wired into CI (decision:
  CI already runs the same families by path filter).
- L2 is rebased on L1's merged tip before its final run; its validate
  description change regenerates the validate twin and catalog rows, which
  L2 commits.
- Acceptance: `bash tests/run-all.sh` exits 0; `./scripts/regen-all.sh
  --check` runs without a `bash` prefix and is clean; `bash
  scripts/check-shell-exec-bits.sh` exits 0 (and its RED on `main` is
  reproduced by the validator with `git stash`-free means: run it against
  `origin/main` in a scratch worktree); `bash tests/goals/validate-goals.sh`
  passes on `GOALS.md` and fails on each negative fixture; validate
  description length ≤ 180 by `awk`; `cd cli && go build ./... && go vet
  ./... && go test ./...` green; `ao gate check --scope head` green and lists
  the new gate.

**L3 — runtime honesty** (S)
- Read each of rpi, plan, implement, validate and each skill whose SKILL.md
  mentions an `ao` subcommand (bootstrap, cc-hooks, fitness, goals,
  human-only-skills, handoff, status, using-gc). Classify by what the
  procedure executes: HARD (the procedure runs `ao <sub>` or a `.py` script
  and cannot complete without it), OPTIONAL (mentioned; procedure completes
  without it), NONE. Known from the 09-02 read: implement has no python3 or
  `.py` reference (its only script is bash); bootstrap and human-only-skills
  invoke nothing; rpi (`run_once.py`), plan and validate (`validate.py`) are
  HARD python3.
- `README.md`: replace line 25 with a short table (Skill | Needs | Why; HARD
  and OPTIONAL rows only) and one sentence that the plugin and `npx skills add
  --all` install all 56 skills regardless. The "Optional: ao CLI" section
  (~line 123, "Skip it if you only need the skills") is rewritten to agree
  with the table. No promise of absence-handling behavior is made anywhere
  (no skill implements one; cc-hooks fails open on a missing dependency).
  Same table in `docs/install-day2-ops.md`, and its "the skills themselves
  need none of it" sentence and line 43's bare "optional" are reconciled with
  it.
- No skill file is touched in this lane. No em-dashes in added text.
- Acceptance: `tests/scripts/legible-l3-runtime-truth.bats` extracts the
  table rows from both files and asserts they are identical, asserts every
  HARD row's skill file contains an `ao ` or `.py` invocation line
  (word-boundary grep, not substring), and asserts `No other runtime is
  required` is absent under README.md and docs/; `rg -n 'No other runtime is
  required' README.md docs` empty; `ao gate check --scope head` green.
  Semantic judgment for the validator: each HARD/OPTIONAL classification
  against the skill's procedure text (path:line cited per row).

**I-1 — integration subject**
- Merge L1, L3, then L2 onto a fresh `origin/main`; `bash
  scripts/regen-all.sh`; commit the generated outputs alone.
- Acceptance: `scripts/regen-all.sh --check` clean; `ao gate check --full`
  green; the three lane bats files green; the diff of I-1's own commit
  touches only the paths in the I-1 row of the ownership table.

## Non-goals (this plan)

No new `ao` root commands. No ADRs. No change to any `skills/*/SKILL.md`
except validate's description line. No directory moves, deletions, or
retirements (Train 2). No CLI shrink. No probe or measurement work. No
release or version bump. No advisory gate promoted to blocking. No npx
packaging change. Nothing copied from compound-engineering, jsm, or any
`claude -p` route.

## Decisions for Bo (defaults asserted; overrule any)

1. `tests/run-all.sh` stays local-only and green; CI is authoritative.
2. The Codex catalog average limit rises to the measured value rounded up
   (expected well above 45); the alternative — shortening source prose to
   fit 45 — is Train 2 work on skill bodies and is not taken here.
3. Train 2 is a separate intent whose first lane is the `rg`-built consumer
   disposition table, and it does not start before one product skill carries
   a real probe verdict (F11).

## Validation strategy

Per the repo contract: each lane in an isolated worktree off fresh
`origin/main`; a fresh per-lane validator replays the acceptance commands
against the exact branch content and returns `PASS | FAIL | NOT_PROVEN` with
`checked` / `not_checked` disclosed. I-1 gets its own fresh validation.
Cross-family (Codex) review of the integrated train before push. One bounded
repair round; a second non-PASS returns to Bo. One PR, auto-merge on green,
branch updated onto main before merge. Before push: `bash
scripts/check-go-lint.sh`, `ao gate check --full`, `bash
scripts/security-gate.sh --mode quick`.

## First useful check

`tests/scripts/legible-l1-codex-descriptions.bats` assertion (a) on current
`main`: fails listing exactly 51 twins. Runnable before any other work; it is
the regression fence for the projection.
