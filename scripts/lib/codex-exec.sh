#!/usr/bin/env bash
# shellcheck shell=bash
# scripts/lib/codex-exec.sh — sourced library: ONE fail-closed hardened runner for
# `codex exec`, so every non-pawl harness that shells to codex shares the same
# defenses instead of re-solving a subset of them.
#
# Source it (do NOT execute it):
#     . "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/codex-exec.sh"
#
# Extracted from the pawl surfaces (scripts/pawl-review.sh — the timeout-array
# wrapper, the missing-codex PRECONDITION exit, the anti-ECHO/anti-WANDER output
# defense, and the retry-once-on-flat-0-byte stall handling) and the
# scripts/eval-membrane.sh membrane timeout wrapper, so the three known
# codex-exec failure modes are defended ONCE (age-gate-the-ungated-egwt.8):
#   STALL   — a hung `codex exec` once froze a run for 22 min (eval-membrane
#             age-9h3d). Killed by the timeout/gtimeout wrapper.
#   ECHO    — the prompt is reflected back with no review and no `tokens used`
#             marker (pawl-review age-a9iv). Detected here (output ≈ prompt AND
#             no `tokens used` marker) and reported as a DISTINCT exit code.
#   WANDER  — the model greps the filesystem instead of answering (pawl-review
#             age-a9iv, observed on large read-files packets). The caller owns the
#             prompt shape that avoids it; the lib does not run WANDER-prone tools.
#
# IMPORTANT — behavior-preserving contract: this lib does NOT `set -euo pipefail`
# on behalf of its callers, and it NEVER edits the pawl surfaces (pawl-review.sh /
# pawl.sh / pawl-verdict.sh), which own their own richer, verdict-bound flow. The
# functions below are pure/idempotent and safe under either shell mode.
#
# THE HARD CONTRACT: NO-VERDICT ≠ REFUTED. A caller must be able to tell a run
# that produced NOTHING TRUSTWORTHY (stall, echo, missing codex) apart from a run
# that produced a real (possibly negative) answer. The distinct exit codes below
# make that legible: a STALL/ECHO/MISSING is a PRECONDITION/degraded outcome, not
# a review result — callers must never read it as a clean pass OR a refutation.

# ---------------------------------------------------------------------------
# Documented exit codes (stable contract — callers switch on these):
#   0  CODEX_EXEC_OK             SUCCESS: codex ran to completion, produced output.
#   2  CODEX_EXEC_MISSING        MISSING-CODEX: the codex binary is not on PATH — a
#                                PRECONDITION failure, NOT a result (matches
#                                pawl-review's exit-2 precondition semantics).
#   3  CODEX_EXEC_GENUINE_NONZERO GENUINE-NONZERO: codex launched and exited
#                                non-zero for its OWN reason (auth error, refusal,
#                                a real task failure) — distinct from a timeout.
#                                For this category the runner returns codex's OWN
#                                exit code VERBATIM (so a caller that records the
#                                agent exit — e.g. eval-agent-harness's
#                                `agent_exit` field — preserves the real code); the
#                                constant 3 is the representative/default value.
#                                MISSING is checked BEFORE exec, so a returned 2
#                                here is unambiguously codex's own code, not MISSING.
#   124 CODEX_EXEC_STALL_TIMEOUT STALL-TIMEOUT: the run exceeded the timeout budget
#                                and was killed (124 = the value `timeout` itself
#                                returns on kill; preserved so callers already
#                                keyed on 124 keep working).
#   125 CODEX_EXEC_ECHO          ECHO: output reflected the prompt back with no
#                                `tokens used` marker — no real review happened.
# These names are exported as readonly ints so sourcing callers can switch on
# names rather than magic numbers.
# ---------------------------------------------------------------------------
: "${CODEX_EXEC_OK:=0}"
: "${CODEX_EXEC_MISSING:=2}"
: "${CODEX_EXEC_GENUINE_NONZERO:=3}"
: "${CODEX_EXEC_STALL_TIMEOUT:=124}"
: "${CODEX_EXEC_ECHO:=125}"

# codex_exec_timeout_cmd — echo the timeout-wrapper argv (space-separated) for a
# budget, or nothing when no timeout binary exists. Ported READ-ONLY from
# pawl-review.sh (~line 233): PREFER `timeout`, fall back to `gtimeout`, and if
# NEITHER exists degrade to running codex with no timeout rather than failing
# closed and being unusable on a bo-mac that ships no coreutils `timeout`.
# Usage: read -r -a _to <<<"$(codex_exec_timeout_cmd 300)"; "${_to[@]}" codex ...
codex_exec_timeout_cmd() {
  local budget="${1:-0}"
  [ "$budget" = "0" ] && return 0
  if command -v timeout >/dev/null 2>&1; then printf 'timeout %s' "$budget"
  elif command -v gtimeout >/dev/null 2>&1; then printf 'gtimeout %s' "$budget"
  fi
}

# codex_exec_looks_echoed — return 0 (true) when the captured output looks like an
# ECHO of the prompt (a WANDER/ECHO failure with no real review), 1 otherwise.
# Ported READ-ONLY from the pawl-review institutional knowledge: a real codex run
# prints a `tokens used` marker; an echo does not, AND its bytes closely match the
# prompt bytes. Both conditions must hold to call it an echo, so a legitimately
# short answer that merely lacks the marker is NOT mis-flagged.
#   $1 = output file   $2 = prompt file
codex_exec_looks_echoed() {
  local out_file="$1" prompt_file="$2"
  [ -s "$out_file" ] || return 1                      # empty is a STALL, not an echo
  # A real run emits a `tokens used` marker; its presence means NOT an echo.
  if grep -qi 'tokens used' "$out_file" 2>/dev/null; then return 1; fi
  [ -f "$prompt_file" ] || return 1
  local out_bytes prompt_bytes
  out_bytes="$(wc -c < "$out_file" | tr -d ' ')"
  prompt_bytes="$(wc -c < "$prompt_file" | tr -d ' ')"
  [ "$prompt_bytes" -gt 0 ] || return 1
  # Echo heuristic: no marker AND the output is within a small band of the prompt
  # size (>= 80% of the prompt bytes). A genuine short answer is far smaller than
  # its prompt; an echo reflects (most of) the prompt back verbatim.
  [ "$out_bytes" -ge $(( prompt_bytes * 80 / 100 )) ]
}

# codex_exec_guarded — the one fail-closed hardened `codex exec` runner.
#
# The prompt/model/sandbox/dir/output surface is an env-var + args hybrid (house
# style): the run is configured by env vars set by the caller before the call.
#   CODEX_EXEC_PROMPT_FILE   file whose contents are the prompt (mutually exclusive
#                            with CODEX_EXEC_PROMPT_ARG; a file wins if both set).
#   CODEX_EXEC_PROMPT_ARG    the prompt as a single positional argument.
#                            If NEITHER is set, the prompt is read from stdin.
#   CODEX_EXEC_TIMEOUT       timeout budget in seconds (0/unset = no timeout).
#   CODEX_EXEC_SANDBOX       sandbox value, e.g. read-only / workspace-write
#                            (default: read-only — the fail-closed default).
#   CODEX_EXEC_MODEL         model id for `-m` (empty/unset = codex default).
#   CODEX_EXEC_DIR           working dir for `-C` (empty/unset = no -C).
#   CODEX_EXEC_SKIP_GIT_CHECK 1 => pass --skip-git-repo-check (non-git workspaces).
#   CODEX_EXEC_EXTRA_ARGS    a bash array (name via nameref is overkill; callers
#                            set the array `CODEX_EXEC_EXTRA_ARGS=(...)`) of extra
#                            passthrough flags appended verbatim (e.g. --json).
#   CODEX_EXEC_OUT_FILE      write captured stdout+stderr here. If empty, output is
#                            captured to a temp file used only for echo-detection
#                            and then streamed to the caller's stdout on success.
#   CODEX_EXEC_RETRY_ON_EMPTY 1 (default) => retry ONCE on a flat 0-byte first run
#                            (the pawl-review stall-retry). 0 disables the retry.
#   CODEX_EXEC_EXPECT_OUTPUT 1 (default) => the caller CONSUMES codex output, so a
#                            flat 0-byte run is a STALL and an output≈prompt run is
#                            an ECHO (both fail-closed). 0 => the caller only cares
#                            about the EXIT CODE and discards output (e.g. a
#                            fire-and-score producer): a clean exit-0 with empty
#                            output is SUCCESS, and the stall/echo reclassification
#                            is skipped. A killed/timeout run is STILL a STALL under
#                            both settings (a kill is never a success).
#   CODEX_EXEC_BIN           the codex binary (default: codex) — lets a test feed a
#                            stub, matching second-poll.sh's CODEX_BIN convention.
#
# Returns one of the documented exit codes above. On CODEX_EXEC_OK the output is
# in CODEX_EXEC_OUT_FILE (if set) or on stdout.
codex_exec_guarded() {
  local bin="${CODEX_EXEC_BIN:-codex}"

  # PRECONDITION (ported from pawl-review ~line 204): a missing codex binary is a
  # PRECONDITION failure with its OWN exit code — never a review result. Name it,
  # say it is installable, and return the distinct code so a caller can tell a
  # missing dep apart from a real refutation.
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "codex-exec: MISSING DEPENDENCY — '$bin' is not on PATH." >&2
    echo "  This is NOT a result. Install the codex CLI and put it on PATH, then re-run." >&2
    echo "  (exit $CODEX_EXEC_MISSING = precondition, not a REFUTE / not a genuine failure)" >&2
    return "$CODEX_EXEC_MISSING"
  fi

  # Assemble the codex argv. Order is stable so stub-`codex` tests that inspect
  # positional args (e.g. eval-agent-harness.bats records the -C dir) keep working.
  local -a argv=(exec)
  [ "${CODEX_EXEC_SKIP_GIT_CHECK:-0}" = "1" ] && argv+=(--skip-git-repo-check)
  local sandbox="${CODEX_EXEC_SANDBOX:-read-only}"
  argv+=(--sandbox "$sandbox")
  [ -n "${CODEX_EXEC_MODEL:-}" ] && argv+=(-m "$CODEX_EXEC_MODEL")
  [ -n "${CODEX_EXEC_DIR:-}" ] && argv+=(-C "$CODEX_EXEC_DIR")
  # Append caller extra args ONLY if the array is set + non-empty. The `+set`
  # guard is the robust idiom under `set -u` (a bare ${#arr[@]} on a genuinely
  # unset array can trip "unbound variable" on bash 3.2 / macOS default).
  if [ -n "${CODEX_EXEC_EXTRA_ARGS+set}" ] && [ "${#CODEX_EXEC_EXTRA_ARGS[@]}" -gt 0 ]; then
    argv+=("${CODEX_EXEC_EXTRA_ARGS[@]}")
  fi

  # Resolve the prompt source and (for echo-detection) a prompt file.
  local prompt_file="" cleanup_prompt="" stdin_mode=0
  if [ -n "${CODEX_EXEC_PROMPT_FILE:-}" ]; then
    prompt_file="$CODEX_EXEC_PROMPT_FILE"
  elif [ -n "${CODEX_EXEC_PROMPT_ARG:-}" ]; then
    argv+=("$CODEX_EXEC_PROMPT_ARG")
    prompt_file="$(mktemp "${TMPDIR:-/tmp}/codex-exec-prompt.XXXXXX")"
    cleanup_prompt="$prompt_file"
    printf '%s' "$CODEX_EXEC_PROMPT_ARG" > "$prompt_file"
  else
    stdin_mode=1
  fi

  # Resolve the output sink. A caller-provided file is written in place; otherwise
  # a temp file backs echo-detection and is streamed to stdout on success.
  local out_file="${CODEX_EXEC_OUT_FILE:-}" cleanup_out=""
  if [ -z "$out_file" ]; then
    out_file="$(mktemp "${TMPDIR:-/tmp}/codex-exec-out.XXXXXX")"
    cleanup_out="$out_file"
  fi

  # Build the timeout wrapper (may be empty). `read` returns non-zero on the
  # no-trailing-newline / empty-here-string case, so `|| true` keeps `set -e`
  # callers alive; to_cmd is already initialized empty for the no-timeout path.
  local -a to_cmd=()
  # shellcheck disable=SC2046,SC2206  # intentional word-split of the wrapper argv.
  read -r -a to_cmd <<<"$(codex_exec_timeout_cmd "${CODEX_EXEC_TIMEOUT:-0}")" || true

  _codex_exec_run() {
    if [ "$stdin_mode" = "1" ]; then
      if [ "${#to_cmd[@]}" -gt 0 ]; then "${to_cmd[@]}" "$bin" "${argv[@]}" >"$out_file" 2>&1
      else "$bin" "${argv[@]}" >"$out_file" 2>&1; fi
    elif [ -n "${CODEX_EXEC_PROMPT_FILE:-}" ]; then
      # File-prompt mode: feed the file on stdin (pawl-review's `< prompt_file`).
      if [ "${#to_cmd[@]}" -gt 0 ]; then "${to_cmd[@]}" "$bin" "${argv[@]}" <"$prompt_file" >"$out_file" 2>&1
      else "$bin" "${argv[@]}" <"$prompt_file" >"$out_file" 2>&1; fi
    else
      # Arg-prompt mode: the prompt is already the last positional in argv.
      if [ "${#to_cmd[@]}" -gt 0 ]; then "${to_cmd[@]}" "$bin" "${argv[@]}" >"$out_file" 2>&1
      else "$bin" "${argv[@]}" >"$out_file" 2>&1; fi
    fi
  }

  local expect_output="${CODEX_EXEC_EXPECT_OUTPUT:-1}"

  local rc=0
  _codex_exec_run || rc=$?

  # Retry-once on a flat 0-byte first run (ported from pawl-review ~line 417): a
  # stall that produced NOTHING gets one more chance before we call it a timeout.
  # Only for output-consuming callers — a fire-and-score producer legitimately
  # produces no stdout, so retrying it would be spurious.
  if [ "$expect_output" = "1" ] && [ ! -s "$out_file" ] \
     && [ "${CODEX_EXEC_RETRY_ON_EMPTY:-1}" = "1" ]; then
    echo "codex-exec: no output on first run (stall) — retrying once…" >&2
    rc=0
    _codex_exec_run || rc=$?
  fi

  _codex_exec_cleanup() {
    unset -f _codex_exec_run
    [ -n "$cleanup_prompt" ] && rm -f "$cleanup_prompt"
  }

  # Classify the outcome into the documented exit codes.
  # 1) TIMEOUT: the wrapper kills with 124 (or 137 on some `timeout` builds when it
  #    escalates to SIGKILL). Map both to STALL-TIMEOUT so a killed run is never
  #    read as a clean pass — ALWAYS, regardless of CODEX_EXEC_EXPECT_OUTPUT (a
  #    kill is never a success). Only meaningful when a timeout was applied.
  if [ "${#to_cmd[@]}" -gt 0 ] && { [ "$rc" -eq 124 ] || [ "$rc" -eq 137 ]; }; then
    echo "codex-exec: STALL — run exceeded the ${CODEX_EXEC_TIMEOUT}s budget and was killed (rc=$rc)." >&2
    echo "  (exit $CODEX_EXEC_STALL_TIMEOUT = stall/timeout, NOT a review result)" >&2
    [ -n "$cleanup_out" ] && rm -f "$cleanup_out"
    _codex_exec_cleanup
    return "$CODEX_EXEC_STALL_TIMEOUT"
  fi

  # The STALL(empty)/ECHO reclassification is only meaningful for callers that
  # CONSUME output. A fire-and-score caller (CODEX_EXEC_EXPECT_OUTPUT=0) discards
  # output, so a clean exit-0 with empty output is a real SUCCESS there.
  if [ "$expect_output" = "1" ]; then
    # 2) Still empty after the retry => a STALL that never produced output.
    if [ ! -s "$out_file" ]; then
      echo "codex-exec: STALL — codex produced no output after a retry." >&2
      echo "  (exit $CODEX_EXEC_STALL_TIMEOUT = stall, NOT a review result)" >&2
      [ -n "$cleanup_out" ] && rm -f "$cleanup_out"
      _codex_exec_cleanup
      return "$CODEX_EXEC_STALL_TIMEOUT"
    fi

    # 3) ECHO: output reflected the prompt back with no `tokens used` marker.
    if [ "$stdin_mode" != "1" ] && codex_exec_looks_echoed "$out_file" "$prompt_file"; then
      echo "codex-exec: ECHO — the output reflected the prompt back with no 'tokens used' marker; no real run happened." >&2
      echo "  (exit $CODEX_EXEC_ECHO = echo, NOT a review result)" >&2
      [ -n "$cleanup_out" ] && rm -f "$cleanup_out"
      _codex_exec_cleanup
      return "$CODEX_EXEC_ECHO"
    fi
  fi

  # 4) GENUINE-NONZERO: codex launched and exited non-zero for its own reason.
  # Return codex's OWN exit code verbatim (category = CODEX_EXEC_GENUINE_NONZERO,
  # but the real code is more useful to a caller recording the agent exit — e.g.
  # eval-agent-harness's `agent_exit`). MISSING is caught before exec, so a codex
  # rc of 2 here is unambiguous.
  if [ "$rc" -ne 0 ]; then
    echo "codex-exec: codex exited non-zero (rc=$rc) with output preserved (genuine codex failure, distinct from a stall/timeout)." >&2
    # Stream the captured output for the caller when we own the sink.
    [ -n "$cleanup_out" ] && { cat "$out_file"; rm -f "$cleanup_out"; }
    _codex_exec_cleanup
    return "$rc"
  fi

  # 5) SUCCESS.
  [ -n "$cleanup_out" ] && { cat "$out_file"; rm -f "$cleanup_out"; }
  _codex_exec_cleanup
  return "$CODEX_EXEC_OK"
}

# codex_exec_producer_template — emit the DEFAULT codex producer/membrane command
# TEMPLATE string used by eval-membrane.sh's pluggable --producer-cmd / --membrane
# -cmd surface, so the literal `codex exec` invocation lives ONLY in this lib (an
# acceptance-allowed file) and NOT in the migrated caller (the acceptance grep
# wants the string off every non-pawl caller). The strings below are byte-
# identical to eval-membrane's historical defaults so behavior is preserved
# exactly.
#   $1 = which template: "producer" (frontier producer) or "membrane" (verifier).
# The returned string is a bash-c template expanded later as: bash -c "<tpl>" _ ...
# (producer: $1=workspace $2=prompt $3=timeout; membrane: $1=reviewer_prompt).
codex_exec_producer_template() {
  case "${1:-producer}" in
    producer)
      printf 'timeout "$3" codex exec --skip-git-repo-check -C "$1" -s workspace-write "$2" >/dev/null 2>&1' ;;
    membrane)
      printf 'codex exec --skip-git-repo-check "$1" 2>/dev/null' ;;
    *) return 2 ;;
  esac
}
