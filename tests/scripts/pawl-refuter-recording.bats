#!/usr/bin/env bats
# age-pawl-good-bar #4: the REFUTED/HOLD-path refuter list must carry each pane's ACTUAL emitted
# verdict. A timed-out pane has NO verdict — recording it as REFUTED (the old `${vc:-REFUTED}`)
# FABRICATED a refutation the pane never made, corrupting the membrane's own evidence (metrics.jsonl
# logs these as "timeout"). _refuted_refuters is the pure builder; these lock the honest behavior.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"   # source-guard returns before dispatch
}

@test "real votes recorded as-is; a timed-out pane is OMITTED (not fabricated as REFUTED)" {
  run _refuted_refuters CONFIRMED REFUTED "" e1 e2 e3
  [ "$status" -eq 0 ]
  [[ "$output" == *"claude:CONFIRMED:opus-pawl-pane-fresh:e1"* ]]
  [[ "$output" == *"gpt:REFUTED:codex-pawl-pane-gpt55:e2"* ]]
  # the timed-out agy pane must NOT appear at all — no fabricated gemini:REFUTED
  [[ "$output" != *"gemini:"* ]]
}

@test "the fabrication bug is gone: a lone timed-out pane is NOT recorded as a real REFUTED" {
  # only codex refuted; opus + agy timed out -> refuters = [codex:REFUTED] only, no fake opus/agy
  run _refuted_refuters "" REFUTED "" e1 e2 e3
  [[ "$output" == *"gpt:REFUTED:codex-pawl-pane-gpt55:e2"* ]]
  [[ "$output" != *"opus-pawl-pane-fresh"* ]]
  [[ "$output" != *"agy-pawl-pane-flash35"* ]]
}

@test "all-timeout fallback: schema's >=1 refuter satisfied with honest -timeout context" {
  run _refuted_refuters "" "" "" e1 e2 e3
  [ -n "$output" ]   # must not be empty (schema needs >=1)
  [[ "$output" == *"claude:REFUTED:opus-pawl-pane-timeout:e1"* ]]
  [[ "$output" == *"gpt:REFUTED:codex-pawl-pane-timeout:e2"* ]]
  [[ "$output" == *"gemini:REFUTED:agy-pawl-pane-timeout:e3"* ]]
  # the -timeout context distinguishes these from a real refutation
  [[ "$output" != *"opus-pawl-pane-fresh"* ]]
}

@test "disabled (n/a) panes are never recorded, even in the fallback" {
  # only opus enabled+refuted; codex/agy disabled (n/a)
  run _refuted_refuters REFUTED n/a n/a e1 e2 e3
  [[ "$output" == *"claude:REFUTED:opus-pawl-pane-fresh:e1"* ]]
  [[ "$output" != *"codex"* ]]
  [[ "$output" != *"agy"* ]]
}

@test "all-CONFIRMED (a degraded REFUTED e.g. insufficient count) records the real CONFIRMs" {
  # e.g. 1 CONFIRMED + 2 n/a -> disposition REFUTED:insufficient, but the one real vote is honest
  run _refuted_refuters CONFIRMED n/a n/a e1 e2 e3
  [[ "$output" == *"claude:CONFIRMED:opus-pawl-pane-fresh:e1"* ]]
  [[ "$output" != *"REFUTED"* ]]   # no fabricated refute
}
