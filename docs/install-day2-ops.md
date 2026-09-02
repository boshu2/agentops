# Install And Day-2 Operations

AgentOps 3.3 supports three install paths:

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
| `rpi` | `python3`, conditional | invokes plan and validate, which may run `python3` (see below); rpi's own procedure only cites `scripts/run_once.py` as reference behavior |
| `plan` | `python3`, conditional | runs `scripts/validate.py snapshot-intent` only when the intent source is not durable |
| `validate` | `python3` | its helper commands run `python3` against `scripts/validate.py` |
| `fitness` | `ao` | its whole procedure is running one `ao goals` subcommand |
| `goals` | `ao` | delegates entirely to fitness's `ao goals` procedure |
| `using-gc` | `ao` | rig prep runs `ao gc prepare` and `ao gc check` |
| `handoff` | `ao`, optional | `ao session handoff`/`rehydrate` cover the same artifact; the skill can write it directly |
| `status` | `ao`, optional | describes `ao status`'s output shape; the report can be read directly from `.agents/ao/` |
| `reverse-engineer` | `python3` | Phase 1's mechanical teardown runs `scripts/reverse_engineer.py` |
| `skill-builder` | `python3`, conditional | Create mode's `build.sh` runs `scripts/generate-skill-mesh.py`; heal/check/audit modes are bash-only |
| `ms` | `python3`, conditional, plus `ms` binary | the MCP-search fallback runs `python3 skills/ms/scripts/mcp-search.py`; the `ms` binary is required for CLI load, write, and admin operations |
| `toil-mining` | `python3`, conditional | the recent-human extractor runs `scripts/recent_human.py` for Codex JSONL session sources |
| `security` | `python3`, conditional | the composable suite and offline redteam surfaces run `security_suite.py` when that scan type is selected |
| `cass` | `python3`, optional | `scripts/prompt_miner.py` mines repeated prompts; one of several selectable Scripts-table entries |

The plugin and `npx skills@latest add boshu2/agentops --all -g` install all 56 skills today, regardless of whether you have `python3` or `ao`.

## Maintainer / contributor: the `ao` binary

The `ao` CLI is optional: `fitness`, `goals`, and `using-gc` call it directly
(see the table above); the rest of the skills work without it. Install it on
its own:

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
```

Without Homebrew: `go install github.com/boshu2/agentops/cli/cmd/ao@latest`

To track skills from a local checkout instead of a release bundle, run
`ao skills link` from that checkout — the full flow is below.

## Install (source checkout)

Install the optional `ao` CLI, clone AgentOps, and link its skills:

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops
ao skills link
```

Without Homebrew:

```bash
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops/cli
go install ./cmd/ao
cd ..
"$(go env GOPATH)/bin/ao" skills link
```

The command links each canonical `skills/<slug>/` directory into
`~/.agents/skills` and every detected runtime skills root. It refuses to replace
real directories, foreign links, or user-owned skills.

## Update

```bash
cd ~/.local/share/agentops
git pull --ff-only
ao skills link
```

Existing links immediately see edits to their targets. Rerunning `ao skills
link` adds newly introduced skills and reports conflicts. It does not copy the
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
