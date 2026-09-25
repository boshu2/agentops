# Install And Day-2 Operations

The default is your native coding agent plus the `ao` binary, with zero
mandatory AgentOps skills. Install AO and read the native quickstart:

```bash
go install github.com/boshu2/agentops/cli/cmd/ao@latest
ao quick-start
ao demo
```

These guidance commands do not write skills, hooks, or project state. `ao init`
is optional local evidence setup, not a prerequisite. Development features can
be built from this checkout with `cd cli && go install ./cmd/ao`.

Three skill installation paths are supported. Pick one per agent: a plugin
plus npx on the same agent gives you every skill twice. The
[host/install mapping](contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping)
defines each consumer and separates structural checks from actual host loading
and execution evidence.

- Claude Code plugin — managed bundle with skills, four agents and hooks;
  updates with the release ([below](#install-and-update-runtime-plugins)).
- Codex plugin — managed bundle of the generated Codex skills
  ([below](#install-and-update-runtime-plugins)).
- `npx skills@latest add boshu2/agentops` — everything else: the external Skills
  installer puts the same `SKILL.md` skills into the agents you pick.

Run npx from your project directory and choose agents and skills
interactively; add `-g` for a user-level install. In scripts, name the agents:
`npx skills@latest add boshu2/agentops -g -a cursor opencode -y`, and add
`--skill test refactor` for a subset. Installer targets include `cursor`,
`opencode`, `gemini-cli`, `antigravity`, `pi`, `grok` (Grok Build) and
`openclaw`. Use `-g` for OpenClaw: its project scope writes `./skills` into
your repository. Grok Bot has no installer target; add the same `SKILL.md`
folders through [its skill settings](https://docs.x.ai/grok-bot/skills-routines-and-automations).

The npx path installs skills, not runtime plugins, AO, hooks or native roles.
Its link/copy behavior belongs to that installer; use `npx skills@latest --help`
for the current contract. Don't pass `-y` without `-a`, and don't use `--all`:
both can target every agent the installer knows and create config directories
for agents you don't have. Contributors link a checkout instead; see
[the contributor section](#install-source-checkout).

Whichever path you install through, the skills themselves have runtime
requirements. Most need nothing beyond the coding agent; these need more:

| Skill | Needs | Why |
|---|---|---|
| `rpi` | `ao`, conditional | delegates exact-subject checks to Validate; only persists `verdict.v2` when requested, with the fixed-dispatch adapter optional |
| `plan` | `ao`, conditional | runs `ao provenance snapshot-intent` with an explicit evidence root when the intent source is not durable |
| `implement` | `ao`, conditional | at an integration boundary whose changed paths affect bound evidence, runs `ao provenance evidence-orphans` |
| `validate` | `ao` | derives exact subject identity with the helper and uses `ao provenance store-verdict` when persistence is requested; Python/schema checks are developer-only |
| `reality-check` | `ao`, conditional | inspect selected goal measurements with `ao goals` or evidence-store facts with `ao status` |
| `using-gc` | `ao` | rig prep runs `ao gc prepare` and `ao gc check` |
| `doc` | `ao`, optional | a requested continuity handoff may use `ao session handoff`/`rehydrate` |
| `reverse-engineer` | `python3` | Phase 1's mechanical teardown runs `scripts/reverse_engineer.py` |
| `skill-builder` | `python3`, conditional | Create mode's `build.sh` runs `scripts/generate-skill-mesh.py`; heal/check/audit modes are bash-only |
| `ms` | `python3`, conditional, plus `ms` binary | the MCP-search fallback runs `python3 skills/ms/scripts/mcp-search.py`; the `ms` binary is required for CLI load, write, and admin operations |
| `memory` | `python3`, conditional | a selected toil investigation can use the repository helper `scripts/toil-mining/recent_human.py` on cleared Codex sources |
| `security` | `python3`, conditional | the composable suite and offline redteam surfaces run `security_suite.py` when that scan type is selected |
| `cass` | `python3`, optional | `scripts/prompt_miner.py` mines repeated prompts; one of several selectable Scripts-table entries |

The plugins and `npx skills@latest add boshu2/agentops` install the skill library, regardless of whether you have `python3` or `ao`.

## Install and update runtime plugins

Install the full bundle using the runtime's plugin manager:

```bash
# Claude Code
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace
claude plugin details agentops@agentops-marketplace

# Codex
codex plugin marketplace add boshu2/agentops
codex plugin add agentops@agentops-marketplace
codex plugin list --json
```

To update an existing installation, refresh its marketplace and installed cache:

```bash
# Claude Code
claude plugin marketplace update agentops-marketplace
claude plugin update agentops@agentops-marketplace

# Codex (Git-backed marketplace)
codex plugin marketplace upgrade agentops-marketplace
codex plugin add agentops@agentops-marketplace
```

For a local Codex marketplace, re-run `codex plugin add` after updating its
source; `marketplace upgrade` refreshes Git snapshots. Start a new session after
an update. Claude's component inventory should show the
[current skill catalog](SKILL-ROUTER.md) and four agents; Codex exposes the same
skills with `agentops:` names.
The inventory commands inspect package metadata; confirm the selected skill's
actual loaded content in the fresh session before claiming host loading proof.

The Claude plugin also installs its policy dispatcher. The read-budget guard
remains separately opt-in in both runtimes. Codex plugin installation does not
register the `bulk-reader` and `code-writer` native roles. From a matching
AgentOps checkout, install those roles explicitly:

```bash
bash scripts/install-codex-context-agents.sh
# Optional, separate read-budget enforcement:
bash scripts/install-codex-read-budget-guard.sh
```

The role installer requires Node.js and Codex, writes regular role files under
`${CODEX_HOME:-$HOME/.codex}/agents`, and preserves backups when replacing changed
configuration. Use `--project` from the target project for project-local setup.
Restart Codex to discover the roles. If installing the optional hook, review and
trust that exact hook in Codex's native hook manager before expecting enforcement.
After a plugin upgrade, update the matching checkout and re-run any separately
selected role or hook installer; plugin updates do not refresh those copied files.
See [the context-budget design](design/codex-context-budget.md) for runtime
limitations and validation evidence.

## Maintainer / contributor: the `ao` binary

Install `ao` on its own; no skill linking is needed for native execution.
Selected skills may also require it as listed above:

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
```

Without Homebrew: `go install github.com/boshu2/agentops/cli/cmd/ao@latest`

To track skills from a local checkout instead of a release bundle, run
`ao skills link` from that checkout — the full flow is below.

<a id="install-source-checkout"></a>

## Contributors: source checkout and `ao skills link`

This path is for contributors and people who edit skills; users install through
a plugin or npx. Install the CLI, clone AgentOps, and link selected skills:

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops
ao skills link --skill test --skill refactor
```

Without Homebrew:

```bash
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops/cli
go install ./cmd/ao
cd ..
"$(go env GOPATH)/bin/ao" skills link --skill test --skill refactor
```

Repeat `--skill` to select exact catalog names. Unknown or invalid names fail
before any links are created. Omit selectors for the existing full-library
behavior; choosing a subset does not remove previously installed skills.
Use `--dry-run` to preview and `--dest /path/to/skills` for one discovery root.

The command links selected canonical `skills/<slug>/` directories into
`~/.agents/skills` and detected runtime roots: `~/.claude/skills`,
`~/.codex/skills`, `~/.gemini/skills`, `~/.cursor/skills` and `~/.pi/agent/skills`
when their parent config directories exist (`~/.pi/agent` for Pi, since its
project-level dir `.pi/skills` is a separate location). Links an earlier version
made in `~/.pi/skills` are no longer swept by default; remove them with
`ao skills unlink --dest ~/.pi/skills`.
It refuses to replace real directories, foreign links, or user-owned skills.
For OpenCode's dedicated discovery root, pass
`--dest ~/.config/opencode/skills`; its portable `~/.agents/skills` root is
already included. A `--dest` installation must use the same destination for
later audit, update and unlink commands.

Invocation uses the host's actual registered name, independently of the catalog
slug passed to `--skill`. Observed Codex source discovery registered Plan as
`agentops:plan`, invoked as `$agentops:plan`, with `scope: repo`, `pluginId: null`
and a canonical `skills/plan/SKILL.md` path. Inspect native inventory for your
installed host version and identify the installation from its path, scope and
plugin identity; the `agentops:` prefix alone does not identify a plugin.
Registration does not establish loaded content or successful execution; see the
[host evidence limits](contracts/multi-runtime-tier-charter.md#host-and-install-surface-mapping).

## Update

Plugins: see [Install and update runtime plugins](#install-and-update-runtime-plugins).
npx installs:

```bash
npx skills@latest update
```

Contributor source links:

```bash
cd ~/.local/share/agentops
git pull --ff-only
ao skills link --skill test --skill refactor
```

Existing links immediately see edits to their targets. Rerunning `ao skills
link` with the same selection creates missing selected links and reports conflicts;
without selectors it also adds newly introduced skills. It does not copy the
corpus or refresh a plugin cache.
It does not repair wrong/broken links, remove obsolete names or roll back a
partially completed fan-out. Inspect every destination's `error` and `conflicts`
fields; a successful exit alone does not mean there are no conflicts.

For npx installations, use that installer's selected-skill update and removal
operations, preserving its install scope and recorded source. Do not use
`ao skills link` to update an npx-managed copy. Before any update, record the
installed revision/version and selected destinations, and preserve local edits
or user-owned collisions. See [failed-upgrade recovery](#recover).

Skills 1.7.0's [update path resolution](https://github.com/vercel-labs/skills/blob/v1.7.0/src/update.ts)
skips names present at multiple paths, including canonical/generated duplicates;
`update` can still exit successfully with old copies installed. Inspect its
output and compare installed `SKILL.md` bytes with the intended source before
declaring the update complete.

## Audit

For contributor source links, preview the complete runtime fan-out without
changing anything:

```bash
cd ~/.local/share/agentops
ao skills link --dry-run --json
```

For every destination, `present` means the link target matches the expected
canonical source path. This is filesystem evidence, not host loading proof.
`conflicts` are deliberately untouched and require operator
judgment. A conflict is not evidence that the user-owned entry should be
deleted.

## Switch from plugins to source links

Running the plugin bundle is fully supported. If you switch to a source-linked
checkout (to edit skills or contribute), remove the plugin first so only one
AgentOps corpus is visible.

### Claude Code

```bash
claude plugin uninstall agentops@agentops-marketplace
claude plugin marketplace remove agentops-marketplace
```

### Codex

```bash
codex plugin remove agentops@agentops-marketplace
codex plugin marketplace remove agentops-marketplace
```

If the `codex plugin` verb is unavailable, use a Codex version with the native
plugin manager. For legacy manual removal, first preserve the relevant config,
cache and install manifest; identify the exact AgentOps-owned entries and remove
only those after review. Never delete an entire shared cache or config file to
resolve one collision. The legacy marker is `~/.codex/.agentops-codex-install.json`;
its presence alone does not establish ownership of every adjacent directory.

### Retired Gemini / Antigravity package

The `agentops-core-gemini` package is retired; Gemini CLI and Antigravity use
npx. If you installed that package, remove it:

```bash
agy plugin disable agentops-core-gemini
agy plugin uninstall agentops-core-gemini
```

The 3.x curl installers were removed; install through a plugin or npx.

## Uninstall

Plugins: `claude plugin uninstall agentops@agentops-marketplace` or
`codex plugin remove agentops@agentops-marketplace`. npx installs:
`npx skills@latest remove` (add `-g` for a user-level install).

Contributor source links: from the canonical checkout, rehearse and then remove
only links pointing into that checkout:

```bash
ao skills unlink --dry-run
ao skills unlink
```

This does not remove foreign skills, real directories, the checkout, or data in
project-local `.agents/` directories. Remove the checkout separately when it is
no longer needed. If Homebrew installed the CLI, use `brew uninstall agentops`.
`unlink` sweeps all links owned by this checkout at the chosen destinations;
it has no `--skill` selector. Preview its complete removal set first, and retain
your selected-skill list for any subsequent relink.

## Workflows (Claude Code only)

The canonical `workflows/` directory (a sibling of `skills/`) holds workflow
scripts for the Claude Code Workflow tool — multi-agent orchestration conveyors
such as `implement-wave` and `verify-fixes`. Workflows are a Claude-only
runtime adapter, the same doctrine as the Codex-only `skills-codex/` tree;
other runtimes ignore them.

Install or update the links from the canonical checkout, inside the project
where you want them available:

```bash
ao workflows link
```

Uninstall:

```bash
ao workflows unlink --dry-run
ao workflows unlink
```

The command links each canonical `workflows/` script into the project-local
`.claude/workflows/` directory, where the Claude Code harness resolves named
workflows. It carries the same semantics as `ao skills link`: idempotent
relinking, and it refuses to replace real files, foreign links, or user-owned
entries — a reported conflict requires operator judgment, never silent
replacement. `ao workflows unlink` removes only links that point back into the
checkout.

Claude Code snapshots its named-workflow registry at session start, so newly
linked workflows appear in the next session, not a session already running.

## Cold resume

Start a new host session after installation or update. Confirm the intended
corpus and selected names, then load only the skill and references needed by the
task. Resume from the caller's existing task/handoff and current source state.
For project knowledge, follow the consumer checkout's `.context/README.md` when
present and read relevant pages and their authoritative owners. A skill install
does not create project knowledge or restore runtime/tracker state.

## Recover

A failed upgrade can leave some destinations updated and others unchanged.
Keep the previous source/version and user-owned files until the new installation
has been checked in a cold session; there is no cross-host atomic rollback.

1. Identify the install path, intended source revision, selected skills and
   destinations. Preserve local changes and any installer backups before repair.
2. For source links, run `ao skills link --skill test --skill refactor --dry-run
   --json` from the canonical checkout, substituting the original selection and
   adding the original `--dest` if used. Inspect every error, conflict and stale
   old-name entry against [migration](MIGRATION.md).
3. Resolve ownership before touching a conflicting path. Keep real directories
   and foreign links intact. After fixing the cause, rerun the same selected
   link command without `--dry-run`; do not accidentally expand to a full install.
4. For plugin/npx failures, use that manager to inspect and retry the same
   installation; preserve edits before replacing owned copies. For npx skills
   skipped by `update`, repeat the original `add` command from the original
   destination with the same source, skill selection, agents, project/global
   scope and link/copy flags. Compare installed bytes with the intended source
   afterward; adding new skill names is a separate selection change. Recheck
   optional copied roles/hooks separately. If returning to an earlier source revision,
   preserve current changes and use the repository/manager's recovery policy;
   source links immediately follow the checkout and need no cache reinstall.
5. Start a fresh session, verify actual selected loading and resume the bounded
   task. Re-run the affected journey; successful repair commands do not prove
   recovery or validate work produced under the failed installation.

`ao doctor` and `ao version` provide additional read-only CLI diagnostics.
Final host qualification must execute a failed/partial-upgrade recovery and
verify user-owned entries survive; these instructions alone do not prove it.

## Escalate

Include the runtime and OS, your install method, `ao version`, for source links
the JSON result of `ao skills link --dry-run --json`, and the output of
`readlink` for one affected skill. Do not
include credentials or unrelated runtime configuration.
