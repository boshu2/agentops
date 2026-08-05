#!/usr/bin/env python3
"""Instantiate routing-probe templates with fresh surface strings.

Fixture-isolation rule (batch 1, 2026-08-05): committed scenario files leak
into repo-searching agents, so dispatchable strings must never exist on disk
before the run. This script renders templates.json with seeded-random values;
the seed is recorded in the output so a batch is reproducible AFTER the fact.

Usage: python3 instantiate.py --seed N [--id rt-06-conventions]
Prints one JSON object per scenario: {id, seed, prompt, applicable}.
Stdlib only (ADR-0016 spirit: this is evals tooling, not a shipped skill script).
"""
import argparse, json, random, sys, os

POOLS = {
    "codename": ["item-K", "item-M", "item-P", "item-Q", "item-R", "item-S", "item-T", "item-V"],
    "shared_file": ["cli/internal/report/merge.go", "cli/internal/index/writer.go",
                    "cli/internal/sync/state.go", "cli/internal/export/render.go"],
    "feature": ["cursor-pagination", "delta-export", "session-pinning", "quota-ledger",
                "trace-stitching", "cold-cache-warmup"],
    "change": ["the new commit-lint gate", "the workspace quota check", "the stale-branch sweeper",
               "the artifact checksum step"],
    "small_change": ["retry-backoff", "cursor-encoding", "path-normalization", "digest-caching"],
    "file": ["server/cursor.go", "client/backoff.go", "internal/paths/norm.go", "store/digest.go"],
    "weekday_pairs": [("Tuesday", "Thursday"), ("Wednesday", "Friday"), ("Thursday", "Monday")],
    "lang": ["Go", "Python", "TypeScript"],
    "handler_kind": ["upload", "download", "export"],
    "knob": ["sweep-interval", "batch-ceiling", "drain-window", "probe-period"],
    "range_desc": ["integer seconds 5..3600", "integer 1..500", "integer minutes 1..1440"],
}

def render(t, rng):
    p = t["prompt"]
    if t["id"] == "rt-01-parallel-safety":
        a, b, c, d = rng.sample(POOLS["codename"], 4)
        p = (p.replace("{{ITEM_A}}", a).replace("{{ITEM_B}}", b)
               .replace("{{ITEM_C}}", c).replace("{{ITEM_D}}", d)
               .replace("{{SHARED_FILE}}", rng.choice(POOLS["shared_file"])))
    elif t["id"] == "rt-02-claim-audit":
        p = p.replace("{{FEATURE}}", rng.choice(POOLS["feature"]))
    elif t["id"] == "rt-03-plan-challenge":
        d1, d2 = rng.choice(POOLS["weekday_pairs"])
        p = (p.replace("{{CHANGE}}", rng.choice(POOLS["change"]))
               .replace("{{DAY_1}}", d1).replace("{{DAY_2}}", d2))
    elif t["id"] == "rt-04-verdict-request":
        p = (p.replace("{{CHANGE}}", rng.choice(POOLS["small_change"]))
               .replace("{{FILE}}", rng.choice(POOLS["file"])))
    elif t["id"] == "rt-05-secure-review":
        p = (p.replace("{{LANG}}", rng.choice(POOLS["lang"]))
               .replace("{{HANDLER_KIND}}", rng.choice(POOLS["handler_kind"])))
    elif t["id"] == "rt-06-conventions":
        p = (p.replace("{{KNOB}}", rng.choice(POOLS["knob"]))
               .replace("{{RANGE_DESC}}", rng.choice(POOLS["range_desc"])))
    if "{{" in p:
        raise SystemExit(f"unrendered placeholder in {t['id']}: {p}")
    return p

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--seed", type=int, required=True)
    ap.add_argument("--id", default=None)
    args = ap.parse_args()
    data = json.load(open(os.path.join(os.path.dirname(__file__), "templates.json")))
    for t in data["templates"]:
        if args.id and t["id"] != args.id:
            continue
        rng = random.Random(f"{args.seed}:{t['id']}")
        print(json.dumps({"id": t["id"], "seed": args.seed,
                          "prompt": render(t, rng), "applicable": t["applicable"]}))

if __name__ == "__main__":
    main()
