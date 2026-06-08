# Codex Execution Profile — storage-watchdog-ops

Read the base skill `../../skills/storage-watchdog-ops/SKILL.md` first (what the
daemon does, the status → interpret → remediate → escalate procedure, the
decision-log line grammar, and the safety policy). The base is the source of
truth; this is the step-ordered Codex path. One inviolable rule: **the watchdog
only ever frees Rust `target/` dirs whose parent has a `Cargo.toml` — never widen
that policy as a "fix."**

## Codex tool mapping

- **Read a log / unit / journal** → `shell_command` (`tail`, `journalctl --user`,
  `systemctl --user status`, `df -h`). This skill is almost entirely shell.
- **Edit the unit override** → `systemctl --user edit` is interactive; prefer
  writing a drop-in with `apply_patch`/redirection to
  `~/.config/systemd/user/acfs-storage-watchdog.service.d/override.conf`, then
  `systemctl --user daemon-reload`.
- **Forced assessment** → `go run ./cmd/storage-watchdog --once --dry-run …` is a
  background-safe shell command; capture its stdout as the evidence.

## Steps

1. **Status, ground truth, decisions — in one read.** `systemctl --user status
   acfs-storage-watchdog.service`, `journalctl --user -u … -n 50 --no-pager`,
   `tail -n 50 "${STORAGE_WATCHDOG_LOG_FILE:-$HOME/.local/state/acfs-storage-watchdog/watchdog.log}"`,
   and `df -h "$HOME/dev" "$HOME/acfs"`. Don't reason from the unit alone — read
   the `check`/`cleanup complete` lines against the actual `df`.
2. **Interpret against the §2 table, not vibes.** The decisive distinction is
   *pressure with `candidates=0`* (watchdog correctly can't help → escalate to
   `system-performance-remediation`) vs. *daemon down* (it isn't protecting the
   disk → restart) vs. *healthy `ok: no pressure`*.
3. **Remediate at the lowest blast radius.** Restart a down daemon; run
   `--once --dry-run` to see what it *would* delete before any real `--once`; run
   `--self-test` to prove the safety policy still holds before trusting a forced
   cleanup.
4. **Hand-clean only after both safety tests pass** (`Cargo.toml` parent AND
   `basename == target`). Never blind `rm -rf` a path from a log line.
5. **Escalate, don't widen.** Persistent pressure with `candidates=0` is a
   *different* remediation, not a watchdog bug. Disable the daemon on non-Rust
   hosts rather than fighting `candidates=0`.

## Guardrails

- **Narrow deletion policy is the safety guarantee.** Never edit the daemon to
  delete non-`target` dirs, drop the `Cargo.toml`-parent check, or follow
  symlinks. Broadening it turns a reclaimer into a data-loss incident.
- **Exit code / log lines are the verdict, not self-report.** Branch on
  `cleanup complete candidates=…`/`deleted_kb=…`, not on assumption.
- **Destructive commands stay gated** (`dcg`); `rm -rf` only after the two-test
  guard passes, and prefer the daemon's own `--once` over manual deletion.
- **Backstage only.** Never surface this ops content in client-facing material.
