#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  CASE=0
}

json_file() {
  CASE=$((CASE + 1))
  local path="$BATS_TEST_TMPDIR/case-$CASE.json"
  printf '%s\n' "$1" > "$path"
  printf '%s\n' "$path"
}

assert_accepts() {
  local validator="$1" payload="$2" path
  [ -x "$validator" ]
  path="$(json_file "$payload")"
  run "$validator" "$path"
  [ "$status" -eq 0 ]
}

assert_rejects() {
  local validator="$1" payload="$2" path
  [ -x "$validator" ]
  path="$(json_file "$payload")"
  run "$validator" "$path"
  [ "$status" -ne 0 ]
}

write_valid_recon_baseline() {
  local path="$1" commit="${2:-deadbeef}"
  write_recon_pack "$path" "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"$commit\",\"flows\":[{\"entry\":\"cli/main.go\",\"domain\":\"internal/domain\",\"integration\":\"internal/adapters\",\"tests\":\"internal/domain/x_test.go\"}],\"claims\":[],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}"
}

sha256_stream() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
  else
    shasum -a 256 | awk '{print $1}'
  fi
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

write_recon_pack() {
  local path="$1" payload="$2" dir raw report commit mode flows_sha claims_sha coverage_sha report_sha
  dir="$(dirname "$path")"
  mkdir -p "$dir"
  raw="$dir/.manifest.raw.json"
  report="$dir/codebase-recon.md"
  printf '%s\n' "$payload" > "$raw"
  commit="$(jq -r '.commit // empty' "$raw")"
  mode="$(jq -r '.mode // empty' "$raw")"
  flows_sha="$(jq -cS '.flows' "$raw" | sha256_stream)"
  claims_sha="$(jq -cS '.claims' "$raw" | sha256_stream)"
  coverage_sha="$(jq -cS '.coverage' "$raw" | sha256_stream)"
  cat > "$report" <<EOF
<!-- codebase-recon-report.v1 -->
manifest_commit: $commit
manifest_mode: $mode
flows_sha256: $flows_sha
claims_sha256: $claims_sha
coverage_sha256: $coverage_sha

# Codebase recon fixture
EOF
  report_sha="$(sha256_file "$report")"
  jq --arg report_sha "$report_sha" \
    '. + {report:{path:"codebase-recon.md",sha256:$report_sha}}' \
    "$raw" > "$path"
  rm -f "$raw"
}

recon_file() {
  CASE=$((CASE + 1))
  local dir="$BATS_TEST_TMPDIR/recon-case-$CASE" path="$BATS_TEST_TMPDIR/recon-case-$CASE/codebase-recon.json"
  mkdir -p "$dir"
  write_recon_pack "$path" "$1"
  printf '%s\n' "$path"
}

init_recon_repo() {
  local target="$1"
  mkdir -p "$target/cli" "$target/internal/domain" "$target/internal/adapters"
  git -C "$target" init -q
  git -C "$target" config user.name fixture
  git -C "$target" config user.email fixture@example.invalid
  printf 'package main\n' > "$target/cli/main.go"
  printf 'package domain\n' > "$target/internal/domain/model.go"
  printf 'package domain\n' > "$target/internal/domain/x_test.go"
  printf 'package adapters\n' > "$target/internal/adapters/adapter.go"
  printf 'entry -> domain -> test\n' > "$target/evidence.txt"
  git -C "$target" add cli/main.go internal evidence.txt
  git -C "$target" commit -qm baseline
  git -C "$target" rev-parse HEAD
}

write_slow_recon_baseline() {
  local path="$1" commit="$2" payload
  payload="$(jq -cn --arg commit "$commit" '
    {
      schema_version: "codebase-recon.v1",
      mode: "baseline",
      commit: $commit,
      flows: [{
        entry: "cli/main.go",
        domain: "internal/domain",
        integration: "internal/adapters",
        tests: "internal/domain/x_test.go"
      }],
      claims: [range(0; 150) | {
        kind: "fact",
        text: ("race witness " + tostring),
        confidence: "high",
        evidence: ["evidence.txt:1"]
      }],
      coverage: {inspected: ["cli"], uninspected: ["images"]}
    }
  ')"
  write_recon_pack "$path" "$payload"
}

run_recon_race() {
  local validator="$1" target="$2" pack="$3" mutation="$4"
  local sync_root="$BATS_TEST_TMPDIR/race-$mutation" output_file pid snapshot="" file_count=0 ready=0 i
  mkdir -p "$sync_root"
  output_file="$sync_root/output"

  TMPDIR="$sync_root" "$validator" --repo-root "$target" "$pack" >"$output_file" 2>&1 &
  pid=$!
  for ((i = 0; i < 1000; i++)); do
    snapshot="$(find "$sync_root" -mindepth 1 -maxdepth 1 -type d -name 'codebase-recon-validate.*' -print -quit)"
    if [[ -n "$snapshot" ]]; then
      file_count="$(find "$snapshot" -mindepth 1 -maxdepth 1 -type f | wc -l | tr -d ' ')"
      if [[ "$file_count" -ge 2 ]]; then
        ready=1
        break
      fi
    fi
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.005
  done

  if [[ "$ready" != "1" ]]; then
    if wait "$pid"; then
      RACE_STATUS=0
    else
      RACE_STATUS=$?
    fi
    RACE_OUTPUT="$(cat "$output_file")"
    return 1
  fi

  case "$mutation" in
    manifest) printf ' ' >> "$pack" ;;
    report) printf '\nlate report mutation\n' >> "$(dirname "$pack")/codebase-recon.md" ;;
    worktree) printf '// late worktree mutation\n' >> "$target/cli/main.go" ;;
    index)
      printf '// late index mutation\n' >> "$target/cli/main.go"
      git -C "$target" add cli/main.go
      ;;
    head) git -C "$target" commit --allow-empty -qm 'race commit' ;;
    *) return 2 ;;
  esac

  if wait "$pid"; then
    RACE_STATUS=0
  else
    RACE_STATUS=$?
  fi
  RACE_OUTPUT="$(cat "$output_file")"
}

# B1.1
@test "idea-genie produces an evidence-grounded idea-portfolio artifact" {
  v="$REPO_ROOT/skills/idea-genie/scripts/validate-output.sh"
  assert_accepts "$v" '{"schema_version":"idea-portfolio.v1","status":"candidates","observations":[{"claim":"users cannot discover the route","evidence":"docs/SKILLS.md:1"}],"assumptions":["generated maps are current"],"candidates":[{"id":"I1","evidence":["docs/SKILLS.md:1"],"overlaps":[],"scenario":{"given":"a user has a goal","when":"routing runs","then":"the owner is named"}}],"termination":{"reason":"novelty-saturated","novel_candidates_last_pass":0}}'
  assert_rejects "$v" '{"schema_version":"idea-portfolio.v1","status":"candidates","observations":[],"assumptions":[],"candidates":[{"id":"I1","evidence":[],"overlaps":[],"scenario":{"given":"x","when":"y","then":"z"}}],"termination":{"reason":"novelty-saturated","novel_candidates_last_pass":0}}'
}

# B1.2
@test "idea-genie can return no-new-work without manufacturing candidates" {
  v="$REPO_ROOT/skills/idea-genie/scripts/validate-output.sh"
  assert_accepts "$v" '{"schema_version":"idea-portfolio.v1","status":"no-new-work","observations":[{"claim":"all candidates overlap","evidence":"skills/catalog.json"}],"assumptions":[],"candidates":[],"termination":{"reason":"all-overlap-or-unsupported","novel_candidates_last_pass":0}}'
  assert_rejects "$v" '{"schema_version":"idea-portfolio.v1","status":"no-new-work","observations":[],"assumptions":[],"candidates":[{"id":"padding"}],"termination":{"reason":"fixed-count-reached","novel_candidates_last_pass":1}}'
}

# B2.1
@test "idea-genie duel mode emits sealed advisory evidence for Plan" {
  v="$REPO_ROOT/skills/idea-genie/scripts/validate-challenge.sh"
  assert_accepts "$v" '{"schema_version":"idea-challenge.v1","door_class":"one-way","sealed_generation":true,"perspectives":[{"id":"P1","context_id":"c1"},{"id":"P2","context_id":"c2"}],"cross_reviews":[{"reviewer":"P1","subject":"P2","dimensions":{"evidence":"WARN"}}],"disagreements":["port ownership"],"refutations":[{"claim":"P1","attempt":"existing seam","result":"survived"}],"handoff":{"owner":"plan","artifact_dir":".agents/scratch/ideas/run-1"}}'
  assert_rejects "$v" '{"schema_version":"idea-challenge.v1","door_class":"one-way","sealed_generation":false,"perspectives":[{"id":"P1","context_id":"same"},{"id":"P2","context_id":"same"}],"cross_reviews":[],"disagreements":[],"refutations":[],"handoff":{"owner":"self-score"}}'
  assert_rejects "$v" '{"schema_version":"idea-challenge.v1","door_class":"one-way","sealed_generation":true,"perspectives":[{"id":"P1","context_id":"c1"},{"id":"P2","context_id":"c2"}],"cross_reviews":[{"reviewer":"P1","subject":"P2","dimensions":{"evidence":"WARN"}}],"disagreements":["x"],"refutations":[{"claim":"P1","attempt":"x","result":"survived"}],"handoff":{"owner":"plan","artifact_dir":".agents/scratch/ideas/run-1"},"readiness":"PASS"}'
}

# B2.2
@test "idea-genie duel mode routes reversible choices without NTM ceremony" {
  v="$REPO_ROOT/skills/idea-genie/scripts/validate-challenge.sh"
  assert_accepts "$v" '{"schema_version":"idea-challenge.v1","door_class":"two-way","sealed_generation":false,"perspectives":[],"cross_reviews":[],"disagreements":[],"refutations":[],"handoff":{"owner":"plan","artifact_dir":".agents/scratch/ideas/run-2","route":"single-fresh-context"},"requires_ntm":false}'
  assert_rejects "$v" '{"schema_version":"idea-challenge.v1","door_class":"two-way","sealed_generation":true,"perspectives":[],"cross_reviews":[],"disagreements":[],"refutations":[],"handoff":{"owner":"ntm"},"requires_ntm":true}'
}

# B3.1
@test "codebase-recon validates evidence-bounded fact inference unknown claims" {
  v="$REPO_ROOT/skills/codebase-recon/scripts/validate-output.sh"
  target="$BATS_TEST_TMPDIR/recon-baseline"
  commit="$(init_recon_repo "$target")"
  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"$commit\",\"flows\":[{\"entry\":\"cli/main.go\",\"domain\":\"internal/domain\",\"integration\":\"internal/adapters\",\"tests\":\"internal/domain/x_test.go\"}],\"claims\":[{\"kind\":\"fact\",\"text\":\"a flow exists\",\"confidence\":\"high\",\"evidence\":[\"evidence.txt\"]},{\"kind\":\"unknown\",\"text\":\"remote behavior\",\"confidence\":\"low\",\"evidence\":[]}],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -eq 0 ]

  short_commit="${commit:0:7}"
  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"$short_commit\",\"flows\":[{\"entry\":\"cli/main.go\",\"domain\":\"internal/domain\",\"integration\":\"internal/adapters\",\"tests\":\"internal/domain/x_test.go\"}],\"claims\":[],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]

  git -C "$target" tag deadbee "$commit"
  artifact="$(recon_file '{"schema_version":"codebase-recon.v1","mode":"baseline","commit":"deadbee","flows":[{"entry":"cli/main.go","domain":"internal/domain","integration":"internal/adapters","tests":"internal/domain/x_test.go"}],"claims":[],"coverage":{"inspected":["cli"],"uninspected":["images"]}}')"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]

  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"$commit\",\"flows\":[{\"entry\":\"cli/main.go\",\"domain\":\"internal/domain\",\"integration\":\"internal/adapters\",\"tests\":\"internal/domain/x_test.go\"}],\"claims\":[{\"kind\":\"fact\",\"text\":\"external evidence\",\"confidence\":\"high\",\"evidence\":[\"$target/evidence.txt\"]}],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]

  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"$commit\",\"flows\":[{\"entry\":\"cli/main.go\",\"domain\":\"internal/domain\",\"integration\":\"internal/adapters\",\"tests\":\"internal/domain/x_test.go\"}],\"claims\":[{\"kind\":\"fact\",\"text\":\"pathspec evidence\",\"confidence\":\"high\",\"evidence\":[\":(top)evidence.txt\"]}],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]

  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"$commit\",\"flows\":[{\"entry\":\"cli/main.go\",\"domain\":\"internal/domain\",\"integration\":\"internal/adapters\",\"tests\":\"internal/domain/x_test.go\"}],\"claims\":[{\"kind\":\"fact\",\"text\":\"control-byte evidence\",\"confidence\":\"high\",\"evidence\":[\"evidence.txt\\u0000\"]}],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]

  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"$commit\",\"flows\":[{\"entry\":\"cli/main.go\",\"domain\":\"internal/domain\",\"integration\":\"internal/adapters\",\"tests\":\"internal/domain/x_test.go\"}],\"claims\":[{\"kind\":\"fact\",\"text\":\"bad line\",\"confidence\":\"high\",\"evidence\":[\"evidence.txt:99\"]}],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]

  artifact="$(recon_file '{"schema_version":"codebase-recon.v1","mode":"baseline","commit":"deadbeef","flows":[],"claims":[{"kind":"fact","text":"unsupported","confidence":"high","evidence":[]}],"coverage":{"inspected":[],"uninspected":[]}}')"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]
}

# B3.2
@test "codebase-recon requires a verified delta when a prior pack exists" {
  v="$REPO_ROOT/skills/codebase-recon/scripts/validate-output.sh"
  target="$BATS_TEST_TMPDIR/recon-delta"
  baseline_commit="$(init_recon_repo "$target")"
  prior="$target/.agents/recon/prior/codebase-recon.json"
  write_valid_recon_baseline "$prior" "$baseline_commit"
  printf 'package x\n' > "$target/cli/x.go"
  git -C "$target" add cli/x.go
  git -C "$target" commit -qm delta
  current_commit="$(git -C "$target" rev-parse HEAD)"

  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"delta\",\"commit\":\"$current_commit\",\"prior_recon\":\"$prior\",\"baseline_verified\":true,\"delta\":[{\"path\":\"cli/x.go\",\"change\":\"new adapter\"}],\"flows\":[],\"claims\":[],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -eq 0 ]

  printf '// dirty\n' >> "$target/cli/main.go"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]
  [[ "$output" == *"source changes not bound"* ]]
  git -C "$target" restore cli/main.go

  printf 'untracked source\n' > "$target/untracked.txt"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]
  [[ "$output" == *"source changes not bound"* ]]
  rm -f "$target/untracked.txt"

  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"delta\",\"commit\":\"$baseline_commit\",\"prior_recon\":\"$prior\",\"baseline_verified\":true,\"delta\":[{\"path\":\"cli/x.go\",\"change\":\"new adapter\"}],\"flows\":[],\"claims\":[],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]

  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"delta\",\"commit\":\"$current_commit\",\"prior_recon\":\"$prior\",\"baseline_verified\":true,\"delta\":[{\"path\":\"cli/unrelated.go\",\"change\":\"fabricated\"}],\"flows\":[],\"claims\":[],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]

  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"delta\",\"commit\":\"$current_commit\",\"prior_recon\":\"$prior:1\",\"baseline_verified\":true,\"delta\":[{\"path\":\"cli/x.go\",\"change\":\"new adapter\"}],\"flows\":[],\"claims\":[],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]

  unknown_prior="$BATS_TEST_TMPDIR/unknown/codebase-recon.json"
  write_valid_recon_baseline "$unknown_prior" deadbeef
  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"delta\",\"commit\":\"$current_commit\",\"prior_recon\":\"$unknown_prior\",\"baseline_verified\":true,\"delta\":[{\"path\":\"cli/x.go\",\"change\":\"new adapter\"}],\"flows\":[],\"claims\":[],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]

  invalid_prior="$BATS_TEST_TMPDIR/invalid/codebase-recon.json"
  mkdir -p "$(dirname "$invalid_prior")"
  printf '%s\n' '{"schema_version":"codebase-recon.v1","mode":"baseline"}' > "$invalid_prior"
  artifact="$(recon_file "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"delta\",\"commit\":\"$current_commit\",\"prior_recon\":\"$invalid_prior\",\"baseline_verified\":true,\"delta\":[{\"path\":\"cli/x.go\",\"change\":\"new adapter\"}],\"flows\":[],\"claims\":[],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}")"
  run "$v" --repo-root "$target" "$artifact"
  [ "$status" -ne 0 ]
}

# B3.3
@test "codebase-recon prior-discovery finds validated packs at current and earlier default paths" {
  v="$REPO_ROOT/skills/codebase-recon/scripts/validate-output.sh"
  target="$BATS_TEST_TMPDIR/target"
  legacy="$target/.agents/recon/legacy-run/codebase-recon.json"
  current="$target/.agents/scratch/codebase-recon/current-run/codebase-recon.json"

  baseline_commit="$(init_recon_repo "$target")"
  run "$v" --repo-root "$target" --discover-priors
  [ "$status" -eq 0 ]
  [ -z "$output" ]

  write_valid_recon_baseline "$legacy" "$baseline_commit"
  printf 'package x\n' > "$target/cli/x.go"
  git -C "$target" add cli/x.go
  git -C "$target" commit -qm delta
  current_commit="$(git -C "$target" rev-parse HEAD)"
  write_valid_recon_baseline "$current" "$current_commit"
  invalid="$target/.agents/recon/invalid-run/codebase-recon.json"
  write_valid_recon_baseline "$invalid" deadbeef

  run "$v" --repo-root "$target" --discover-priors
  [ "$status" -eq 0 ]
  [[ "$output" == *"$legacy"* ]]
  [[ "$output" == *"$current"* ]]
  ! grep -Fxq "$invalid" <<<"$output"

  delta="$BATS_TEST_TMPDIR/delta/codebase-recon.json"
  write_recon_pack "$delta" "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"delta\",\"commit\":\"$current_commit\",\"prior_recon\":\".agents/recon/legacy-run/codebase-recon.json\",\"baseline_verified\":true,\"delta\":[{\"path\":\"cli/x.go\",\"change\":\"new adapter\"}],\"flows\":[],\"claims\":[],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}"
  run "$v" --repo-root "$target" "$delta"
  [ "$status" -eq 0 ]
  [ -f "$legacy" ]
  [ -f "$current" ]
}

# B3.4
@test "codebase-recon resolves historical evidence against each manifest commit" {
  v="$REPO_ROOT/skills/codebase-recon/scripts/validate-output.sh"
  target="$BATS_TEST_TMPDIR/recon-evidence-history"
  baseline_commit="$(init_recon_repo "$target")"
  good="$target/.agents/recon/good/codebase-recon.json"
  bad="$target/.agents/recon/bad/codebase-recon.json"
  mkdir -p "$(dirname "$good")" "$(dirname "$bad")"
  write_recon_pack "$good" "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"$baseline_commit\",\"flows\":[{\"entry\":\"cli/main.go\",\"domain\":\"internal/domain\",\"integration\":\"internal/adapters\",\"tests\":\"internal/domain/x_test.go\"}],\"claims\":[{\"kind\":\"fact\",\"text\":\"historical evidence\",\"confidence\":\"high\",\"evidence\":[\"evidence.txt:1\"]}],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}"

  printf 'rewritten later\n' > "$target/evidence.txt"
  printf 'future only\n' > "$target/future.txt"
  git -C "$target" add evidence.txt future.txt
  git -C "$target" commit -qm later
  write_recon_pack "$bad" "{\"schema_version\":\"codebase-recon.v1\",\"mode\":\"baseline\",\"commit\":\"$baseline_commit\",\"flows\":[{\"entry\":\"cli/main.go\",\"domain\":\"internal/domain\",\"integration\":\"internal/adapters\",\"tests\":\"internal/domain/x_test.go\"}],\"claims\":[{\"kind\":\"fact\",\"text\":\"future evidence\",\"confidence\":\"high\",\"evidence\":[\"future.txt\"]}],\"coverage\":{\"inspected\":[\"cli\"],\"uninspected\":[\"images\"]}}"

  run "$v" --repo-root "$target" --discover-priors
  [ "$status" -eq 0 ]
  good_physical="$(cd "$(dirname "$good")" && pwd -P)/$(basename "$good")"
  bad_physical="$(cd "$(dirname "$bad")" && pwd -P)/$(basename "$bad")"
  grep -Fxq "$good_physical" <<<"$output"
  ! grep -Fxq "$bad_physical" <<<"$output"
}

# B3.5
@test "codebase-recon binds its companion and rejects validation-time mutation" {
  v="$REPO_ROOT/skills/codebase-recon/scripts/validate-output.sh"
  target="$BATS_TEST_TMPDIR/recon-companion"
  commit="$(init_recon_repo "$target")"
  pack="$target/.agents/scratch/codebase-recon/run/codebase-recon.json"
  report="$(dirname "$pack")/codebase-recon.md"
  write_valid_recon_baseline "$pack" "$commit"

  run "$v" --repo-root "$target" "$pack"
  [ "$status" -eq 0 ]

  rm -f "$report"
  run "$v" --repo-root "$target" "$pack"
  [ "$status" -ne 0 ]

  write_valid_recon_baseline "$pack" "$commit"
  printf '\nmutated report\n' >> "$report"
  run "$v" --repo-root "$target" "$pack"
  [ "$status" -ne 0 ]

  write_valid_recon_baseline "$pack" "$commit"
  outside="$BATS_TEST_TMPDIR/outside-report.md"
  cp "$report" "$outside"
  rm -f "$report"
  ln -s "$outside" "$report"
  run "$v" --repo-root "$target" "$pack"
  [ "$status" -ne 0 ]

  rm -f "$report"
  write_valid_recon_baseline "$pack" "$commit"
  manifest_link="$BATS_TEST_TMPDIR/manifest-link.json"
  ln -s "$pack" "$manifest_link"
  run "$v" --repo-root "$target" "$manifest_link"
  [ "$status" -ne 0 ]

  write_slow_recon_baseline "$pack" "$commit"
  run_recon_race "$v" "$target" "$pack" manifest
  [ "$RACE_STATUS" -ne 0 ]

  write_slow_recon_baseline "$pack" "$commit"
  run_recon_race "$v" "$target" "$pack" report
  [ "$RACE_STATUS" -ne 0 ]

  write_slow_recon_baseline "$pack" "$commit"
  run_recon_race "$v" "$target" "$pack" worktree
  [ "$RACE_STATUS" -ne 0 ]
  git -C "$target" restore cli/main.go

  write_slow_recon_baseline "$pack" "$commit"
  run_recon_race "$v" "$target" "$pack" index
  [ "$RACE_STATUS" -ne 0 ]
  git -C "$target" restore --staged cli/main.go
  git -C "$target" restore cli/main.go

  write_slow_recon_baseline "$pack" "$commit"
  run_recon_race "$v" "$target" "$pack" head
  [ "$RACE_STATUS" -ne 0 ]
  git -C "$target" reset --soft "$commit"
}

# B3.6
@test "Codex projection executes the hardened recon validator contract" {
  canonical="$REPO_ROOT/skills/codebase-recon/scripts/validate-output.sh"
  projected="$REPO_ROOT/skills-codex/codebase-recon/scripts/validate-output.sh"
  [ -x "$projected" ]
  cmp -s "$canonical" "$projected"

  target="$BATS_TEST_TMPDIR/recon-projection"
  commit="$(init_recon_repo "$target")"
  pack="$target/.agents/recon/projected/codebase-recon.json"
  write_valid_recon_baseline "$pack" "$commit"
  run "$projected" --repo-root "$target" --discover-priors
  [ "$status" -eq 0 ]
  [[ "$output" == *"$pack"* ]]

  short_pack="$BATS_TEST_TMPDIR/projected-short/codebase-recon.json"
  write_valid_recon_baseline "$short_pack" "${commit:0:7}"
  run "$projected" --repo-root "$target" "$short_pack"
  [ "$status" -ne 0 ]

  printf 'dirty\n' > "$target/untracked.txt"
  run "$projected" --repo-root "$target" "$pack"
  [ "$status" -ne 0 ]
  [[ "$output" == *"source changes not bound"* ]]
}

# B4.1
@test "pattern-mining promotes only a three-exemplar holdout-proven pattern" {
  v="$REPO_ROOT/skills/pattern-mining/scripts/validate-output.sh"
  assert_accepts "$v" '{"schema_version":"pattern-mining.v1","outcome":"promote","exemplars":["a.go:1","b.go:2","c.go:3"],"invariants":["fail closed"],"variations":["transport"],"incidental":["identifier"],"holdout":{"source":"d.go:4","result":"pass"},"back_application":"pass","route":"operationalize"}'
  assert_rejects "$v" '{"schema_version":"pattern-mining.v1","outcome":"promote","exemplars":["a.go:1","b.go:2"],"invariants":["x"],"variations":[],"incidental":[],"holdout":{"source":"c.go:3","result":"pass"},"back_application":"pass","route":"skill"}'
}

# B4.2
@test "pattern-mining keeps weak evidence as a hypothesis" {
  v="$REPO_ROOT/skills/pattern-mining/scripts/validate-output.sh"
  assert_accepts "$v" '{"schema_version":"pattern-mining.v1","outcome":"hypothesis","exemplars":["a.go:1","b.go:2"],"invariants":[],"variations":[],"incidental":[],"holdout":{"source":"","result":"not-run"},"back_application":"not-run","route":"no-action"}'
  assert_rejects "$v" '{"schema_version":"pattern-mining.v1","outcome":"hypothesis","exemplars":["a.go:1"],"invariants":[],"variations":[],"incidental":[],"holdout":{"source":"","result":"not-run"},"back_application":"not-run","route":"skill"}'
}
