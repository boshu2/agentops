# Plugin evals

These cases measure whether an agent with the AgentOps plugin installed behaves
differently from one without it. They run with Claude Code's own evaluator,
`claude plugin eval`. Results from the last full run are in
[the 2026-10-05 report](../../docs/evals/2026-10-05-plugin-eval-opus-5-5.md).

| Suite | Cases | Question | Arms |
|---|---|---|---|
| `behavior/` | 29, one per skill | Does the answer follow the practice the skill teaches? Each case has 4 or 5 criteria and also records whether the skill loaded. | with the plugin and without it |
| `routing/` | 25, one per skill the model can load by itself | Does the matching skill load on a request that never names it? | plugin only |

A case is one `case.yaml`: a request a user would type, the criteria, and a
`skill-loaded` check on the Skill tool call. The routing requests were written
by an agent that had not seen the skill descriptions, and were not used to tune
them. The behavior requests were.

The deterministic half of routing (which skill `ao skills find` ranks first)
lives in [`routing-probes/`](../routing-probes/README.md).

## Run

From the repository root:

```bash
# behavior: with the plugin and without it
claude plugin eval . --eval-dir evals/plugin-eval/behavior \
  --model claude-opus-5-5 --no-publish --json behavior.json

# routing: plugin only
claude plugin eval . --eval-dir evals/plugin-eval/routing \
  --ablation none --model claude-opus-5-5 --no-publish --json routing.json
```

Without `--no-publish` the evaluator uploads its HTML report, with every prompt
and response, to claude.ai. It writes run output to a results directory beside
the cases, which this repository ignores.

Every run is a full Claude Code session on your account. On Claude Opus 5.5 a
run cost about $0.13 to generate (168 runs for $21.25 on 2026-10-04). The
behavior suite is 174 runs at the default three per case; routing is 75.

## Grade

The evaluator grades each criterion with three judge calls. Two things went
wrong with that on this suite:

- The default judge (Haiku) failed criteria that responses plainly met. On the
  first run it failed every criterion of every `validate` response.
- With `--judge-model claude-opus-5-5` the grades were right, and grading cost
  several times more than generating the responses.

So the published numbers use `grade.py`: one Opus call per response, all of its
criteria at once, about $0.02 per response.

```bash
python3 evals/plugin-eval/grade.py behavior.json --out grades.json
```

On 103 responses graded both ways, `grade.py` agreed with the evaluator's
three-vote Opus judge on 439 of 458 criteria (95.9%). Where they differed,
`grade.py` was usually the stricter one (15 of 19). Whichever judge you use,
read a few graded responses before you trust a score.

The `skill-loaded` check is deterministic and needs no judge. In a two-arm run
the evaluator reports it beside the score; in a single-arm run it is the score.

## Reading a result

- **Score** is the share of criteria met, pooled over runs.
- **Check loading before you read a zero.** If the skill never loaded, a zero
  delta says nothing about the skill's content. Fix the description first.
- **One case per skill, three runs.** One run meeting or missing one criterion
  moves a five-criterion case by 0.07. Treat a delta under about 0.15 as noise.
- **The criteria come from each skill's own rules.** A pass shows the rule
  landed on that request. It does not show a better outcome on a real task.
- **Four skills are user-only** (`craft-goal`, `interview`, `postmortem`,
  `rpi`). Their cases invoke them by slash command, so the arm without the
  plugin sees an unknown command.

## Changing a case

- Write the request the way a user would type it. Never name the skill.
- Write each criterion as one statement a reader can check against the answer.
- Fix the criteria before you run. Do not edit a criterion to make a run pass.
- Do not copy a case's scenario into the skill it tests. A skill that quotes its
  own case makes the next result meaningless.

The same layout works for any plugin: `claude plugin eval init` scaffolds a
suite, and `grade.py` reads any run it writes.
