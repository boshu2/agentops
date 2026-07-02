#!/usr/bin/env bash
# diff-identity.sh — the SINGLE source of truth for a commit's diff-identity signature,
# sourced by BOTH scripts/pawl-verdict.sh (REBOUND rebind/check) and scripts/pawl-review.sh
# (--converge adversarial lineage). Two copies of this signature drifting is the exact parity
# class that kept leaking byte-categories (whitespace → mode → binary → `\ No newline`); one
# shared implementation makes converge and rebind/check impossible to drift (age-rk3r.9).
#
# It exports three functions:
#   commit_patch_id     <sha> [root] — the REBASE-STABLE key (git patch-id --stable).
#   commit_content_lines <sha> [root] — the BYTE-EXACT content signature (denylist; see below).
#   commit_content_sig  <sha> [root] — sha256 of commit_content_lines (a fixed-width digest).
#
# CALLERS supply the repo root explicitly, or it defaults to $REPO_ROOT (both scripts set it).
# This file is a PURE library: it defines functions and sets nothing global. It rides in the
# embedded pawl bundle (cli/embedded/pawl/scripts/lib/, synced by `make -C cli sync-hooks` and
# checked by scripts/validate-embedded-sync.sh + cli/cmd/ao/pawl_embed_test.go) the same way
# scripts/lib/codex-exec.sh and scripts/lib/verify-config.sh do.

# commit_patch_id <sha> [repo-root] — print the STABLE git patch-id of a commit's diff:
# the REBASE-STABLE key. `git patch-id --stable` normalizes the @@ hunk line-numbers that
# legitimately shift across a rebase, so two commits with the same change on different
# bases share one patch-id — exactly the property that lets a REBOUND reuse a review across
# a no-op rebase. Empty on any failure (unknown sha, empty diff, git error) — the caller
# MUST treat empty as "cannot prove identity" and fail-closed, never as a match. The diff is
# rendered with the same helper-disabling flags the reviewer uses (--no-ext-diff/--no-textconv/
# -c core.fsmonitor=) so an untrusted repo's diff drivers cannot execute here either.
#
# CAVEAT — patch-id ALONE is NOT sufficient for the REBOUND safety claim. `git patch-id` is
# WHITESPACE-INSENSITIVE AND ignores file mode / trailing-newline, so a diff whose only change
# is leading whitespace (e.g. Python indentation), a chmod, or a dropped final newline can
# produce the SAME patch-id — the diff BYTES changed but patch-id says "same". So the identity
# ALSO requires commit_content_lines (the byte-exact signature) to match; and a REBOUND further
# requires a green full gate on the new tip (the behavior check for a semantic conflict).
commit_patch_id() {
  local sha="${1:-}" root="${2:-$REPO_ROOT}"
  [[ -n "$sha" ]] || { printf ''; return 1; }
  local pid
  pid="$(git -c core.fsmonitor= -C "$root" show "$sha" --no-ext-diff --no-textconv --no-color 2>/dev/null \
        | git patch-id --stable 2>/dev/null | awk 'NR==1{print $1}')"
  [[ -n "$pid" ]] || { printf ''; return 1; }
  printf '%s' "$pid"
}

# commit_content_lines <sha> [repo-root] — print a commit diff's BYTE-EXACT change identity,
# normalized ONLY where a legitimate rebase of the SAME change provably shifts. This is a
# DENYLIST (keep-everything-except-provably-volatile), NOT an allowlist (keep-only-known):
# an allowlist is structurally leaky — it silently drops any diff line it forgot to list
# (successive escapes: whitespace, then file-mode, then binary blobs, then git's
# `\ No newline at end of file` marker), each authorizing changed bytes without re-review.
# The denylist closes the whole class at once (age-rk3r.9).
#
# The content signature is BYTE-EXACT EXCEPT the only two parts that legitimately shift across
# a rebase of the same change:
#   1. TEXT-hunk blob ids — in an `index <pre>..<post> <mode>` line belonging to a TEXT hunk,
#      the two blob ids are replaced with a constant ("index BLOB..BLOB[ <mode>]") while the
#      trailing MODE is KEPT. (A BINARY hunk — an `index <pre>..<post>` followed by
#      `Binary files … differ` — KEEPS the full line VERBATIM: for a binary file there are no
#      +/- content lines, so the blob id IS the content identity, and a git blob id is a
#      content hash — rebase-STABLE for identical content, DIFFERENT on any content change.)
#   2. `@@ -a,b +c,d @@ [ctx]` hunk-position line numbers — the whole `@@ … @@` line (positions
#      AND trailing function-context, which is a copy of a shifting nearby source line) is
#      normalized to a constant "@@ POS @@"; the structural marker is kept.
# EVERYTHING ELSE is kept VERBATIM and is SIGNIFICANT: all `+`/`-` content (whitespace-exact),
# `diff --git`/`--- `/`+++ ` headers, `old mode`/`new mode`/`new file mode`/`deleted file mode`,
# `Binary files … differ`, AND git's `\ No newline at end of file` marker — every byte.
# So the signature is IDENTICAL across a genuine rebase (only blob ids + @@ positions moved),
# but DIFFERS if ANY other diff byte differs — whitespace, file mode (e.g. a data file made
# EXECUTABLE), binary content, or a trailing-newline flip. Empty on any failure; a caller MUST
# treat empty as "cannot prove content identity" and fail-closed. The untrusted-repo diff-driver
# guards (--no-ext-diff/--no-textconv/-c core.fsmonitor=) apply.
commit_content_lines() {
  local sha="${1:-}" root="${2:-$REPO_ROOT}"
  [[ -n "$sha" ]] || { printf ''; return 1; }
  local out
  out="$(git -c core.fsmonitor= -C "$root" show "$sha" --no-ext-diff --no-textconv --no-color --format= 2>/dev/null \
        | awk '
            # An `index` line is BUFFERED until the NEXT significant line reveals whether its
            # file-hunk is binary (followed by "Binary files …") or text (followed by @@ / +/-).
            # Binary -> keep the buffered line VERBATIM (blob id = content identity). Text ->
            # emit "index BLOB..BLOB[ <mode>]" (blob ids constant, trailing mode kept).
            function flush_index() {
              if (idx_pending != "") {
                if (is_binary) { print idx_pending }
                else if (idx_pending ~ /^index [0-9a-f]+\.\.[0-9a-f]+( [0-7]+)?$/) {
                  mode=""
                  if (idx_pending ~ /^index [0-9a-f]+\.\.[0-9a-f]+ [0-7]+$/) { n=split(idx_pending,a," "); mode=" " a[n] }
                  print "index BLOB..BLOB" mode
                } else { print idx_pending }   # unexpected index shape: keep VERBATIM (fail-safe, never silently drop)
                idx_pending=""
              }
            }
            /^diff --git / { flush_index(); is_binary=0; print; next }
            /^index / { flush_index(); idx_pending=$0; next }            # buffer; classify on the next line
            /^Binary files / { is_binary=1; flush_index(); print; next } # binary marker -> the buffered index kept verbatim
            /^@@ / { is_binary=0; flush_index(); print "@@ POS @@"; next }  # normalize the WHOLE @@ line (positions + ctx)
            { is_binary=0; flush_index(); print }                       # EVERY other line kept VERBATIM (incl. the \ No newline marker)
            END { flush_index() }
          ')"
  # A commit with no diff content yields nothing; let the caller decide
  # (empty return code 1 = cannot prove content identity).
  [[ -n "$out" ]] || { printf ''; return 1; }
  printf '%s' "$out"
}

# commit_content_sig <sha> [repo-root] — the sha256 of commit_content_lines: a fixed-width
# digest of the byte-exact content signature, for callers (the --converge lineage record)
# that store/compare a compact hash rather than the full multi-line signature. Empty on any
# failure (fail-closed, same as commit_content_lines).
commit_content_sig() {
  local sha="${1:-}" root="${2:-$REPO_ROOT}"
  local lines sig
  lines="$(commit_content_lines "$sha" "$root")" || { printf ''; return 1; }
  [[ -n "$lines" ]] || { printf ''; return 1; }
  if command -v shasum >/dev/null 2>&1; then sig="$(printf '%s' "$lines" | shasum -a 256 | cut -d' ' -f1)"
  else sig="$(printf '%s' "$lines" | sha256sum | cut -d' ' -f1)"; fi
  printf '%s' "$sig"
}
