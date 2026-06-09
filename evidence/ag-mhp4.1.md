# ag-mhp4.1 Evidence

## Scope

Verdict and council gates now require a typed judge identity:

- `author`
- `judge`
- `judge_program`
- `judge_model_family`

The council gate fails closed unless all binding PASS verdicts are from distinct
judges and at least two model families. A verdict-authored self-judge waiver is
rejected; any self-judge exception must be external and principal-logged before
this gate can trust it.

## Admission

- Athena/HazySummit, Claude-family: APPROVE with C1-C5.
- Gemini/WindyElm, Gemini-family: APPROVE with Athena C1-C5 standing.
- Codex is author and excluded as judge.
- pr/QuietForge recorded that the cross-family quorum is the substantive green
  gate and Bo directed "send codex the GO".

Coordination thread: control-plane AM thread 552.

## Commands Run

```bash
cd /Users/bo/dev/agentops/cli
go test ./cmd/ao -run 'TestTickVerdictIdentityRequiresTypedIndependentJudge|TestTickCouncilGateMatrix|TestTickCouncilGateRejectsSameFamilyAndSelfJudgeQuorum' -count=1 -v
```

Result: PASS.

```bash
cd /Users/bo/dev/agentops/cli
go test ./cmd/ao -run 'TestTickCommandSurfaceCovered|TestTickSmoke' -count=1 -v
```

Result: PASS for the matched command-surface test.

```bash
cd /Users/bo/dev/agentops/cli
go test ./cmd/ao -run 'TestSkillContract_AllSKILLMDHaveDescription/security-suite' -count=1 -v
```

Result: FAIL, unrelated pre-existing contract failure:
`skills/security-suite/SKILL.md` is missing.

## Acceptance Map

- C1 shared contract: implemented in `tickVerdictIdentity` using a typed judge
  tuple and model-family quorum checks.
- C2 no self-grantable waiver: verdict-authored `allow_self` or self-waiver
  metadata is rejected.
- C3 red-first proof: tests cover self-judge, same-family quorum, duplicate
  judge, unknown family, and missing judge program as fail-closed cases.
- C4 no close before C3: focused tests pass before this evidence was written.
- C5 hold later slices: no `ag-mhp4.2` through `ag-mhp4.5` implementation was
  started.
