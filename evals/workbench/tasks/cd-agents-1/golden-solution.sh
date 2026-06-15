#!/usr/bin/env bash
set -euo pipefail
file="${1:?Usage: check-no-runtime-agents.sh <changed-files.txt>}"
bad=0
while IFS= read -r path; do
  case "$path" in
    .agents/rpi/*|.agents/yield/*|.agents/swarm/*|.agents/ao/*|.agents/handoff/*)
      echo "runtime .agents artifact: $path"
      bad=1
      ;;
  esac
done < "$file"
exit "$bad"
