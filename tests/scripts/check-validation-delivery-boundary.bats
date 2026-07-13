#!/usr/bin/env bats

@test "validation and delivery remain separate authority boundaries" {
  run bash "$BATS_TEST_DIRNAME/../../scripts/check-validation-delivery-boundary.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"validation-delivery boundary: PASS"* ]]
}
