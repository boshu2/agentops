# Install And Day-2 Operations

AgentOps 3.3 uses one canonical checkout and source symlinks. The checkout is the
source of truth; runtime plugin caches and copied skill mirrors are not part of
the active path.

## Install

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

## Migrate from 3.x plugins

Remove the old runtime plugin before enabling source links so only one AgentOps
corpus is visible.

### Claude Code

```bash
claude plugin uninstall agentops@agentops-marketplace
claude plugin marketplace remove agentops-marketplace
```

### Codex

Remove the old cache and install manifest, then delete the AgentOps plugin enable
entry from `~/.codex/config.toml`:

```bash
rm -rf ~/.codex/plugins/cache/agentops-marketplace
rm -f ~/.codex/.agentops-codex-install.json
```

### Gemini / Antigravity

```bash
agy plugin disable agentops-core-gemini
agy plugin uninstall agentops-core-gemini
```

The legacy runtime-specific installers remain in this release only as migration
compatibility. They are not the recommended install path.

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
