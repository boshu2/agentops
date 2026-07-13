# Learn Post-Verdict Bookkeeping

Learn owns bounded bookkeeping after Validate:

- preserve the verdict reference and digest;
- preserve every structured observation and its evidence reference;
- classify each observation as `record`, `candidate`, or `no_change`;
- emit `remaining_work` and a cited `plan_impact` disposition of
  `material_change`, `no_change`, or `terminal`;
- return a receipt and optional causal-analysis request to the orchestrator.

Learn does not promote rules, mutate the verdict, change the plan, invoke
Premortem, or operate delivery. The orchestrator consumes plan impact and owns
the next lifecycle transition.
