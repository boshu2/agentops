#!/bin/sh
# install-ao.sh — pre_start glue: ensure the `ao` runtime is installed into a
# spawned agent's working directory.
#
# Usage: install-ao.sh <work-dir>
#
# Called from the refinery/mayor agent.toml pre_start (config.md:146), which runs
# on the target filesystem BEFORE the session is created — so the agent starts
# with `ao` on PATH and the .claude/skills + hooks + .agents scaffolding present.
#
# THIN-SEAM / provisioning (design doc Part 3): GC has NO skill primitive
# (`skills`/`mcp` are tombstones, config.md:209-212). AgentOps self-provisions its
# runtime: this script handles the `ao` BINARY + scaffolding; the static skill
# corpus is copied separately by the agent's `overlay_dir` (overlay/.claude/).
#
# FUNCTIONAL STUB — the real install path is finalized post-rip (gvkj6-finalize).
# TODO(gvkj6-finalize): pin the install source (release tarball vs curl|bash vs
# `ao install`) once the lean post-rip skills/ set and the `ao` release artifact
# are settled. The design doc (Part 5, gap #1) leans toward a PINNED snapshot in
# the overlay + a versioned `ao` binary, not pull-latest at spawn.

set -eu

WORK_DIR="${1:?usage: install-ao.sh <work-dir>}"

# 1) Ensure the `ao` binary is on PATH. If absent, install it (idempotent).
if ! command -v ao >/dev/null 2>&1; then
    echo "install-ao: 'ao' not found on PATH" >&2
    # TODO(gvkj6-finalize): replace with the pinned install path, e.g.:
    #   curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh | bash
    # or a vendored release binary copied from the pack assets. Until then this
    # is a loud no-op so a misconfigured city fails visibly rather than silently.
    echo "install-ao: STUB — no install source pinned yet (gvkj6-finalize)" >&2
fi

# 2) Scaffold the AgentOps runtime into the working dir (idempotent). The overlay
#    already copies .claude/skills/** and .claude/settings.json; this ensures the
#    .agents/ corpus scaffolding exists so `ao inject`/`ao compile` have a home.
mkdir -p "$WORK_DIR/.agents"
if command -v ao >/dev/null 2>&1; then
    # `ao install` is expected to be idempotent (config.md surface / design doc
    # Part 4 "ao install / install.sh — idempotent install of skills/hooks").
    ao install --target "$WORK_DIR" 2>/dev/null \
        || echo "install-ao: 'ao install' unavailable or failed (stub-tolerant)" >&2
fi

exit 0
