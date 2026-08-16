#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  DISC="$REPO_ROOT/evals/skill-probes/anti-ceremony-creation-gate-v2/discriminator.sh"
  TRANSCRIPT="$BATS_TEST_TMPDIR/transcript.txt"
}

write_transcript() {
  {
    printf 'OpenAI Codex fixture\n'
    printf 'user\n'
    printf 'A: CREATE\nA: DROP\nB: CREATE\nB: DROP\n'
    printf 'codex\n'
    printf '%s\n' "$1"
    printf 'tokens used\n1\n'
  } > "$TRANSCRIPT"
}

@test "anti-ceremony discriminator scores a complete response segment" {
  write_transcript $'A: DROP\nB: CREATE'

  run "$DISC" "$TRANSCRIPT"

  [ "$status" -eq 0 ]
  [[ "$output" == PRESENT:* ]]
}

@test "anti-ceremony discriminator cannot borrow a missing decision from the prompt echo" {
  write_transcript 'B: CREATE'

  run "$DISC" "$TRANSCRIPT"

  [ "$status" -eq 1 ]
  [[ "$output" == *"missing one or both"* ]]
}

@test "anti-ceremony discriminator degrades without a runtime response marker" {
  printf 'user\nA: DROP\nB: CREATE\n' > "$TRANSCRIPT"

  run "$DISC" "$TRANSCRIPT"

  [ "$status" -eq 2 ]
  [[ "$output" == *"no codex response segment"* ]]
}
