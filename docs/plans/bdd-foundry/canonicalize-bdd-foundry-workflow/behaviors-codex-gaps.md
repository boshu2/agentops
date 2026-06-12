# Cross-Family Gap Review: canonicalize-bdd-foundry-workflow

Independent review of `behaviors.md`. Ranked by how badly an implementer could
ship a broken canonicalization while still satisfying the written scenarios.

Applied prior review context:
- `.agents/learnings/2026-05-30-claim-green-verify-first.md` influenced gap 15.
- `.agents/learnings/2026-06-12-epic-close-needs-target-disposition-not-children-closed.md` influenced gap 13.
- `.agents/learnings/2026-06-12-go-exec-waitdelay-grandchild-pipes.md` was reviewed and did not apply to this Markdown behavior audit.

## Top 15 Missing Behaviors

1. **Immutable pre-copy source snapshot is not required before any mutation.**
   - Current hole: S2 compares against `~/.claude/workflows/bdd-foundry.js`, but that path is also the installed target changed by S6.
   - Broken-but-passing implementation: edit or overwrite `~/.claude/workflows/bdd-foundry.js` first, then copy it into the repo; the canonical file matches the mutated source and S2 passes while the original v5 source is lost.
   - Missing behavior: before any write to canonical or installed paths, save a timestamped source snapshot and SHA256 in the plan dir; final canonical content must equal that immutable snapshot except for the single HAZARD-line replacement.

2. **Same-version divergent reconciliation can drop real code while preserving lineage comments.**
   - Current hole: E2 only requires a saved diff, all distinct `// v` lineage lines, and `node --check`.
   - Broken-but-passing implementation: merge only the header comments from a second v5 claimant and silently discard changed functions, prompts, schemas, or gates.
   - Missing behavior: every non-comment hunk in `reconciliation.diff` must have an explicit disposition (`kept`, `superseded`, or `rejected with reason`), and the canonical file must include or justify every semantic hunk before passing.

3. **No behavior requires discovering all candidate workflow copies before declaring a winner.**
   - Current hole: E1/E2 handle newer or divergent files only if the implementer already knows they exist.
   - Broken-but-passing implementation: inspect only `~/.claude/workflows/bdd-foundry.js`, miss another worktree or saved copy with v6/v5 changes, and still claim "single v5 source".
   - Missing behavior: record the candidate search over `~/.claude/workflows/`, repo `.claude/workflows/`, `git worktree list` checkouts, and plan snapshots; the run log must list every claimant and the chosen winner.

4. **The normal install/update path is not proven; manual repair can satisfy S6.**
   - Current hole: S6 says "the install step applied" but does not name the command or prove a clean install produces the follow mechanism.
   - Broken-but-passing implementation: hand-create a symlink or copy on Bo's machine while leaving `scripts/install.sh` / Codex install scripts unaware of workflows.
   - Missing behavior: run the exact supported installer or update command in an isolated `HOME` fixture, then assert it creates `~/.claude/workflows/bdd-foundry.js` following the canonical repo file.

5. **The drift/freshness check can exist without being blocking automation.**
   - Current hole: S7 proves a named command can fail red, and X4 says failure is visible in "whatever validates installs", but no exact gate invocation is required.
   - Broken-but-passing implementation: add `scripts/check-workflow-drift.sh` that fails in a manual fixture, but never call it from the cockpit gate, install validation, or CI.
   - Missing behavior: a named blocking command such as `ao gate check --fast --scope head` or a specific validation script must invoke the drift check; the mutation fixture must make that parent command exit non-zero.

6. **Existing installed-copy local changes are not protected from overwrite.**
   - Current hole: S6/S7 focus on the final follow relationship, not data-loss safety during replacement.
   - Broken-but-passing implementation: blindly overwrite a regular `~/.claude/workflows/bdd-foundry.js` that contains local v6 edits; after overwrite, cmp/readlink passes.
   - Missing behavior: if the installed file is a regular file that differs from the chosen source/canonical bytes, the installer must either refuse with an actionable message or back it up to a named path before replacing it.

7. **Clean-home installation is not covered.**
   - Current hole: all scenarios assume `~/.claude/workflows/` already exists on Bo's machine.
   - Broken-but-passing implementation: update only the existing local directory; a fresh user with no `.claude/workflows` directory gets no workflow.
   - Missing behavior: in a temp `HOME` with no `.claude/workflows`, the installer creates the directory and installs/follows `bdd-foundry.js` without requiring pre-existing workflow state.

8. **Install/follow idempotency is not required.**
   - Current hole: S6 validates one final state only.
   - Broken-but-passing implementation: first install passes, second install creates nested symlinks, duplicate config entries, stale copies, or a non-zero exit.
   - Missing behavior: running the install/follow step twice leaves the same readlink/cmp result, no duplicate registration, and no new repo dirty state.

9. **Worktree isolation is conditional, but the repo contract requires it for every change.**
   - Current hole: E4 protects a dirty main checkout; E3 assumes a worktree but does not fail the implementation if work happened in main when it was clean.
   - Broken-but-passing implementation: edit the shared main checkout directly on a clean tree, then land the change; final file checks still pass while violating the shared-checkout discipline.
   - Missing behavior: the run log must show the implementation worktree path and branch, and the main checkout must remain unchanged by this lane except for explicitly allowed plan artifacts.

10. **Hard-coded local paths can satisfy the current assertions.**
    - Current hole: scenarios use `/Users/bo/dev/agentops` and `~` throughout, but do not test repo-root or HOME parameterization in install/freshness code.
    - Broken-but-passing implementation: hard-code Bo's checkout path into the installer or drift check; it passes here and fails for any clone, temp fixture, or moved checkout.
    - Missing behavior: installer and drift checks must run in a temp repo/HOME fixture or accept explicit `--repo-root`/`--home` arguments; output must prove they did not depend on `/Users/bo`.

11. **Marker greps do not prove the critical workflow gates still enforce anything.**
    - Current hole: S4 checks strings like `DRIFT_SCHEMA`, `DIR-MISAIM`, and `beads.json`, but a marker can remain in comments while the fail-hard branch is removed.
    - Broken-but-passing implementation: leave the marker strings in comments, but remove the `DIR-MISAIM` throw, allow tracker writes when `driftOk` is false, or stop requiring `gap_dispositions`.
    - Missing behavior: static assertions must cover the actual enforcement patterns: `DIR-MISAIM` throws before later phases, tracker write requires `score >= threshold && cycleFree && uncovered.length === 0 && driftOk`, and frozen behaviors require `gap_dispositions`.

12. **LAW 0 is checked only for recorded verification commands, not changed executable surfaces.**
    - Current hole: X3 inspects commands in plan/PR evidence, but not newly edited install scripts, drift scripts, or the canonical workflow body.
    - Broken-but-passing implementation: add a helper script that invokes `claude -p` during install or validation; recorded evidence avoids that path, so X3 passes.
    - Missing behavior: every changed executable file and `.claude/workflows/bdd-foundry.js` must be scanned for executable `claude -p`, `claude --print`, and `gemini -p` invocations, with documented comment-only exceptions.

13. **Bead citation and closure do not cover the whole landed arc.**
    - Current hole: S8 requires the commits that add/change `.claude/workflows/bdd-foundry.js` to cite the bead, but install/freshness commits can be uncited and the bead may remain open after landing.
    - Broken-but-passing implementation: cite the bead only on the workflow-file commit, land uncited installer changes, and leave the bead `in_progress`.
    - Missing behavior: every commit in the canonicalization arc that touches allowed paths must cite the same `ag-*` bead, and after landing the bead must be closed with evidence naming the final commit.

14. **Pre-existing sibling workflow drift is not dispositioned.**
    - Current hole: the plan names live `bead-crank.js` drift and says fixing it is out of scope, but S6/S7 also imply a same mechanism for repo workflows.
    - Broken-but-passing implementation: either widen the change by fixing `bead-crank.js`, or avoid a real global check because it would fail on known drift.
    - Missing behavior: the drift check must have an explicit scoped mode for `bdd-foundry.js` and a report-only disposition for pre-existing sibling drift, or the sibling remediation must be filed as a separate bead before any global gate is made blocking.

15. **Verification evidence is not anchored to the final landed content.**
    - Current hole: scenarios require commands to pass, but not that they ran after the last edit or against the commit that landed on `main`.
    - Broken-but-passing implementation: run `node --check`, drift mutation, and gate checks on an earlier candidate, then edit install wiring or the workflow file and land unverified bytes.
    - Missing behavior: evidence must record the final HEAD SHA, canonical file SHA256, installed-copy/readlink result, and gate command exit after the last file change; the landed commit must match those recorded hashes.
