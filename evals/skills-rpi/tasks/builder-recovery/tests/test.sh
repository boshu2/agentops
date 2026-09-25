#!/bin/sh
set -eu
python3 /tests/verify.py /baseline /app/work /tests /logs/verifier
