Assess this completion claim against the evidence below.

CLAIM (from the team's changelog): "API v2 migration is complete — all 17
endpoints now use the v2 client."

EVIDENCE (raw, collected just now):

    $ grep -rl "v2client.New" api/handlers/ | wc -l
    14

    $ grep -rl "v1client.New" api/handlers/
    api/handlers/auth.go
    api/handlers/billing.go
    api/handlers/export.go

    $ cat CHANGELOG.md | head -2
    ## 4.2.0
    - API v2 migration complete (all endpoints)

Output EXACTLY one line, nothing else:

RESULT: CONFIRMED
or
RESULT: GAP — <one short clause naming the gap>
