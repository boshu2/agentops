#!/usr/bin/env bash
# shellcheck shell=bash
# scripts/lib/codex-exec.sh — one-shot runner for a caller-selected reviewer.
#
# Source it (do NOT execute it):
#     . "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/codex-exec.sh"
#
# It executes once, records output, distinguishes missing binaries, timeouts,
# and prompt echoes, and returns. It owns no reviewer selection, retry,
# validation, admission, merge, or continuation decision.
#
# ---------------------------------------------------------------------------
# REVIEWER ADAPTER CONTRACT (age-rk3r.1)
# ---------------------------------------------------------------------------
# The review path used to be codex-only: the binary, the exec argv shape, and
# the literal "tokens used" genuine-run marker were hardwired. This library dispatches
# a per-adapter CONTRACT keyed by the REVIEWER env var (default: codex):
#
#   FIELD              codex                agy (cold)              local-mlx (eval-only)
#   -----              -----                ----------             --------------------
#   bin                CODEX_EXEC_BIN|codex REVIEWER_BIN|agy       REVIEWER_BIN (required)
#   argv template      `exec --sandbox …`   `--sandbox -p <ptr>`  `<prompt>` (single positional)
#   genuine marker     "tokens used"        "VERDICT:"            "VERDICT:"  (override: REVIEWER_MARKER)
#   echo-detector      out ≈ packet, no mk  sentinel + packet-     packet-containment +
#                                           containment + wrapper  payload band
#                                           band (marker CANNOT
#                                           veto packet echo)
#   sandbox mapping    --sandbox <value>    --sandbox (toggle)    n/a (local endpoint)
#   sandbox mapping    --dangerously-bypass- n/a (ignores the     n/a (ignores the
#   (codex, wrapped)   approvals-and-sandbox wrap prefix)          wrap prefix)
#                      (the outer CODEX_EXEC_WRAP wrapper IS the sandbox; codex's
#                      own is bypassed because seatbelt does not nest — see below)
#   prompt delivery    file(stdin)/arg      FILE-PATH pointer     single positional arg
#                                           (sentinel-wrapped)
#   local?             no                   no                    yes
#
#   Adapter 1 = codex — preserves the historical execution/classification behavior.
#     Arg-mode prompt delivery inserts the standard `--` option terminator so a
#     prompt beginning with `-` remains prompt data rather than CLI options. The
#     codex-exec.sh Bats contract (tests/scripts/codex-exec-lib.bats) is the lock.
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
#   Adapter 3 = local-mlx, a caller-selected local adapter. REVIEWER_BIN is required
#     (no default wrapper ships): a program that takes the reviewer prompt as $1 and
#     echoes the model review.
#
# This library does not set shell options or mutate caller state. Its outcome is
# runtime evidence, never a semantic verdict.
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
#                                the runner's exit-2 precondition semantics).
#   3  CODEX_EXEC_GENUINE_NONZERO GENUINE-NONZERO: the reviewer launched and exited
#                                non-zero for its OWN reason (auth error, refusal,
#                                a real task failure) — distinct from a timeout.
#                                For this category the runner returns the reviewer's OWN
#                                exit code VERBATIM (so a caller that records the
#                                agent exit — e.g. eval-agent-harness's
#                                `agent_exit` field — preserves the real code); the
#                                constant 3 is the representative/default value.
#                                Reserved adapter codes can also describe runtime
#                                failures; use the accompanying diagnostic rather
#                                than treating status alone as reviewer provenance.
#   124 CODEX_EXEC_STALL_TIMEOUT STALL-TIMEOUT: the run exceeded the timeout budget
#                                and was killed (124 = the value `timeout` itself
#                                returns on kill; preserved so callers already
#                                keyed on 124 keep working).
#   125 CODEX_EXEC_ECHO          ECHO: output reflected the prompt back with no
#                                genuine-run marker — no real review happened.
#   123 CODEX_EXEC_OUTPUT_LIMIT  OUTPUT-LIMIT: capture or prompt preparation exceeded
#                                its byte cap; partial evidence is retained.
#   122 CODEX_EXEC_DESCENDANT_LEAK rep-survivor: the owned group retained members
#                                after direct-parent exit. The run is degraded
#                                even when the adapter cleans them successfully.
#   129/130/143                 Cancellation by HUP/INT/TERM, respectively.
#   2 also covers invalid bounds, unavailable capture/cleanup capability, and
#     cleanup that remains unverified after its bounded window.
# These shell defaults let sourcing callers switch on names rather than magic
# numbers; they are runtime categories, not semantic judgments.
# ---------------------------------------------------------------------------
: "${CODEX_EXEC_OK:=0}"
: "${CODEX_EXEC_MISSING:=2}"
: "${CODEX_EXEC_GENUINE_NONZERO:=3}"
: "${CODEX_EXEC_STALL_TIMEOUT:=124}"
: "${CODEX_EXEC_ECHO:=125}"
: "${CODEX_EXEC_OUTPUT_LIMIT:=123}"
: "${CODEX_EXEC_DESCENDANT_LEAK:=122}"

# Private execution mechanism for this adapter, using only the host Perl core.
# `limits` validates before any reviewer launch; `run` captures one owned process
# group. No setsid executable, global file-size ulimit, retry, or service is used.
# The child establishes its own group BEFORE exec. The parent alone writes the
# bounded capture files and kills ordinary descendants even after a clean exit.
# Descendants that deliberately escape into other groups/sessions are out of scope.
_codex_exec_control() {
  local control_exec=command
  [ "$1" != run ] || control_exec="exec" # run is always a dedicated background child.
  # shellcheck disable=SC2016 # This is Perl source, not shell interpolation.
  "$control_exec" /usr/bin/perl -MPOSIX=:sys_wait_h,isfinite,setpgid -MTime::HiRes=time,clock_gettime,CLOCK_MONOTONIC -MIO::Select -MFcntl=:DEFAULT -e '
    use strict; use warnings;
    sub fail { print STDERR "codex-exec: $_[0]\n"; exit 2 }
    sub positive {
      my ($name, $value, $integer) = @_;
      my $pattern = $integer ? qr/\A[0-9]+\z/ : qr/\A(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)\z/;
      fail("INVALID-LIMIT: $name must be positive and finite")
        unless defined($value) && $value =~ $pattern && isfinite(0 + $value) && $value > 0;
      return 0 + $value;
    }
    my $mode = shift @ARGV;
    if ($mode eq "limits") {
      my ($timeout, $cap, $deadline, $has_deadline, $has_timeout) = @ARGV;
      # Existing direct callers supply an explicit timeout in four arguments.
      # The fifth presence flag distinguishes omission from an empty value.
      $has_timeout = 1 if @ARGV < 5;
      positive("CODEX_EXEC_TIMEOUT", $timeout, 0) if $has_timeout;
      positive("CODEX_EXEC_MAX_OUTPUT_BYTES", $cap, 1);
      fail("INVALID-LIMIT: capture cap exceeds exact integer range") if $cap > 9007199254740991;
      positive("CODEX_EXEC_DEADLINE_EPOCH", $deadline, 0) if $has_deadline;
      fail("MISSING-LIMIT: set CODEX_EXEC_TIMEOUT or CODEX_EXEC_DEADLINE_EPOCH") unless $has_timeout || $has_deadline;
      my $now = time;
      my $end;
      if ($has_timeout) {
        $end = $now + $timeout;
        fail("INVALID-LIMIT: timeout is too large") unless isfinite($end);
      }
      if ($has_deadline) {
        if ($deadline <= $now) { print STDERR "codex-exec: STALL — caller deadline already expired; reviewer not launched.\n"; exit 124 }
        if (!$has_timeout || $deadline < $end) { $end = 0 + $deadline; $timeout = $end - $now }
      }
      clock_gettime(CLOCK_MONOTONIC); # Unsupported clock capability fails before launch.
      printf "%s %s %.6f\n", $timeout, $cap, $end;
      exit 0;
    }
    fail("unrecognized adapter control mode") unless $mode eq "run";
    my $source_owner = shift @ARGV;
    my ($budget, $cap, $end, $out_path, $err_path, $input_path) = splice @ARGV, 0, 6;
    my $remaining = $end - time;
    if ($remaining <= 0) { print STDERR "codex-exec: STALL — deadline expired before launch.\n"; exit 124 }
    $remaining = $budget if $budget < $remaining;
    my $expires = clock_gettime(CLOCK_MONOTONIC) + $remaining;
    my ($cancel, $owned, $pid, $reaped, $child_status) = (0, 0, 0, 0, 0);
    $SIG{HUP} = sub { $cancel = 129 }; $SIG{INT} = sub { $cancel = 130 }; $SIG{TERM} = sub { $cancel = 143 };
    # Even an unexpected capture error must not abandon an owned worker group.
    END { if ($owned && $pid) { kill "KILL", -$pid; kill "KILL", $pid unless $reaped; waitpid($pid, WNOHANG) unless $reaped } }
    sub sink {
      my ($path) = @_;
      sysopen(my $fh, $path, O_WRONLY | O_CREAT | O_TRUNC | O_NONBLOCK, 0600) or fail("CAPTURE-UNAVAILABLE: $path: $!");
      fail("CAPTURE-UNAVAILABLE: sink must be a regular file or /dev/null") unless -f $fh || $path eq "/dev/null";
      return $fh;
    }
    my $out = sink($out_path);
    my $err = length($err_path) ? ($err_path eq $out_path ? $out : sink($err_path)) : $out;
    pipe(my $read_out, my $write_out) or fail("CAPTURE-UNAVAILABLE: pipe: $!");
    pipe(my $read_err, my $write_err) or fail("CAPTURE-UNAVAILABLE: pipe: $!");
    my $parent = getppid();
    $pid = fork(); defined $pid or fail("CAPTURE-UNAVAILABLE: fork: $!");
    if (!$pid) {
      $owned = 0;
      # A failed group setup must never run the reviewer in the caller group.
      setpgid(0, 0) == 0 or fail("CLEANUP-UNAVAILABLE: setpgid: $!");
      $SIG{HUP} = $SIG{INT} = $SIG{TERM} = "DEFAULT";
      open STDOUT, ">&", $write_out or POSIX::_exit(2);
      open STDERR, ">&", (length($err_path) ? $write_err : $write_out) or POSIX::_exit(2);
      if (length $input_path) { open STDIN, "<", $input_path or fail("PROMPT-UNAVAILABLE: $input_path: $!") }
      close $read_out; close $read_err; close $write_out; close $write_err; close $out; close $err;
      exec { $ARGV[0] } @ARGV or fail("EXEC-UNAVAILABLE: $ARGV[0]: $!");
    }
    $owned = 1;
    # Close the fork/exec race too: cleanup can signal the group immediately.
    setpgid($pid, $pid);
    close $write_out; close $write_err;
    my $select = IO::Select->new($read_out, $read_err);
    my %sinks = (fileno($read_out) => $out, fileno($read_err) => $err);
    my ($written, $reason, $stop_at, $killed, $survivor) = (0, "", undef, 0, 0);
    while (1) {
      my $now = clock_gettime(CLOCK_MONOTONIC);
      $cancel ||= 143 if getppid() != $parent || !kill(0, $source_owner);
      if (!$reaped) {
        my $done = waitpid($pid, WNOHANG);
        if ($done == $pid) { $child_status = $?; $reaped = 1 }
      }
      if (!defined $stop_at) {
        if ($cancel) { $reason = "cancel" }
        elsif ($now >= $expires || time >= $end) { $reason = "timeout" }
        elsif ($reaped) {
          $reason = "exit";
          # Preserve the defect BEFORE cleanup hides it from an outer caller.
          # With the direct parent reaped, remaining group members outlived it.
          if (kill(0, -$pid)) {
            $survivor = 1;
            print STDERR "codex-exec: rep-survivor — owned group $pid retained members after direct-parent exit; run degraded.\n";
          }
        }
        if (length $reason) { $stop_at = $now; kill "TERM", -$pid }
      }
      # Fixed, short cleanup allowance; it never renews the caller deadline.
      if (defined($stop_at) && !$killed && $now >= $stop_at + .2) {
        kill "KILL", -$pid; kill "KILL", $pid unless $reaped;
        $killed = 1;
      }
      $killed = 1 if defined($stop_at) && $reaped && !kill(0, -$pid);
      for my $fh ($select->can_read(.02)) {
        my $n = sysread($fh, my $chunk, 16384);
        if (!defined $n) { next if $!{EINTR} || $!{EAGAIN}; fail("CAPTURE-UNAVAILABLE: read: $!") }
        if (!$n) { $select->remove($fh); close $fh; next }
        my $keep = $cap - $written; $keep = $n if $keep > $n;
        if ($keep > 0) {
          my $offset = 0;
          while ($offset < $keep) {
            my $count = syswrite($sinks{fileno($fh)}, $chunk, $keep - $offset, $offset);
            if (!defined $count) { next if $!{EINTR}; fail("CAPTURE-UNAVAILABLE: write: $!") }
            fail("CAPTURE-UNAVAILABLE: zero-byte write") unless $count;
            $offset += $count;
          }
          $written += $keep;
        }
        if ($n > $keep && $reason ne "limit" && $reason ne "timeout" && $reason ne "cancel") {
          $reason = "limit";
          if (!defined $stop_at) { $stop_at = clock_gettime(CLOCK_MONOTONIC); kill "TERM", -$pid }
        }
      }
      # Waiting for EOF alone is unsafe: descendants can inherit the pipes.
      last if $killed && (($reaped && !$select->count && !kill(0, -$pid)) || $now >= $stop_at + .5);
    }
    # Signal delivery is not proof of cleanup. Do not claim success if the
    # process group still exists; unresolved zombies also conservatively fail.
    if (!$reaped || kill(0, -$pid)) {
      print STDERR "codex-exec: CLEANUP-UNVERIFIED — owned process group still present after bounded cleanup; partial output preserved.\n";
      exit 2;
    }
    $owned = 0;
    close $out; close $err;
    if ($reason eq "limit") { print STDERR "codex-exec: OUTPUT-LIMIT — captured $written bytes; owned process group stopped.\n"; exit 123 }
    if ($reason eq "timeout") { print STDERR "codex-exec: STALL — deadline expired; owned process group stopped.\n"; exit 124 }
    if ($reason eq "cancel") { print STDERR "codex-exec: CANCELLED — owned process group stopped.\n"; exit $cancel }
    exit 122 if $survivor;
    exit(($child_status & 127) ? 128 + ($child_status & 127) : $child_status >> 8);
  ' -- "$@"
}

# Wait interruptibly so signalling a shell that sourced the library cancels the
# owned invocation. Restore its signal handlers before returning; no caller shell
# options or global limits change. The supervisor also detects parent death.
_codex_exec_bounded() {
  local saved_hup saved_int saved_term control_pid="" cancelled=0 rc=0
  saved_hup="$(trap -p HUP)"; saved_int="$(trap -p INT)"; saved_term="$(trap -p TERM)"
  trap 'cancelled=129; [ -z "$control_pid" ] || kill -HUP "$control_pid" 2>/dev/null || true' HUP
  trap 'cancelled=130; [ -z "$control_pid" ] || kill -INT "$control_pid" 2>/dev/null || true' INT
  trap 'cancelled=143; [ -z "$control_pid" ] || kill -TERM "$control_pid" 2>/dev/null || true' TERM
  _codex_exec_control run "$$" "$@" <&0 &
  control_pid=$!
  [ "$cancelled" -eq 0 ] || kill -TERM "$control_pid" 2>/dev/null || true
  while :; do
    rc=0; wait "$control_pid" || rc=$?
    kill -0 "$control_pid" 2>/dev/null || break
  done
  trap - HUP INT TERM
  [ -z "$saved_hup" ] || eval "$saved_hup"
  [ -z "$saved_int" ] || eval "$saved_int"
  [ -z "$saved_term" ] || eval "$saved_term"
  [ "$cancelled" -eq 0 ] || return "$cancelled"
  return "$rc"
}

# codex_exec_timeout_bin — the ABSOLUTE path of a timeout that supports
# `--foreground`, or return 3 when it is missing or unsupported. The bounded
# capability probe cannot launch a reviewer or hang indefinitely.
#
# WHY --foreground: keep timeout and the reviewer in the supervisor-owned group.
#
# WHY absolute: the path is recorded in the probe seal and the wrapper runs
# INSIDE the sandbox, so a `timeout` resolved fresh from PATH at dispatch time
# could be a different binary than the one the record names.
codex_exec_timeout_bin() {
  local candidate resolved
  # A caller that already resolved and probed one passes it down, so the probe
  # does not exec a PATH-resolved timeout OUTSIDE the seal once per rep.
  if [ -n "${CODEX_EXEC_TIMEOUT_BIN:-}" ]; then
    case "$CODEX_EXEC_TIMEOUT_BIN" in
      /*) [ -x "$CODEX_EXEC_TIMEOUT_BIN" ] || return 3
          printf '%s' "$CODEX_EXEC_TIMEOUT_BIN"; return 0 ;;
      *) return 3 ;;
    esac
  fi
  for candidate in timeout gtimeout; do
    resolved="$(command -v "$candidate" 2>/dev/null)" || continue
    [ -n "$resolved" ] || continue
    case "$resolved" in /*) ;; *) continue ;; esac
    # The FIRST candidate that resolves is the resolved timeout. Falling through
    # to the next one would quietly run a different binary than the one a
    # shadowing PATH selected, which is the failure this is meant to surface.
    # Functional probe, not `--help` parsing: BSD timeout has no --help at all.
    local probe_limits probe_budget probe_cap probe_end
    probe_limits="$(_codex_exec_control limits 1 1024 "${CODEX_EXEC_DEADLINE_EPOCH-}" "${CODEX_EXEC_DEADLINE_EPOCH+1}" 1)" || return 3
    read -r probe_budget probe_cap probe_end <<<"$probe_limits"
    if _codex_exec_bounded "$probe_budget" "$probe_cap" "$probe_end" /dev/null "" /dev/null "$resolved" --foreground 1 true >/dev/null 2>&1; then
      printf '%s' "$resolved"
      return 0
    fi
    return 3
  done
  return 3
}

# codex_exec_timeout_cmd — echo the timeout-wrapper argv (space-separated) for a
# supplied positive budget, or CODEX_EXEC_TIMEOUT / CODEX_EXEC_DEADLINE_EPOCH.
# A positional budget overrides CODEX_EXEC_TIMEOUT; a deadline always clamps it.
# Invalid/missing bounds or unsupported enforcement fail closed (return 3).
# Resolve immediately before use; reuse the caller absolute deadline across retries.
# Usage: _argv="$(codex_exec_timeout_cmd "$caller_timeout")" || return "$?"
#        read -r -a _to <<<"$_argv"; "${_to[@]}" codex ...
codex_exec_timeout_cmd() {
  local budget="${1-${CODEX_EXEC_TIMEOUT-}}" has_timeout="${CODEX_EXEC_TIMEOUT+1}" bin="" rc=0
  local limits _cap deadline
  [ "$#" -eq 0 ] || has_timeout=1
  limits="$(_codex_exec_control limits "$budget" 1 "${CODEX_EXEC_DEADLINE_EPOCH-}" "${CODEX_EXEC_DEADLINE_EPOCH+1}" "$has_timeout")" || return 3
  read -r budget _cap deadline <<<"$limits"
  bin="$(CODEX_EXEC_DEADLINE_EPOCH="$deadline" codex_exec_timeout_bin)" || rc=$?
  [ "$rc" = "0" ] || return "$rc"
  [ -n "$bin" ] || return 3
  # A capability probe consumes this budget too; do not restart it on return.
  limits="$(_codex_exec_control limits "$budget" 1 "$deadline" 1 1)" || return 3
  read -r budget _cap deadline <<<"$limits"
  printf '%s --foreground %s' "$bin" "$budget"
}

# codex_exec_looks_echoed — return 0 (true) when the captured output looks like an
# ECHO of the prompt (a WANDER/ECHO failure with no real review), 1 otherwise.
# A real run prints a
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

# reviewer_adapter_bin <reviewer> — the adapter's default binary, honoring overrides.
# codex keeps CODEX_EXEC_BIN (byte-compat with the .8 fixture tests); the other
# adapters share the REVIEWER_BIN override.
reviewer_adapter_bin() {
  case "$1" in
    codex)     printf '%s' "${CODEX_EXEC_BIN:-codex}" ;;
    agy)       printf '%s' "${REVIEWER_BIN:-agy}" ;;
    local-mlx) printf '%s' "${REVIEWER_BIN:-}" ;;
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
#   CODEX_EXEC_PROMPT_FILE   file whose contents are the prompt (mutually exclusive
#                            with CODEX_EXEC_PROMPT_ARG; a file wins if both set).
#   CODEX_EXEC_PROMPT_ARG    the prompt as a single positional argument.
#                            If NEITHER is set, the prompt is read from stdin.
#   CODEX_EXEC_TIMEOUT       positive finite seconds; no default, empty invalid.
#   CODEX_EXEC_DEADLINE_EPOCH optional positive absolute Unix timestamp in seconds;
#                            supplies the remaining timeout when timeout is omitted;
#                            otherwise the earlier bound wins. One bound is required.
#                            Includes preparation; reuse the same absolute value
#                            across calls to preserve a caller deadline.
#   CODEX_EXEC_MAX_OUTPUT_BYTES positive integer, default 10485760 (10 MiB), across
#                            captured stdout + stderr combined. File-prompt copies
#                            and non-codex stdin preparation each use the same cap.
#   CODEX_EXEC_TIMEOUT_BIN   absolute path of an already-probed timeout. Set it
#                            when the caller resolved one, so the --foreground
#                            capability probe does not run per invocation.
#   CODEX_EXEC_SANDBOX       (codex) sandbox value, e.g. read-only / workspace-write
#                            (default: read-only — the fail-closed default).
#   CODEX_EXEC_MODEL         (codex) model id for `-m` (empty/unset = codex default).
#   CODEX_EXEC_DIR           (codex) working dir for `-C` (empty/unset = no -C).
#   CODEX_EXEC_SKIP_GIT_CHECK (codex) 1 => pass --skip-git-repo-check (non-git workspaces).
#   CODEX_EXEC_EXTRA_ARGS    (codex) a bash array of extra passthrough flags appended
#                            verbatim (e.g. --json). Ignored by non-codex adapters (they
#                            are codex-specific flags).
#   CODEX_EXEC_WRAP          (codex) a bash array prefixed BEFORE timeout and codex,
#                            e.g. CODEX_EXEC_WRAP=(sandbox-exec -p
#                            "<profile>") — an EXTERNAL filesystem seal around the whole
#                            rep. When non-empty the assembled command is
#                              "${CODEX_EXEC_WRAP[@]}" "${to_cmd[@]}" <codex-bin> exec …
#                            and the sandbox mapping emits
#                            --dangerously-bypass-approvals-and-sandbox IN PLACE OF
#                            --sandbox <value>: the outer wrapper IS the sandbox, and
#                            codex's own seatbelt is bypassed because macOS seatbelt
#                            does not nest inside an outer sandbox-exec profile (codex
#                            dies with `sandbox_apply: Operation not permitted`); codex
#                            documents that flag for exactly "externally sandboxed" use.
#                            Empty/unset => byte-identical to the unwrapped behavior.
#                            Codex-only: agy/local-mlx/reference adapters ignore it. Do
#                            NOT wrap via CODEX_EXEC_BIN instead — the metadata tool
#                            flips coverage_eligible to false on a non-default bin.
#   CODEX_EXEC_OUT_FILE      write captured stdout here. If empty, output is captured
#                            to a temp file used for echo-detection, then streamed
#                            on success or an interrupted/limited run. Capture
#                            files must be regular files (or /dev/null).
#   CODEX_EXEC_STDERR_FILE   optional separate stderr sink. If empty, stderr is merged
#                            into CODEX_EXEC_OUT_FILE for backward compatibility.
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
# Returns one of the documented exit codes above. Output and interrupted partial
# capture are in CODEX_EXEC_OUT_FILE (if set) or on stdout. Only ordinary children
# in the owned process group are covered; escaped groups/sessions are excluded.
codex_exec_guarded() {
  local reviewer; reviewer="$(reviewer_normalize "${REVIEWER:-codex}")"

  # Resolve the adapter's binary + genuine-run marker. The caller selected the
  # adapter; this helper reports one invocation and has no admission authority.
  local bin marker
  bin="$(reviewer_adapter_bin "$reviewer")"
  marker="$(reviewer_adapter_marker "$reviewer")"
  # local-mlx has NO default binary. The reference wrapper it used to fall back to
  # (evals/membrane/membranes/local-mlx-membrane.sh) was deleted with the membrane
  # eval tree; a caller who selects this adapter must name the runtime in REVIEWER_BIN.
  # An unset REVIEWER_BIN is a PRECONDITION failure, never a fallback to whatever
  # happens to sit on PATH under the old wrapper name.
  if [ "$reviewer" = "local-mlx" ] && [ -z "${REVIEWER_BIN:-}" ]; then
    echo "codex-exec: MISSING DEPENDENCY: REVIEWER=local-mlx requires REVIEWER_BIN (no default wrapper ships)." >&2
    echo "  This is NOT a result. Set REVIEWER_BIN to the local reviewer executable, then re-run." >&2
    echo "  (exit $CODEX_EXEC_MISSING = precondition, not a REFUTE / not a genuine failure)" >&2
    return "$CODEX_EXEC_MISSING"
  fi

  # A missing reviewer binary is a
  # PRECONDITION failure with its OWN exit code — never a review result. Name it,
  # say it is installable, and return the distinct code so a caller can tell a
  # missing dep apart from a real refutation.
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "codex-exec: MISSING DEPENDENCY — '$bin' is not on PATH." >&2
    echo "  This is NOT a result. Install the '$reviewer' reviewer CLI and put it on PATH, then re-run." >&2
    echo "  (exit $CODEX_EXEC_MISSING = precondition, not a REFUTE / not a genuine failure)" >&2
    return "$CODEX_EXEC_MISSING"
  fi

  # Validate all caller bounds before touching prompt/output files or starting
  # the reviewer. Empty values are invalid; timeout has no implicit default.
  if [ ! -x /usr/bin/perl ]; then
    echo "codex-exec: CLEANUP-UNAVAILABLE — core Perl/POSIX support is required." >&2
    return "$CODEX_EXEC_MISSING"
  fi
  local limits budget capture_cap deadline limit_rc=0
  limits="$(_codex_exec_control limits "${CODEX_EXEC_TIMEOUT-}" "${CODEX_EXEC_MAX_OUTPUT_BYTES-10485760}" "${CODEX_EXEC_DEADLINE_EPOCH-}" "${CODEX_EXEC_DEADLINE_EPOCH+1}" "${CODEX_EXEC_TIMEOUT+1}")" || limit_rc=$?
  if [ "$limit_rc" -ne 0 ]; then
    [ "$limit_rc" -ne 124 ] || return "$CODEX_EXEC_STALL_TIMEOUT"
    echo "codex-exec: execution limits or required Perl/POSIX capability unavailable; reviewer not launched." >&2
    return "$CODEX_EXEC_MISSING"
  fi
  read -r budget capture_cap deadline <<<"$limits"
  local -a to_cmd=()
  local timeout_bin="" to_rc=0
  # The capability probe shares the invocation end, including preparation time.
  timeout_bin="$(CODEX_EXEC_DEADLINE_EPOCH="$deadline" codex_exec_timeout_bin)" || to_rc=$?
  if [ "$to_rc" -ne 0 ] || [ -z "$timeout_bin" ]; then
    limit_rc=0
    _codex_exec_control limits "$budget" "$capture_cap" "$deadline" 1 1 >/dev/null || limit_rc=$?
    [ "$limit_rc" -ne 124 ] || return "$CODEX_EXEC_STALL_TIMEOUT"
    echo "codex-exec: MISSING-TIMEOUT — a resolved executable supporting --foreground is required; reviewer not launched." >&2
    return "$CODEX_EXEC_MISSING"
  fi
  to_cmd=("$timeout_bin" --foreground "$budget")

  # (2) assemble the reviewer argv + resolve the prompt delivery + the echo-compare
  # target, per adapter. `delivery` is stdin_file (redirect a file on stdin) or plain
  # (no redirect); `run_echo_check` gates echo-detection off for stdin-pipe callers;
  # `echo_cmp_file` is the bytes an echo would reflect (what the model received INLINE).
  local -a argv=()
  local -a _cleanup=()
  _codex_exec_cleanup() {
    [ "${#_cleanup[@]}" -eq 0 ] || rm -f "${_cleanup[@]}"
  }
  local prompt_file="" delivery="plain" run_echo_check=1 echo_cmp_file=""
  # Packet-echo compare targets (agy/local-mlx — the VERDICT:-marker adapters, whose
  # packets legitimately CONTAIN the marker so marker-presence cannot veto echo checks;
  # see reviewer_packet_echoed). Empty for codex => the check is skipped (byte-compat).
  local packet_echo_file="" packet_nonce=""
  # External wrap prefix (codex adapter ONLY; see CODEX_EXEC_WRAP above). Same `+set`
  # guard idiom as CODEX_EXEC_EXTRA_ARGS so an unset array is safe under `set -u`.
  local -a wrap=()
  if [ "$reviewer" = "codex" ] && [ -n "${CODEX_EXEC_WRAP+set}" ] && [ "${#CODEX_EXEC_WRAP[@]}" -gt 0 ]; then
    wrap=("${CODEX_EXEC_WRAP[@]}")
  fi

  # File prompts and non-codex stdin need a local copy before argv/echo setup.
  # Capture that copy under the SAME deadline and byte cap, including FIFOs or
  # a producer that never closes stdin. Codex raw stdin stays with its reviewer.
  local selected_prompt_file="${CODEX_EXEC_PROMPT_FILE:-}"
  if [ -n "$selected_prompt_file" ] || { [ "$reviewer" != codex ] && [ -z "${CODEX_EXEC_PROMPT_ARG:-}" ]; }; then
    local input_file prepare_rc=0
    input_file="$(mktemp "${TMPDIR:-/tmp}/reviewer-input.XXXXXX")"; _cleanup+=("$input_file")
    _codex_exec_bounded "$budget" "$capture_cap" "$deadline" "$input_file" "" "$selected_prompt_file" cat || prepare_rc=$?
    if [ "$prepare_rc" -ne 0 ]; then
      echo "codex-exec: prompt preparation stopped; partial input preserved at $input_file" >&2
      return "$prepare_rc"
    fi
    selected_prompt_file="$input_file"
  fi

  case "$reviewer" in
    codex)
      # Order is stable so stub-`codex` tests that inspect positional args
      # (eval-agent-harness.bats records the -C dir) keep working. Arg-mode
      # prompts use `--` below so leading hyphens cannot be parsed as options.
      argv=(exec)
      [ "${CODEX_EXEC_SKIP_GIT_CHECK:-0}" = "1" ] && argv+=(--skip-git-repo-check)
      local sandbox="${CODEX_EXEC_SANDBOX:-read-only}"
      if [ "${#wrap[@]}" -gt 0 ]; then
        # sandbox mapping (codex, wrapped): the outer CODEX_EXEC_WRAP wrapper IS the
        # sandbox; codex's own is bypassed because seatbelt does not nest (codex's
        # documented flag for an externally sandboxed run). CODEX_EXEC_SANDBOX is
        # deliberately not emitted — it would re-arm the nested seatbelt.
        argv+=(--dangerously-bypass-approvals-and-sandbox)
      else
        argv+=(--sandbox "$sandbox")
      fi
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
      if [ -n "$selected_prompt_file" ]; then
        prompt_file="$selected_prompt_file"; delivery="stdin_file"; echo_cmp_file="$prompt_file"
      elif [ -n "${CODEX_EXEC_PROMPT_ARG:-}" ]; then
        argv+=(-- "$CODEX_EXEC_PROMPT_ARG")
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
      if [ -n "$selected_prompt_file" ]; then
        packet_file="$selected_prompt_file"
      else
        packet_file="$(mktemp "${TMPDIR:-/tmp}/reviewer-packet.XXXXXX")"; _cleanup+=("$packet_file")
        printf '%s' "$CODEX_EXEC_PROMPT_ARG" > "$packet_file"
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
      argv+=(--print-timeout "${budget}s")
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
      if [ -n "$selected_prompt_file" ]; then payload="$(cat "$selected_prompt_file")"
      else payload="$CODEX_EXEC_PROMPT_ARG"; fi
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
  local out_file="${CODEX_EXEC_OUT_FILE:-}" stderr_file="${CODEX_EXEC_STDERR_FILE:-}" cleanup_out=""
  if [ -z "$out_file" ]; then
    out_file="$(mktemp "${TMPDIR:-/tmp}/codex-exec-out.XXXXXX")"
    cleanup_out="$out_file"
  fi

  # The launch prefix, in this fixed order: the external CODEX_EXEC_WRAP prefix
  # (if any), then the timeout wrapper (if any), then the reviewer binary. The
  # WRAP is outermost so the sandbox is the outermost process: with timeout
  # outside it, the timeout binary itself was resolved from PATH and ran
  # UNSEALED, so a shadowed timeout could have dropped the wrapper entirely.
  # `--foreground` leaves the entire sealed launch in the supervisor-owned group.
  # `launch` always holds at least the binary, so its expansion is safe under
  # `set -u` on bash 3.2.
  local -a launch=()
  [ "${#wrap[@]}" -gt 0 ] && launch+=("${wrap[@]}")
  [ "${#to_cmd[@]}" -gt 0 ] && launch+=("${to_cmd[@]}")
  launch+=("$bin")

  local expect_output="${CODEX_EXEC_EXPECT_OUTPUT:-1}"
  local rc=0 input_path=""
  [ "$delivery" != stdin_file ] || input_path="$prompt_file"
  _codex_exec_bounded "$budget" "$capture_cap" "$deadline" "$out_file" "$stderr_file" "$input_path" "${launch[@]}" "${argv[@]}" || rc=$?

  if [ "$rc" -eq "$CODEX_EXEC_MISSING" ] || [ "$rc" -eq "$CODEX_EXEC_DESCENDANT_LEAK" ] || [ "$rc" -eq "$CODEX_EXEC_OUTPUT_LIMIT" ] || [ "$rc" -eq 129 ] || [ "$rc" -eq 130 ] || [ "$rc" -eq 143 ]; then
    [ -z "$cleanup_out" ] || { cat "$out_file"; rm -f "$cleanup_out"; }
    _codex_exec_cleanup
    return "$rc"
  fi

  # Classify the outcome into the documented exit codes.
  # 1) TIMEOUT: the wrapper kills with 124 (or 137 on some `timeout` builds when it
  #    escalates to SIGKILL). Map both to STALL-TIMEOUT so a killed run is never
  #    read as a clean pass — ALWAYS, regardless of CODEX_EXEC_EXPECT_OUTPUT (a
  #    kill is never a success). Only meaningful when a timeout was applied.
  if [ "${#to_cmd[@]}" -gt 0 ] && { [ "$rc" -eq 124 ] || [ "$rc" -eq 137 ]; }; then
    echo "codex-exec: STALL — run stopped within the ${budget}s allowance (rc=$rc); partial output preserved." >&2
    echo "  (exit $CODEX_EXEC_STALL_TIMEOUT = stall/timeout, NOT a review result)" >&2
    [ -z "$cleanup_out" ] || { cat "$out_file"; rm -f "$cleanup_out"; }
    _codex_exec_cleanup
    return "$CODEX_EXEC_STALL_TIMEOUT"
  fi

  # The STALL(empty)/ECHO reclassification is only meaningful for callers that
  # CONSUME output. A fire-and-score caller (CODEX_EXEC_EXPECT_OUTPUT=0) discards
  # output, so a clean exit-0 with empty output is a real SUCCESS there.
  if [ "$expect_output" = "1" ]; then
    # 2) Empty output is reported after the single invocation.
    if [ ! -s "$out_file" ]; then
      echo "codex-exec: STALL — reviewer produced no output." >&2
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

  # 4) Preserve other nonzero codes verbatim. Exit status is runtime evidence;
  # by itself it does not prove whether the reviewer or execution setup failed.
  if [ "$rc" -ne 0 ]; then
    echo "codex-exec: execution exited non-zero (rc=$rc); captured output preserved. This is runtime evidence, not a review verdict." >&2
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
# wants the string off every caller). The strings below are byte-
# identical to eval-membrane's historical defaults so behavior is preserved
# exactly.
#   $1 = which template: "producer" (frontier producer) or "membrane" (verifier).
# The returned string is a bash-c template expanded later as: bash -c "<tpl>" _ ...
# (producer: $1=workspace $2=prompt $3=timeout; membrane: $1=reviewer_prompt).
codex_exec_producer_template() {
  case "${1:-producer}" in
    producer)
      # shellcheck disable=SC2016 # Positional parameters expand later inside bash -c.
      printf 'timeout "$3" codex exec --skip-git-repo-check -C "$1" -s workspace-write "$2" >/dev/null 2>&1' ;;
    membrane)
      # shellcheck disable=SC2016 # Positional parameters expand later inside bash -c.
      printf 'codex exec --skip-git-repo-check "$1" 2>/dev/null' ;;
    *) return 2 ;;
  esac
}
