#!/usr/bin/env bash

# shellcheck source=scripts/lib/preamble.sh
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

mode="${1:-}"
shift || true

if [[ "$mode" == "--production" ]]; then
  [[ $# -eq 2 && "$1" == "--candidate-sha" ]] || {
    echo "usage: scripts/check-go-cli-semantic-seals.sh --production --candidate-sha <sha>" >&2
    exit 2
  }
  cd "$REPO_ROOT/cli" || exit 1
  exec go run ./internal/archcheck/cmd --semantic-production-gate --candidate-sha "$2"
fi

if [[ "$mode" != "--self-test" || $# -ne 0 ]]; then
  echo "usage: scripts/check-go-cli-semantic-seals.sh --self-test | --production --candidate-sha <sha>" >&2
  exit 2
fi

checker=(go run ./internal/archcheck/cmd)
classes=(
  tracker-execution
  effects
  output
  recursive-contracts
  generated-evidence
  evidence-binding
)
rules=(
  semantic.tracker-execution
  semantic.effects
  semantic.output
  semantic.recursive-contracts
  semantic.generated-evidence
  semantic.evidence-binding
)

cd "$REPO_ROOT/cli" || exit 1
for index in "${!classes[@]}"; do
  class="${classes[$index]}"
  fixture="$REPO_ROOT/tests/fixtures/go-cli-architecture/$class"
  extra=()
  if [[ "$class" == "evidence-binding" ]]; then
    extra=(--candidate-sha aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa)
  fi
  if ! "${checker[@]}" --root "$fixture/valid" "${extra[@]}"; then
    printf 'go-cli semantic seals FAIL: valid %s fixture was rejected\n' "$class" >&2
    exit 1
  fi
  if output="$("${checker[@]}" --root "$fixture/mutated" "${extra[@]}" 2>&1)"; then
    printf 'go-cli semantic seals FAIL: mutated %s fixture was accepted\n' "$class" >&2
    exit 1
  fi
  if [[ "$output" != *"${rules[$index]}"* ]]; then
    printf 'go-cli semantic seals FAIL: mutated %s fixture did not emit %s\n%s\n' \
      "$class" "${rules[$index]}" "$output" >&2
    exit 1
  fi
done

if ! "${checker[@]}" --self-test >/dev/null; then
  echo "go-cli semantic seals FAIL: context self-test was rejected" >&2
  exit 1
fi

printf 'go-cli semantic seals self-test PASS: context,%s\n' "$(IFS=,; echo "${classes[*]}")"
