#!/usr/bin/env python3
"""Emit more than the probe's retained-byte bound on both streams."""

import sys


sys.stdout.buffer.write(b"O" * 200_000)
sys.stderr.buffer.write(b"E" * 180_000)
