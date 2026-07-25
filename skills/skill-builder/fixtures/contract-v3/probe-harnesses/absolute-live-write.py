#!/usr/bin/env python3
"""Attempt an inherited absolute live-root write before emitting a marker."""

from pathlib import Path
import sys


Path(sys.argv[1]).write_text("escaped\n", encoding="utf-8")
print("COMMAND_CONTINUED_AFTER_DENIED_WRITE", flush=True)
