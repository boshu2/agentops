#!/usr/bin/env bash
# shellcheck shell=bash
# scripts/lib/codex-exec.sh — sourced library: ONE fail-closed hardened runner for a
# cold-path REVIEWER, so every non-pawl harness that shells to a reviewer shares the same
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
#   STALL   — a hung reviewer once froze a run for 22 min (eval-membrane
#             age-9h3d). Killed by the timeout/gtimeout wrapper.
#   ECHO    — the prompt is reflected back with no review and no genuine-run
#             marker (pawl-review age-a9iv). Detected here (output ≈ prompt AND
#             no marker) and reported as a DISTINCT exit code.
#   WANDER  — the model greps the filesystem instead of answering (pawl-review
#             age-a9iv, observed on large read-files packets). The caller owns the
#             prompt shape that avoids it; the lib does not run WANDER-prone tools.
#
# ---------------------------------------------------------------------------
# REVIEWER ADAPTER CONTRACT (age-rk3r.1)
# ---------------------------------------------------------------------------
# The cold review path used to be codex-ONLY: the binary, the exec argv shape, and
# the literal "tokens used" genuine-run marker were hardwired. Combined with the
# same-family author rejection in pawl-review.sh (a gpt/codex author has NO cold
# reviewer), a codex outage halted ALL portable validation. This lib now dispatches
# a per-adapter CONTRACT keyed by the REVIEWER env var (default: codex):
#
#   FIELD              codex                agy (cold)              local-mlx (eval-only)
#   -----              -----                ----------             --------------------
#   bin                CODEX_EXEC_BIN|codex REVIEWER_BIN|agy       REVIEWER_BIN|reference wrapper
#   argv template      `exec --sandbox …`   `--sandbox -p <ptr>`  `<prompt>` (single positional)
#   genuine marker     "tokens used"        "VERDICT:"            "VERDICT:"  (override: REVIEWER_MARKER)
#   echo-detector      out ≈ packet, no mk  sentinel + packet-     packet-containment +
#                                           containment + wrapper  payload band
#                                           band (marker CANNOT
#                                           veto packet echo)
#   sandbox mapping    --sandbox <value>    --sandbox (toggle)    n/a (local endpoint)
#   prompt delivery    file(stdin)/arg      FILE-PATH pointer     single positional arg
#                                           (sentinel-wrapped)
#   eval-only?         no                   no                    YES (refuses in prod)
#
#   Adapter 1 = codex — BYTE-COMPATIBLE with the historical behavior: REVIEWER unset
#     (or =codex) produces exactly the pre-adapter argv/exec/classify. The codex-exec.sh
#     bats contract (tests/scripts/codex-exec-lib.bats) is the behavior lock.
#   Adapter 2 = agy (cold, ROUTINE tier + degraded-fallback ONLY, per the A7 bench
#     ruling) — invokes the agy CLI headlessly (`agy -p`, the sanctioned path). The
#     review packet is delivered as a file PATH the model is TOLD to read (a short `-p`
#     pointer + `--add-dir`), NOT a giant inline paste — this DESIGNS OUT the big-content
#     drop bug class (age-9rmh documents that class on the WARM path; the cold adapter is
#     file-PATH-first so it cannot recur cold). agy `-p` emits no CLI runtime footer, so
#     the genuine-run marker is the reviewer-emitted "VERDICT:" token. ASSUMPTION
#     (documented, overridable via REVIEWER_MARKER): "VERDICT:" is a genuine-run signal
#     for agy — but it is a WEAK one, because the PACKET itself contains "VERDICT:"
#     strings (format instructions, diff context), so marker-presence must NEVER veto
#     echo detection against the packet (the age-rk3r.1 refutation's fail-open). The
#     packet is therefore SENTINEL-WRAPPED (random-nonce first/last boundary lines the
#     model is told never to repeat) and the output is additionally screened by a
#     packet-line containment check (reviewer_packet_echoed): a cat/echo of the packet
#     is classified ECHO regardless of any "VERDICT:" content it carries. The short
#     `-p` pointer still contains no "VERDICT:" string and not the nonce, so a pointer
#     echo forges nothing. Because agy is ROUTINE/fallback tier, its (weaker, different)
#     untrusted-repo posture is accepted; it runs `--sandbox` (terminal restrictions) +
#     `--dangerously-skip-permissions` so a headless file-read review does not block on
#     a permission prompt.
#   Adapter 3 = local-mlx — EVAL-ONLY, flag-gated per the 2026-06-23 ruling (local models
#     are EVAL-PATH ONLY; the production gate stays a frontier cross-family review). If
#     invoked in a prod context WITHOUT PAWL_EVAL_ADAPTERS_OK=1 it HARD-REFUSES, naming
#     the ruling, and returns CODEX_EXEC_REFUSED. Reference invocation shape:
#     evals/membrane/membranes/local-mlx-membrane.sh (a wrapper that takes the reviewer
#     prompt as $1 and echoes the model review).
#
# IMPORTANT — behavior-preserving contract: this lib does NOT `set -euo pipefail`
# on behalf of its callers, and it NEVER edits the pawl surfaces (pawl-review.sh /
# pawl-verdict.sh), which own their own richer, verdict-bound flow. The
# functions below are pure/idempotent and safe under either shell mode.
#
# THE HARD CONTRACT: NO-VERDICT ≠ REFUTED. A caller must be able to tell a run
# that produced NOTHING TRUSTWORTHY (stall, echo, missing bin, eval-only refusal)
# apart from a run that produced a real (possibly negative) answer. The distinct
# exit codes below make that legible: a STALL/ECHO/MISSING/REFUSED is a
# PRECONDITION/degraded outcome, not a review result — callers must never read it
# as a clean pass OR a refutation.

# ---------------------------------------------------------------------------
# Documented exit codes (stable contract — callers switch on these):
#   0  CODEX_EXEC_OK             SUCCESS: the reviewer ran to completion, produced output.
#   2  CODEX_EXEC_MISSING        MISSING-BIN: the reviewer binary is not on PATH — a
#                                PRECONDITION failure, NOT a result (matches
#                                pawl-review's exit-2 precondition semantics).
#   3  CODEX_EXEC_GENUINE_NONZERO GENUINE-NONZERO: the reviewer launched and exited
#                                non-zero for its OWN reason (auth error, refusal,
#                                a real task failure) — distinct from a timeout.
#                                For this category the runner returns the reviewer's OWN
#                                exit code VERBATIM (so a caller that records the
#                                agent exit — e.g. eval-agent-harness's
#                                `agent_exit` field — preserves the real code); the
#                                constant 3 is the representative/default value.
#                                MISSING is checked BEFORE exec, so a returned 2
#                                here is unambiguously the reviewer's own code, not MISSING.
#   124 CODEX_EXEC_STALL_TIMEOUT STALL-TIMEOUT: the run exceeded the timeout budget
#                                and was killed (124 = the value `timeout` itself
#                                returns on kill; preserved so callers already
#                                keyed on 124 keep working).
#   125 CODEX_EXEC_ECHO          ECHO: output reflected the prompt back with no
#                                genuine-run marker — no real review happened.
#   126 CODEX_EXEC_REFUSED       REFUSED: an EVAL-ONLY adapter (local-mlx) was invoked in a
#                                prod context without PAWL_EVAL_ADAPTERS_OK=1. A refusal to
#                                run, NOT a result (2026-06-23 ruling: local models are
#                                eval-path only; the prod gate stays frontier cross-family).
# These names are exported as readonly ints so sourcing callers can switch on
# names rather than magic numbers.
# ---------------------------------------------------------------------------
: "${CODEX_EXEC_OK:=0}"
: "${CODEX_EXEC_MISSING:=2}"
: "${CODEX_EXEC_GENUINE_NONZERO:=3}"
: "${CODEX_EXEC_STALL_TIMEOUT:=124}"
: "${CODEX_EXEC_ECHO:=125}"
: "${CODEX_EXEC_REFUSED:=126}"

# codex_exec_timeout_cmd — echo the timeout-wrapper argv (space-separated) for a
# budget, or nothing when no timeout binary exists. Ported READ-ONLY from
# pawl-review.sh (~line 233): PREFER `timeout`, fall back to `gtimeout`, and if
# NEITHER exists degrade to running the reviewer with no timeout rather than failing
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
# Ported READ-ONLY from the pawl-review institutional knowledge: a real run prints a
# genuine-run marker; an echo does not, AND its bytes closely match the prompt bytes.
# Both conditions must hold to call it an echo, so a legitimately short answer that
# merely lacks the marker is NOT mis-flagged.
#   $1 = output file   $2 = prompt file   $3 = genuine-run marker (default 'tokens used')
codex_exec_looks_echoed() {
  local out_file="$1" prompt_file="$2" marker="${3:-tokens used}"
  [ -s "$out_file" ] || return 1                      # empty is a STALL, not an echo
  # A real run emits the genuine-run marker; its presence means NOT an echo.
  if grep -qiF -- "$marker" "$out_file" 2>/dev/null; then return 1; fi
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

# reviewer_packet_echoed — PACKET-content echo detector (age-rk3r.1 refutation fix).
# For adapters whose genuine-run marker is the model-emitted "VERDICT:" token (agy,
# local-mlx), the PACKET ITSELF contains that marker (the verdict-format instructions,
# and diff context lines like " VERDICT: CONFIRMED") — so marker-presence must NOT veto
# echo detection: an invocation that merely cats/reflects the packet would otherwise be
# classified GENUINE, and a downstream last-verdict parser could extract a CONFIRMED
# from echoed packet content (a verdict with no real review — fail-open). Returns 0
# (echo) when EITHER:
#   (a) SENTINEL: the output contains the injected boundary nonce ($3). The agy adapter
#       wraps the packet with first/last sentinel lines the model is told never to
#       repeat, so a full/head/tail cat of the wrapped packet deterministically
#       reproduces one.
#   (b) CONTAINMENT: >= REVIEWER_PACKET_ECHO_PCT% (default 60) of up to 20 distinctive
#       packet lines (length >= 20, evenly sampled across the file; needs >= 3 such
#       lines to make the call) appear VERBATIM (full-line) in the output — a partial
#       echo without the sentinels. A genuine review quoting a few packet lines stays
#       far below the band; reproducing most sampled lines IS an echo. Misclassifying
#       a packet-dominated "review" as ECHO is the SAFE direction (degraded, no false
#       verdict) — fail-closed by design.
#   $1 = output file   $2 = packet file (the ORIGINAL caller content)   $3 = nonce ("" = none)
reviewer_packet_echoed() {
  local out_file="$1" packet_file="$2" nonce="${3:-}"
  [ -s "$out_file" ] || return 1                      # empty is a STALL, not an echo
  if [ -n "$nonce" ] && grep -qF -- "$nonce" "$out_file" 2>/dev/null; then return 0; fi
  [ -f "$packet_file" ] || return 1
  local sample_file n step total=0 matched=0 line
  sample_file="$(mktemp "${TMPDIR:-/tmp}/reviewer-echo-sample.XXXXXX")" || return 1
  awk 'length($0) >= 20' "$packet_file" 2>/dev/null > "$sample_file"
  n="$(wc -l < "$sample_file" | tr -d ' ')"
  if [ "${n:-0}" -lt 3 ]; then rm -f "$sample_file"; return 1; fi
  step=$(( (n + 19) / 20 )); [ "$step" -lt 1 ] && step=1
  while IFS= read -r line; do
    total=$((total + 1))
    grep -qxF -- "$line" "$out_file" 2>/dev/null && matched=$((matched + 1))
  done < <(awk -v s="$step" '(NR - 1) % s == 0' "$sample_file" | head -20)
  rm -f "$sample_file"
  [ "$total" -ge 3 ] || return 1
  [ $(( matched * 100 / total )) -ge "${REVIEWER_PACKET_ECHO_PCT:-60}" ]
}

# --- reviewer adapter contract fields (small pure helpers; the argv template is
#     assembled inline in codex_exec_guarded so it can populate the argv array) ----

# reviewer_normalize <raw> — canonical adapter name (lowercased; aliases folded).
reviewer_normalize() {
  local r; r="$(printf '%s' "${1:-codex}" | tr '[:upper:]' '[:lower:]')"
  case "$r" in
    ""|codex|codex-exec)     printf 'codex' ;;
    agy|gemini|antigravity)  printf 'agy' ;;
    local-mlx|localmlx|local_mlx|mlx) printf 'local-mlx' ;;
    *)                       printf '%s' "$r" ;;
  esac
}

# reviewer_adapter_is_eval_only <reviewer> — return 0 for EVAL-ONLY (local) adapters
# that must never authorize a prod merge without the explicit opt-in (2026-06-23 ruling).
reviewer_adapter_is_eval_only() {
  case "$1" in local-mlx) return 0 ;; *) return 1 ;; esac
}

# reviewer_adapter_bin <reviewer> — the adapter's default binary, honoring overrides.
# codex keeps CODEX_EXEC_BIN (byte-compat with the .8 fixture tests); the other
# adapters share the REVIEWER_BIN override.
reviewer_adapter_bin() {
  case "$1" in
    codex)     printf '%s' "${CODEX_EXEC_BIN:-codex}" ;;
    agy)       printf '%s' "${REVIEWER_BIN:-agy}" ;;
    local-mlx) printf '%s' "${REVIEWER_BIN:-local-mlx-membrane.sh}" ;;
    *)         printf '%s' "${REVIEWER_BIN:-$1}" ;;
  esac
}

# reviewer_adapter_marker <reviewer> — the genuine-run marker substring (grep -iF).
# codex = the CLI runtime footer "tokens used"; agy/local-mlx have no CLI footer, so
# the marker is the model-emitted "VERDICT:" token (documented assumption; override
# via REVIEWER_MARKER).
reviewer_adapter_marker() {
  case "$1" in
    codex)               printf '%s' "${REVIEWER_MARKER:-tokens used}" ;;
    agy|local-mlx)       printf '%s' "${REVIEWER_MARKER:-VERDICT:}" ;;
    *)                   printf '%s' "${REVIEWER_MARKER:-tokens used}" ;;
  esac
}

# codex_exec_guarded — the one fail-closed hardened cold-reviewer runner.
#
# The prompt/model/sandbox/dir/output surface is an env-var + args hybrid (house
# style): the run is configured by env vars set by the caller before the call.
#   REVIEWER                 which adapter to use (codex|agy|local-mlx; default codex).
#   REVIEWER_BIN             override the non-codex adapter binary (a stub in tests).
#   REVIEWER_MODEL           override the non-codex adapter model (empty = adapter default).
#   REVIEWER_MARKER          override the adapter's genuine-run marker.
#   PAWL_EVAL_ADAPTERS_OK    1 => permit an EVAL-ONLY adapter (local-mlx) to run. Absent =>
#                            an eval-only adapter HARD-REFUSES (CODEX_EXEC_REFUSED).
#   CODEX_EXEC_PROMPT_FILE   file whose contents are the prompt (mutually exclusive
#                            with CODEX_EXEC_PROMPT_ARG; a file wins if both set).
#   CODEX_EXEC_PROMPT_ARG    the prompt as a single positional argument.
#                            If NEITHER is set, the prompt is read from stdin.
#   CODEX_EXEC_TIMEOUT       timeout budget in seconds (0/unset = no timeout).
#   CODEX_EXEC_SANDBOX       (codex) sandbox value, e.g. read-only / workspace-write
#                            (default: read-only — the fail-closed default).
#   CODEX_EXEC_MODEL         (codex) model id for `-m` (empty/unset = codex default).
#   CODEX_EXEC_DIR           (codex) working dir for `-C` (empty/unset = no -C).
#   CODEX_EXEC_SKIP_GIT_CHECK (codex) 1 => pass --skip-git-repo-check (non-git workspaces).
#   CODEX_EXEC_EXTRA_ARGS    (codex) a bash array of extra passthrough flags appended
#                            verbatim (e.g. --json). Ignored by non-codex adapters (they
#                            are codex-specific flags).
#   CODEX_EXEC_OUT_FILE      write captured stdout+stderr here. If empty, output is
#                            captured to a temp file used only for echo-detection
#                            and then streamed to the caller's stdout on success.
#   CODEX_EXEC_RETRY_ON_EMPTY 1 (default) => retry ONCE on a flat 0-byte first run
#                            (the pawl-review stall-retry). 0 disables the retry.
#   CODEX_EXEC_EXPECT_OUTPUT 1 (default) => the caller CONSUMES reviewer output, so a
#                            flat 0-byte run is a STALL and an output≈prompt run is
#                            an ECHO (both fail-closed). 0 => the caller only cares
#                            about the EXIT CODE and discards output (e.g. a
#                            fire-and-score producer): a clean exit-0 with empty
#                            output is SUCCESS, and the stall/echo reclassification
#                            is skipped. A killed/timeout run is STILL a STALL under
#                            both settings (a kill is never a success).
#   CODEX_EXEC_BIN           (codex) the codex binary (default: codex) — lets a test feed
#                            a stub, matching second-poll.sh's CODEX_BIN convention.
#
# Returns one of the documented exit codes above. On CODEX_EXEC_OK the output is
# in CODEX_EXEC_OUT_FILE (if set) or on stdout.
codex_exec_guarded() {
  local reviewer; reviewer="$(reviewer_normalize "${REVIEWER:-codex}")"

  # (0) EVAL-ONLY GATE (2026-06-23 ruling: local models are EVAL-PATH ONLY; the prod
  # gate stays a frontier cross-family review). An eval-only adapter invoked WITHOUT the
  # explicit opt-in HARD-REFUSES — it must never authorize a prod merge.
  if reviewer_adapter_is_eval_only "$reviewer" && [ "${PAWL_EVAL_ADAPTERS_OK:-0}" != "1" ]; then
    echo "codex-exec: REFUSED — reviewer '$reviewer' is a LOCAL, EVAL-ONLY model adapter." >&2
    echo "  Per the 2026-06-23 ruling, local models are EVAL-PATH ONLY; the production pawl" >&2
    echo "  gate must stay a frontier cross-family review (a weaker local reviewer must never" >&2
    echo "  authorize a merge). Set PAWL_EVAL_ADAPTERS_OK=1 to use this adapter on the EVAL" >&2
    echo "  path only. (exit $CODEX_EXEC_REFUSED = refused, NOT a review result)" >&2
    return "$CODEX_EXEC_REFUSED"
  fi

  # (1) resolve the adapter's binary + genuine-run marker (the contract fields).
  local bin marker
  bin="$(reviewer_adapter_bin "$reviewer")"
  marker="$(reviewer_adapter_marker "$reviewer")"
  # local-mlx opted-in: if no REVIEWER_BIN override, default the bin to the reference
  # membrane wrapper next to this lib (evals/membrane/membranes/local-mlx-membrane.sh).
  if [ "$reviewer" = "local-mlx" ] && [ -z "${REVIEWER_BIN:-}" ]; then
    local _lib_dir _mlx_ref
    # `CDPATH=` is an intentional env-prefix (clears CDPATH for that one cd), not an
    # assignment — hence the SC1007 disable (matches scripts/lib/preamble.sh convention).
    # shellcheck disable=SC1007
    _lib_dir="$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    _mlx_ref="$_lib_dir/../../evals/membrane/membranes/local-mlx-membrane.sh"
    [ -x "$_mlx_ref" ] && bin="$_mlx_ref"
  fi

  # PRECONDITION (ported from pawl-review ~line 204): a missing reviewer binary is a
  # PRECONDITION failure with its OWN exit code — never a review result. Name it,
  # say it is installable, and return the distinct code so a caller can tell a
  # missing dep apart from a real refutation.
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "codex-exec: MISSING DEPENDENCY — '$bin' is not on PATH." >&2
    echo "  This is NOT a result. Install the '$reviewer' reviewer CLI and put it on PATH, then re-run." >&2
    echo "  (exit $CODEX_EXEC_MISSING = precondition, not a REFUTE / not a genuine failure)" >&2
    return "$CODEX_EXEC_MISSING"
  fi

  # (2) assemble the reviewer argv + resolve the prompt delivery + the echo-compare
  # target, per adapter. `delivery` is stdin_file (redirect a file on stdin) or plain
  # (no redirect); `run_echo_check` gates echo-detection off for stdin-pipe callers;
  # `echo_cmp_file` is the bytes an echo would reflect (what the model received INLINE).
  local -a argv=()
  local -a _cleanup=()
  local prompt_file="" delivery="plain" run_echo_check=1 echo_cmp_file=""
  # Packet-echo compare targets (agy/local-mlx — the VERDICT:-marker adapters, whose
  # packets legitimately CONTAIN the marker so marker-presence cannot veto echo checks;
  # see reviewer_packet_echoed). Empty for codex => the check is skipped (byte-compat).
  local packet_echo_file="" packet_nonce=""

  case "$reviewer" in
    codex)
      # BYTE-COMPATIBLE with the historical codex path. Order is stable so stub-`codex`
      # tests that inspect positional args (eval-agent-harness.bats records the -C dir)
      # keep working.
      argv=(exec)
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
      # Resolve the prompt source (byte-compat): FILE => stdin redirect; ARG =>
      # positional; NEITHER => stdin (no echo-detection).
      if [ -n "${CODEX_EXEC_PROMPT_FILE:-}" ]; then
        prompt_file="$CODEX_EXEC_PROMPT_FILE"; delivery="stdin_file"; echo_cmp_file="$prompt_file"
      elif [ -n "${CODEX_EXEC_PROMPT_ARG:-}" ]; then
        argv+=("$CODEX_EXEC_PROMPT_ARG")
        prompt_file="$(mktemp "${TMPDIR:-/tmp}/codex-exec-prompt.XXXXXX")"; _cleanup+=("$prompt_file")
        printf '%s' "$CODEX_EXEC_PROMPT_ARG" > "$prompt_file"; echo_cmp_file="$prompt_file"
      else
        run_echo_check=0
      fi
      ;;

    agy)
      # File-PATH-first (designs out the age-9rmh giant-inline drop class): resolve the
      # review packet to a FILE, then hand agy a SHORT pointer telling it to READ that
      # file — never a giant inline paste on the argv.
      local packet_file
      if [ -n "${CODEX_EXEC_PROMPT_FILE:-}" ]; then
        packet_file="$CODEX_EXEC_PROMPT_FILE"
      elif [ -n "${CODEX_EXEC_PROMPT_ARG:-}" ]; then
        packet_file="$(mktemp "${TMPDIR:-/tmp}/reviewer-packet.XXXXXX")"; _cleanup+=("$packet_file")
        printf '%s' "$CODEX_EXEC_PROMPT_ARG" > "$packet_file"
      else
        packet_file="$(mktemp "${TMPDIR:-/tmp}/reviewer-packet.XXXXXX")"; _cleanup+=("$packet_file")
        cat > "$packet_file"   # drain stdin into the packet file
      fi
      # SENTINEL WRAP (age-rk3r.1 refutation fix): the pointer references a lib-owned
      # WRAPPED copy of the packet whose first and last lines carry a random boundary
      # nonce the model is told never to repeat. A broken/echoing run that cats the
      # packet (head, tail, or whole) reproduces a sentinel and is caught as ECHO by
      # reviewer_packet_echoed — the "VERDICT:" marker cannot save it, because the
      # packet itself legitimately contains "VERDICT:" strings.
      local wrapped_packet
      packet_nonce="$(od -An -N8 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n')"
      [ -n "$packet_nonce" ] || packet_nonce="$$-${RANDOM}${RANDOM}-$(date +%s)"
      wrapped_packet="$(mktemp "${TMPDIR:-/tmp}/reviewer-packet-wrapped.XXXXXX")"; _cleanup+=("$wrapped_packet")
      {
        printf '[[REVIEWER-PACKET-SENTINEL %s]] internal boundary marker — NEVER repeat, quote, or reference this line in your reply.\n' "$packet_nonce"
        cat "$packet_file"
        printf '\n[[REVIEWER-PACKET-SENTINEL %s]] internal boundary marker — NEVER repeat, quote, or reference this line in your reply.\n' "$packet_nonce"
      } > "$wrapped_packet"
      packet_echo_file="$packet_file"   # containment compares against the ORIGINAL content
      # The SHORT pointer agy receives inline. It deliberately contains NO marker string
      # ("VERDICT:") and NOT the nonce itself, so an echo of the pointer can forge
      # neither the genuine-run marker nor evade the sentinel check.
      local wrapper
      wrapper="Read the code-review packet at the absolute path ${wrapped_packet} and perform the review it describes. The packet's first and last lines are internal boundary markers — never repeat them. Reply with ONLY your review, following the packet's required final-line format exactly. Do not modify anything or use the terminal."
      # sandbox mapping (agy): `--sandbox` toggles terminal restrictions ON (the read-only
      # reviewer posture); `--dangerously-skip-permissions` so a headless file-read review
      # does not block on a permission prompt. ROUTINE/fallback tier (A7 ruling): the
      # (weaker) untrusted-repo posture is accepted for this cold secondary reviewer.
      argv+=(--sandbox --dangerously-skip-permissions)
      [ -n "${REVIEWER_MODEL:-}" ] && argv+=(--model "$REVIEWER_MODEL")
      argv+=(--add-dir "$(dirname "$wrapped_packet")")
      # Align agy's own print-mode wait to our kill budget so it does not wait past it.
      [ -n "${CODEX_EXEC_TIMEOUT:-}" ] && [ "${CODEX_EXEC_TIMEOUT:-0}" != "0" ] && argv+=(--print-timeout "${CODEX_EXEC_TIMEOUT}s")
      argv+=(-p "$wrapper")
      delivery="plain"
      # A pointer-echo reflects the SHORT wrapper agy received inline — compare vs THAT.
      echo_cmp_file="$(mktemp "${TMPDIR:-/tmp}/reviewer-wrapper.XXXXXX")"; _cleanup+=("$echo_cmp_file")
      printf '%s' "$wrapper" > "$echo_cmp_file"
      ;;

    local-mlx|*)
      # local-mlx (opted-in, EVAL-ONLY) + the generic fallback adapter: invoke the bin
      # with the prompt as a single positional arg (the reference membrane wrapper's
      # `bash <script> "<prompt>"` shape). A local endpoint has no CLI big-content drop
      # bug, so inline is fine; the wrapper relays the model review.
      local payload
      if [ -n "${CODEX_EXEC_PROMPT_FILE:-}" ]; then payload="$(cat "$CODEX_EXEC_PROMPT_FILE")"
      elif [ -n "${CODEX_EXEC_PROMPT_ARG:-}" ]; then payload="$CODEX_EXEC_PROMPT_ARG"
      else payload="$(cat)"; fi
      argv+=("$payload")
      delivery="plain"
      echo_cmp_file="$(mktemp "${TMPDIR:-/tmp}/reviewer-mlx-prompt.XXXXXX")"; _cleanup+=("$echo_cmp_file")
      printf '%s' "$payload" > "$echo_cmp_file"
      # The positional IS the packet, and it may contain the "VERDICT:" marker — run the
      # marker-independent packet-containment echo check against it (no sentinel here;
      # the reference wrapper pins a 2-line output, so containment alone suffices).
      packet_echo_file="$echo_cmp_file"
      ;;
  esac

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
    if [ "$delivery" = "stdin_file" ]; then
      # File-prompt mode: feed the file on stdin (pawl-review's `< prompt_file`).
      if [ "${#to_cmd[@]}" -gt 0 ]; then "${to_cmd[@]}" "$bin" "${argv[@]}" <"$prompt_file" >"$out_file" 2>&1
      else "$bin" "${argv[@]}" <"$prompt_file" >"$out_file" 2>&1; fi
    else
      # Arg/stdin-pipe/pointer mode: the prompt is already in argv (or on the caller's stdin).
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
    [ "${#_cleanup[@]}" -gt 0 ] && rm -f "${_cleanup[@]}"
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
      echo "codex-exec: STALL — reviewer produced no output after a retry." >&2
      echo "  (exit $CODEX_EXEC_STALL_TIMEOUT = stall, NOT a review result)" >&2
      [ -n "$cleanup_out" ] && rm -f "$cleanup_out"
      _codex_exec_cleanup
      return "$CODEX_EXEC_STALL_TIMEOUT"
    fi

    # 3a) PACKET ECHO (age-rk3r.1 refutation fix): the output reflected the review
    # PACKET content back (sentinel nonce present, or most sampled packet lines appear
    # verbatim). Checked FIRST and INDEPENDENTLY of the genuine-run marker — the packet
    # itself contains "VERDICT:" strings, so an echoed packet WOULD carry the marker
    # and a downstream verdict parser could extract a false CONFIRMED from it.
    if [ -n "$packet_echo_file" ] && reviewer_packet_echoed "$out_file" "$packet_echo_file" "$packet_nonce"; then
      echo "codex-exec: ECHO — the output reflected the review PACKET content back (sentinel or verbatim packet lines present); no real review happened." >&2
      echo "  (exit $CODEX_EXEC_ECHO = echo, NOT a review result)" >&2
      [ -n "$cleanup_out" ] && rm -f "$cleanup_out"
      _codex_exec_cleanup
      return "$CODEX_EXEC_ECHO"
    fi

    # 3b) ECHO: output reflected the prompt back with no genuine-run marker.
    if [ "$run_echo_check" = "1" ] && codex_exec_looks_echoed "$out_file" "$echo_cmp_file" "$marker"; then
      echo "codex-exec: ECHO — the output reflected the prompt back with no '$marker' marker; no real run happened." >&2
      echo "  (exit $CODEX_EXEC_ECHO = echo, NOT a review result)" >&2
      [ -n "$cleanup_out" ] && rm -f "$cleanup_out"
      _codex_exec_cleanup
      return "$CODEX_EXEC_ECHO"
    fi
  fi

  # 4) GENUINE-NONZERO: the reviewer launched and exited non-zero for its own reason.
  # Return the reviewer's OWN exit code verbatim (category = CODEX_EXEC_GENUINE_NONZERO,
  # but the real code is more useful to a caller recording the agent exit — e.g.
  # eval-agent-harness's `agent_exit`). MISSING is caught before exec, so a rc of 2
  # here is unambiguous.
  if [ "$rc" -ne 0 ]; then
    echo "codex-exec: reviewer exited non-zero (rc=$rc) with output preserved (genuine reviewer failure, distinct from a stall/timeout)." >&2
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
