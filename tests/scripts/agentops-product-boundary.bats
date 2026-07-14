#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  ACTIVE_AUTHORITY=(
    AGENTS.md
    AGENTS-CI.md
    PROGRAM.md
    PRODUCT.md
    GOALS.md
    docs/CI-CD.md
    docs/3.0.md
    docs/MIGRATION.md
    docs/UPGRADING.md
    docs/agent-workflow-reference.md
    docs/adr/ADR-0012-focus-surface-on-membrane-bookkeeper-archive-satellites.md
    docs/architecture/build-tags.md
    docs/architecture/go-cli-architecture-guide.md
    docs/architecture/operating-loop.md
    docs/contracts/local-pre-push-gate-retirement.md
    docs/contracts/pawls.md
    docs/doctrine/operating-discipline.md
    docs/documentation-index.md
    docs/newcomer-guide.md
    docs/software-factory.md
    cli/README.md
  )
}

scan_obsolete_authority() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    echo "invalid harness: missing authority file: $file" >&2
    return 2
  fi

  local active_text matches
  active_text="$(grep -Ev \
    '^[[:space:]]*>?[[:space:]]*HISTORICAL EVIDENCE \(NON-AUTHORITATIVE\):' \
    "$file" || true)"
  matches="$(printf '%s\n' "$active_text" | grep -Eni \
    'ao[[:space:]]+land|ao[[:space:]]+pawl|/pawl-review|run[^[:cntrl:]]*pawl|pawl(-review|-land)?[[:space:]]+(gate|route|review|verdict|authority)|push-as-CI|never[[:space:]]+delete|strangl[a-z]*|archiv(e|ed|ing)[^[:cntrl:]]{0,240}(rather than|instead of|not)[^[:cntrl:]]{0,80}delet|((release[[:space:]]+)?tag[[:space:]]+push|CI)[^[:cntrl:]]{0,160}(run|produce|emit|write|record)[^[:cntrl:]]{0,120}Validate[[:space:]]+verdict|Validation[[:space:]]+Gates[^[:cntrl:]]*(premortem|postmortem|council)|Knowledge[[:space:]]+Flywheel[^[:cntrl:]]*postmortem|semantic[[:space:]]+pre-push|local[[:space:]]+gate[^[:cntrl:]]*release[[:space:]]+authority' \
    || true)"
  if [[ -n "$matches" ]]; then
    printf 'obsolete authority in %s:\n%s\n' "$file" "$matches" >&2
    return 1
  fi
}

require_text() {
  local relative="$1" expected="$2"
  local file="$REPO_ROOT/$relative"
  if [[ ! -f "$file" ]]; then
    echo "invalid harness: missing authority file: $relative" >&2
    return 2
  fi
  grep -Fq -- "$expected" "$file" || {
    echo "$relative is missing product-boundary contract: $expected" >&2
    return 1
  }
}

@test "active authority teaches the lean four-umbrella product boundary" {
  local relative
  for relative in "${ACTIVE_AUTHORITY[@]}"; do
    run scan_obsolete_authority "$REPO_ROOT/$relative"
    [ "$status" -eq 0 ] || {
      echo "$output" >&2
      return "$status"
    }
  done

  require_text AGENTS.md \
    "Discovery, Crank, Validate, and Learn are the four lifecycle umbrellas."
  require_text docs/architecture/operating-loop.md \
    "A verdict is immutable evidence from fresh context."
  require_text docs/CI-CD.md \
    "Repositories own delivery policy for local and cloud agents."
  require_text docs/adr/ADR-0012-focus-surface-on-membrane-bookkeeper-archive-satellites.md \
    "Delete legacy code directly in the same owning leaf that installs its replacement or removes its last consumer."
  require_text docs/architecture/go-cli-architecture-guide.md \
    "CLI implementation deletion is owned by K5, K7, K9, and the exact CLI leaves."
  require_text docs/architecture/go-cli-architecture-guide.md \
    "Build-profile deletion is owned by F4."
  require_text docs/architecture/go-cli-architecture-guide.md \
    "Generated command-reference deletion or regeneration is owned by D2."
  require_text docs/documentation-index.md \
    "| **Discovery** | Shapes accepted behavior and consumes Premortem as its plan stress-test |"
  require_text docs/documentation-index.md \
    "| **Validate** | Binding judgment umbrella: one author-distinct fresh context judges a frozen candidate once |"
  require_text docs/documentation-index.md \
    "| **Learn** | Records one minimal consequence before optional Postmortem |"
  require_text docs/documentation-index.md \
    "Council is optional validator composition inside Premortem or Validate; it is"
}

@test "negative fixture rejects a model verdict as a Git delivery gate" {
  local fixture="$BATS_TEST_TMPDIR/model-gate.md"
  printf '%s\n' 'Run the Pawl before Git delivery.' >"$fixture"

  run scan_obsolete_authority "$fixture"

  [ "$status" -eq 1 ]
  [[ "$output" == *"Pawl"* ]]
}

@test "negative fixture rejects repository delivery through ao land" {
  local fixture="$BATS_TEST_TMPDIR/cli-delivery.md"
  printf '%s\n' 'The terminal command is ao land AGE-123.' >"$fixture"

  run scan_obsolete_authority "$fixture"

  [ "$status" -eq 1 ]
  [[ "$output" == *"ao land"* ]]
}

@test "negative fixture rejects compatibility-first retention" {
  local fixture="$BATS_TEST_TMPDIR/retention.md"
  printf '%s\n' 'Use a compatibility-first strangler and never delete the old owner.' >"$fixture"

  run scan_obsolete_authority "$fixture"

  [ "$status" -eq 1 ]
  [[ "$output" == *"compatibility-first strangler"* ]]
  [[ "$output" == *"never delete"* ]]
}

@test "negative fixture rejects semantic push-hook authority" {
  local fixture="$BATS_TEST_TMPDIR/push-hook.md"
  printf '%s\n' 'The local gate is the release authority; require semantic pre-push review.' >"$fixture"

  run scan_obsolete_authority "$fixture"

  [ "$status" -eq 1 ]
  [[ "$output" == *"local gate is the release authority"* ]]
  [[ "$output" == *"semantic pre-push"* ]]
}

@test "negative fixture rejects strangled and strangling migration variants" {
  local fixture="$BATS_TEST_TMPDIR/incremental-cut.md"
  printf '%s\n' \
    'Legacy commands are being strangled while the new root lands.' \
    'The migration is strangling one family at a time.' >"$fixture"

  run scan_obsolete_authority "$fixture"

  [ "$status" -eq 1 ]
  [[ "$output" == *"strangled"* ]]
  [[ "$output" == *"strangling"* ]]
}

@test "negative fixture rejects archive-rather-than-delete retention" {
  local fixture="$BATS_TEST_TMPDIR/archive-retention.md"
  printf '%s\n' \
    'Archive the satellites behind build tags rather than delete them.' >"$fixture"

  run scan_obsolete_authority "$fixture"

  [ "$status" -eq 1 ]
  [[ "$output" == *"Archive the satellites"* ]]
}

@test "negative fixture rejects deterministic CI claiming a Validate verdict" {
  local fixture="$BATS_TEST_TMPDIR/ci-verdict.md"
  printf '%s\n' \
    'Release tag pushes run a full Validate verdict for the tagged SHA.' >"$fixture"

  run scan_obsolete_authority "$fixture"

  [ "$status" -eq 1 ]
  [[ "$output" == *"Validate verdict"* ]]
}

@test "negative fixture rejects obsolete parallel umbrella routing" {
  local fixture="$BATS_TEST_TMPDIR/umbrella-routing.md"
  printf '%s\n' \
    '| Validation Gates | /council, /premortem, /postmortem |' \
    '| Knowledge Flywheel | /postmortem --quick, /curate |' >"$fixture"

  run scan_obsolete_authority "$fixture"

  [ "$status" -eq 1 ]
  [[ "$output" == *"Validation Gates"* ]]
  [[ "$output" == *"Knowledge Flywheel"* ]]
}

@test "bounded historical evidence remains legal" {
  local fixture="$BATS_TEST_TMPDIR/historical-evidence.md"
  printf '%s\n' \
    'HISTORICAL EVIDENCE (NON-AUTHORITATIVE): CI produced a Validate verdict while commands were being strangled and satellites were archived rather than deleted.' \
    >"$fixture"

  run scan_obsolete_authority "$fixture"

  [ "$status" -eq 0 ]
}
