# Shell PATH Rationalization

> **Status:** Operator-facing runbook for auditing and cleaning shell PATH (`.bashrc`, `.zshrc`, `.zshenv`, `.profile`, `.zprofile`, `.bash_profile`). Use when binaries resolve to the wrong copy, shells start slowly, or `echo $PATH` is full of stale entries.

A polluted PATH is one of the most common sources of "works for the agent, broken for the operator" footguns. Installer-test scripts and AI agents frequently append `export PATH=...` lines to shell rc files and never remove them. Over months these accumulate, shadow real binaries, and slow shell startup.

This runbook codifies a non-destructive cleanup methodology: back up first, audit, edit one file at a time, verify in a subshell, and only then ask the operator to source. If anything breaks, the backup is one `cp` away.

## When to run this

| Symptom | Likely cause |
|---------|--------------|
| `which <tool>` returns a path under `/tmp/`, `/data/tmp/`, or `/run/user/` | Installer leftover or ephemeral session path persisted into rc |
| Newly installed binary still shows old version | Earlier copy shadowing the new one in PATH order |
| `echo $PATH \| tr ':' '\n' \| wc -l` is over 30 | Years of accumulated entries; many likely duplicates |
| New shell takes more than a second to render the prompt | Excess PATH entries plus per-entry stat calls during completion init |
| `type -a <tool>` lists the same binary three or more times | Duplicate PATH entries re-prepended each time the rc file is sourced |

## Safety contract (do not skip)

1. Back up every rc file you intend to touch before the first edit.
2. Verify the backup exists by reading it back.
3. Make the edit.
4. Test in a subshell (`zsh -l -c '...'` or `bash -l -c '...'`) — never source untested changes into the live shell.
5. Only when the subshell test passes, ask the operator to open a new terminal or source the file.
6. If anything misbehaves after sourcing, restore the backup before doing anything else.

A botched rc file can lock an operator out of normal commands. Always work from a backup, and always test in a subshell first.

## Audit

Inventory the live PATH with line numbers:

```bash
echo "$PATH" | tr ':' '\n' | nl
```

List every PATH-touching line across the common rc files:

```bash
grep -nE 'PATH=' \
  ~/.zshenv ~/.zshrc ~/.zprofile \
  ~/.profile ~/.bashrc ~/.bash_profile 2>/dev/null \
  | grep -v '^[^:]*:[[:space:]]*#'
```

Find every copy of a specific binary on disk that PATH currently exposes:

```bash
type -a <binary>     # e.g. type -a node, type -a ao, type -a bd
```

Count duplicate PATH entries (anything > 0 means cleanup is warranted):

```bash
echo "$PATH" | tr ':' '\n' | sort | uniq -d
```

Flag entries pointing into volatile storage:

```bash
echo "$PATH" | tr ':' '\n' | grep -E '^/tmp/|^/data/tmp/|^/run/user/'
```

## Cleanup methodology

Work in this order. Each step is reversible from the backup taken in step 1.

1. **Back up** every rc file with a timestamped suffix:

   ```bash
   ts=$(date +%Y%m%d-%H%M%S)
   for f in ~/.zshenv ~/.zshrc ~/.zprofile ~/.profile ~/.bashrc ~/.bash_profile; do
     [ -f "$f" ] && cp -p "$f" "$f.bak.$ts"
   done
   ls -la ~/.*.bak.$ts 2>/dev/null
   ```

2. **Classify** each PATH-touching line. Keep the line if the directory is a stable user or system bin (`~/.local/bin`, `~/.cargo/bin`, `~/go/bin`, `~/.bun/bin`, `/usr/local/bin`, `/usr/bin`, `/bin`, `/snap/bin`). Remove temp-dir entries, installer-test leftovers, and persisted ephemeral session paths. Ask the operator before removing anything in a custom project directory.

3. **Remove duplicates.** When the same directory is added in more than one rc file, pick one canonical home and delete the others. The convention used in this repo's installers:
   - Core PATH (tools that scripts also need) lives in `~/.zshenv` (and `~/.profile` for bash users).
   - Interactive-only additions (completion paths, convenience tools) live in `~/.zshrc` (and `~/.bashrc`).

4. **Collapse temp dirs.** Strip every line that prepends `/tmp/...`, `/data/tmp/...`, or `/run/user/.../bin` to PATH. These are session-scoped and never belong in a persistent rc file. For tools like fnm that legitimately need a per-shell directory, use the dynamic guard pattern below rather than a hardcoded snapshot.

5. **Switch to idempotent prepends.** Replace bare `export PATH="$DIR:$PATH"` with the guarded form so re-sourcing the rc file does not stack duplicates:

   ```bash
   case ":$PATH:" in
     *:"$HOME/.local/bin":*) ;;
     *) export PATH="$HOME/.local/bin:$PATH" ;;
   esac
   ```

6. **Re-source in a subshell, not the live shell.** Verify before committing:

   ```bash
   zsh -l -c 'echo "entries: $(echo "$PATH" | tr ":" "\n" | wc -l)"; \
              echo "$PATH" | tr ":" "\n" | sort | uniq -d'
   bash -l -c 'echo "$PATH" | tr ":" "\n" | nl'
   ```

   The subshell exits without affecting the calling session, so a syntax error in the rc file shows up here instead of breaking the operator's terminal.

7. **Resolve key binaries** the operator depends on (replace with the real list for the host):

   ```bash
   for b in ao bd node cargo bun go; do
     command -v "$b" >/dev/null && echo "$b -> $(command -v "$b")"
   done
   ```

8. **Hand off to the operator.** Tell them to open a new terminal (preferred) or `source ~/.zshrc`. Have them re-run `which <tool>` to confirm the canonical resolution.

## Rollback

Every edit produced a `.bak.<timestamp>` sibling. Restore is always one `cp`:

```bash
ts=<the timestamp from step 1>
for f in ~/.zshenv ~/.zshrc ~/.zprofile ~/.profile ~/.bashrc ~/.bash_profile; do
  [ -f "$f.bak.$ts" ] && cp -p "$f.bak.$ts" "$f"
done
```

After restoring, open a new terminal — do not source into the broken shell. Re-verify that the operator's day-one tools (`git`, the package manager, the editor) all resolve before doing anything else.

If you only edited one file, restore only that file. Surgical rollbacks are safer than blanket ones.

## Bushido-relevant patterns

The bushido WSL host (see `/home/boful/CLAUDE.md` and `~/.claude/reference/bushido.md`) carries an extra layer of PATH risk because installer scripts run from both the WSL and Windows sides:

- **WSL inherits Windows PATH** by default. Many `/mnt/c/...` entries appear automatically; do not duplicate them in `~/.zshenv` unless the binary is explicitly needed inside non-interactive WSL scripts.
- **Mirrored networking mode** does not affect PATH but can cause people to debug PATH issues when the actual problem is binding to `0.0.0.0` vs. `127.0.0.1` — verify the binary actually runs before blaming PATH.
- **`~/.local/bin` is the canonical install target** on bushido per the AGENTS.md vault contract. Anything installed elsewhere (especially under `~/dev/<repo>/bin`) should be either symlinked into `~/.local/bin` or invoked by absolute path, not added to PATH.
- **fnm and nvm** both manage Node.js via per-shell dirs under `/run/user/<uid>/`. Use the dynamic guard pattern (`if [ -n "${FNM_MULTISHELL_PATH:-}" ]`) rather than persisting a snapshot, or non-interactive WSL services will pick up paths that no longer exist.

## Validation

After cleanup, the host should pass these checks:

```bash
# No volatile-storage entries
echo "$PATH" | tr ':' '\n' | grep -E '^/tmp/|^/data/tmp/|^/run/user/' | wc -l   # 0

# No duplicates
echo "$PATH" | tr ':' '\n' | sort | uniq -d | wc -l                              # 0

# Reasonable entry count for a developer host
echo "$PATH" | tr ':' '\n' | wc -l                                               # < 25

# Key binaries resolve to canonical locations, not temp dirs
for b in ao bd node cargo; do command -v "$b" >/dev/null && type -a "$b"; done
```

If any of those fail, restore the backup and rework the offending step rather than patching the live PATH from the command line.

---

> Pattern adopted from `path-rationalization` (ACFS skill corpus). Methodology only — no verbatim text.
