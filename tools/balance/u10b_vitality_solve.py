"""U10b-0: solve vitality's progression multiplier on play time.

Vitality was left at 4.5 by Phase D because its faucets are not uses/hour tracks
in the way every other stat's are. They ARE time-based, though, so it can be put
on the same footing -- it just needs both paths modelled, because vitality is the
only stat fed by two structurally different formulas.

THE TWO PATHS, verified in code at HEAD:

1. REGEN  (OnRegenTick -> CheckRegenProgression)
     chance = RegenProgressionBase * (1-ratio)^RegenProgressionCurve
              * statMult * regenDamperFactor(rank)
   - Fires every 3 rounds (NewRound_AutoHeal.go: `evt.RoundNumber%3 != 0`),
     so 300 ticks/hour, NOT 900.
   - Vitality is in BOTH the Health {vitality,willpower} and Stamina
     {strength,vitality} lists, so up to TWO rolls per tick.
   - Each roll is CONDITIONAL: regenTickChance returns 0 when the pool is at or
     above its reachable max. A rested character earns nothing.
   - StatProgressionRate is deliberately NOT applied on this path.
   - regenDamperFactor = CalculateProgressionChance(rank)/base = exp(-decay*rank/softCap).

2. CRIT-TOUGHEN  (BonusEvents -> applyBonusProgression -> CheckStatProgression)
     chance = BaseProgressionChance * exp(-decay*rank/softCap)
              * statMult * StatProgressionRate * ObservedCritProgressionBonus
   - Fires when you RECEIVE a physical crit. ToughenStatFor("physical") is
     vitality; the event is ClassObserved, so it carries
     ObservedCritProgressionBonus (0.5), not CritProgressionBonus (2.0).
   - This path DOES take StatProgressionRate and the full 0.12 base.

THE PROBLEM. Both paths are scaled by the SAME per-stat multiplier, but their
bases differ by 12x (0.12 vs 0.01) and only one takes the 2.25 rate. A multiplier
tuned for the regen path is wildly overtuned for the crit path, and vice versa.
At 4.5 the crit path pays 60.75% at rank 0 -- receiving a physical crit is close
to a coin flip for a free vitality point.

MEASURED INPUT: crit rate 4.88% of combat events (4,720 of 96,723), from
_datafiles/logs/combat-analytics.jsonl. Pre-arc, same caveat as Phase D.
"""
import math

# ── shipped config at HEAD ──────────────────────────────────────────────────
BASE = 0.12           # BaseProgressionChance
RATE = 2.25           # StatProgressionRate  (crit path only)
D_BELOW = 3.0
SOFT = 50.0
REGEN_BASE = 0.01     # RegenProgressionBase
REGEN_CURVE = 3.0     # RegenProgressionCurve
OBSERVED_BONUS = 0.5  # ObservedCritProgressionBonus
ROUND_SECONDS = 4
REGEN_EVERY = 3       # rounds

ROUNDS_PER_HOUR = 3600 / ROUND_SECONDS          # 900
TICKS_PER_HOUR = ROUNDS_PER_HOUR / REGEN_EVERY  # 300
ENG_COMBAT = 0.10                               # owner ruling
REF_RANK = 25
TARGET = 3.0                                    # combat-track target, pts/hr

# ── measured ────────────────────────────────────────────────────────────────
CRIT_RATE = 4720 / 96723.0        # 0.0488 of all combat events
# DERIVED, not assumed: median 2.34 combat events/round across 271 runs, of which
# 26.0% are mob-on-player (25,187 of 96,723). The per-run spread is wide
# (p10 0.45, p90 9.33 events/round), so this is the softest input in the model
# and the sensitivity block below reports what that costs.
INCOMING_SWINGS_PER_ROUND = 2.34 * 0.260   # = 0.61


def damper(rank):
    """exp(-decay*rank/softCap) below the cap; the shared rank decay."""
    if rank <= 0:
        return 1.0
    r = rank / SOFT
    if rank <= SOFT:
        return math.exp(-D_BELOW * r)
    return math.exp(-D_BELOW) * math.exp(-2.0 * (r - 1.0))


def crit_chance(rank, mult):
    return BASE * damper(rank) * mult * RATE * OBSERVED_BONUS


def regen_chance(ratio, rank, mult):
    return REGEN_BASE * (1.0 - ratio) ** REGEN_CURVE * mult * damper(rank)


def gains_per_hour(mult, rank, combat_ratio=0.60, rest_ratio=0.85,
                   rest_fraction=0.25):
    """Expected vitality gains per hour.

    combat_ratio  : mean pool fill during engaged combat
    rest_ratio    : mean pool fill while recovering afterwards
    rest_fraction : share of the hour spent recovering with pools still below max
    """
    crits = ROUNDS_PER_HOUR * ENG_COMBAT * INCOMING_SWINGS_PER_ROUND * CRIT_RATE
    from_crit = crits * min(crit_chance(rank, mult), 1.0)

    # two rolls per tick (Health and Stamina), both conditional on depletion
    ticks_fighting = TICKS_PER_HOUR * ENG_COMBAT
    ticks_resting = TICKS_PER_HOUR * rest_fraction
    from_regen = (2 * ticks_fighting * regen_chance(combat_ratio, rank, mult)
                  + 2 * ticks_resting * regen_chance(rest_ratio, rank, mult))
    return from_crit, from_regen, crits


def main():
    print("Vitality faucets, shipped multiplier 4.5\n")
    print("%-6s %12s %12s %10s %10s %10s"
          % ("rank", "crit chance", "regen@60%", "crit/hr", "regen/hr", "TOTAL/hr"))
    for rank in (0, 10, 14, 25, 40, 50):
        fc, fr, crits = gains_per_hour(4.5, rank)
        print("%-6d %11.1f%% %11.3f%% %10.2f %10.2f %10.2f"
              % (rank, 100 * min(crit_chance(rank, 4.5), 1.0),
                 100 * regen_chance(0.60, rank, 4.5), fc, fr, fc + fr))
    print("\n  physical crits received per hour: %.1f" % gains_per_hour(4.5, 0)[2])

    print("\nThe two paths are scaled by the SAME multiplier but differ %.0fx at the base:"
          % ((BASE * RATE * OBSERVED_BONUS) / REGEN_BASE))
    print("  crit  base = BaseProgressionChance %.2f x rate %.2f x observed %.2f = %.4f"
          % (BASE, RATE, OBSERVED_BONUS, BASE * RATE * OBSERVED_BONUS))
    print("  regen base = RegenProgressionBase  %.2f (no rate)                  = %.4f"
          % (REGEN_BASE, REGEN_BASE))

    print("\nSolve: multiplier that hits %.0f pts/hr at rank %d" % (TARGET, REF_RANK))
    lo, hi = 0.001, 200.0
    for _ in range(200):
        mid = (lo + hi) / 2
        fc, fr, _ = gains_per_hour(mid, REF_RANK)
        if fc + fr < TARGET:
            lo = mid
        else:
            hi = mid
    solved = (lo + hi) / 2
    fc, fr, _ = gains_per_hour(solved, REF_RANK)
    print("  solved multiplier %.2f   (crit %.2f/hr + regen %.2f/hr = %.2f)"
          % (solved, fc, fr, fc + fr))
    print("  shipped 4.5 gives %.2f/hr at the same rank -- %.1fx the target."
          % (sum(gains_per_hour(4.5, REF_RANK)[:2]),
             sum(gains_per_hour(4.5, REF_RANK)[:2]) / TARGET))

    print("\nSensitivity to how much of the hour is spent hurt:")
    for rf in (0.10, 0.25, 0.50, 0.75):
        fc, fr, _ = gains_per_hour(solved, REF_RANK, rest_fraction=rf)
        print("  rest_fraction %.2f -> %.2f/hr (crit %.2f + regen %.2f)"
              % (rf, fc + fr, fc, fr))
    print("  -> regen barely moves the total. It is NOT vitality's dominant")
    print("     faucet: (1-ratio)^3 crushes it to near zero above ~40% fill.")

    print("\nSensitivity to the SOFTEST input (incoming swings/round):")
    global INCOMING_SWINGS_PER_ROUND
    keep = INCOMING_SWINGS_PER_ROUND
    for epr, label in ((0.45, "p10 events/round"),
                       (2.34, "median (used)"),
                       (9.33, "p90 events/round")):
        INCOMING_SWINGS_PER_ROUND = epr * 0.260
        fc, fr, crits = gains_per_hour(4.5, REF_RANK)
        print("  %-18s %.2f swings/rd -> %4.1f crits/hr -> %.2f gains/hr at mult 4.5"
              % (label, epr * 0.260, crits, fc + fr))
    INCOMING_SWINGS_PER_ROUND = keep
    print("  -> even at p90 the rate stays far under the %.0f/hr target, so the" % TARGET)
    print("     conclusion (vitality is too SLOW) survives the uncertainty.")

    print("\nWhy raising the multiplier is the WRONG lever:")
    print("  at the solved %.1f, the rank-0 crit chance is %.0f%% -- clamped to 100%%."
          % (solved, 100 * crit_chance(0, solved)))
    print("  A rare event forced to ~certainty is a degenerate shape: guaranteed")
    print("  early, then collapsing. Vitality needs MORE EVENTS, not a bigger dial.")


if __name__ == "__main__":
    main()
