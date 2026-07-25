#!/usr/bin/env python3
"""Mutate only the disposable copy so isolation evidence can observe it."""

from pathlib import Path


Path("skills/skill-builder/ISOLATION-MUTATION").write_text("isolated\n", encoding="utf-8")
