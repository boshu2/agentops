#!/usr/bin/env python3
"""Exit successfully while a descendant keeps the process group alive."""

import subprocess
import sys


child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(60)"])
print(child.pid, flush=True)
