#!/usr/bin/env bash

# shellcheck source=scripts/lib/preamble.sh
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

if [[ $# -ne 1 || "$1" != "--self-test" ]]; then
  echo "usage: scripts/check-go-cli-semantic-seals.sh --self-test" >&2
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
  if ! "${checker[@]}" --root "$fixture/valid"; then
    printf 'go-cli semantic seals FAIL: valid %s fixture was rejected\n' "$class" >&2
    exit 1
  fi
  if output="$("${checker[@]}" --root "$fixture/mutated" 2>&1)"; then
    printf 'go-cli semantic seals FAIL: mutated %s fixture was accepted\n' "$class" >&2
    exit 1
  fi
  if [[ "$output" != *"${rules[$index]}"* ]]; then
    printf 'go-cli semantic seals FAIL: mutated %s fixture did not emit %s\n%s\n' \
      "$class" "${rules[$index]}" "$output" >&2
    exit 1
  fi
done

if ! "$REPO_ROOT/scripts/check-go-cli-architecture.sh" --self-test >/dev/null; then
  echo "go-cli semantic seals FAIL: context self-test was rejected" >&2
  exit 1
fi

printf 'go-cli semantic seals self-test PASS: context,%s\n' "$(IFS=,; echo "${classes[*]}")"
