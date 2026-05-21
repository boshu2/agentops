# AGENTS-RUNTIME.md — Runtime constraints, canonical-root rules, worktree discipline

> Sibling of [`AGENTS.md`](AGENTS.md), [`AGENTS-WORKFLOW.md`](AGENTS-WORKFLOW.md), [`AGENTS-CI.md`](AGENTS-CI.md), [`AGENTS-CODEX.md`](AGENTS-CODEX.md). Split out of the monolithic AGENTS.md per soc-vuu6.3.

### Canonical Root and Worktrees

This repo has a canonical root worktree. It owns the common `.git` directory and must remain a non-disposable anchor.

- Keep the canonical root clean and attached to `main`.
- Do not use the canonical root as scratch space for task work.
- Create task branches in linked worktrees and do the actual edits there.
- Every foreign worktree must end the session as `merged`, `preserved`, `exported`, or `deleted`.
- Preserve unfinished branch work on `codex/preserve-*` when it is not ready to merge.
- Every surviving `codex/preserve-*` ref must have an entry in `docs/preserved-refs.tsv` with owner and retirement rule.
- Run `bash scripts/check-worktree-disposition.sh` before push and session close.


### Key Constraints Agents Must Follow

**No symlinks.** The plugin-load-test rejects ALL symlinks. If you need the same file in multiple skill `references/` dirs, **copy the file** — do not symlink.

**Do not track repo-root `.agents/`.** `.agents/` is local agent runtime state for Codex, Claude, AgentOps, and other agents. It can churn and may contain sensitive session context. The pre-push and CI gates run `scripts/check-no-tracked-agents.sh`; use durable docs or release artifacts outside `.agents/` for anything that belongs in git history.

**Treat `ao codex start/stop` lifecycle shims as deprecated.** Do not use `ao codex start`, `ao codex stop`, `ao codex ensure-start`, or `ao codex ensure-stop` for routine validation, RPI closeout, or merge work. These commands are legacy lifecycle shims and can mutate local agent state. Prefer non-mutating validation commands, `bd` notes, and tracked docs/runbooks. If a targeted lifecycle task truly requires one of these commands, run it in an isolated/disposable worktree with isolated agent state, then verify `git status` and `git ls-files .agents` before committing.

**Merge older `.agents` branches defensively.** Branches created before the no-tracked-`.agents` policy may contain RPI packets, summaries, or runtime metadata under `.agents/`. During merge conflict resolution, keep `main`'s deletion of repo-root `.agents/*` unless the user explicitly asks to reintroduce a tracked artifact. After resolving conflicts, run `git ls-files .agents` and expect no output.

**Skill counts must be synced.** When adding or removing a skill directory, run:
```bash
scripts/sync-skill-counts.sh
```
This updates counts in SKILL-TIERS.md, PRODUCT.md, README.md, docs/SKILLS.md, docs/ARCHITECTURE.md, and using-agentops/SKILL.md.

**Every reference file must be linked.** If a file exists in a skill's `references/` directory, the skill's SKILL.md must link to it via markdown link or Read instruction. Run `heal.sh --strict` to check.

**Codex checked-in artifacts are manually maintained, with manifest/marker provenance metadata used for validation.** If `skills-codex/` still contains Claude-era primitives (`TaskCreate`, `TaskList`, `Tool: Task`), Claude backend refs, or duplicated runtime rewrites, run:
```bash
bash scripts/audit-codex-parity.sh --skill <name>
```
Then update the canonical source and, when the Codex runtime copy itself must change, patch `skills-codex/<name>/` and/or add a durable override under `skills-codex-overrides/<name>/`. Finish by running `bash scripts/refresh-codex-artifacts.sh --scope worktree` and re-running the audit.

**Embedded hooks must stay in sync.** After editing anything in `hooks/`, `lib/hook-helpers.sh`, or `skills/standards/references/`, run:
```bash
cd cli && make sync-hooks
```

**CLI docs must stay in sync.** After adding/changing CLI commands or flags, run:
```bash
scripts/generate-cli-reference.sh
```

**Contracts must be catalogued.** When adding files to `docs/contracts/`, add a corresponding entry in `docs/documentation-index.md`. The contract gate discovers files dynamically but checks for orphans.

**Go complexity budget.** New or modified functions must stay under cyclomatic complexity 25 (warning at 15). The check only flags new/worsened violations, not legacy ones.

**No TODOs in SKILL.md files.** The smoke test greps for `TODO` and `FIXME` in `skills/*/SKILL.md`. Use issue tracking (`bd`) for follow-up work instead.

**Validate before proposing.** Before suggesting a new capability or safeguard, verify it doesn't already exist: check `ao rpi serve --help`, `hooks/hooks.json`, `GOALS.md`, and existing SKILL.md files. Three suggested features in our March 2026 validation review were already implemented.

