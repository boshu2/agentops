#!/usr/bin/env bash
# check-verdict-contract-corpus.sh — cross-language verdict.v2 contract gate.
#
# Runs the shared golden corpus (tests/fixtures/verdict-contract/cases/)
# through all three implementations of the contract:
#   1. the JSON schema (schemas/verdict.v2.schema.json, via python jsonschema);
#   2. the Python Validate writer/validator (skills/validate/scripts/validate.py);
#   3. the Go evidence reader (cli/internal/verdictcheck).
# A case any implementation judges differently from the corpus expectation is
# a contract fork and fails this check.
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

if ! command -v python3 >/dev/null 2>&1; then
  echo "check-verdict-contract-corpus: SKIP — python3 unavailable"
  exit 0
fi

python3 "$REPO_ROOT/skills/validate/scripts/check_contract_corpus.py"

if command -v go >/dev/null 2>&1; then
  (cd "$REPO_ROOT/cli" && go test ./internal/verdictcheck -run TestGoldenCorpus -count=1)
else
  echo "check-verdict-contract-corpus: WARN — go unavailable, Go leg skipped"
fi

echo "check-verdict-contract-corpus: PASS"
