"""Evaluator-side source scope check; candidate files are untrusted data."""
import pathlib
import sys

baseline, candidate = map(pathlib.Path, sys.argv[1:])
allowed = "internal/gates/checks/native_inline.go"

def files(root):
    result = {}
    for path in root.rglob("*"):
        relative = path.relative_to(root).as_posix()
        if relative == ".git" or relative.startswith(".git/"):
            continue
        if path.is_symlink():
            raise SystemExit("symlink in candidate subject: " + relative)
        if path.is_file():
            result[relative] = path.read_bytes()
    return result

before, after = files(baseline), files(candidate)
for path in sorted(before.keys() | after.keys()):
    if before.get(path) == after.get(path):
        continue
    if path == allowed or (path.startswith("internal/gates/checks/") and path.endswith("_test.go")):
        continue
    raise SystemExit("out-of-scope source change: " + path)
if allowed not in after:
    raise SystemExit("required source is missing")
