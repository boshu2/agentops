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
#
# FAIL-CLOSED: a PASS from this gate must prove all THREE legs actually ran —
# the JSON schema, the Python validator, and the Go reader. A missing tool is a
# hard failure, never a silent skip that still prints PASS. (Previously a
# missing python3 exited 0 and a missing go printed WARN+PASS, so a green here
# could mean "nothing checked".)
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

# require_cmd (from preamble.sh) prints a clear error and exits non-zero when the
# command is absent — turning each absent leg into a fail-closed error.
require_cmd python3
require_cmd go

# CONTRACT_CORPUS_REQUIRE_SCHEMA=1 makes the Python harness fail (not skip) when
# jsonschema is unavailable, so the schema leg is proven to have run too.
CONTRACT_CORPUS_REQUIRE_SCHEMA=1 python3 "$REPO_ROOT/skills/validate/scripts/check_contract_corpus.py"

(cd "$REPO_ROOT/cli" && go test ./internal/verdictcheck -run TestGoldenCorpus -count=1)

echo "check-verdict-contract-corpus: PASS (schema + python + go all ran)"
