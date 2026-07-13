#!/usr/bin/env bash
set -euo pipefail

ROOT="${AO_CLI_COMPAT_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
CLI="$ROOT/cli"
BASELINE_ROOT="${AO_CLI_COMPAT_BASELINE_DIR:-$CLI/testdata/compatibility-baseline}"
V2_DIR="$BASELINE_ROOT/v2"
BASELINE="$BASELINE_ROOT"
NORMALIZER="$BASELINE/normalize.jq"
PROFILE_DIR="$BASELINE/profiles"
FAMILY_DIR="$BASELINE/families"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

mode=check
family=""
profiles_csv="default,flywheel,legacy,combined"
verify_frozen=0
all_migrated=0
execution_base=""
raw_file=""
profile_file=""
oracle_version="v1"
equivalent_main_sha=""
source_decision_c59=""
source_decision_main=""
v2_stage=""
v2_lock="$BASELINE_ROOT/.v2.lock"

usage() {
  cat <<'EOF'
usage: check-go-cli-compatibility.sh [options]

  --capture --execution-base SHA  Capture the immutable four-profile baseline.
  --oracle-version V              Select v1, v2, or current (default: v1).
  --equivalent-main-sha SHA       Bind a behaviorally equivalent main SHA during v2 capture.
  --verify-source-decision A B    Compare clean-archive four-profile behavior and emit JSON.
  --family NAME                   Validate one frozen family fixture.
  --all-migrated                  Validate every family with lineage.json.
  --verify-frozen                 Enforce family fixture digest and git lineage.
  --profiles CSV                  Profiles to compile (default: all four).
  --validate-family-fixture NAME  Validate/execute a fixture without compiling ao.
  --verify-baseline-integrity     Verify baseline files and hashes without compiling ao.
  --validate-raw FILE             Reject unclassified volatile capability fields.
  --verify-profile-file P FILE    Compare one supplied capability document to baseline.
EOF
}

while (($#)); do
  case "$1" in
    --capture) mode=capture; shift ;;
    --oracle-version) oracle_version="${2:?missing oracle version}"; shift 2 ;;
    --equivalent-main-sha) equivalent_main_sha="${2:?missing equivalent main SHA}"; shift 2 ;;
    --verify-source-decision) mode=source_decision; source_decision_c59="${2:?missing source SHA}"; source_decision_main="${3:?missing main SHA}"; shift 3 ;;
    --execution-base) execution_base="${2:?missing SHA}"; shift 2 ;;
    --family) family="${2:?missing family}"; shift 2 ;;
    --all-migrated) all_migrated=1; shift ;;
    --verify-frozen) verify_frozen=1; shift ;;
    --profiles) profiles_csv="${2:?missing profiles}"; shift 2 ;;
    --validate-family-fixture) mode=family_only; family="${2:?missing family}"; shift 2 ;;
    --verify-baseline-integrity) mode=integrity; shift ;;
    --validate-raw) mode=raw; raw_file="${2:?missing file}"; shift 2 ;;
    --verify-profile-file) mode=profile_file; family="${2:?missing profile}"; profile_file="${3:?missing file}"; shift 3 ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'unknown argument: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
done

case "$oracle_version" in
  v1|v2|current) ;;
  *) printf 'unknown oracle version: %s (want v1, v2, or current)\n' "$oracle_version" >&2; exit 2 ;;
esac

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

is_commit() {
  [[ "$1" =~ ^[0-9a-f]{40}$ ]] && git -C "$ROOT" cat-file -e "$1^{commit}" 2>/dev/null
}

maybe_fail() {
  local point="$1"
  if [[ "${AO_CLI_COMPAT_TEST_FAIL:-}" == "$point" ]]; then
    printf 'injected %s failure\n' "$point" >&2
    return 1
  fi
}

cleanup_v2_capture() {
  if [[ -n "$v2_stage" && -e "$v2_stage" ]]; then
    rm -rf "$v2_stage"
  fi
  if [[ -d "$v2_lock" ]]; then
    rmdir "$v2_lock" 2>/dev/null || true
  fi
}

select_oracle() {
  case "$oracle_version" in
    v1)
      BASELINE="$BASELINE_ROOT"
      ;;
    v2)
      [[ -d "$V2_DIR" && ! -L "$V2_DIR" ]] || {
        printf 'compatibility oracle v2 is absent or not a real directory: %s\n' "$V2_DIR" >&2
        return 1
      }
      BASELINE="$V2_DIR"
      ;;
    current)
      if [[ ! -e "$V2_DIR" && ! -L "$V2_DIR" ]]; then
        BASELINE="$BASELINE_ROOT"
      elif [[ -d "$V2_DIR" && ! -L "$V2_DIR" ]]; then
        BASELINE="$V2_DIR"
      else
        printf 'compatibility oracle v2 is partial or corrupt: %s\n' "$V2_DIR" >&2
        return 1
      fi
      ;;
  esac
  NORMALIZER="$BASELINE_ROOT/normalize.jq"
  PROFILE_DIR="$BASELINE/profiles"
  FAMILY_DIR="$BASELINE_ROOT/families"
}

verify_git_freeze() {
  local capture_sha="" intro rel repo_rel snapshot
  local -a frozen_paths=(
    metadata.json
    normalize.jq
    family-case.schema.json
    ownership.schema.json
    lineage.schema.json
    profiles/default.json
    profiles/flywheel.json
    profiles/legacy.json
    profiles/combined.json
  )

  git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
    printf 'cannot verify frozen baseline outside a git worktree\n' >&2
    return 1
  }

  for rel in "${frozen_paths[@]}"; do
    repo_rel="cli/testdata/compatibility-baseline/$rel"
    test -f "$BASELINE/$rel" || {
      printf 'missing frozen baseline artifact: %s\n' "$rel" >&2
      return 1
    }
    intro="$(git -C "$ROOT" log --diff-filter=A --format=%H HEAD -- "$repo_rel" | tail -n 1)"
    test -n "$intro" || {
      printf 'frozen baseline artifact is not committed: %s\n' "$rel" >&2
      return 1
    }
    if [[ -z "$capture_sha" ]]; then
      capture_sha="$intro"
      git -C "$ROOT" merge-base --is-ancestor "$capture_sha" HEAD || {
        printf 'baseline capture commit is not an ancestor of HEAD: %s\n' "$capture_sha" >&2
        return 1
      }
    elif [[ "$intro" != "$capture_sha" ]]; then
      printf 'baseline artifacts do not share one capture commit: %s\n' "$rel" >&2
      return 1
    fi
    snapshot="$TMP/frozen-${rel//\//_}"
    git -C "$ROOT" show "$capture_sha:$repo_rel" >"$snapshot" || {
      printf 'baseline artifact missing from capture tree: %s\n' "$rel" >&2
      return 1
    }
    cmp -s "$snapshot" "$BASELINE/$rel" || {
      printf 'frozen baseline drift from capture commit %s: %s\n' "$capture_sha" "$rel" >&2
      return 1
    }
  done
}

verify_v2_git_freeze() {
  local intro="" rel repo_rel snapshot
  local -a paths=(metadata.json profiles/default.json profiles/flywheel.json profiles/legacy.json profiles/combined.json)
  [[ "$BASELINE_ROOT" == "$CLI/testdata/compatibility-baseline" ]] || return 0
  for rel in "${paths[@]}"; do
    repo_rel="cli/testdata/compatibility-baseline/v2/$rel"
    current_intro="$(git -C "$ROOT" log --diff-filter=A --format=%H HEAD -- "$repo_rel" | tail -n 1)"
    test -n "$current_intro" || { printf 'v2 artifact is not committed: %s\n' "$rel" >&2; return 1; }
    if [[ -z "$intro" ]]; then
      intro="$current_intro"
      git -C "$ROOT" merge-base --is-ancestor "$intro" HEAD || return 1
    elif [[ "$intro" != "$current_intro" ]]; then
      printf 'v2 artifacts do not share one capture commit: %s\n' "$rel" >&2
      return 1
    fi
    snapshot="$TMP/v2-frozen-${rel//\//_}"
    git -C "$ROOT" show "$intro:$repo_rel" >"$snapshot"
    cmp -s "$snapshot" "$V2_DIR/$rel" || {
      printf 'v2 baseline drift from capture commit %s: %s\n' "$intro" "$rel" >&2
      return 1
    }
  done
}

verify_tracked_family_lineages() {
  local lineage family rel intro snapshot
  [[ "$BASELINE_ROOT" == "$CLI/testdata/compatibility-baseline" ]] || return 0
  while IFS= read -r lineage; do
    family="$(basename "$(dirname "$lineage")")"
    rel="cli/testdata/compatibility-baseline/families/$family/lineage.json"
    intro="$(git -C "$ROOT" log --diff-filter=A --format=%H HEAD -- "$rel" | tail -n 1)"
    test -n "$intro" || { printf 'family lineage is not committed: %s\n' "$family" >&2; return 1; }
    snapshot="$TMP/lineage-$family.json"
    git -C "$ROOT" show "$intro:$rel" >"$snapshot"
    cmp -s "$snapshot" "$lineage" || { printf 'frozen family lineage mutated: %s\n' "$family" >&2; return 1; }
  done < <(find "$BASELINE_ROOT/families" -mindepth 2 -maxdepth 2 -name lineage.json -type f 2>/dev/null | sort)
}

verify_exact_v2_deltas() {
  local profile old new
  local -a expected=(environment_projection pawl_review_hold_5 provenance_reconcile verify_hold_5)
  test "$(jq -r '.intentional_deltas[]' "$V2_DIR/metadata.json" | sort | paste -sd ' ' -)" = "$(printf '%s\n' "${expected[@]}" | sort | paste -sd ' ' -)" || {
    printf 'v2 intentional delta allowlist is not exact\n' >&2
    return 1
  }
  ! jq -e '.intentional_deltas[] | select(test("plan-pawl"; "i"))' "$V2_DIR/metadata.json" >/dev/null || {
    printf 'plan-pawl exit 5 predates v1 and must not be a v2 delta\n' >&2
    return 1
  }
  for profile in default flywheel legacy combined; do
    old="$BASELINE_ROOT/profiles/$profile.json"
    new="$V2_DIR/profiles/$profile.json"
    jq -S --slurpfile old "$old" '
      ($old[0]) as $o
      | .env_vars = $o.env_vars
      | .command_exit_codes = $o.command_exit_codes
      | .commands = [.commands[]
          | select(.path != "ao provenance reconcile")
          | . as $n
          | ($o.commands[] | select(.id == $n.id)) as $prior
          | if ($n.id == "ao.pawl.review" or $n.id == "ao.verify")
            then .exit_codes = $prior.exit_codes
            else . end]
    ' "$new" >"$TMP/v2-$profile-sanitized.json"
    cmp -s "$old" "$TMP/v2-$profile-sanitized.json" || {
      printf 'unclassified v1 -> v2 compatibility delta in profile %s\n' "$profile" >&2
      return 1
    }
    jq -e '
      ([.commands[] | select(.path == "ao provenance reconcile")] | length) == 1
      and .command_exit_codes["pawl review"]["5"] != null
      and .command_exit_codes.verify["5"] != null
    ' "$new" >/dev/null || { printf 'required v2 delta missing in profile %s\n' "$profile" >&2; return 1; }
  done
}

verify_v2_integrity() {
  local metadata="$V2_DIR/metadata.json" profile expected actual count sha
  test -f "$metadata" && [[ ! -L "$metadata" ]] || { printf 'missing v2 metadata.json\n' >&2; return 1; }
  jq -e '
    .schema_version == 2
    and (.behavioral_source_sha | test("^[0-9a-f]{40}$"))
    and (.capture_sha | test("^[0-9a-f]{40}$"))
    and ((.equivalent_main_sha == null) or (.equivalent_main_sha | test("^[0-9a-f]{40}$")))
    and (.profiles | keys | sort) == ["combined","default","flywheel","legacy"]
    and (.intentional_deltas | type == "array")
  ' "$metadata" >/dev/null || { printf 'invalid schema-2 v2 metadata\n' >&2; return 1; }
  for sha in "$(jq -r '.behavioral_source_sha' "$metadata")" "$(jq -r '.capture_sha' "$metadata")"; do
    is_commit "$sha" || { printf 'v2 metadata references unreachable commit: %s\n' "$sha" >&2; return 1; }
  done
  sha="$(jq -r '.equivalent_main_sha // empty' "$metadata")"
  [[ -z "$sha" ]] || is_commit "$sha" || { printf 'v2 metadata references unreachable equivalent main: %s\n' "$sha" >&2; return 1; }
  test -d "$V2_DIR/profiles" && [[ ! -L "$V2_DIR/profiles" ]] || return 1
  count="$(find "$V2_DIR/profiles" -mindepth 1 -maxdepth 1 -type f -name '*.json' | wc -l | tr -d ' ')"
  test "$count" -eq 4 || { printf 'v2 requires exactly four profile files\n' >&2; return 1; }
  for profile in default flywheel legacy combined; do
    test -f "$V2_DIR/profiles/$profile.json" || { printf 'missing v2 profile: %s\n' "$profile" >&2; return 1; }
    jq -e . "$V2_DIR/profiles/$profile.json" >/dev/null || { printf 'invalid v2 profile JSON: %s\n' "$profile" >&2; return 1; }
    expected="$(jq -r --arg p "$profile" '.profiles[$p].sha256' "$metadata")"
    actual="$(sha256 "$V2_DIR/profiles/$profile.json")"
    test "$expected" = "$actual" || { printf 'v2 profile hash mismatch: %s\n' "$profile" >&2; return 1; }
  done
  verify_exact_v2_deltas
  verify_tracked_family_lineages
  [[ "$mode" == capture ]] || verify_v2_git_freeze
}

profile_tags() {
  case "$1" in
    default) printf '%s' "" ;;
    flywheel) printf '%s' flywheel ;;
    legacy) printf '%s' legacy ;;
    combined) printf '%s' 'flywheel legacy' ;;
    *) printf 'unknown profile: %s\n' "$1" >&2; return 2 ;;
  esac
}

validate_raw() {
  local file="$1"
  jq -e . "$file" >/dev/null
  if jq -e '
      [paths(scalars) as $p
       | ($p[-1] | tostring) as $key
       | select($key | test("(^|_)(timestamp|created_at|updated_at|generated_at|generated_on)$"; "i"))]
      | length > 0
    ' "$file" >/dev/null; then
    printf 'unclassified volatile time field in %s\n' "$file" >&2
    return 1
  fi
  if jq -e '[.. | strings | select(test("^/(Users|home|tmp|private/tmp)/"))] | length > 0' "$file" >/dev/null; then
    printf 'unclassified absolute path in %s\n' "$file" >&2
    return 1
  fi
}

normalize() {
  local input="$1" output="$2"
  validate_raw "$input"
  jq -S -f "$NORMALIZER" "$input" >"$output"
}

build_profile() {
  local profile="$1" out="$2" tags
  tags="$(profile_tags "$profile")"
  if [[ -n "$tags" ]]; then
    (cd "$CLI" && go build -tags "$tags" -o "$out" ./cmd/ao)
  else
    (cd "$CLI" && go build -o "$out" ./cmd/ao)
  fi
}

build_profile_from_cli() {
  local source_cli="$1" profile="$2" out="$3" tags
  tags="$(profile_tags "$profile")"
  if [[ -n "$tags" ]]; then
    (cd "$source_cli" && go build -tags "$tags" -o "$out" ./cmd/ao)
  else
    (cd "$source_cli" && go build -o "$out" ./cmd/ao)
  fi
}

capture_profile_from_cli() {
  local source_cli="$1" profile="$2" bin raw1 raw2 actual second
  bin="$TMP/capture-$profile-ao"
  raw1="$TMP/capture-$profile-1.json"
  raw2="$TMP/capture-$profile-2.json"
  actual="$TMP/capture-$profile.norm.json"
  second="$TMP/capture-$profile-2.norm.json"
  build_profile_from_cli "$source_cli" "$profile" "$bin"
  "$bin" capabilities --json >"$raw1"
  "$bin" capabilities --json >"$raw2"
  normalize "$raw1" "$actual"
  normalize "$raw2" "$second"
  cmp -s "$actual" "$second" || {
    printf 'nondeterministic capture execution source: %s\n' "$profile" >&2
    return 1
  }
  mkdir -p "$PROFILE_DIR"
  cp "$actual" "$PROFILE_DIR/$profile.json"
}

bind_measured_v2_profile() {
  local profile="$1" measured="$2" tracked
  local measured_hash tracked_hash metadata_hash
  tracked="$V2_DIR/profiles/$profile.json"
  measured_hash="$(sha256 "$measured")"
  tracked_hash="$(sha256 "$tracked")"
  metadata_hash="$(jq -r --arg p "$profile" '.profiles[$p].sha256' "$V2_DIR/metadata.json")"
  if ! cmp -s "$measured" "$tracked" \
      || [[ "$measured_hash" != "$tracked_hash" ]] \
      || [[ "$measured_hash" != "$metadata_hash" ]]; then
    printf 'v2 measured source mismatch: %s\n' "$profile" >&2
    return 1
  fi
}

verify_v2_source_binding_at() {
  local source="$1" label="$2" tree profile bin raw1 raw2 norm1 norm2
  tree="$TMP/v2-$label-source"
  mkdir -p "$tree"
  git -C "$ROOT" archive "$source" | tar -x -C "$tree"
  for profile in default flywheel legacy combined; do
    bin="$TMP/v2-$label-$profile-ao"
    raw1="$TMP/v2-$label-$profile-1.json"
    raw2="$TMP/v2-$label-$profile-2.json"
    norm1="$TMP/v2-$label-$profile-1.norm.json"
    norm2="$TMP/v2-$label-$profile-2.norm.json"
    build_profile_from_cli "$tree/cli" "$profile" "$bin"
    "$bin" capabilities --json >"$raw1"
    "$bin" capabilities --json >"$raw2"
    normalize "$raw1" "$norm1"
    normalize "$raw2" "$norm2"
    cmp -s "$norm1" "$norm2" || {
      printf 'nondeterministic v2 %s source: %s\n' "$label" "$profile" >&2
      return 1
    }
    bind_measured_v2_profile "$profile" "$norm1"
  done
}

verify_v2_source_binding() {
  local source
  source="$(jq -r '.behavioral_source_sha' "$V2_DIR/metadata.json")"
  verify_v2_source_binding_at "$source" behavioral
}

verify_source_decision() {
  local left="$source_decision_c59" right="$source_decision_main" label sha tree profile bin raw1 raw2 norm1 norm2 hash equal
  local recorded_left="" recorded_right="" v2_selected=0
  local records="$TMP/source-runs.jsonl" comparisons="$TMP/source-comparisons.jsonl"
  maybe_fail git
  is_commit "$left" || { printf 'invalid source-decision commit: %s\n' "$left" >&2; return 1; }
  is_commit "$right" || { printf 'invalid source-decision commit: %s\n' "$right" >&2; return 1; }
  if [[ -e "$BASELINE_ROOT/v2" || -L "$BASELINE_ROOT/v2" ]]; then
    v2_selected=1
    V2_DIR="$BASELINE_ROOT/v2"
    verify_v2_integrity
    recorded_left="$(jq -r '.behavioral_source_sha' "$V2_DIR/metadata.json")"
    recorded_right="$(jq -r '.equivalent_main_sha' "$V2_DIR/metadata.json")"
    [[ "$left" == "$recorded_left" ]] || {
      printf 'source-decision LEFT does not match recorded behavioral source: %s\n' "$left" >&2
      return 1
    }
    is_commit "$recorded_right" || {
      printf 'invalid recorded equivalent main commit: %s\n' "$recorded_right" >&2
      return 1
    }
    git -C "$ROOT" merge-base --is-ancestor "$recorded_right" "$right" || {
      printf 'source-decision RIGHT is not a descendant of recorded equivalent main: %s\n' "$right" >&2
      return 1
    }
  fi
  touch "$records" "$comparisons"
  for label in c59 main; do
    [[ "$label" == c59 ]] && sha="$left" || sha="$right"
    tree="$TMP/source-$label"
    mkdir -p "$tree"
    git -C "$ROOT" archive "$sha" | tar -x -C "$tree"
    for profile in default flywheel legacy combined; do
      bin="$TMP/source-$label-$profile-ao"
      build_profile_from_cli "$tree/cli" "$profile" "$bin"
      raw1="$TMP/source-$label-$profile-1.json"
      raw2="$TMP/source-$label-$profile-2.json"
      norm1="$TMP/source-$label-$profile-1.norm.json"
      norm2="$TMP/source-$label-$profile-2.norm.json"
      "$bin" capabilities --json >"$raw1"
      "$bin" capabilities --json >"$raw2"
      jq -e . "$raw1" >/dev/null && jq -e . "$raw2" >/dev/null
      jq -S -f "$BASELINE_ROOT/normalize.jq" "$raw1" >"$norm1"
      jq -S -f "$BASELINE_ROOT/normalize.jq" "$raw2" >"$norm2"
      cmp -s "$norm1" "$norm2" || { printf 'nondeterministic source decision: %s/%s\n' "$label" "$profile" >&2; return 1; }
      hash="$(sha256 "$norm1")"
      jq -nc --arg source "$label" --arg sha "$sha" --arg profile "$profile" --arg hash "$hash" '{source:$source,sha:$sha,profile:$profile,normalized_sha256:$hash,deterministic:true}' >>"$records"
    done
  done
  for profile in default flywheel legacy combined; do
    if cmp -s "$TMP/source-c59-$profile-1.norm.json" "$TMP/source-main-$profile-1.norm.json"; then equal=true; else equal=false; fi
    jq -nc --arg profile "$profile" --argjson equal "$equal" '{profile:$profile,equal:$equal}' >>"$comparisons"
    [[ "$equal" == true ]] || { printf 'source-decision behavior differs: %s\n' "$profile" >&2; return 1; }
  done
  if [[ "$v2_selected" == 1 ]]; then
    for profile in default flywheel legacy combined; do
      bind_measured_v2_profile "$profile" "$TMP/source-c59-$profile-1.norm.json"
      bind_measured_v2_profile "$profile" "$TMP/source-main-$profile-1.norm.json"
    done
  fi
  jq -s '.' "$records" >"$TMP/source-runs.json"
  jq -s '.' "$comparisons" >"$TMP/source-comparisons.json"
  jq -n --arg c59 "$left" --arg main "$right" --slurpfile runs "$TMP/source-runs.json" --slurpfile comparisons "$TMP/source-comparisons.json" '{schema_version:1,c59_sha:$c59,main_sha:$main,runs:$runs[0],cross_source:$comparisons[0],all_deterministic:true,all_cross_source_equal:true}'
}

capture_v2() {
  local target="$BASELINE_ROOT/v2" capture_sha profile execution_tree
  test -n "$execution_base" || { printf '--capture v2 requires --execution-base SHA\n' >&2; return 2; }
  test -n "$equivalent_main_sha" || { printf '--capture v2 requires --equivalent-main-sha SHA\n' >&2; return 2; }
  maybe_fail git
  is_commit "$execution_base" || { printf 'unreachable v2 behavioral source: %s\n' "$execution_base" >&2; return 1; }
  is_commit "$equivalent_main_sha" || { printf 'unreachable equivalent main: %s\n' "$equivalent_main_sha" >&2; return 1; }
  [[ "$profiles_csv" == "default,flywheel,legacy,combined" ]] || { printf 'v2 capture requires all four profiles\n' >&2; return 1; }
  [[ ! -e "$target" && ! -L "$target" ]] || { printf 'compatibility oracle v2 target already exists: %s\n' "$target" >&2; return 1; }
  maybe_fail lock
  mkdir "$v2_lock" || { printf 'cannot acquire v2 capture lock: %s\n' "$v2_lock" >&2; return 1; }
  trap 'cleanup_v2_capture; rm -rf "$TMP"' EXIT
  trap 'cleanup_v2_capture; exit 143' HUP INT TERM
  maybe_fail stage
  v2_stage="$(mktemp -d "$BASELINE_ROOT/.v2-stage.XXXXXX")"
  mkdir -p "$v2_stage/profiles"
  V2_DIR="$v2_stage"
  PROFILE_DIR="$v2_stage/profiles"
  BASELINE="$v2_stage"
  maybe_fail build
  execution_tree="$TMP/capture-execution-source"
  mkdir -p "$execution_tree"
  git -C "$ROOT" archive "$execution_base" | tar -x -C "$execution_tree"
  for profile in default flywheel legacy combined; do
    capture_profile_from_cli "$execution_tree/cli" "$profile"
  done
  maybe_fail binary
  maybe_fail json
  maybe_fail jq
  capture_sha="$(git -C "$ROOT" rev-parse HEAD)"
  jq -n --arg source "$execution_base" --arg capture "$capture_sha" --arg main "$equivalent_main_sha" \
    --arg d "$(sha256 "$PROFILE_DIR/default.json")" \
    --arg f "$(sha256 "$PROFILE_DIR/flywheel.json")" \
    --arg l "$(sha256 "$PROFILE_DIR/legacy.json")" \
    --arg c "$(sha256 "$PROFILE_DIR/combined.json")" \
    '{schema_version:2,behavioral_source_sha:$source,capture_sha:$capture,equivalent_main_sha:$main,normalized_fields:["tool_version","platform.os","platform.arch"],intentional_deltas:["provenance_reconcile","pawl_review_hold_5","verify_hold_5","environment_projection"],profiles:{default:{tags:[],sha256:$d},flywheel:{tags:["flywheel"],sha256:$f},legacy:{tags:["legacy"],sha256:$l},combined:{tags:["flywheel","legacy"],sha256:$c}}}' >"$v2_stage/metadata.json"
  BASELINE="$BASELINE_ROOT"
  PROFILE_DIR="$BASELINE_ROOT/profiles"
  verify_v1_integrity
  BASELINE="$v2_stage"
  PROFILE_DIR="$v2_stage/profiles"
  verify_v2_integrity
  verify_v2_source_binding_at "$equivalent_main_sha" capture-equivalent
  [[ ! -e "$target" && ! -L "$target" ]] || { printf 'compatibility oracle v2 target appeared during capture: %s\n' "$target" >&2; return 1; }
  maybe_fail rename
  mv "$v2_stage" "$target"
  v2_stage=""
  rmdir "$v2_lock"
  trap 'rm -rf "$TMP"' EXIT
  trap - HUP INT TERM
  V2_DIR="$target"
  BASELINE="$target"
  PROFILE_DIR="$target/profiles"
  printf 'captured append-only Go CLI compatibility oracle v2 at %s\n' "$execution_base"
}

capture_or_check_profile() {
  local profile="$1" action="$2" bin raw1 raw2 actual
  bin="$TMP/ao-$profile"
  raw1="$TMP/$profile.raw1.json"
  raw2="$TMP/$profile.raw2.json"
  actual="$TMP/$profile.json"
  build_profile "$profile" "$bin"
  "$bin" capabilities --json >"$raw1"
  "$bin" capabilities --json >"$raw2"
  normalize "$raw1" "$actual"
  normalize "$raw2" "$TMP/$profile.second.json"
  cmp -s "$actual" "$TMP/$profile.second.json" || {
    printf 'nondeterministic normalized capabilities for profile %s\n' "$profile" >&2
    return 1
  }
  if [[ "$action" == capture ]]; then
    mkdir -p "$PROFILE_DIR"
    cp "$actual" "$PROFILE_DIR/$profile.json"
  else
    test -f "$PROFILE_DIR/$profile.json" || { printf 'missing profile baseline: %s\n' "$profile" >&2; return 1; }
    if ! cmp -s "$PROFILE_DIR/$profile.json" "$actual"; then
      printf 'CLI compatibility drift in profile %s\n' "$profile" >&2
      diff -u "$PROFILE_DIR/$profile.json" "$actual" >&2 || true
      return 1
    fi
  fi
}

verify_v1_integrity() {
  local metadata="$BASELINE/metadata.json" profile expected actual
  test -f "$metadata" || { printf 'missing compatibility metadata.json\n' >&2; return 1; }
  jq -e '.schema_version == 1 and (.execution_base | length) == 40 and (.profiles | keys | sort) == ["combined","default","flywheel","legacy"]' "$metadata" >/dev/null
  for profile in default flywheel legacy combined; do
    test -f "$PROFILE_DIR/$profile.json" || { printf 'missing profile baseline: %s\n' "$profile" >&2; return 1; }
    expected="$(jq -r --arg p "$profile" '.profiles[$p].sha256' "$metadata")"
    actual="$(sha256 "$PROFILE_DIR/$profile.json")"
    test "$actual" = "$expected" || { printf 'baseline hash mismatch: %s\n' "$profile" >&2; return 1; }
  done
  if git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    base="$(jq -r '.execution_base' "$metadata")"
    git -C "$ROOT" merge-base --is-ancestor "$base" HEAD || {
      printf 'execution base %s is not an ancestor of HEAD\n' "$base" >&2
      return 1
    }
  fi
  if [[ "$mode" != capture ]]; then
    verify_git_freeze
  fi
}

verify_integrity() {
  local selected="$BASELINE" selected_profiles="$PROFILE_DIR"
  if [[ "$BASELINE" == "$V2_DIR" ]]; then
    BASELINE="$BASELINE_ROOT"
    PROFILE_DIR="$BASELINE_ROOT/profiles"
    verify_v1_integrity
    BASELINE="$selected"
    PROFILE_DIR="$selected_profiles"
    verify_v2_integrity
    verify_v2_source_binding
  else
    verify_v1_integrity
  fi
}

validate_dimension() {
  local case_file="$1" name="$2"
  jq -e --arg name "$name" '
    .checks as $checks
    | ($checks | map(.id)) as $ids
    | .dimensions[$name] as $d
    | ($d.status == "not_applicable" and ($d.reason | type == "string" and length > 0))
      or ($d.status == "evidence" and ($d.evidence | type == "array" and length > 0)
          and all($d.evidence[]; . as $id | $ids | index($id) != null))
  ' "$case_file" >/dev/null
}

validate_family() {
  local name="$1" rel dir case_file ownership lineage
  rel="cli/testdata/compatibility-baseline/families/$name"
  dir="$FAMILY_DIR/$name"
  case_file="$dir/case.json"
  ownership="$dir/ownership.json"
  lineage="$dir/lineage.json"
  local dimension check command
  test -f "$case_file" || { printf 'missing family case: %s\n' "$name" >&2; return 1; }
  test -f "$ownership" || { printf 'missing family ownership: %s\n' "$name" >&2; return 1; }
  jq -e --arg family "$name" '
    .schema_version == 1 and .family == $family
    and (.checks | type == "array" and length > 0)
    and ([.checks[].id] | length == (unique | length))
    and all(.checks[]; (.id | type == "string" and length > 0) and (.command | type == "string" and length > 0))
  ' "$case_file" >/dev/null
  for dimension in path aliases accepted_args rejected_args stdout stderr exit_classes tracker_selection ordered_effects; do
    validate_dimension "$case_file" "$dimension" || { printf 'invalid or missing dimension %s for %s\n' "$dimension" "$name" >&2; return 1; }
  done
  jq -e --arg family "$name" '
    .schema_version == 1 and .family == $family
    and (.profiles | keys | sort) == ["combined","default","flywheel","legacy"]
    and all(.profiles[]; . == "present" or . == "absent")
    and (.legacy_symbols | type == "array")
    and (.live_owner | type == "string" and length > 0)
    and (.allowed_paths | type == "array" and length > 0)
  ' "$ownership" >/dev/null

  if [[ "$verify_frozen" == 1 ]]; then
    test -f "$lineage" || { printf 'missing family lineage: %s\n' "$name" >&2; return 1; }
    jq -e --arg family "$name" '.schema_version == 1 and .family == $family and (.old_implementation_sha|length)==40 and (.freeze_sha|length)==40 and (.fixture_sha256|length)==64 and (.ownership_sha256|length)==64' "$lineage" >/dev/null
    test "$(sha256 "$case_file")" = "$(jq -r '.fixture_sha256' "$lineage")" || { printf 'fixture digest drift: %s\n' "$name" >&2; return 1; }
    test "$(sha256 "$ownership")" = "$(jq -r '.ownership_sha256' "$lineage")" || { printf 'ownership digest drift: %s\n' "$name" >&2; return 1; }
    old="$(jq -r '.old_implementation_sha' "$lineage")"
    freeze="$(jq -r '.freeze_sha' "$lineage")"
    git -C "$ROOT" merge-base --is-ancestor "$freeze" HEAD || { printf 'freeze SHA is not an ancestor: %s\n' "$name" >&2; return 1; }
    test "$(git -C "$ROOT" rev-parse "$freeze^")" = "$old" || { printf 'old/freeze lineage mismatch: %s\n' "$name" >&2; return 1; }
    git -C "$ROOT" cat-file -e "$freeze:$rel/case.json"
    git -C "$ROOT" cat-file -e "$freeze:$rel/ownership.json"
    while IFS= read -r changed; do
      case "$changed" in
        "$rel"/*) ;;
        *) printf 'non-fixture path changed in freeze commit for %s: %s\n' "$name" "$changed" >&2; return 1 ;;
      esac
    done < <(git -C "$ROOT" diff --name-only "$old..$freeze")
    git -C "$ROOT" diff --quiet "$freeze..HEAD" -- "$rel/case.json" "$rel/ownership.json" || { printf 'frozen family fixture mutated: %s\n' "$name" >&2; return 1; }
  fi

  while IFS= read -r check; do
    command="$(jq -r '.command' <<<"$check")"
    (cd "$ROOT" && bash -euo pipefail -c "$command") || {
      printf 'family evidence check failed: %s/%s\n' "$name" "$(jq -r '.id' <<<"$check")" >&2
      return 1
    }
  done < <(jq -c '.checks[]' "$case_file")
}

if [[ "$mode" == source_decision ]]; then
  verify_source_decision
  exit 0
fi

if [[ "$mode" == capture && "$oracle_version" == v2 ]]; then
  capture_v2
  exit 0
fi

select_oracle

case "$mode" in
  raw)
    validate_raw "$raw_file"
    printf 'raw capabilities stable: %s\n' "$raw_file"
    exit 0
    ;;
  integrity)
    verify_integrity
    printf 'compatibility baseline integrity PASS\n'
    exit 0
    ;;
  profile_file)
    verify_integrity
    profile_tags "$family" >/dev/null
    normalize "$profile_file" "$TMP/$family.supplied.json"
    cmp -s "$PROFILE_DIR/$family.json" "$TMP/$family.supplied.json" || {
      printf 'CLI compatibility drift in supplied profile %s\n' "$family" >&2
      exit 1
    }
    printf 'supplied profile compatible: %s\n' "$family"
    exit 0
    ;;
  family_only)
    validate_family "$family"
    printf 'family fixture valid: %s\n' "$family"
    exit 0
    ;;
esac

test -f "$NORMALIZER" || { printf 'missing normalizer: %s\n' "$NORMALIZER" >&2; exit 1; }
IFS=',' read -r -a profiles <<<"$profiles_csv"
for profile in "${profiles[@]}"; do profile_tags "$profile" >/dev/null; done

if [[ "$mode" == capture ]]; then
  test -n "$execution_base" || { printf '--capture requires --execution-base SHA\n' >&2; exit 2; }
  test "${#execution_base}" -eq 40
  if [[ -f "$BASELINE/metadata.json" ]]; then
    printf 'baseline already exists; capture is immutable\n' >&2
    exit 1
  fi
  for profile in "${profiles[@]}"; do capture_or_check_profile "$profile" capture; done
  test "${#profiles[@]}" -eq 4 || { printf 'capture requires all four profiles\n' >&2; exit 1; }
  jq -n --arg base "$execution_base" \
    --arg d "$(sha256 "$PROFILE_DIR/default.json")" \
    --arg f "$(sha256 "$PROFILE_DIR/flywheel.json")" \
    --arg l "$(sha256 "$PROFILE_DIR/legacy.json")" \
    --arg c "$(sha256 "$PROFILE_DIR/combined.json")" \
    '{schema_version:1,execution_base:$base,normalized_fields:["tool_version","platform.os","platform.arch"],profiles:{default:{tags:[],sha256:$d},flywheel:{tags:["flywheel"],sha256:$f},legacy:{tags:["legacy"],sha256:$l},combined:{tags:["flywheel","legacy"],sha256:$c}}}' \
    >"$BASELINE/metadata.json"
  verify_integrity
  printf 'captured immutable Go CLI compatibility baseline at %s\n' "$execution_base"
  exit 0
fi

verify_integrity
for profile in "${profiles[@]}"; do capture_or_check_profile "$profile" check; done

if [[ "$all_migrated" == 1 ]]; then
  found=0
  while IFS= read -r lineage; do
    found=1
    validate_family "$(basename "$(dirname "$lineage")")"
  done < <(find "$FAMILY_DIR" -mindepth 2 -maxdepth 2 -name lineage.json -type f 2>/dev/null | sort)
  test "$found" -eq 1 || { printf 'no migrated family fixtures found\n' >&2; exit 1; }
elif [[ -n "$family" ]]; then
  validate_family "$family"
fi

printf 'Go CLI compatibility PASS: profiles=%s%s\n' "$profiles_csv" "${family:+ family=$family}"
