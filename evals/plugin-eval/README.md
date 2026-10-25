# Plugin evals

These cases measure whether an agent with the AgentOps plugin installed behaves
differently from one without it. They run with Claude Code's own evaluator,
`claude plugin eval`. Results from the last full run are in
[the 2026-10-22 report](../../docs/evals/2026-10-22-plugin-eval-opus-5-5.md);
the [2026-10-05 report](../../docs/evals/2026-10-05-plugin-eval-opus-5-5.md)
records 3.9.0 against 3.10.

| Suite | Cases | Question | Arms |
|---|---|---|---|
| `behavior/` | 32, covering 31 skills | Does the answer follow the practice the skill teaches? Each case has 4 or 5 criteria and also records whether the skill loaded. | with the plugin and without it |
| `routing/` | 32, covering 31 skills | Does the matching skill load on a request a user would type? | plugin only |

Research has two cases in each suite: `research` and `research-claim-check`,
the case Reality Check had before it folded into Research on 2026-10-23. Its
prompt and criteria are unchanged; its `skill-loaded` check now expects
Research.

A case is one `case.yaml`: a request a user would type, the criteria, and a
`skill-loaded` check on the Skill tool call. Most routing requests never name
the skill; they were written by an agent that had not seen the skill
descriptions, and were not used to tune them. The behavior requests were.
The `rpi`, `postmortem`, `interview` and `craft-goal` routing cases are the
exception: those skills load when the user asks for them by name, so their
requests name them in plain words ("run this through RPI", "interview me").
The author of those descriptions wrote these four requests in the same change,
so they check that naming the skill loads it; they do not measure blind
routing.

The deterministic half of routing (which skill `ao skills find` ranks first)
lives in [`routing-probes/`](../routing-probes/README.md).

## Run

From the repository root:

```bash
OUT=$(mktemp -d)   # keep run output out of the repository

# behavior: with the plugin and without it
claude plugin eval . --eval-dir evals/plugin-eval/behavior \
  --model claude-opus-5-5 --no-publish --json "$OUT/behavior.json"

# routing: plugin only
claude plugin eval . --eval-dir evals/plugin-eval/routing \
  --ablation none --model claude-opus-5-5 --no-publish --json "$OUT/routing.json"
```

The JSON files hold every prompt and response, so write them outside the
repository as shown. Without `--no-publish` the evaluator also uploads its HTML
report to claude.ai. Its own run directory sits beside the cases and is ignored
by Git.

Every run is a full Claude Code session on your account. On Claude Opus 5.5 a
run cost about $0.13 to generate (168 runs for $21.25 on 2026-10-04). The
behavior suite is 192 runs at the default three per case; routing is 96.

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
python3 evals/plugin-eval/grade.py "$OUT/behavior.json" --out "$OUT/grades.json"
```

A run that errored or returned no text cannot be graded. `grade.py` lists each
one and exits nonzero, so a score is never computed over fewer runs unnoticed.

On 101 responses graded both ways, `grade.py` agreed with the evaluator's
three-vote Opus judge on 439 of 458 criteria (95.9%). Where they differed,
`grade.py` was usually the stricter one (15 of 19). The counts are in the
`grader_agreement` block of the 2026-10-05 scorecard. Whichever judge you use,
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
- **Four behavior cases use a slash command** (`craft-goal`, `interview`,
  `postmortem`, `rpi`). These skills were user-only until 2026-10-23 and now
  also load when named in plain words; their behavior cases still start with
  `/agentops:<name>`, so the arm without the plugin sees an unknown command.

## Changing a case

- Write the request the way a user would type it. Never name the skill,
  except in a routing case for a skill whose description says to load it when
  named.
- Write each criterion as one statement a reader can check against the answer.
- Fix the criteria before you run. Do not edit a criterion to make a run pass.
- Do not copy a case's scenario into the skill it tests. A skill that quotes its
  own case makes the next result meaningless.

The same layout works for any plugin: `claude plugin eval init` scaffolds a
suite, and `grade.py` reads any run it writes.
