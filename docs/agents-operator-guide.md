# Working with `.agents/`

`.agents/` is AgentOps' repo-local operating ledger. It holds the durable
state, evidence, and generated artifacts that let agent sessions continue
across turns, runtimes, and worktrees without depending on a separate service.

The directory is intentionally structured. Production write surfaces are
catalogued in [`docs/contracts/agents-write-surfaces.md`](contracts/agents-write-surfaces.md),
and the contract gate rejects undocumented top-level subdirs. Skill-owned
subdirs follow the `.agents/<skill-name>/` convention and are validated against
the active skill catalog.

Use `.agents/` for agent-operational state, not for ad hoc scratch files. If an
artifact should be preserved, searched, cited, or replayed by AgentOps, give it
a documented owner, lifecycle, and validation path.

## Inspect the Current Surface

Use these commands before changing the surface:

```bash
ao agents inspect
ao agents lint
ao agents doctor
scripts/check-agents-write-surfaces.sh
```

`ao agents inspect` shows the documented surface. `ao agents lint` and the
script check production code against the contract. `ao agents doctor` combines
the contract, active skill-owned subdirs, lint status, and observed on-disk
top-level `.agents/` directories into one read-only diagnostic.

`ao agents doctor` exits `0` when the surface is clean, `1` when it finds
contract drift, and `2` for setup or invocation errors. Use `--json` when a
script needs stable fields such as `lint_status`, `allowlist_count`,
`skill_owned_count`, and `unknown_on_disk_dirs`.

## Add a New Surface

Follow the layering in
[`docs/patterns/agents-hygiene-contract.md`](patterns/agents-hygiene-contract.md):
contract doc, shell lint, regression tests, CLI surface, and pre-push wire-in.
Small stacked changes are easier to validate than one broad surface expansion.

For a new top-level `.agents/<name>/` write:

1. Add the production write in the owning Go, shell, hook, or script code.
2. Add the surface to the table and allowlist in
   [`docs/contracts/agents-write-surfaces.md`](contracts/agents-write-surfaces.md).
3. Add or update regression coverage in
   `tests/scripts/check-agents-write-surfaces.bats` when the change introduces
   a new contract dimension.
4. Run `ao agents lint`, `ao agents doctor`, or
   `scripts/check-agents-write-surfaces.sh`, then run the relevant local gate
   before pushing.

Pure read-side mirrors usually do not need a new contract. Prefer an
end-to-end smoke test that locks the read invariant, as described in the
hygiene pattern.

## Contributor Flow

1. Identify the owner and lifecycle: decide whether the artifact is persistent,
   rolling, regenerated, or skill-owned.
2. Patch code and contract together: production writes and the documented
   allowlist must land in the same change.
3. Prove the surface: run `ao agents inspect`, `ao agents lint`,
   `ao agents doctor`, and focused tests for the changed owner.
4. Close the loop: update linked docs or CLI help when behavior changes, record
   validation evidence in bd, and keep the branch clean before push.

## When a Gate Fails

If the lint reports an undocumented `.agents/<name>` literal, either document it
as a real write surface or remove the production write. If the surface is owned
by a skill, confirm `skills/<name>/SKILL.md` exists and the code follows the
skill-owned directory convention.

The lint now reports source-location evidence. Human output lists source files
under each undocumented subdir, and `--json` emits a `source_locations` map keyed
by subdir. Fix the code path shown in that evidence first; only update the
contract when the write surface is intentional and has an owner, lifecycle, and
cleanup expectation.

If `ao agents doctor` reports unknown on-disk directories while lint is clean,
the code may be constructing paths without a literal `.agents/<name>` string.
Search for split joins such as `filepath.Join(..., ".agents", "<name>")`,
then either document the surface or move the artifact under the correct
skill-owned directory.

Do not bypass the contract by moving writes to a less precise path. The purpose
of the gate is to make ownership and cleanup rules visible to the next operator.

## Hash-Gate Triage

The pre-push gate snapshots selected `~/.agents/` hub subtrees before tests and
diffs them afterward. In CI this remains strict: timeout or mutation failures
block the run. Locally, snapshot or diff timeouts warn and skip only the
content-hash mutation check, because concurrent local agents can make shared
hub scans slow or noisy.

Prefer isolation over skipping:

```bash
AGENTS_HUB_OVERRIDE=/tmp/empty-agents-hub-for-agentops scripts/pre-push-gate.sh --fast
```

Use `HASH_GATE_IGNORE_UNTRACKED=1` only for local scratch-state noise when you
have already isolated or inspected the shared hub. CI does not set that
variable. To tune a slow local machine without changing behavior, set
`HASH_GATE_TIMEOUT_SECONDS=<seconds>`.
