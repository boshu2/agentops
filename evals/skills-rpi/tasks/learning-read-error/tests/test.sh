#!/bin/sh
set -eu
mkdir -p /logs/verifier
printf '0\n' > /logs/verifier/reward.txt
python3 /tests/check_scope.py /baseline /app/work
# Use the pristine evaluator-owned tree, including its tests and module files.
# Only permitted production source bytes cross this boundary.
cp /app/work/internal/gates/checks/native_inline.go /baseline/internal/gates/checks/native_inline.go
cp /tests/oracle_test.go /baseline/internal/gates/checks/eval_oracle_test.go
cd /baseline
go test ./internal/gates/checks > /logs/verifier/go-test.log 2>&1
printf '1\n' > /logs/verifier/reward.txt
