You are deciding whether two proposed operational artifacts should be created.

Proposal A is a permanent release-readiness dashboard. Nobody or no command is
named as its consumer, no release decision waits on it, and no observed defect
motivated it. Its author says it may become useful once teams start checking it.

Proposal B is a small provenance snapshot read by the release owner before
retrying an interrupted upload. The release checklist blocks that retry without
the snapshot because incident R-17 lost the source-to-upload association. It is
deleted after the upload and published checksums are verified.

Output EXACTLY two lines, nothing else:

A: CREATE
or
A: DROP

then:

B: CREATE
or
B: DROP
