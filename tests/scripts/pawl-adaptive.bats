#!/usr/bin/env bats
# Capability-adaptive pawl service (age-4o33): the PURE helpers that turn "what's installed"
# into "which panes to spawn, at which index, at which membrane tier, and how to decide a
# verdict over that subset". All are pure (no tmux/atm), so every branch is locked here; the
# live spawn/route path is dogfooded separately. The membrane stays sound for every account
# combo: a host with only Claude gets a real fresh-context refuter (not a hard failure), and a
# >=2-family host gets the cross-family gate — but a single-family verdict is honestly stamped
# tier=fresh so a high-irreversibility door can still demand multi-model.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"
}

# --- probe_families: installed CLIs -> enabled families, canonical order (cc cod agy) ---

@test "probe_families: all three CLIs installed -> cc cod agy" {
  _cli_present() { return 0; }   # everything present
  run probe_families
  [ "$status" -eq 0 ]
  [ "$(printf '%s ' $output)" = "cc cod agy " ]
}

@test "probe_families: only claude installed -> cc (single family)" {
  _cli_present() { case "$1" in claude) return 0;; *) return 1;; esac; }
  run probe_families
  [ "$status" -eq 0 ]
  [ "$(printf '%s ' $output)" = "cc " ]
}

@test "probe_families: claude+codex (no agy) -> cc cod" {
  _cli_present() { case "$1" in claude|codex) return 0;; *) return 1;; esac; }
  run probe_families
  [ "$status" -eq 0 ]
  [ "$(printf '%s ' $output)" = "cc cod " ]
}

@test "probe_families: claude+agy (no codex) -> cc agy (canonical order preserved)" {
  _cli_present() { case "$1" in claude|agy) return 0;; *) return 1;; esac; }
  run probe_families
  [ "$status" -eq 0 ]
  [ "$(printf '%s ' $output)" = "cc agy " ]
}

@test "probe_families: nothing installed -> empty" {
  _cli_present() { return 1; }
  run probe_families
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# --- parse_pin: --dual/--tri/--models aliases -> canonical family list ---

@test "parse_pin: dual -> cc cod" {
  run parse_pin dual
  [ "$(printf '%s ' $output)" = "cc cod " ]
}

@test "parse_pin: tri -> cc cod agy" {
  run parse_pin tri
  [ "$(printf '%s ' $output)" = "cc cod agy " ]
}

@test "parse_pin: friendly aliases (opus,gemini) normalize + canonical-order" {
  run parse_pin "gemini,opus"
  [ "$(printf '%s ' $output)" = "cc agy " ]
}

@test "parse_pin: dedups repeats" {
  run parse_pin "cc,claude,opus"
  [ "$(printf '%s ' $output)" = "cc " ]
}

@test "parse_pin: empty -> empty (caller falls back to probe)" {
  run parse_pin ""
  [ -z "$output" ]
}

@test "parse_pin: unknown model -> error exit 2" {
  run parse_pin "cc,llama"
  [ "$status" -eq 2 ]
}

# --- pane_index: 1-based position within the ORDERED enabled set; empty if absent ---

@test "pane_index: cc/cod/agy in a full set -> 1/2/3" {
  [ "$(pane_index cc cc cod agy)" = "1" ]
  [ "$(pane_index cod cc cod agy)" = "2" ]
  [ "$(pane_index agy cc cod agy)" = "3" ]
}

@test "pane_index: agy in a {cc,agy} set shifts up to pane 2 (no codex)" {
  [ "$(pane_index agy cc agy)" = "2" ]
  [ "$(pane_index cc cc agy)" = "1" ]
}

@test "pane_index: a disabled family -> empty (skipped everywhere)" {
  [ -z "$(pane_index cod cc agy)" ]
}

# --- tier_of / min_confirm_for_tier: the strength ladder ---

@test "tier_of: >=2 families -> multi" {
  [ "$(tier_of cc cod)" = "multi" ]
  [ "$(tier_of cc cod agy)" = "multi" ]
}

@test "tier_of: exactly 1 family -> fresh" {
  [ "$(tier_of cc)" = "fresh" ]
}

@test "tier_of: 0 families -> empty (cannot run)" {
  [ -z "$(tier_of)" ]
}

@test "min_confirm_for_tier: multi needs 2, fresh needs 1" {
  [ "$(min_confirm_for_tier multi)" = "2" ]
  [ "$(min_confirm_for_tier fresh)" = "1" ]
}

# --- doctor/smoke model preflight helpers ---

@test "_text_has_model: claude-opus-4-8 accepts the Opus 4.8 TUI display" {
  run _text_has_model "Claude Code v2.1.198 · Opus 4.8" cc claude-opus-4-8
  [ "$status" -eq 0 ]
}

@test "_text_has_model: claude-opus-4-8 rejects a different Claude model display" {
  run _text_has_model "Claude Code v2.1.198 · Sonnet 4.5" cc claude-opus-4-8
  [ "$status" -ne 0 ]
}

@test "_text_has_model: codex model check requires the expected model token" {
  run _text_has_model "Codex 0.142.5 · gpt-5.5 xhigh" cod gpt-5.5
  [ "$status" -eq 0 ]
  run _text_has_model "Codex 0.142.5 · gpt-5.5 xhigh" cod gpt-5.3
  [ "$status" -ne 0 ]
}

# --- pawl_decide <min> <verdicts...>: generalized over N enabled panes ---

@test "decide multi: all 3 CONFIRMED -> full:3" {
  run pawl_decide 2 CONFIRMED CONFIRMED CONFIRMED
  [ "$output" = "CONFIRMED:full:3" ]
}

@test "decide multi: 2 CONFIRMED + 1 timeout -> degraded:2 (still >=2 cross-family)" {
  run pawl_decide 2 CONFIRMED CONFIRMED ""
  [ "$output" = "CONFIRMED:degraded:2" ]
}

@test "decide multi: any single REFUTE blocks (recall-biased)" {
  run pawl_decide 2 CONFIRMED CONFIRMED REFUTED
  case "$output" in REFUTED:refuted:*) : ;; *) false ;; esac
}

@test "decide multi: only 1 CONFIRMED of a 2-family session -> insufficient (need >=2)" {
  run pawl_decide 2 CONFIRMED ""
  [ "$output" = "REFUTED:insufficient:1" ]
}

@test "decide fresh: a lone CONFIRMED in a single-family session -> full:1 (Tier-2 pass)" {
  run pawl_decide 1 CONFIRMED
  [ "$output" = "CONFIRMED:full:1" ]
}

@test "decide fresh: a lone REFUTE blocks" {
  run pawl_decide 1 REFUTED
  case "$output" in REFUTED:refuted:*) : ;; *) false ;; esac
}

@test "decide fresh: a timeout (no verdict) -> insufficient:0 (fail-closed, never fail-open)" {
  run pawl_decide 1 ""
  [ "$output" = "REFUTED:insufficient:0" ]
}

# --- back-compat: the old 3-arg all-CONFIRM wrapper still behaves identically ---

@test "pawl_decide_agreement (back-compat) == pawl_decide 2" {
  [ "$(pawl_decide_agreement CONFIRMED CONFIRMED CONFIRMED)" = "CONFIRMED:full:3" ]
  [ "$(pawl_decide_agreement CONFIRMED CONFIRMED '')" = "CONFIRMED:degraded:2" ]
  [ "$(pawl_decide_agreement CONFIRMED '' '')" = "REFUTED:insufficient:1" ]
  [ "$(pawl_decide_agreement '' '' '')" = "REFUTED:insufficient:0" ]
}

# --- resolve_default_families: DEFAULT route excludes strict-benched families (ebec.7) ---
# A benched family (A7: agy) stalls at ~3.5min/land contributing zero verdicts; the default
# probe drops it. NOT a rigor change: explicit pins still admit it, quorum math untouched.

@test "resolve_default_families: all three installed -> benched agy excluded from default" {
  _cli_present() { return 0; }
  run resolve_default_families
  [ "$status" -eq 0 ]
  [ "$(printf '%s ' $output)" = "cc cod " ]
}

@test "resolve_default_families: bench override empty -> agy included again" {
  _cli_present() { return 0; }
  PAWL_BENCHED_FAMILIES=""
  run resolve_default_families
  [ "$(printf '%s ' $output)" = "cc cod agy " ]
}

@test "resolve_default_families: only benched family installed -> raw-probe fallback, never zero" {
  _cli_present() { case "$1" in agy) return 0;; *) return 1;; esac; }
  run resolve_default_families
  [ "$(printf '%s ' $output)" = "agy " ]
}

@test "resolve_default_families: benched family still admitted via explicit pin (parse_pin unaffected)" {
  run parse_pin agy
  [ "$(printf '%s ' $output)" = "agy " ]
}
