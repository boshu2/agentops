#!/usr/bin/env bash
# validate-skill-triggers.sh — backs gate `skill.triggers`.
#
# Every skill description must carry trigger markers (a `Triggers:`/`Use when:`
# clause, a `description: |` block, or a metadata.triggers list). Skill selection
# is pure LLM reasoning over the `description` field, so a skill with no trigger
# phrase SILENTLY NEVER FIRES. The per-skill auditor only WARNs on this, so the
# gap accumulates (it reached 68% before this gate). Fail-closed on any offender.
#
# Source of the scan + suggested-stub remediation:
#   skills/skill-builder/scripts/scan_descriptions.py
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
exec python3 "$REPO_ROOT/skills/skill-builder/scripts/scan_descriptions.py" "$REPO_ROOT/skills" --strict
