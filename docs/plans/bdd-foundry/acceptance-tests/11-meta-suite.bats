#!/usr/bin/env bats
# §11 Meta: the acceptance suite itself — B73

load helpers

SUITE_DIR="$BATS_TEST_DIRNAME"

# Expand "B19–B25" / "B19-B25" ranges and bare "B7" ids to one id per line.
expand_b_ids() {
  grep -oE 'B[0-9]+([–-]B[0-9]+)?' \
    | while IFS= read -r tok; do
        if [[ "$tok" =~ ^B([0-9]+)[–-]B([0-9]+)$ ]]; then
          seq "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" | sed 's/^/B/'
        else
          echo "$tok"
        fi
      done
}

@test "B73: the acceptance suite is itself gated — one command, hermetic, total, deterministic" {
  repo="$(repo_under_test)"
  behaviors="$repo/docs/plans/bdd-foundry/behaviors.md"
  [ -f "$behaviors" ]

  # ── ONE command runs EVERY scenario: the pinned entry point exists ──
  entry="$repo/tests/landing/run-acceptance.sh"
  [ -f "$entry" ]
  [ -x "$entry" ]
  # the entry point runs this suite (every .bats file, or the suite dir)
  grep -q 'docs/plans/bdd-foundry/acceptance-tests' "$entry"
  # it enforces the no-skip/no-focus rule and double-run determinism
  grep -qE 'bats:focus|focus' "$entry"
  grep -qE 'skip' "$entry"
  grep -qE 'twice|run2|second run|deterministic' "$entry"

  # ── totality: every scenario B1..B73 exists exactly once as a @test ──
  suite_ids="$(grep -hoE '@test "B[0-9]+' "$SUITE_DIR"/*.bats | grep -oE 'B[0-9]+' | sort -u)"
  for n in $(seq 1 73); do
    grep -qx "B$n" <<<"$suite_ids"
  done
  dup="$(grep -hoE '@test "B[0-9]+:' "$SUITE_DIR"/*.bats | sort | uniq -d)"
  [ -z "$dup" ]

  # ── no skip/pending/focus marker anywhere in the suite ──
  ! grep -nE '^[[:space:]]*skip([[:space:]]|$)' "$SUITE_DIR"/*.bats
  ! grep -nE '^[[:space:]]*#[[:space:]]*bats:focus' "$SUITE_DIR"/*.bats

  # ── hermetic fixtures: temp dirs + the B25 sandbox-marked bare remote ──
  grep -q 'land-sandbox' "$SUITE_DIR/helpers.bash"
  grep -q 'mktemp -d' "$SUITE_DIR/helpers.bash"
  # concurrent-lane scenarios capture per-lane stdout/stderr/exit status and
  # reap background processes before reporting
  grep -q 'start_land' "$SUITE_DIR/helpers.bash"
  grep -q 'wait_land' "$SUITE_DIR/helpers.bash"
  grep -q 'sandbox_teardown' "$SUITE_DIR/helpers.bash"

  # ── coverage map check: every scenario tagged with ≥1 risk class ──
  map="$(sed -n '/^## Coverage map/,/^## Gap disposition/p' "$behaviors")"
  [ -n "$map" ]
  mapped="$(expand_b_ids <<<"$map" | sort -u)"
  unmapped=""
  for n in $(seq 1 73); do
    grep -qx "B$n" <<<"$mapped" || unmapped="$unmapped B$n"
  done
  # the map check fails on any unmapped scenario
  [ -z "$unmapped" ] || {
    echo "unmapped scenarios in the behaviors coverage map:$unmapped" >&2
    false
  }
}
