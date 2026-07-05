# finalize.jq — the deterministic close-door verdict, ported faithfully from
# gascity internal/reviewquorum/{finalize,classify,types}.go.
#
# WHY a port and not a call to reviewquorum.Finalize: that function lives in
# gascity's `internal/` package, so it is un-importable by any external module
# (Go internal visibility), and gascity is a READ-ONLY fork we cannot add a
# production caller to. A pack's close gate is also run by the control
# dispatcher as an `exec` check that should stay toolchain-free (bash + jq only,
# same posture as the law0 doctor check) rather than shipping a per-arch Go
# binary. So we reimplement the rollup CONTRACT here and pin parity with a unit
# test (tests/finalize.bats). Divergences from stock gc, all deliberate:
#   - a per-round NONCE the lane must echo (anti-replay / anti-stale-verdict);
#   - a cross-family precondition (>=2 distinct provider families required);
#   - degradation-awareness surfaced as a distinct DEGRADED disposition
#     (transient lane loss must never be a false REFUTE and must not consume a
#     redo attempt) — stock Finalize returns verdict=blocked/failure_class
#     transient; we map that to DEGRADED for the gate's retry semantics.
#
# Input (via --slurpfile lanes / --arg):
#   $nonce             expected per-round nonce (string)
#   $round             round number (string)
#   $subject           subject/bead id
#   $base_ref          diff baseline ref
#   $expected_families comma-separated distinct provider families expected
#                      (e.g. "gpt,gemini") — encodes the cross-family posture
#   lanes              array of review-quorum.lane.v1 objects (may be empty)
#
# Precedence (finalize.go lines 114-130): hard > transient > findings > pass.
# Output: a decision object { disposition, failure_class, failure_reason,
#   findings_count, findings, present_families }. disposition is one of
#   CONFIRMED | REFUTED | DEGRADED. The wrapper maps that to exit code + the
#   pawl-verdict.v1 artifact + the gc.failure_class bead metadata.

def norm: (. // "") | ascii_downcase | gsub("^[[:space:]]+|[[:space:]]+$"; "");

# classify.go: transientFailureReasons
def transient_reasons:
  { "rate_limited": true, "provider_rate_limited": true,
    "temporary_unavailable": true, "provider_unavailable": true,
    "provider_timeout": true, "transport_interrupted": true,
    "transient_provider_err": true };

# classify.go: ClassifyFailure -> "none" | "transient" | "hard"
def classify(cls; rsn):
  (cls | norm) as $c | (rsn | norm) as $r
  | if $c == "none" and $r == "" then "none"
    elif $c == "transient" then "transient"
    elif $c == "hard" then "hard"
    elif $c == "" then (if transient_reasons[$r] then "transient" else "hard" end)
    else "hard" end;

def lane_id_of: (.lane_id // "unknown_lane" | norm);
def fail(id; reason): "lane=\(id) reason=\(reason)";

# Per-lane contract + verdict evaluation. Emits {hard:[..], transient:[..],
# findings:int} contributions for one lane. (Ports laneContractFailures + the
# verdict switch + read-only contract from finalize.go, plus our nonce check.)
def eval_lane($nonce):
  . as $l
  | (lane_id_of) as $id
  | (.verdict | norm) as $v
  | { hard: [], transient: [], findings: 0 }
  # --- our nonce anti-replay guard (fail-closed hard) -----------------------
  | (if (($l.agentops_nonce // "") | tostring) != $nonce
       then .hard += [fail($id; "nonce_mismatch")] else . end)
  # --- lane identity contract (finalize.go laneContractFailures) ------------
  | (if (($l.lane_id // "") | norm) == "" then .hard += [fail($id; "lane_id_missing")] else . end)
  | (if (($l.provider // "") | norm) == "" then .hard += [fail($id; "provider_missing")] else . end)
  | (if (($l.model // "") | norm) == "" then .hard += [fail($id; "model_missing")] else . end)
  | (if (($l.findings_count // 0)) != (($l.findings // []) | length)
       then .hard += [fail($id; "findings_count_mismatch")] else . end)
  # --- explicit lane failure_class (transient vs hard) ----------------------
  | (classify($l.failure_class; $l.failure_reason)) as $fc
  | (if $fc == "transient" then .transient += [fail($id; ($l.failure_reason | norm))]
     elif $fc == "hard" and (($l.failure_class // "" | norm) != "" or ($l.failure_reason // "" | norm) != "")
        then .hard += [fail($id; ($l.failure_reason | norm))]
     else . end)
  # --- read-only enforcement contract (self-attested; hard on violation) ----
  | (($l.read_only_enforcement // {}) as $ro
     | if ($ro.observed == false) then .hard += [fail($id; "read_only_enforcement_missing")]
       elif ($ro.enabled == false) then .hard += [fail($id; "read_only_enforcement_disabled")]
       elif ($ro.passed == false) then .hard += [fail($id; "read_only_mutation_detected")]
       elif ((($l.mutations_delta.changed // []) | length) > 0) then .hard += [fail($id; "read_only_mutation_detected")]
       else . end)
  # --- verdict rollup (finalize.go switch) ----------------------------------
  | (if $v == "pass" then .
     elif $v == "pass_with_findings" then .findings += (($l.findings // []) | length | if . == 0 then 1 else . end)
     elif $v == "fail" then .hard += [fail($id; "lane_failed")] | .findings += (($l.findings // []) | length)
     elif $v == "blocked"
        then (if $fc == "transient" then .    # already recorded by the failure_class block above
              else .hard += [fail($id; ($l.failure_reason | norm | if . == "" then "blocked" else . end))] end)
     else .hard += [fail($id; "unknown_verdict_value")] end);

# ---- top-level rollup -------------------------------------------------------
($expected_families | split(",") | map(norm) | map(select(. != "")) | unique) as $expected
| ($lanes | map(.provider | norm) | map(select(. != "")) | unique) as $present
| ($expected - $present) as $missing_fam
# aggregate per-lane contributions
| (reduce $lanes[] as $l ({hard: [], transient: [], findings: 0};
     ($l | eval_lane($nonce)) as $e
     | .hard += $e.hard | .transient += $e.transient | .findings += $e.findings)) as $agg
| ($agg.hard) as $hard
# a missing expected family = a lane that did not report = transient degradation
| ($agg.transient + ($missing_fam | map("lane=" + . + " reason=provider_unavailable"))) as $transient
| ($agg.findings) as $findings
# cross-family precondition: expected posture must itself carry >=2 families
| (if ($expected | length) < 2
     then { disp: "REFUTED", class: "hard", reason: "cross_family_precondition_unmet:expected<2_families" }
   elif ($lanes | length) == 0
     then { disp: "DEGRADED", class: "transient", reason: "awaiting_reviewers_no_lane_output" }
   elif ($hard | length) > 0
     then { disp: "REFUTED", class: "hard", reason: ($hard | join("; ")) }
   elif ($transient | length) > 0
     then { disp: "DEGRADED", class: "transient", reason: ($transient | join("; ")) }
   elif ($present | length) < 2
     then { disp: "DEGRADED", class: "transient", reason: "fewer_than_two_families_present" }
   else { disp: "CONFIRMED", class: "none", reason: "" } end) as $d
| { disposition: $d.disp,
    failure_class: $d.class,
    failure_reason: $d.reason,
    findings_count: $findings,
    present_families: $present,
    expected_families: $expected,
    subject: $subject,
    base_ref: $base_ref,
    round: ($round | tonumber? // 0),
    nonce: $nonce }
