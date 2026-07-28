# adapters-13 — skill improvement checklist

Distilled from the Opus 5 adapters-13 deep audit (technical sections only; provenance/verdict-machinery dropped).

**Every item below is a HYPOTHESIS to verify against the live tree.** File:line citations are the
AUDIT's claims, not confirmed facts — open the file and confirm before acting. Tags: `[defect]` =
factually wrong/broken today; `[enhance]` = contract/clarity/safety improvement.

**Skills covered (audit order):** account-rotation, agent-mail, agent-native, agy-native, cass,
cc-hooks, codex-exec, converter, dcg, ms, ntm, rch, swarm.

> Recurring systemic threads: `S1` false `metadata.effects: []` (10 skills — each "declare effects"
> item); `S2` missing `output_contract` in frontmatter (cass, cc-hooks, dcg, ms); `S3` thin proof
> layer stated per actual proof owner (account-rotation, agent-mail, ntm, rch); `S4` cross-adapter
> model-dispatch contract privately owned by one consumer (agent-native, codex-exec, ntm); `GOV-1`
> grandfathered execution-path Python under the shrink-only ADR-0016 ratchet (agent-native, cass, ms, swarm).

## account-rotation
- [defect] `metadata.effects: []` false — mutates host credential state; declare real effects (S1).
- [defect] operator-private `claude-acct`/`caam` route baked into a shipped skill (AR-3, N17); document a consumer route or mark operator-only.
- [enhance] both-tools-absent behavior undefined — must report absence as a disclosed fact and stop, never fall back to byte comparison (AR-4).
- [enhance] `standalone` obligation, before/after identity + cleanup receipt, and partial-rotation proof are all absent (S3).

## agent-mail
- [defect] untyped admin/disaster-recovery surface — `clear-and-reset-everything --force --no-archive` + `install_precommit_guard` reachable with no typed mode or caller-authority boundary (AM-1, P0). Split typed coordination modes from an explicitly caller-authorized admin/DR mode.
- [defect] `effects: []` false — writes durable AM rows plus a git pre-commit hook and a full destructive reset; declare effects (S1).
- [defect] retired `uv run python -m mcp_agent_mail.cli` entrypoint in `RECOVERY.md`; rewrite onto the `am` CLI (N18).
- [enhance] state "advisory" in the same sentence as any collision guarantee — reservation semantics currently stated two ways (AM-3).
- [enhance] `force_release_file_reservation` has no authority precondition (AM-4); `consumes: [task-intent]` on a transport (AM-6).
- [enhance] T7a: identity/reservation/ACK/TTL/conflict/degraded results untyped; capability-unavailable behavior unstated; cleanup unobservable (→ NOT_PROVEN).

## agent-native
- [defect] `fake_model_runner.validate_cross` accepts equal `--author-context-id`/`--validator-context-id` with no equality rejection, carries no intent or subject manifest digest, may set `validator_model = author_model`, yet still emits `freshness_attestation` (AN-2). Reject equal context IDs and require intent+subject digests before emitting any attestation.
- [defect] `diversity_unsatisfied` routed to a sidecar because the challenge validator forbids unknown top-level keys; admit it into the challenge schema (AN-3).
- [defect] wall-clock `utc_now()` makes fixture output non-reproducible — accept `--generated-at`/`SOURCE_DATE_EPOCH` (AN-4).
- [enhance] hosts the cross-adapter model-dispatch contract two suppliers cite; move `model-dispatch.md` to `docs/contracts/` (S4).
- [enhance] `tier: meta` against an undefined tier vocabulary (AN-5); `fake_model_runner.py` is grandfathered Python under `scripts/` — move to `tests/` to discharge the debt at zero cost (GOV-1).

## agy-native
- [defect] canonical `SKILL.md` (50 lines) not reconciled with its live root adapter `scripts/lib/codex-exec.sh` (520 lines), tests, or eval use — name the adapter, the tier ruling, and the permission posture (AGY-R).
- [defect] prescribes live discovery but names no discovery command (AGY-2).
- [defect] `codex_exec_timeout_cmd` degrades to unbounded execution when neither `timeout` nor `gtimeout` exists — fail closed or explicitly disclose the missing timeout (LIVE-1, shared with codex-exec).
- [enhance] AGY-absent path unspecified (AGY-3); thinnest kernel in the set for a full validator role (AGY-4).
- [enhance] `--sandbox --dangerously-skip-permissions` posture undisclosed in the canonical skill — a reader cannot discover the posture they get (LIVE-2).

## cass
- [defect] `effects: []` false — index rebuild, `doctor --fix`, ~90MB model download, ssh/rsync fan-out, `pages encrypt`; declare effects, separating local-index from remote-ssh (S1).
- [defect] `SKILL.md:230` "None mutate state without explicit confirmation" contradicted by `recover.sh` autonomous index refresh, `doctor --fix`, force-rebuild (CASS-M).
- [defect] `quick_analysis.sh:64` runs `cass index --json` unbounded, violating `SKILL.md:140`; wrap in `timeout` (N8).
- [defect] `casr` folded-in vs "invoke the standalone skill"; malformed `../..casr/SKILL.md`; no `skills/casr` exists — resolve (N7).
- [defect] stale `skill.spec.json`; `PROMPTS.md` anchor points at a removed section (N6). Always-failing "universal extractor" in `SESSION_FORMATS.md` (N14); `.summary` self-contradiction in `RECOVERY.md` (N15).
- [defect] `TMPDIR` shadowing + temp-dir leak in `multi_machine_search.sh` (N19).
- [enhance] add `output_contract` to frontmatter (S2); add the `post_verdict` seam — entirely absent from the skill.
- [enhance] `prompt_miner.py` grandfathered Python debt with an `ao` destination (GOV-1).

## cc-hooks
- [defect] three-way default-on / inert / ships-by-DEFAULT contradiction across `SKILL.md:279`, `:34`/`:170`, and `GUARDRAIL-VALUE-PROOF.md` — reconcile the doctrine across all three (CH-1, P0).
- [defect] default cohort asserts host-global deny over explicit `_beads` paths in unrelated repos — legit `git add _beads/notes.md` blocked (rc 2) in a non-AgentOps repo; move repo-internal `_beads`/`ledger.jsonl` policies out of the default user-scope cohort (CH-2).
- [defect] `effects: []` false — writes `~/.claude/hooks/aop/*`, rewrites `~/.claude/settings.json`, appends telemetry, writes session sentinels; declare effects (S1).
- [defect] pure predicates fire on quoted data; no false-positive test despite a pre-registered CUT criterion requiring one — §2.3's blocked probe is a ready-made fixture (N12).
- [defect] `installed-skill-edit-guard.sh` calls `jq` with no preflight — fails open by accident (CH-7).
- [enhance] no uninstall/cleanup path specified or tested — add and test hook removal (target gap).
- [enhance] add `output_contract` (S2); `skill.spec.json` hard-dependency divergence from frontmatter (CH-3); shipped guard message names the operator's factory repo (CH-6); `PATH`-clobbering hooks recipe in `PATTERNS.md` (N13).

## codex-exec
- [defect] `effects: []` false — process execution at a declared tier incl. `workspace-write` and optionally network; make effects sandbox-tiered and checkable against the invoked `-s` flag (S1).
- [defect] `codex_exec_timeout_cmd` degrades to unbounded execution with neither `timeout` nor `gtimeout` — fail closed or disclose (LIVE-1).
- [defect] `consumes: []` while the skill requires a caller packet (CE-4); two overlapping trigger vocabularies — collapse to one source (CE-5).
- [enhance] cite `scripts/lib/codex-exec.sh` (520 lines) as the implementation and `codex-exec-lib.bats` as its lock — neither is currently cited (S4).
- [enhance] T7a: wall-clock/output bounds and process-tree cleanup implemented in the shell lib but typed in neither skill nor a result schema; cancellation unspecified. Type the run result.

## converter
- [defect] `rm -rf "$output_dir"` (convert.sh:631) uncontained over a caller-supplied path — destroys the source package (W3: 4→2 files, exit 0). Refuse an output dir that is the source, an ancestor, the repo root, or outside an allowlisted root — by refusal, not by silently appending `/$BUNDLE_NAME` (CV-1, P0).
- [defect] `codex_rewrite_text` deletes whole lines matching runtime tokens on every copied `.md`; parity check compares only entry names (W1). Rewrite instead of delete; make parity content-aware (N1, P0).
- [defect] not the shipped Codex-projection owner — two independent rewriters, no parity test; `codex-sync.sh` already owns the shipped path. State the projection-ownership boundary (N2).
- [defect] validator vacuous — "2 passed, 0 failed" while the skill can delete its own source; replace it (CV-2).
- [defect] `collect_files()` non-recursive vs `copy_passthrough_resources()` recursive (W2) — make recursive or declare the limit (CV-7).
- [defect] `local -n` (convert.sh:283-284) is a Bash ≥4.3 nameref; on `/bin/bash` 3.2.57 it's an invalid option and `set -euo pipefail` (line 5) aborts exit 2, zero files — no bash-version precondition, so unrunnable on stock macOS. Drop the nameref or add a `BASH_VERSINFO` preflight (CV-8; fails closed, portability/diagnosability fix).
- [defect] `skill-bundle-schema.md` contradicts the code on frontmatter parsing and both live adapter behaviors (N9).
- [enhance] unquoted `description:` yields invalid Cursor YAML (CV-9); Codex-specific rewrites applied to Cursor and `test` targets (CV-10); Cursor budget counts characters, not bytes (N20); single-skill output path should fix by refusal not silent rewrite (CV-5).

## dcg
- [defect] `SKILL.md:125` false — `rm -rf ./build` IS blocked; same false claim in `COMMANDS.md` and `cc-hooks/DCG-RCH.md` (DCG-1).
- [defect] rule IDs, `explain`/`packs` format, version, and hook path diverge from the installed binary; the wrong rule ID makes the documented allowlist fix a silent no-op — re-derive every ID/sample/version from a pinned supported version (N3).
- [defect] temp-root carve-out narrower than documented: at 0.5.6 it recognizes literal `/tmp`, `/private/tmp`, and the `$TMPDIR` form, but NOT arbitrary unresolved vars (`$LAB`, `$output_dir`) or relative paths (`./build`). Correct the context-awareness claim to the measured matrix (N4).
- [defect] upstream identity/version unresolved across three inconsistent routes: installed `dcg 0.5.6`; real upstream is `Dicklesworthstone/destructive_command_guard` (not `anthropics/…`); package docs claim v0.8.2. Pin one install route to the correct upstream; every claim must name its class (DCG-U).
- [enhance] `bash -c`/inline-fragment false-positive surface undocumented — document it with the safe-alternative pattern (N5).
- [enhance] `effects: []` false — writes `.dcg.toml`/`.dcg/allowlist.toml`; declare typed config-write effects (S1). Add `output_contract` (S2); fail-open-on-timeout has no caller-visible signal contract (DCG-7).

## ms
- [defect] `scripts/validate.sh:12-14` mechanically ASSERTS the false `effects: []` — invert the validator to require accurate effects (MS-1; enforces the S1 falsehood: spawns MCP server, writes feedback/outcome rows to a live DB, rebuilds index).
- [defect] requires a private operator checkout on a named branch while shipping canonically to three runtimes — resolve the private-checkout assumption (MS-3).
- [defect] reindex mechanism unreachable from a copied install and misplaced (repo-root script) — route into `ao`; belongs in Go (MS-4).
- [defect] `ms-reindex.sh` (301 lines): substring process discovery then TERM/KILL of every match (can kill an unrelated process), unescaped `MS_REINDEX_PROBE_QUERY` interpolated into JSON, embedded Python verifier. Require executable identity, not a command-line substring.
- [enhance] add `output_contract` (S2); MCP feedback tool exposed against the CLI-only rule (MS-5); `mcp-search.py` grandfathered Python debt (GOV-1).

## ntm
- [defect] `effects: []` false — creates/selects panes, sends commands; declare effects (S1).
- [enhance] type the seven T7a obligations, starting with observation window and bounded robot output; roles/commands/scopes, deadlines, capability-unavailable behavior, cancellation, and cleanup are all untyped (`--robot-capabilities` proves the discovery surface is real).
- [enhance] hosts the cross-adapter model-dispatch contract two suppliers cite — point at the relocated contract (S4).
- [enhance] `consumes: [task-intent]` on a transport (NTM-4); external-doc delegation with no version pin (NTM-5). Keep the liveness truth-stack doctrine in the skill; add a truth-stack probe.

## rch
- [defect] owned references instruct "always do, never ask" for daemon start, cooldown removal, fleet toolchain sync, and remote `sudo chown -R`/`chmod`, against `SKILL.md:29-32`/`:53-55` explicit-authority requirement (`RECOVERY_PLAYBOOKS.md:19` "Don't ask the human if the fix is in the playbook"). Either the kernel authority requirement governs and the references are rewritten, or declare an explicit autonomous-remediation envelope with a named blast-radius ceiling (RCH-A, P0).
- [defect] `rch doctor --json` returns `success: true`, `failed: 0` with the daemon down while `rch check` correctly exits 2 — inverts T7a clause 6. State that `rch check` exit status adjudicates readiness and `doctor.success` does not (N11).
- [defect] six dangling sibling-doc references, all on the remediation path — write them or inline them (N10).
- [defect] `effects: []` false — daemon restarts, worker deployment, remote mutation; declare effects, separating read-only diagnosis from remote mutation (S1).
- [enhance] seven references with no authority ordering (RCH-3); `not_proven` reused without a no-verdict-weight disclaimer (RCH-4); internal tracker id `bd-w5r9` leaked into a shipped reference — strip it (RCH-5).

## swarm
- [defect] case-varying scopes admitted as disjoint on a case-insensitive filesystem — both packets dispatch, silently (SW-1). Case-fold conditionally on detected filesystem case-sensitivity.
- [defect] symlink-aliased scopes admitted as disjoint (`_overlap("alias","src/auth") -> False`); reachable and silent (SW-3). Resolve paths against a canonicalized workspace root before comparison, or declare and enforce a no-symlink precondition. `dispatch_once._overlap` compares lexical prefixes only — T7a clause 3 requires isolation proven, not inferred from paths.
- [defect] `write_scope.exclude` silently ignored by `_includes()` — honor it or raise on its presence (SW-2).
- [enhance] transitive executor effects undeclared — `effects: [invoke_selected_executor]` understates blast radius; state packet effects are the caller's to declare (SW-4).
- [enhance] `dispatch_once.py` grandfathered Python debt with an `ao` destination (GOV-1); move admission mechanism to Go.

---

*~14 lower-priority P2 items dropped: stale titles/section references (CASS-4, CH-5), degenerate
triggers (CV-4, DCG-6), operator-private path notes duplicated by S1 sites (N17 across cass/ms/
account-rotation/cc-hooks), doc/live version-drift restatements (N16), and RCH-3/S3-only echoes —
all low-impact cosmetic or already-implied-by-a-listed-item findings.*
