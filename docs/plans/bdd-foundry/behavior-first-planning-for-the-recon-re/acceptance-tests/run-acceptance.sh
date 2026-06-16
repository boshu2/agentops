#!/usr/bin/env bash
# run-acceptance.sh — run the WHOLE recon-recommended-work acceptance suite.
#
# Phase 2 (ATDD) of bdd-foundry: every frozen scenario in behaviors.md as a
# runnable test. TEST-FIRST — these are RED until the features land (except the
# explicit guardrail scenarios noted in acceptance-tests.md).
#
# One-line invocation (from anywhere):
#   bash docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/run-acceptance.sh
#
# Exit code: 0 only if every test passes (i.e. once the whole feature set ships).
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

exec bats \
  "$HERE/stream-a-codebase-recon.bats" \
  "$HERE/stream-b-recon-actions.bats" \
  "$HERE/stream-b-go.bats"
