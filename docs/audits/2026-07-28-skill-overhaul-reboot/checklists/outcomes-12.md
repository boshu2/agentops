# Outcomes-12 skill checklist

Distilled from the twelve-skill technical audit (`outcomes-12` v13). **Items are hypotheses to verify against the live tree** — every file:line citation below is the *audit's* claim, not a confirmed fact; open the file before acting. Verdict-machinery, proof-epoch, packet-freeze, and provenance/lineage bookkeeping from the source doc are intentionally omitted. `[defect]` = wrong/broken today; `[enhance]` = contract/clarity gap; `[disputed]` = the audit's final review flagged this class as unreliable — verify before trusting.

Skills (audit order): `automation-shape-routing` · `craft-goal` · `doc` · `domain` · `goals` · `operationalize` · `pattern-mining` · `product` · `security` · `skill-builder` · `standards` · `toil-mining`

## automation-shape-routing
- [defect] `skill.spec.json` is a stale second metadata source contradicting `SKILL.md` on 5 fields — sections (8 declared / 0 present), `context_rel` (2 vs 5), `produces` (`[]` vs `[automation-shape-verdict]`), invocability, triggers; its `evidence.sources` cites out-of-repo `~/dev/agentops-3cat-spike/` (unverifiable). Delete or regenerate under a parity check. (P1-ASR-SPEC)
- [defect] `user_invocable` vs `user-invocable` spelling mismatch. (P2-INVOCABLE)
- [enhance] Add a spec↔`SKILL.md` parity check to the corpus gate covering all 3 spec-carrying skills (only 3 of 49 carry `skill.spec.json`).

## craft-goal
- [enhance] `scripts/validate.sh` is a 3-line shim to `heal.sh --check --strict` (structural only); the declared `output_contract` (3 terminal tokens `SAFE_TO_CREATE|USE_RPI|UNSAFE_GOAL` + 14 lint dims) has no validator. Add a real output validator. (P1-CG-VAL)
- [enhance] The continuation-authority boundary of the emitted goal artifact is never stated — what it may authorize a caller-owned runtime to do vs not. (P1-CG-BOUNDARY)
- [enhance] No `context:` block; `bd`/`br`/`bv` coupling is real but undeclared — declare as `context_rel` or external; keep `stability: experimental` until validator + boundary land.
- [disputed] The audit's craft-goal *strength* claims (e.g. "highest craft score of the twelve", "provenance resolves 2/2") — the final review flagged a false craft-goal strength claim; do not rely on these without re-checking.

## doc
- [defect] `effects: []` is false — doc is the most write-heavy of the twelve (`README.md`, root OSS docs, `.agents/doc/YYYY-MM-DD-<target>.md`). Declare real effects. (P0-EFFECTS)
- [defect] `references/validation-rules.md:134-145` ships `--create-issues` emitting `bd create` / `gh issue create`; `references/default-mode.md:172` ships a `## Next Steps` action-box template — both confer work-creation authority doc disclaims. Delete both. (P1-N-D)
- [defect] Seven stale surface names (five skill names, `subagents/explorer.md`, the script's `/oss-docs scaffold` hint). Fix or remove. (P1-N-E)
- [defect] `.agents/doc/` violates the ADR-0016 closed layout (in `SKILL.md`, repeated in `references/default-mode.md`); no live detector. Relocate. (P1-LAYOUT)
- [defect] Hygiene batch: `scripts/audit-oss-docs.sh` unlinked from `SKILL.md` and uses bare `set -e` vs required `set -euo pipefail`; 7 `#` H1s in `references/architecture-report.md`; unclosed fence in `references/oss-pack.md`; 3 `.feature` files lack `@covered-by`. (P2-N-H)
- [disputed] `scripts/validate.sh` characterized as "14 checks, all greps of doc's own prose / cannot fail on behavior" (P1-DOC-VAL) — the final review flagged the doc-validator description as inaccurate. Re-read the actual validator before acting.

## domain
- [enhance] No validator of any kind (package is a single `SKILL.md`, no `scripts/`). Add `scripts/validate.sh` asserting both cited contract paths exist AND the looked-up term still resolves inside the cited file, so `audit.sh` has an entry point. (P2-CEREMONY)

## goals
- [defect] `effects: []` is false — writes `.agents/ao/goals/baselines` snapshots (`RunMeasure`/`RunExport`/`RunDrift`) plus `render --out`; destination is canonical `ao/` so this is a declaration defect. Declare, qualified: snapshot write is reached only on the successful ordinary non-directives `measure` path and is best-effort. (P0-EFFECTS)
- [defect] Eight live subcommands documented as six (`cli/internal/commands/goals/module.go:112-119`) — `scenarios` and `render` omitted; `produces: [result.json]` contradicts `output_contract` and the live artifact set. (P1-GOALS-DRIFT)
- [enhance] State explicitly that the goals *source* is never mutated (so read-only intent survives the effect declaration); add a command-tree parity check so six-vs-eight drift can't recur.
- Note: the v2 P0 that `ao goals … --json` does not exist is REFUTED — `cli/cmd/ao/root.go:153` registers a global persistent `--json`. Do not carry it.

## operationalize
- [defect] `produces: [operationalization-proposal.v1]` names a versioned artifact with no schema, no path, no filename convention, no validator (package is 1 file); the `.v1` suffix promises a contract that doesn't exist. Define it or drop the suffix. (P1-OP-V1)
- [enhance] `effects: [write_advisory_proposal]` vs body "Return the proposal to the caller" — the mutation boundary is unknowable from the contract. Resolve which it is. (P1-OP-RETURN)
- [enhance] No `context:` block; `user-invocable` sits at top level while the rest is under `metadata` — normalize. Any validator built for P1-OP-V1 must assert the three-instance floor + reapply proof, not just schema shape.

## pattern-mining
- [defect] `effects: []` is false — writes `pattern-mining.v1` JSON at `.agents/patterns/<run-id>/`. Declare the write. (P0-EFFECTS)
- [defect] `.agents/patterns/` violates the closed layout. Relocate under a permitted tier. (P1-LAYOUT)
- [enhance] `jq` is an undeclared dependency and fails closed silently under `set -euo pipefail` — emit an explicit error when absent.
- [enhance] Register `skills/pattern-mining/scripts/validate-output.sh` in `standards`' canonical-owners table as the reference output-contract validator (audit found 0 references to it — the corpus's one working example is undiscoverable). Note: this is the best-validated skill; declined further behavioral work.

## product
- [enhance] No validator, no test — the byte-for-byte preservation rule is machine-checkable but unchecked. Add a preservation test that runs a scoped refine through the *real* writer against a fixture `PRODUCT.md` and asserts untouched sections hash identically (and that the target section actually changed). (P1-PROD-BYTE)
- [enhance] The `aspiration laundering` failure mode has no detector — require explicit `## Proven` / `## Assumptions` headings and assert every `## Proven` claim carries a resolvable citation.
- [enhance] No `context:` block.

## security
- [defect] `scripts/security-gate.sh` is a skill-local duplicate one directory deeper, so `SCRIPT_DIR/..` resolves to `skills/security` and quick mode exits 1 on missing `scripts/toolchain-validate.sh`. Delete the skill-local duplicate (keep `SKILL.md:52/:67/:77` documenting the root path). Both copies hash `deeea2c…6ee3f7`. (P0-SEC-GATE)
- [defect] The shipped redteam attack pack fails 4/6 (exit 3) against this landing — `docs/strategic-direction.md` no longer exists. Refresh against current surfaces or retire the dead cases. (P0-SEC-REDTEAM)
- [defect] `produces: [security-report.json]` — no code path writes that filename. (P0-SEC-PRODUCES)
- [defect] `.feature` files + OWASP checklist assert release-gating / "Block merge" authority the skill and `AGENTS.md` disclaim. Strip. (P0-SEC-RELEASE)
- [defect] `effects: []` false and *pinned* false by `scripts/validate.sh:9` (`grep -q '^  effects: \[\]$'`) — declaring real effects turns the skill's own validator red, so remove the `:9` grep in the same change. (P0-EFFECTS)
- [defect] `scripts/validate.sh` is prose-grep + `py_compile` + JSON-parse and exits 0 while the gate and redteam are broken. Make it behavioral (run root gate; assert duplicate gone; run redteam). (P1-SEC-VAL)
- [defect] OWASP cites `/validate --preset=security-audit` and `/postmortem --scope security` — the `validate` and `postmortem` skill contracts expose neither. (P1-OWASP)
- [defect] `validate.sh:19` `py_compile` writes `__pycache__/*.pyc` into the package (`cfile=None` still writes the default) — use `ast.parse` or an explicit temp `cfile` outside the package with cleanup. (P2-N-I)
- [defect] Misc: features say `supplier-to vibe` (now `validate`); `prompt_redteam._evaluate_file` can never return `WARN` yet two aggregators test for it; `security_suite.shutil_which` shells `bash -lc`, sourcing the login profile inside a security tool; `glob.glob(root_dir=…)` needs Python ≥3.10, undeclared.

## skill-builder
- [defect] Five execution-path Python files fail the accepted ADR-0016 `skill.python-ratchet` at upstream scope: `check_migration_readiness.py`, `compile_contracts.py`, `contract_v3.py`, `probe_runtime.py`, `run_contract_probe.py`. None grandfathered; allowlisting is rejected by the growth guard. Route into the `ao` binary with `sh` glue. (P0-RATCHET)
- [defect] The build-report write is undeclared: `scripts/init.sh:149-150` creates `.agents/audits/` + binds `<slug>-build.json`, `scripts/build.sh:32` rewrites it, `SKILL.md:316` declares the path — no API1 or shadow effect covers it (`artifacts.produces: build-report` does NOT discharge the effect). Declare it. (P0-EFFECTS)
- [defect] Five artifacts specify an auto-fixing `heal.sh` that does not exist. Build it or retract from all five. (P0-HEAL-N-A)
- [defect] `scripts/test-mutation-boundaries.sh` is red at its first assertion (`fix_rc=2`, expected 1), never reaches sibling-isolation/traversal assertions, and is not wired into `validate.sh`. Repair owner = the plan's D7 transactional projection owner (not "mirror codex-sync"). (P1-N-B)
- [defect] `heal --fix` regenerates the mesh unscoped. (P1-HEAL-MESH)
- [defect] `.agents/audits/` violates the closed layout — implemented, not merely documented. (P1-LAYOUT)
- [defect] Carried P1s: heal checks only a subset of claimed fields (P1-HEAL-SUBSET); heal crash ≡ ordinary findings, non-strict swallows both (P1-HEAL-CRASH); lifecycle guards fail open without `rg` (P1-GUARD-RG); `skill-conformance-profiles.yaml` load-bearing but unlinked (P1-PROFILE); `init.sh` omits `output_contract` (P1-INIT-OC).
- [defect] P2 batch: dual kernel-length standards (250-line cap vs 220/500/800 bands) with own `SKILL.md` at 353 lines tripping the cap (`audit.sh` WARN); `scan_descriptions.py --probe` unexercised (0 probe pairs across 49 skills); `craft_score.py` false-positive classes; `test_contract_v3.py` writes into the live `skills/` tree; `audit.sh` requires bash ≥4; `validate.sh` doesn't set `PYTHONDONTWRITEBYTECODE`. (P2-N-F)

## standards
- [defect] Owns the frontmatter template that seeds `effects: []` corpus-wide (`references/skill-structure.md:45`) — the mechanical origin of 5 of the 6 `P0-EFFECTS` members. A second canonical template `skill-builder/references/skill-template.md` omits `effects` entirely and `skill-builder/scripts/init.sh:56` (`effects="${SKILL_EFFECTS:-[]}"`) disagrees with both; a new skill's frontmatter depends on which of the three surfaces its author followed. Correct/annotate the `effects` line. (P0-TEMPLATE)
- [enhance] Reconcile all three surfaces to one declared owner under a *semantic* parity check (nested key paths AND default-value meaning, not key presence) asserting `effects` is deliberate and `output_contract` is emitted by `init.sh` + checked by `heal.sh`. (companion to P1-INIT-OC)
- [defect] Dead `#dedup-manifest` TOC anchor; `javascript.md` missing from the canonical-owners table though it exists + is linked; duplicated Security/General sections in `python.md`. (P2-N-G)
- [enhance] No validator — add a reference-link resolution gate (15/15 links resolve today only by reading); register `pattern-mining/scripts/validate-output.sh` in the owners table (0 references today — direct cause of the unvalidated-output cluster in craft-goal / operationalize / product).

## toil-mining
- [defect] `effects: []` is false — conditionally writes `.agents/toil-mining/YYYY-MM-DD-candidates.md`. Declare the conditional write. (P0-EFFECTS)
- [defect] Three-way output mismatch: `produces: [result.json]` (never written) vs `output_contract` (`.agents/toil-mining/`) vs the body's inline-by-default `candidates.md`. Reconcile to actual behavior. (P1-TOIL-OUT)
- [defect] `.agents/toil-mining/` violates the closed layout. Relocate. (P1-LAYOUT)
- [defect] The ranking contract is prose-only (`SKILL.md:55-66` three-occurrence floor; `:67-83` `frequency × cost × error-proneness` product) — `recent_human.py` does no clustering and no scoring, so the headline contract has no machine boundary. Add a fixture-based ranking validator. (acceptance of P1-TOIL-OUT)
- [defect] `user-invocable: false` while the description advertises caller trigger phrases. (P2-INVOCABLE)
- [defect] `audit.sh` WARN (`constraints-frontloaded`, `quality-rubric`) — `SKILL.md` has no `## Constraints` heading. Add the heading + a third quality bullet.

---
No lower-priority items dropped beyond the excluded verdict/provenance machinery; the per-skill "declined deliberately" non-improvements, five-improvement padding, and residual-risk ratings were folded in or omitted as non-actionable.
