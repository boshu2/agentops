#!/usr/bin/env bash
set -euo pipefail

ROOT="${AO_CLI_COMPAT_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
CLI="$ROOT/cli"
BASELINE="${AO_CLI_COMPAT_BASELINE_DIR:-$CLI/testdata/compatibility-baseline}"
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

usage() {
  cat <<'EOF'
usage: check-go-cli-compatibility.sh [options]

  --capture --execution-base SHA  Capture the immutable four-profile baseline.
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

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
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

verify_integrity() {
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
