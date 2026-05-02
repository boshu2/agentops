#!/usr/bin/env python3
from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
SKILL_VALIDATE_CANDIDATES = [
    Path('/home/boful/.claude/plugins/cache/agentops-marketplace/agentops/2.38.0/skills/reverse-engineer-rpi/scripts/validate_feature_registry.py'),
    Path(__file__).resolve().parents[3] / "skills" / "reverse-engineer-rpi" / "scripts" / "validate_feature_registry.py",
    Path(__file__).resolve().parents[2] / "skills" / "reverse-engineer-rpi" / "scripts" / "validate_feature_registry.py",
    Path.cwd() / "skills" / "reverse-engineer-rpi" / "scripts" / "validate_feature_registry.py",
]

def _resolve_validator() -> Path:
    for cand in SKILL_VALIDATE_CANDIDATES:
        if cand.exists():
            return cand
    raise FileNotFoundError("Could not locate validate_feature_registry.py")

def main() -> int:
    # Delegate to the canonical validator, but default paths to this output dir.
    args = sys.argv[1:]
    if not args:
        root_path = HERE / "analysis-root-path.txt"
        local_root = (root_path.read_text(encoding="utf-8").strip() if root_path.exists() else str(HERE / "analysis-root"))
        args = [
            "--feature-registry", str(HERE / "feature-registry.yaml"),
            "--docs-features", str(HERE / "docs-features.txt"),
            "--local-clone-dir", local_root,
        ]
    validator = _resolve_validator()
    p = subprocess.run([sys.executable, str(validator), *args])
    return p.returncode

if __name__ == "__main__":
    raise SystemExit(main())
