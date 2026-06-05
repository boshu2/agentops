#!/usr/bin/env bash
# check-author-judge-convergence.sh — guard the no-self-grade convergence.
#
# DRIFT #149 / the 2nd-occurrence gate: the empty-ID self-grade bypass recurred
# (#737 PG3/PG4 -> #741 citation reward) because the author!=judge invariant was
# reimplemented in divergent places instead of reusing one predicate. #744 + #741
# converged the three sites onto cli/internal/liveness Disjoint. This guard keeps
# it converged: the ONLY author!=judge equality logic may live in liveness.Disjoint
# (cli/internal/liveness/guards.go). Any other Go site doing its own
# `author == judge` / `EqualFold(author, judge)` / `judge_id == author_id`
# equality is a divergent reimplementation and must call liveness.Disjoint instead.
#
# Comments and *_test.go files are not flagged (a comment may describe the
# invariant; tests assert it). Exit non-zero on any divergent site.
#
# Usage: check-author-judge-convergence.sh [ROOT]   (ROOT defaults to "cli")
set -euo pipefail

ROOT="${1:-cli}"

# author<->grader identity equality in CODE, in EITHER operand order. The grader
# side is "judge" (verdict) or "cited" (citation reward); the author!=judge
# invariant covers both author/judge and author/cited pairs. Identifiers are matched
# by *containing* the keyword (e.g. artifactAuthor, citedByAgent), and the equality
# may be written in either direction (artifactAuthor == citedByAgent OR
# citedByAgent == artifactAuthor) — both are the same self-grade predicate.
AUTH='[A-Za-z]*[Aa]uthor[A-Za-z]*'
GRADER='[A-Za-z]*([Jj]udge|[Cc]ited)[A-Za-z]*'
PATTERN="(EqualFold\\([^)]*([Aa]uthor|[Jj]udge|[Cc]ited)|${AUTH}[[:space:]]*==[[:space:]]*${GRADER}|${GRADER}[[:space:]]*==[[:space:]]*${AUTH})"

hits="$(grep -rnE "$PATTERN" "$ROOT" --include='*.go' 2>/dev/null \
  | grep -viE '_test\.go' \
  | grep -vE ':[0-9]+:[[:space:]]*//' \
  | grep -vE '(^|/)internal/liveness/guards\.go:' \
  || true)"

if [ -n "$hits" ]; then
  echo "FAIL: divergent author!=judge equality found outside liveness.Disjoint:" >&2
  echo "$hits" >&2
  echo >&2
  echo "Route the no-self-grade decision through liveness.Disjoint (cli/internal/liveness/guards.go)." >&2
  echo "Reimplementing author!=judge is the DRIFT #149 pattern that let the empty-ID bypass recur (#737->#741)." >&2
  exit 1
fi

echo "ok: author!=judge convergence — all sites route through liveness.Disjoint"
