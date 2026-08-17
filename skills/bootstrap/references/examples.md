# Bootstrap Examples

Bootstrap accepts an explicit target and requested artifacts. It preserves
every existing file and never starts another skill or runtime automatically.

## New repository

**Caller asks:** Initialize AgentOps documentation in `/work/widget` with a
PRODUCT document, GOALS document, AGENTS router, and local verdict storage.

Bootstrap inspects those paths, asks only for product or goal content that is
not supplied, then creates the missing files plus
`.agents/ao/verdicts/sha256/`. It reports the exact created and existing paths.
The output separately records `verdict_storage_requested: true` and classifies
the verdict directory as created, existing-and-untouched, or failed.

## Partial repository

**Caller asks:** Add the missing AgentOps entry documents to `/work/widget`.

If `PRODUCT.md` and `README.md` already exist, Bootstrap leaves them byte-for-
byte unchanged. It creates only explicitly requested missing files such as
`GOALS.md` or `AGENTS.md`, then reports created and skipped paths separately.

## Inspection only

**Caller asks:** Show what Bootstrap would need to create in `/work/widget`;
do not write anything.

Bootstrap reports which requested paths exist and which are missing. It does
not create directories, invoke another skill, initialize Git, install hooks,
or infer permission to write.

## Docs-only request

**Caller asks:** Create the missing `AGENTS.md`; do not initialize verdict
storage.

Bootstrap creates only `AGENTS.md`. Its effect report says
`verdict_storage_requested: false`, includes no `.agents/ao/**` path, and lists
the single document under created or failed.
