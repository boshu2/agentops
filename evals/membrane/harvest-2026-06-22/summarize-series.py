#!/usr/bin/env python3
"""summarize-series.py <series.jsonl>

Turn the accruing escape-rate series into E5-actionable SPC signal. READ-ONLY
analysis -- NOT the governor build (age-wy3 is held); this computes the control
chart the governor would later watch, so a human can see the membrane's
escape-rate process and whether it is in statistical control.

Input: one JSON object per line (run-harvest-series.sh rows), each with
  membrane_miss_rate (n_missed/n_false_dones) and catch_rate (n_caught/n_false_dones).
Output: a compact summary -- n, mean, sample stddev, 3-sigma control limits for
the membrane miss rate (the metric E5 governs), and any out-of-control points
(beyond the limits). Rows with a null miss_rate (no false-dones that run) are
excluded from the rate stats but counted.

Honest stats discipline: with n<8 the control limits are NOT trustworthy (SPC
needs ~8-10 subgroups); the tool says so rather than implying a calibrated chart.
"""
import sys, json, math


def load(path):
    rows = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rows.append(json.loads(line))
    return rows


def stats(xs):
    n = len(xs)
    if n == 0:
        return None
    mean = sum(xs) / n
    # sample stddev (n-1); descriptive only -- 0 when n<2.
    var = sum((x - mean) ** 2 for x in xs) / (n - 1) if n > 1 else 0.0
    sd = math.sqrt(var)
    # I-MR control-limit sigma: estimated from the average MOVING RANGE, not the
    # in-sample stddev. This is the standard for individuals data and is robust
    # to a single contaminating outlier (which appears in only two moving ranges,
    # vs inflating the whole-sample stddev so the 3-sigma band swallows it). d2
    # for MR of size 2 = 1.128.
    if n > 1:
        mrs = [abs(xs[i] - xs[i - 1]) for i in range(1, n)]
        mr_bar = sum(mrs) / len(mrs)
        sigma_imr = mr_bar / 1.128
    else:
        mr_bar = 0.0
        sigma_imr = 0.0
    return {"n": n, "mean": mean, "sd": sd, "min": min(xs), "max": max(xs),
            "mr_bar": mr_bar, "sigma_imr": sigma_imr}


def main():
    if len(sys.argv) < 2:
        print("usage: summarize-series.py <series.jsonl>", file=sys.stderr)
        return 2
    rows = load(sys.argv[1])
    # Degraded runs (a task lost to a producer/membrane stall) change the false-done
    # denominator and may drop the systematically-missed escape, so their per-run
    # miss rate is NOT comparable to a clean run -- it can move EITHER way (up if a
    # caught task is lost, down if the missed one is). So they must NOT enter the
    # I-MR control chart. The PRIMARY metric is over CLEAN runs (degraded==0);
    # degraded runs are reported separately, flagged as non-comparable.
    clean = [r for r in rows if int(r.get("degraded", 0)) == 0]
    degraded_rows = [r for r in rows if int(r.get("degraded", 0)) != 0]
    miss = [r["membrane_miss_rate"] for r in clean if r.get("membrane_miss_rate") is not None]
    catch = [r["catch_rate"] for r in clean if r.get("catch_rate") is not None]
    # Pooled rate over CLEAN runs (false-done-weighted, the robust aggregate).
    total_fd = sum(int(r.get("n_false_dones", 0)) for r in clean)
    total_missed = sum(int(r.get("n_missed", 0)) for r in clean)

    print(f"escape-rate series -- {len(rows)} run(s): {len(clean)} clean (degraded=0), "
          f"{len(degraded_rows)} degraded (excluded -- non-comparable: a dropped task changes the denominator "
          f"and may remove the missed escape, moving the rate EITHER way)")
    if degraded_rows:
        print("  degraded runs (NOT used for the metric; shown for transparency): " +
              ", ".join(f"{r.get('run_id')}={r.get('membrane_miss_rate')}" for r in degraded_rows))
    print(f"  pooled over CLEAN runs: {total_missed} miss(es) / {total_fd} false-done(s) "
          f"= {total_missed/total_fd:.3f} pooled miss rate" if total_fd else "  pooled: no clean false-dones")

    ms = stats(miss)
    if ms:
        # I-MR control limits: mean +/- 3*sigma_imr (moving-range estimate), so a
        # single spike does NOT inflate its own band the way in-sample stddev would.
        ucl = ms["mean"] + 3 * ms["sigma_imr"]
        lcl = max(0.0, ms["mean"] - 3 * ms["sigma_imr"])
        print(f"\n  membrane miss rate (E5's governed metric):")
        print(f"    n={ms['n']}  mean={ms['mean']:.3f}  sd={ms['sd']:.3f}  range=[{ms['min']:.3f}, {ms['max']:.3f}]")
        print(f"    I-MR limits (sigma from avg moving range {ms['mr_bar']:.3f}/1.128={ms['sigma_imr']:.3f}): "
              f"LCL={lcl:.3f}  CL={ms['mean']:.3f}  UCL={ucl:.3f}")
        # OOC is tested ONLY over the clean points the limits were derived from --
        # never the excluded degraded rows (flagging a row we deliberately dropped
        # as "out of control" is nonsense).
        if ms["sigma_imr"] == 0 or ms["n"] < 2:
            print(f"    control limits DEGENERATE (sigma=0: {ms['n']} clean point(s), no variation) -- "
                  f"OOC detection not meaningful yet; need more clean (degraded=0) runs.")
        else:
            ooc = [(r.get("run_id"), r["membrane_miss_rate"]) for r in clean
                   if r.get("membrane_miss_rate") is not None and (r["membrane_miss_rate"] > ucl or r["membrane_miss_rate"] < lcl)]
            if ooc:
                print(f"    OUT-OF-CONTROL clean points (governor would flag HARDEN): {ooc}")
            elif ms["n"] < 8:
                # Do NOT claim "in statistical control" in the provisional range --
                # the limits themselves aren't trustworthy yet (n<8).
                print(f"    all clean points within the PROVISIONAL limits "
                      f"(n={ms['n']}<8 -- limits not yet trustworthy; no in-control claim)")
            else:
                # Honest scope: this is an individual-point (I) check only. A full
                # in-control certification also tests the MR chart + runs/trend rules
                # (Western Electric / Nelson) -- which this tool does NOT implement.
                print(f"    all clean points within I-MR limits (individual-point check only -- does NOT "
                      f"test moving-range / runs / trend rules; not a full in-control certification)")
        if ms["n"] < 8:
            print(f"    ! n={ms['n']} clean < 8 -- control limits NOT yet trustworthy (SPC needs ~8-10 subgroups); "
                  f"provisional. Note: {len(degraded_rows)}/{len(rows)} runs were degraded (stalls) -- "
                  f"the harness's stall rate (age-9h3d) is the binding constraint on clean volume.")

    cs = stats(catch)
    if cs:
        print(f"\n  cross-family catch rate: n={cs['n']}  mean={cs['mean']:.3f}  range=[{cs['min']:.3f}, {cs['max']:.3f}]")
    return 0


if __name__ == "__main__":
    sys.exit(main())
