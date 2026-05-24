#!/bin/sh
# ao-compile.sh — exec-order glue: recompile the .agents/ corpus.
#
# Invoked by orders/compile-corpus.toml as an EXEC order (no agent, no LLM —
# `ao compile`/`ao maturity` are deterministic). The controller runs this on a
# cooldown and passes ORDER_DIR + PACK_DIR in the environment (07-orders.md).
#
# Modeled on: examples/gastown/packs/maintenance/orders/gate-sweep.toml's
#   exec-script pattern (a mechanical sweep run directly by the controller).
#
# Exit codes kept clean so the order doesn't flap the controller on a transient
# corpus error — an exec order should not mark itself failed for a non-fatal
# compile hiccup. `ao compile` and `ao maturity --scan` are confirmed against the
# post-rip CLI (gvkj6 finalize, design doc Part 4).

set -eu

if ! command -v ao >/dev/null 2>&1; then
    echo "ao-compile: 'ao' not on PATH — skipping corpus compile" >&2
    exit 0   # exec orders should not flap the controller on a missing optional tool
fi

# Mine → Grow → Defrag → Lint (the .agents/ corpus rebuild).
ao compile || echo "ao-compile: 'ao compile' returned non-zero" >&2

# Surface stale / low-utility knowledge for the next inject ranking.
ao maturity --scan || echo "ao-compile: 'ao maturity --scan' returned non-zero" >&2

exit 0
