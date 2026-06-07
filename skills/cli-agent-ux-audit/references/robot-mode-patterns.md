# Robot-Mode Patterns — concrete implementations per language & surface

Apply these in Phase 3. Keep the human-readable default; add the machine surface alongside it.

## The universal rules

1. **stdout = data, stderr = everything else** (logs, progress, prompts, warnings). An agent reads
   stdout and parses it; it must never contain a spinner or a log line.
2. **Detect non-interactive and degrade gracefully:** if `!isatty(stdout)` or `NO_COLOR` is set, drop
   color and box-drawing automatically — the agent gets clean output even without an explicit flag.
3. **`--json` is one document; `--ndjson` is one object per line, flushed per event.** Pick `--ndjson`
   for anything streaming or unbounded so the agent can act on each event without waiting for the end.
4. **Errors are data in machine mode:** `{"error":{"code":"not_found","message":"...","hint":"..."}}`
   on stdout, plus the matching exit code — not only prose on stderr.

## Go

```go
// Detect machine mode early; one writer to rule them all.
type Result struct {
    Status  string  `json:"status"`
    Count   int     `json:"count"`
    Items   []Item  `json:"items"`
}

if *jsonOut {
    enc := json.NewEncoder(os.Stdout) // data → stdout
    enc.SetEscapeHTML(false)
    if err := enc.Encode(res); err != nil { os.Exit(1) }
    return
}
// human default below; color only if isTTY && os.Getenv("NO_COLOR") == ""
```

NDJSON streaming: encode one object per event and `os.Stdout.Sync()` (or flush a `bufio.Writer`) so
the agent sees each line immediately.

Exit codes: define `const (ExitOK=0; ExitGeneric=1; ExitUsage=2; ExitNotFound=3; ExitPrecond=4)`
and `os.Exit(ExitUsage)` on flag-parse errors instead of falling through to 1.

## Python

```python
import json, sys
parser = argparse.ArgumentParser()   # argparse already exits 2 on bad args — do NOT override
parser.add_argument("--json", action="store_true")

def emit(result, args):
    if args.json:
        json.dump(result, sys.stdout)   # data → stdout
        sys.stdout.write("\n")
    else:
        render_human(result)            # color/tables here only

# NDJSON streaming:
for event in stream():
    sys.stdout.write(json.dumps(event) + "\n")
    sys.stdout.flush()

# Named exit codes:
class Exit:  OK=0; GENERIC=1; USAGE=2; NOT_FOUND=3; PRECOND=4
sys.exit(Exit.PRECOND)   # e.g. missing credentials
```

Send all logging to stderr: `logging.basicConfig(stream=sys.stderr)`.

## Node / TypeScript

```js
const machine = argv.json || argv.ndjson || !process.stdout.isTTY;

if (argv.json) process.stdout.write(JSON.stringify(result) + "\n");      // data → stdout
else renderHuman(result);                                                 // color only if isTTY

// NDJSON streaming — write + implicit flush per line:
for await (const ev of stream()) process.stdout.write(JSON.stringify(ev) + "\n");

console.error(`[info] connecting…`);   // logs → stderr, never stdout

// Confirmation without a hang:
if (destructive && !argv.force && !argv.dryRun) {
  if (!process.stdin.isTTY) { fail(2, "refusing destructive op without --force"); }
  // only prompt interactively when a human is actually attached
}
process.exitCode = NOT_FOUND;   // 3
```

## Bash / POSIX wrappers

```bash
JSON=0; case "${1:-}" in --json) JSON=1; shift;; esac
[ -t 1 ] || NO_COLOR=1            # auto-plain when piped
if [ "$JSON" = 1 ]; then
  printf '{"status":"ok","count":%d}\n' "$n"   # data → stdout
else
  printf '\033[32m✓\033[0m %d items\n' "$n"     # human
fi
exit 0   # use 2 for usage errors, 3 for not-found, etc.
```

## `--help` as the contract

Make `--help` complete and greppable. Minimum it must contain:

- one-line synopsis per subcommand and the global usage line;
- every flag with its type/accepted values and default;
- the **machine modes** (`--json`/`--ndjson`) listed explicitly;
- the **exit-code table** (so the agent learns the branch points from help alone);
- at least one copy-pasteable example per primary command.

Generated help (cobra/clap/argparse): edit the source annotations/templates, never the generated text.
