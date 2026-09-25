#!/bin/sh
set -eu
. "$1/fixture.env"
python3 - "$1" <<'PYCODE'
import json, pathlib, subprocess, sys
r=pathlib.Path(sys.argv[1]);args=json.loads((r/'read-args.json').read_text())
allowed=subprocess.run(['ao',*args],capture_output=True)
if allowed.returncode:raise SystemExit(allowed.stderr.decode())
(r/'out/allowed.json').write_bytes(allowed.stdout)
denied=args.copy();denied[denied.index('--destination-ref')+1]='denied-destination'
result=subprocess.run(['ao',*denied],capture_output=True)
(r/'out/denied.json').write_bytes(result.stdout);(r/'out/denied.err').write_bytes(result.stderr)
(r/'out/denied.rc').write_text(str(result.returncode)+'\n')
if result.returncode==0:raise SystemExit('denied destination unexpectedly accepted')
required=r/'missing-resource/references/RAW_SOURCE_READS.md'
try:required.read_text()
except OSError:(r/'out/missing-resource.txt').write_text('STOP_REQUIRED_RESOURCE\n')
else:raise SystemExit('fixture requires an absent resource')
observed=json.loads(allowed.stdout)
(r/'out/summary.json').write_text(json.dumps({'allowed_complete_reading':observed['complete_reading'],'allowed_host_delivery':observed['host_delivery'],'denied_destination':'DENIED','missing_resource':'STOP_REQUIRED_RESOURCE'})+'\n')
PYCODE
