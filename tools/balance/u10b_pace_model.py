"""U10b-0 Phase D pace model.

Solves the per-stat progression multipliers so the NEW (Training-rank) model
reaches a given number of trained points in about the same number of uses the
OLD (use-count-rank) model took.

Correct form, per the phase index:

    m_new = usesToReach_new_at_m1(T) / usesToReach_old_at_shipped_m(T)

The trap it avoids: computing usesToReach_new at the SHIPPED multiplier
double-applies it. Silent at m = 1.0, 5x wrong at m = 0.20.
"""

import math

BASE = 0.12          # BaseProgressionChance
RATE = 2.25          # StatProgressionRate (stats only; skills do NOT get this)
D_BELOW = 3.0
D_ABOVE = 2.0
OLD_SOFT = 150.0     # old StatProgressionSoftCap (virtual ranks)
NEW_SOFT = 50.0      # new StatProgressionSoftCap (trained points)
UPR = 25.0           # UsesPerRank

SHIPPED = {
    "strength": 0.20, "dexterity": 0.15, "perception": 1.00,
    "vitality": 4.50, "willpower": 1.00, "charisma": 0.22,
}


def curve(rank, soft):
    """CalculateProgressionChance, unmultiplied."""
    if rank <= 0:
        return BASE
    ratio = rank / soft
    if rank <= soft:
        return BASE * math.exp(-D_BELOW * ratio)
    return BASE * math.exp(-D_BELOW) * math.exp(-D_ABOVE * (ratio - 1.0))


def uses_to_reach_new(T, m):
    """New model: rank IS trained points, so each gain has its own fixed odds."""
    total = 0.0
    for t in range(int(T)):
        c = curve(t, NEW_SOFT) * RATE * m
        if c <= 0:
            return float("inf")
        total += 1.0 / c
    return total


def uses_to_reach_old(T, m, cap_uses=5_000_000):
    """Old model: odds depend on the USE COUNT, so gains accrue as an integral.

    Expected gains after U uses = integral of chance(u) du. Walk u until the
    expected-gain count reaches T. Returns inf if it never does, which is the
    saturation the old model had: with m = 0.15 a stat could never exceed about
    50 gains no matter how long you played.
    """
    gains = 0.0
    u = 0
    step = 25
    while u < cap_uses:
        rank = u / UPR
        gains += curve(rank, OLD_SOFT) * RATE * m * step
        if gains >= T:
            return float(u)
        u += step
    return float("inf")


def old_ceiling(m):
    """Total expected gains the old model could EVER deliver at multiplier m."""
    gains, u, step = 0.0, 0, 5
    while u < 2_000_000:
        gains += min(1.0, curve(u / UPR, OLD_SOFT) * RATE * m) * step
        u += step
    return gains


print("OLD-MODEL LIFETIME CEILING (expected gains, ever) at the shipped multipliers")
print("%-12s %-8s %s" % ("stat", "m", "ceiling"))
for s, m in SHIPPED.items():
    print("%-12s %-8.2f %.0f" % (s, m, old_ceiling(m)))

print()
print("SOLVED MULTIPLIERS -- new pace matches old pace at target T trained points")
print("(blank = the old model could never reach that T at all)")
print()
hdr = "%-12s %-8s" % ("stat", "shipped")
targets = [10, 20, 30, 40, 50]
for T in targets:
    hdr += " %9s" % ("T=%d" % T)
print(hdr)

for s, m in SHIPPED.items():
    row = "%-12s %-8.2f" % (s, m)
    for T in targets:
        old = uses_to_reach_old(T, m)
        if math.isinf(old) or old <= 0:
            row += " %9s" % "-"
            continue
        new1 = uses_to_reach_new(T, 1.0)
        row += " %9.2f" % (new1 / old)
    print(row)

print()
print("USES REQUIRED, for scale")
print("%-12s %-10s %-14s %-14s" % ("stat", "T", "old (shipped m)", "new (m=1)"))
for s, m in SHIPPED.items():
    for T in (20, 40):
        old = uses_to_reach_old(T, m)
        new1 = uses_to_reach_new(T, 1.0)
        olds = "never" if math.isinf(old) else "%.0f" % old
        print("%-12s %-10d %-14s %-14.0f" % (s, T, olds, new1))
