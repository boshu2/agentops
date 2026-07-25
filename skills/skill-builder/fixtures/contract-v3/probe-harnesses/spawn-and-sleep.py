#!/usr/bin/env python3
"""Leave a sleeping descendant while the parent exceeds its timeout."""

import subprocess
import signal
import sys
import time


signal.signal(signal.SIGTERM, signal.SIG_IGN)
child = subprocess.Popen(
    [
        sys.executable,
        "-c",
        "import signal,time; signal.signal(signal.SIGTERM, signal.SIG_IGN); time.sleep(60)",
    ]
)
print(child.pid, flush=True)
time.sleep(60)
