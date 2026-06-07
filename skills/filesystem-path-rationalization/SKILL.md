---
name: filesystem-path-rationalization
description: |-
  Use when rationalizing file or directory layout and updating references without breaking builds.
  Triggers:
skill_api_version: 1
user-invocable: false
hexagonal_role: domain
practices:
- inventory-before-action
- one-convention
- reference-graph-first
- atomic-move-then-fix
consumes:
- repo-tree
- convention-target
produces:
- layout-plan
- move-map
- verified-tree
context_rel: []
context:
  window: inherit
  intent:
    mode: task
  intel_scope: topic
metadata:
  tier: execution
  stability: experimental
  dependencies: []
output_contract: 'artifact: a committed move-map (old -> new paths) plus a green build/test/link check proving no reference broke'
---

# filesystem-path-rationalization — reorganize a project's layout without breaking it

Take a project whose files and directories drifted into inconsistency and bring
it to one stated convention: decide the target shape, move files atomically, and
update every reference (imports, includes, config, docs, CI) so builds, tests,
and links stay green.

## ⚠️ Critical Constraints

- **Build the reference graph BEFORE moving anything.**
  **Why:** a move that updates the file but not its importers compiles locally
  and breaks at integration. Find every inbound reference first.
  - WRONG: `git mv src/util.py src/lib/util.py` then run the build and chase
    errors one by one.
  - CORRECT: `rg -n "util" --type py` (and config/docs) to enumerate referrers,
    record them in the move-map, then move and rewrite all referrers in one pass.

- **Move with `git mv`, never copy-then-delete.**
  **Why:** `git mv` preserves history and blame; copy+`rm` orphans the file's
  past and pollutes diffs.
  - WRONG: `cp old.go new/old.go && rm old.go`
  - CORRECT: `git mv old.go new/old.go`

- **One convention, decided and written down first.**
  **Why:** rationalizing toward two half-conventions leaves the repo as messy as
  it started. Name the rule (e.g. "tests beside source", "kebab-case dirs") in
  the plan, then apply it uniformly.

- **Verify with the real build/test/link check, not a glob.**
  **Why:** "files look organized" is not "references resolve". Only the project's
  own build/test/lint and a link checker prove nothing broke.
  - WRONG: `ls -R` and call it done.
  - CORRECT: run `<build> && <test>` (and a markdown link check for docs) and
    confirm exit 0.

- **Never edit generated, vendored, or `.git/` paths as if they were source.**
  **Why:** moving `node_modules/`, `dist/`, `target/`, or vendored trees breaks
  tooling and is overwritten on next build. Restructure source; let tools
  regenerate the rest.

## Why This Exists

Project layout rots silently. Files land wherever the author was working,
naming conventions fork (`my_module`, `my-module`, `MyModule`), tests scatter,
config piles at the root. Each individual placement was reasonable; the
aggregate is unnavigable, and any cleanup risks breaking imports nobody
remembers. Agents make this worse — a swarm drops artifacts wherever it ran.

The hard part is never the move. It is the **reference graph**: every import,
relative include, build-config path, CI glob, and doc link that points at the
old location. This skill makes the graph explicit before touching the tree, so
the reorganization is a planned transform with a verifiable green endpoint, not
a guess-and-fix chase.

## Quick Start

```bash
# 1. Snapshot the current tree and a clean baseline
git status --porcelain        # MUST be clean before starting
git ls-files | sed 's#/[^/]*$##' | sort -u   # current directory shape

# 2. Establish baseline green (record what "working" means)
<build cmd> && <test cmd>     # e.g. make build && make test

# 3. Enumerate references for a candidate file before moving it
rg -n --hidden -g '!.git' 'old_module_name'

# 4. Move + rewrite referrers in one commit, then re-verify
git mv old/path new/path
#   ...rewrite every referrer found in step 3...
<build cmd> && <test cmd>     # MUST still exit 0
```

## Workflow

### Phase 1 — Inventory & target convention

1. Confirm the working tree is clean (`git status --porcelain` empty). Refuse to
   start on a dirty tree — you need a clean baseline to attribute breakage.
2. Capture the current shape: `git ls-files`, directory counts, naming variants.
3. Name the **target convention** explicitly and write it into the plan: dir
   naming (kebab/snake), where tests live, where config lives, source root,
   public vs. internal split. This is a decision, not a discovery.

**Checkpoint:** you can state the target layout in one paragraph and the
convention rule in one sentence. If not, stop and decide.

### Phase 2 — Reference graph

4. For every file that will move, enumerate inbound references:
   `rg -n --hidden -g '!.git' '<symbol-or-basename>'`. Cover code imports,
   relative `../` includes, build configs (`go.mod`, `tsconfig.json`,
   `pyproject.toml`, `Cargo.toml`, `Makefile`), CI globs, and doc links.
5. Record each planned move as a row in the **move-map**: `old/path -> new/path`
   plus the list of referrers to rewrite.

**Checkpoint:** every move-map row lists its referrers. A row with "0 referrers"
is verified-zero (you ran the grep), not assumed-zero.

### Phase 3 — Execute atomically

6. Apply the move-map in batches grouped by convention rule. For each batch:
   `git mv` the files, then rewrite all recorded referrers.
7. Keep each batch a coherent commit so a single rule can be reverted cleanly.
8. Run the build/test after each batch — do not batch verification to the end.

**Checkpoint:** after each batch, `<build> && <test>` exits 0 and
`git status` shows only intended renames + reference edits.

### Phase 4 — Verify & seal

9. Run the full build, test suite, and linter once more from a clean checkout
   feel (`git stash` nothing — the tree should already be committed).
10. Run a link check on docs (the project's own link checker if it has one, else
    a grep for now-dead relative links).
11. Confirm `git log --follow` still traces moved files (history preserved).

**Checkpoint:** build + test + link check all green; the move-map is committed as
the record of what changed.

## Output Specification

Produce a `MOVE-MAP.md` (or PR description) with:

- **Convention:** the one-sentence rule the layout now follows.
- **Moves table:** `| old path | new path | referrers updated |`
- **Verification:** the exact build/test/link commands run and their exit 0.

Filename: `MOVE-MAP.md` at the repo root (or inline in the PR body).

## Quality Rubric

- [ ] Every moved file used `git mv`; `git log --follow` traces its history.
- [ ] Every referrer in the reference graph was rewritten (re-run the grep after:
      zero hits on old paths in non-archive files).
- [ ] The project's own build + test + lint exit 0 from a committed tree; no
      "looks organized" claim stands in for a real check.
- [ ] The target convention is stated and the resulting tree obeys it uniformly.

## Examples

**Scatter → convention (Python tests).** Tests live mixed in `src/` and a
top-level `tests/`. Convention: all tests under `tests/` mirroring `src/`.
Enumerate importers with `rg -n 'from src\.' tests/`, `git mv` each
`src/foo/test_*.py` → `tests/foo/`, rewrite imports, `pytest` green.

**Inconsistent dir naming.** Dirs mix `my_module/` and `my-module/`. Convention:
kebab-case. Grep configs and imports for both forms, `git mv` the snake variants,
update `pyproject.toml`/`tsconfig.json` path maps, rebuild.

**Root clutter.** Twelve `.md` plan docs and stray `.json` at the root.
Convention: docs under `docs/`, ephemeral artifacts deleted. `git mv` keepers to
`docs/`, `git rm` the rest with a `.gitignore` rule, fix doc cross-links.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Build breaks only in CI, not locally | CI glob still points at old path | grep `.github/`, `.gitlab-ci.yml`, build config for the old dir |
| `git log` shows file as new, not moved | copied instead of `git mv` | redo with `git mv`; or `git log --follow` to confirm rename heuristic |
| Imports resolve locally but fail in package | path map (`tsconfig`/`pyproject`) not updated | update the path-mapping config, not just source imports |
| Doc links 404 after move | relative links not rewritten | run a link checker; grep for the old relative path |
| Reorg diff is huge and unreviewable | moves + edits + refactors mixed | split into per-convention commits; never refactor logic during a move |

## See Also

- `agent-mail` — coordinate file reservations when rationalizing layout inside a
  multi-agent swarm so two agents don't move the same tree.
- `beads-br` — track each convention batch as an issue for reviewable history.
