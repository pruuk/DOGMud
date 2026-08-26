"""Read combat-analytics.jsonl and report the rates Phase D needs.

The engine already records every combat swing (internal/combat/analytics.go,
Analytics.Enabled: true) and flushes an aggregated summary every
FlushIntervalSec. This reads those summaries.

TWO THINGS THAT WILL BURN YOU IF YOU AGGREGATE NAIVELY:

1. **The buffer is cumulative, not per-window.** FlushAnalytics() computes a
   summary from the whole event buffer and does NOT reset it, so consecutive
   lines overlap almost entirely. Summing every line double-counts massively.
   A DROP in total_events marks a server restart, i.e. a genuinely new sample;
   this takes the final snapshot of each such run.

2. **DodgeSuccesses / ParrySuccesses / BlockSuccesses are misnamed.** The
   counter increments on e.DefenseUsed regardless of whether the attack landed
   (analytics.go:527-534, outside the `if e.Hit` branch). They are "which
   defence was best", not "which defence worked" -- which is why they can and
   do exceed the miss count. Do NOT read them as an avoidance rate.

3. **HIT and CLEAN-HIT are different populations, and this script used to
   conflate them.** Until 2026-08-26 it printed hits/(hits+misses) under the
   label "CLEAN-HIT RATE <- the Phase D input". That is the HIT rate.

   CleanHit is assigned inside `if res.hit` as `!res.defended`
   (internal/combat/combat.go:491), so it means HIT **AND NOT DEFENDED**. A
   DEFLECTED swing -- the defence won and partial damage landed anyway, the U6
   Task 16b case -- is a Hit that is NOT a CleanHit.

   On the 96,723-event sample the difference is large: hit rate 0.5752 against
   a real clean-hit rate of 0.3856 (37,297 clean, 18,341 deflected).

   **U10b-0 Phase D solved the SHIPPED SkillProgressionMultipliers on the
   mislabelled number**, so anything fitted before this date needs re-deriving
   -- see internal/skills/skills.go, whose "P(entry clean) is 0.967 against
   0.820" figures are 1-0.4248^2 and 1-0.4248^4, i.e. built from the miss rate.

Usage:  python tools/balance/read_combat_analytics.py [path]
"""
import io
import json
import sys

DEFAULT = "_datafiles/logs/combat-analytics.jsonl"


def load_runs(path):
    lines = []
    for ln in io.open(path, encoding="utf-8"):
        ln = ln.strip()
        if not ln:
            continue
        try:
            lines.append(json.loads(ln))
        except ValueError:
            continue
    runs, cur = [], None
    for d in lines:
        if cur is None or d.get("total_events", 0) < cur.get("total_events", 0):
            if cur is not None:
                runs.append(cur)
            cur = d
        else:
            cur = d
    if cur is not None:
        runs.append(cur)
    return lines, runs


def main():
    path = sys.argv[1] if len(sys.argv) > 1 else DEFAULT
    lines, runs = load_runs(path)
    if not runs:
        print("no data in %s" % path)
        return

    def tot(k):
        return sum(r.get(k, 0) for r in runs)

    ev, hits, miss = tot("total_events"), tot("hits"), tot("misses")
    dg, pr, bl = (tot("dodge_successes"), tot("parry_successes"),
                  tot("block_successes"))

    print("source          %s" % path)
    print("flush lines     %d" % len(lines))
    print("server runs     %d  (a drop in total_events marks a restart)" % len(runs))
    print()
    print("total events    %d" % ev)
    print("hits            %d" % hits)
    print("misses          %d" % miss)
    print("crits           %d" % tot("crits"))
    print("fumbles         %d" % tot("fumbles"))
    print()
    defended = dg + pr + bl
    if hits + miss:
        total = float(hits + miss)
        print("HIT RATE        %.4f      hits / total, INCLUDING deflected hits"
              % (hits / total))
        # CleanHit is assigned inside `if res.hit` as `!res.defended`
        # (internal/combat/combat.go:491), so it means HIT AND NOT DEFENDED.
        # A deflected swing is a Hit that is NOT a CleanHit.
        #
        # Every miss is a defence win, so:  deflected = defended - misses
        # and                               clean     = hits - deflected
        deflected = defended - miss
        if 0 <= deflected <= hits:
            clean = hits - deflected
            print("CLEAN-HIT RATE  %.4f      clean / total  <- the Phase D input"
                  % (clean / total))
            print("  clean %d + deflected %d = hits %d" % (clean, deflected, hits))
        else:
            print("CLEAN-HIT RATE  unavailable: defended %d and misses %d do not"
                  % (defended, miss))
            print("  decompose (deflected would be %d). Check whether every miss"
                  % deflected)
            print("  is still a defence win before trusting any derived rate.")
    if ev:
        print("crit rate       %.4f  of all events" % (tot("crits") / float(ev)))
        print("fumble rate     %.4f  of all events" % (tot("fumbles") / float(ev)))
    print()
    d = dg + pr + bl
    if d:
        print("defence SELECTED (not avoidance -- see docstring):")
        print("  dodge %7d  %5.1f%%" % (dg, 100 * dg / float(d)))
        print("  parry %7d  %5.1f%%" % (pr, 100 * pr / float(d)))
        print("  block %7d  %5.1f%%" % (bl, 100 * bl / float(d)))
    print()
    print("matchups: PvM %d  MvP %d  PvP %d  MvM %d"
          % (tot("pvm_events"), tot("mvp_events"), tot("pvp_events"), tot("mvm_events")))

    last = runs[-1]
    span = last.get("latest_round", 0) - last.get("earliest_round", 0)
    if span > 0 and last.get("total_events"):
        print()
        print("last run: %d events over %d rounds = %.2f swing-events/round"
              % (last["total_events"], span, last["total_events"] / float(span)))

    byat = {}
    for r in runs:
        for k, v in (r.get("by_attack_type") or {}).items():
            a = byat.setdefault(k, {"events": 0, "hits": 0, "crits": 0})
            a["events"] += v.get("events", 0)
            a["hits"] += v.get("hits", 0)
            a["crits"] += v.get("crits", 0)
    if byat:
        print()
        print("by attack type:")
        print("  %-18s %8s %8s %8s" % ("type", "events", "hits", "hit%"))
        for k in sorted(byat, key=lambda x: -byat[x]["events"]):
            a = byat[k]
            hp = (100.0 * a["hits"] / a["events"]) if a["events"] else 0.0
            print("  %-18s %8d %8d %7.1f%%" % (k, a["events"], a["hits"], hp))


if __name__ == "__main__":
    main()
