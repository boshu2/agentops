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

Three optional skill installation paths remain supported:

- `npx skills@latest add boshu2/agentops --all -g` — universal; one command
  installs the skills into all your coding agents.
- Runtime plugins for Claude Code and Codex — managed bundles that update with
  the release.
- One canonical checkout plus `ao skills link` — source-tracked symlinks for
  users who edit skills or contribute.

With npx or a plugin, install and updates are handled by that tool. The rest of
this page covers the checkout path and its day-2 operations.

Whichever path you install through, the skills themselves have runtime
requirements. Most need nothing beyond the coding agent; these need more:

| Skill | Needs | Why |
|---|---|---|
| `rpi` | `ao`, conditional | delegates exact-subject checks to Validate; only persists `verdict.v2` when requested, with the fixed-dispatch adapter optional |
| `plan` | `ao`, conditional | runs `ao provenance snapshot-intent` with an explicit evidence root when the intent source is not durable |
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

The plugin and `npx skills@latest add boshu2/agentops --all -g` install the generated skill catalog, regardless of whether you have `python3` or `ao`.

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

## Install (source checkout)

To make selected guidance discoverable, install the CLI, clone AgentOps, and
link those skills:

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
`~/.agents/skills` and every detected runtime skills root. It refuses to replace
real directories, foreign links, or user-owned skills.

## Update

```bash
cd ~/.local/share/agentops
git pull --ff-only
ao skills link --skill test --skill refactor
```

Existing links immediately see edits to their targets. Rerunning `ao skills
link` with the same selection restores selected links and reports conflicts;
without selectors it also adds newly introduced skills. It does not copy the
corpus or refresh a plugin cache.

## Audit

Preview the complete runtime fan-out without changing anything:

```bash
cd ~/.local/share/agentops
ao skills link --dry-run --json
```

For every destination, `present` means the symlink resolves to the expected
canonical source. `conflicts` are deliberately untouched and require operator
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

If the `codex plugin` verb is unavailable (older Codex), remove the cache and
install manifest by hand, then delete the AgentOps plugin enable entry from
`~/.codex/config.toml`:

```bash
rm -rf ~/.codex/plugins/cache/agentops-marketplace
rm -f ~/.codex/.agentops-codex-install.json
```

### Gemini / Antigravity

```bash
agy plugin disable agentops-core-gemini
agy plugin uninstall agentops-core-gemini
```

The 3.x curl installer scripts are refusing tombstones; install via npx, a
runtime plugin, or the checkout above.

## Uninstall

From the canonical checkout, rehearse and then remove only links pointing into
that checkout:

```bash
ao skills unlink --dry-run
ao skills unlink
```

This does not remove foreign skills, real directories, the checkout, or data in
project-local `.agents/` directories. Remove the checkout separately when it is
no longer needed. If Homebrew installed the CLI, use `brew uninstall agentops`.

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

## Recover

If a runtime cannot see a skill:

1. Run `ao skills link --dry-run --json` from the canonical checkout.
2. Resolve broken links or reported conflicts deliberately.
3. Run `ao skills link` again.
4. Restart the runtime if it snapshots its skill inventory at startup.

`ao doctor` and `ao version` provide additional read-only CLI diagnostics. Do
not reinstall a plugin cache to repair a source-link problem.

## Escalate

Include the runtime and OS, `ao version`, the JSON result of `ao skills link
--dry-run --json`, and the output of `readlink` for one affected skill. Do not
include credentials or unrelated runtime configuration.
