#!/usr/bin/env bash
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"

# adapter.gc-maintainer — the live Gas City maintainer surface. The retired
# prototype (packs/agentops-executor, packs/agentops-factory, deploy/gc) is
# frozen historical bytes after the 2026-07-29 upstream-factories pivot and is
# deliberately not checked here.
maintainer="${GC_MAINTAINER_SCRIPT:-$REPO_ROOT/scripts/gc-maintainer-ops.sh}"
bash -n "$maintainer"
cd "$REPO_ROOT" || exit 2
python3 -m unittest tests.python.test_gc_maintainer_ops
