# Workflow-12 skill checklist

Distilled from `/tmp/agentops-opus5-verified-skill-audit-workflow-12-v8.md`.

**Every item below is a HYPOTHESIS to verify against the live tree** — file:line
citations are the audit's claims, not confirmed facts. Re-check each before acting.

Tags: `[defect]` = factually wrong / broken today · `[enhance]` = contract or
clarity improvement · `[possibly-landed]` = may already be fixed on main by
PRs #995/#996 (ADR-0016 gate), verify first.

Skills (audit order): bootstrap · codebase-recon · handoff · refactor ·
reverse-engineer · sbh · scaffold · shared · status · test · using-gc · workflow-builder

## bootstrap
- [enhance] Declares `effects: []` while doing `filesystem.read/write`; decide the rail then declare truthfully.
- [enhance] Three-way `bootstrap` naming overlap (skill · `ao init` · `ao session bootstrap`) with no cross-reference in any of the three.
- [enhance] No feature file, validator, or test; add a never-overwrite fixture (pre-create `PRODUCT.md` with known bytes, run, assert byte-identity).

## codebase-recon
- [defect] Validator resolves evidence against the wrong repo: `repo_root` derives from the script's own location (`scripts/validate-output.sh:10-11`); add `--repo-root` and default to it.
- [defect] Symlinked invocation breaks root resolution — logical `pwd` through `~/.claude/skills/codebase-recon` yields `.claude`; use `pwd -P`.
- [enhance] The `file:line` evidence floor is prose-only; a directory passes because resolution uses `-e`. Decide whether the manifest validator owns the stricter floor.
- [possibly-landed] Writes `.agents/recon` — noncanonical top-dir vs ADR-0016 closed set.
- [enhance] Declares `effects: []` while doing `filesystem.read/write` + `process.start`.

## handoff
- [defect] `ao session handoff --dry-run` output fails its own `schemas/handoff.v1.schema.json` (3 errors): missing `type`; missing `consumed`; `id` formatted RFC3339Nano `20060102T150405.000000000Z` (`handoff.go:70`) violates `^handoff-[0-9]{8}T[0-9]{6}Z$`. Pick one reconciliation direction, then fix each.
- [enhance] Schema `consumed*` + `$defs.rpi` are stale vs the operating-loop contract; reconcile (likely trim, which may moot the `consumed` requirement above).
- [defect] Archived `tests/claude-code/test-handoff-skill.sh` is stale; exits `SKIPPED` unless `AGENTOPS_ENABLE_CLAUDE_CODE_FUNCTIONAL_TESTS=1`. Fix or delete.
- [possibly-landed] `.agents/handoff` noncanonical top-dir; `initapp.go:28` creates it (product behavior) vs ADR-0016.
- [defect] Schema description names a nonexistent `ao handoff` command.
- [defect] Schema description names a session-start hook that does not exist.
- [enhance] Declares `effects: []` while doing `filesystem.read/write`, `clock.read` (`handoff.go:67`), `process.start` (`:112`), `environment.read` (`git_read.go:18,19`), and a conditional temp-file create/delete lifecycle.

## refactor
- [defect] `references/refactor.feature` grants **commit** authority, contradicting `SKILL.md:47-49`; delete the grant.
- [defect] Same feature grants **automatic revert** authority; delete the grant.
- [defect] Internal contradiction: `SKILL.md:74-75` says a hash mismatch is "to explain or revert" while `:47-49` says the skill does not revert. Resolve this FIRST (SKILL.md governs the feature rewrite).
- [defect] Feature header declares `consumes: complexity / produces: git-changes` against the real frontmatter (metadata drift).
- [defect] Dangling `/complexity` route; remove or implement.
- [enhance] No scenario coverage (0/4); rewrite the four scenarios with `@covered-by`.
- [enhance] Neutrality gates (golden-output hashing, "no vanished red") have no behavioral witness; build a neutrality harness.
- [enhance] Declares `effects: []` while doing `filesystem.read/write` + `process.start`.

## reverse-engineer
- [defect] Undeclared reserved-lane writes rooted at `Path.cwd()` (`.agents/council/*`, `.agents/learnings/*`, `:1845-1871`) land outside `--output-dir`; confine under `--output-dir` and remove from reserved lanes.
- [defect] Hard crash without `.agents/learnings/`: `_ensure_dirs` at `:1846` creates `council_dir` only → `FileNotFoundError` at `:1855` (default state of a fresh checkout). Create `learnings_dir` or drop the write.
- [defect] Security gate certifies placeholders: `validate_security_audit.sh:46-53` greps only `^Evidence:` / `^(Fix|Remediation):`, which the template supplies as literal `_TBD_`; reject `_TBD_`.
- [defect] Row-26 learning file is canned, run-independent fabricated content written into the Learn corpus (`:1855-1871`); delete it.
- [enhance] Declares `effects: []` despite network (`git clone`, `urllib`), caller-supplied target execution, extraction, and out-of-root writes.
- [enhance] Repo cloning is not authorization-gated; `--authorized` check at `:1612` is binary-mode only. Gate repo cloning or narrow the guardrail sentence.
- [defect] Archive materialization defaults ON (`:1665`) despite the "index only" guardrail; flip the default or state it.
- [defect] `SKILL.md` Output Specification is incomplete and mis-conditioned; rewrite to match actual producers (two producers of `spec-cli-surface.md`; `feature-registry.yaml` write vs in-place rewrite; reserved-lane writes; archive/SBOM internal outcomes; scratch explicitly out of scope).
- [defect] Clone-metadata contradiction: `SKILL.md:97` says `clone-metadata.json` is written "only when `--upstream-ref` supplied" but code writes on any first clone (`:1528-1543`); `SKILL.md:176` matches the code, so `:97` is wrong.
- [enhance] `scripts/validate.sh` is only `py_compile` over 8 files; make it run a hermetic behavioral subset.
- [possibly-landed] Writes `.agents/research` — noncanonical top-dir vs ADR-0016.
- [defect] Machine-absolute path baked into the durable wrapper (`:1420-1422`).
- [enhance] Degenerate golden fixture; strengthen.
- [defect] Network-dependent regression test (flaky); make hermetic.
- [defect] Dead `spec-cli-surface.md.tmpl` template.
- [enhance] Unbounded ZIP decompression on attacker-controlled input (`extract_embedded_archives.py:92`); bound it (zip-bomb risk).
- [defect] Gemini projection ships instructions without `scripts/` (broken projection).

## sbh
- [enhance] Widest mutation boundary of the twelve (host mutation + deletion) yet declares `effects: []`; declare the destructive surface, naming deletion (no v3 `filesystem.delete` kind exists).
- [enhance] Irreversibility ordering has no executable witness; add a `.feature` + fixture rejecting a transcript that jumps straight to `emergency --yes`.
- [enhance] External SBH binary contract is unpinned; pin the version.

## scaffold
- [defect] `references/generic-templates.md:189-199` "Step 5: Initial Commit" instructs a git commit, denied by `SKILL.md:34,79,90`; delete Step 5.
- [defect] `references/generic-templates.md:353-356` prints a "Next steps:" continuation block, separately denied; delete it.
- [defect] Validator is structurally blind: `scripts/validate.sh:5` binds `SKILL="$SKILL_DIR/SKILL.md"` and every check targets only that file — `references/**` is never swept, so the two grants above pass. Extend the forbidden-token sweep across `references/**`.
- [enhance] No scenario coverage (0/4); add `@covered-by` tags.
- [enhance] Declares `effects: []` while doing `filesystem.read/write` + `process.start`.
- [enhance] Exemplar-selection and one-way-sync disciplines each have no witness; witness them separately.

## shared
- [enhance] Plural `produces` wording could be clarified (only substantive item; the skill is otherwise clean).

## status
- [defect] `SKILL.md:34-36` overclaims live output — claims subject-manifest reporting plus per-artifact digests and timestamps, but the command emits two counts + one newest-mtime and hard-codes "caller-supplied subject manifests" into `NotChecked`. Delete the overclaim or implement it.
- [enhance] Frontmatter `effects: []` contradicts the command's own Go contract (`EffectFilesystem | EffectClock`); mirror the Go declaration.
- [enhance] Empty `references/` held open only by `.gitkeep`.
- [enhance] `model: haiku` is unexplained.

## test
- [defect] Skill mandates `check-scenario-coverage.sh --run` as its proof mechanism, but its own feature fails 0/3 and the checker is not wired as a corpus gate. Wire it or stop mandating it.
- [defect] `scripts/validate.sh` is vacuous — proves only that five words/shapes exist in `SKILL.md` prose (5/5 pass even after deleting the Workflow section). Replace prose greps with fixture-driven checks.
- [defect] `produces: [result.json]` is unbacked by the Output Specification and by the tree; correct `produces`.
- [enhance] The three core doctrines (oracle-strength hierarchy, mutation-kill proof, harness-health floors) each lack a behavioral witness; build three separate witnesses.
- [possibly-landed] Writes `.agents/tests` — noncanonical top-dir vs ADR-0016.
- [enhance] `golden-artifacts.md` and `golden-artifact-strategy.md` overlap substantially; merge or disambiguate.
- [enhance] Declares `effects: []` while doing `filesystem.read/write` + `process.start`.

## using-gc
- [enhance] Dispatch-once and one-wake-maximum disciplines each lack a behavioral witness; add a `.feature` for "re-dispatch of an `in_progress` bead is a no-op" and a separate one for "one wake maximum, then stop".
- [enhance] Version-pinned external claims (v1.3.5 / issue #4586) have no local guard; add a version/compatibility probe.
- [enhance] `tmux -L <socket>` appears at three call sites without naming that the socket comes from `mayor status`; name the source once.
- [enhance] Conditional `host.configure` (`SKILL.md:78-83` adds exact-path `trust_level` entries to `~/.codex/config.toml`) is undeclared even though `effects` is a legal non-empty legacy string; declare it.

## workflow-builder
- [enhance] Declares `effects: []` while doing `filesystem.write`; declare it.
- [enhance] The at-most-once dispatch invariant has no witness; add one (dry-run/fixture asserting exact dispatch count).

---

_No lower-priority items dropped; all 67 supported findings retained. Excluded per
scope: the audit's provenance/SHA bookkeeping, v6→v8 correction ledgers, review-process
meta, withdrawn/refuted findings, and proof-epoch/verdict-machinery process items._
