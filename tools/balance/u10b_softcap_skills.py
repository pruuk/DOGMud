import math

BASE, RATE, D_BELOW, D_ABOVE = 0.12, 2.25, 3.0, 2.0
STAT_SOFT, SKILL_SOFT, UPR = 50.0, 50.0, 25.0


def curve(rank, soft):
    if rank <= 0:
        return BASE
    r = rank / soft
    if rank <= soft:
        return BASE * math.exp(-D_BELOW * r)
    return BASE * math.exp(-D_BELOW) * math.exp(-D_ABOVE * (r - 1.0))


FLOOR = 1e-5

# ---------------------------------------------------------------- STATS
print("=" * 74)
print("STATS: uses to climb each band, at the T=20-anchored multipliers")
print("=" * 74)
solved = {"strength": 0.33, "dexterity": 0.23, "perception": 1.85,
          "willpower": 1.85, "charisma": 0.37}
bands = [(0, 10), (10, 20), (20, 30), (30, 40), (40, 50),
         (50, 60), (60, 75), (75, 100)]

hdr = "%-12s" % "stat"
for a, b in bands:
    hdr += " %10s" % ("%d-%d" % (a, b))
print(hdr)
for s, m in solved.items():
    row = "%-12s" % s
    for a, b in bands:
        tot = 0.0
        for t in range(a, b):
            c = max(FLOOR, min(1.0, curve(t, STAT_SOFT) * RATE * m))
            tot += 1.0 / c
        row += " %10.0f" % tot
    print(row)

print()
print("The soft cap is a KNEE, not a wall. Chance at each landmark (perception, m=1.85):")
for t in (0, 25, 49, 50, 51, 60, 75, 100, 150):
    c = max(FLOOR, min(1.0, curve(t, STAT_SOFT) * RATE * 1.85))
    print("   Training %-4d %.5f   (1 gain per %s uses)"
          % (t, c, "%.0f" % (1 / c) if c > 0 else "never"))

# --------------------------------------------------------------- SKILLS
print()
print("=" * 74)
print("SKILLS: same curve, but skills do NOT get StatProgressionRate")
print("=" * 74)

SKILL_M = {"weapon-combat": 0.23, "unarmed-combat": 0.23, "spellcasting": 0.63,
           "rhetoric": 0.58, "manifestation": 0.38, "search": 2.0,
           "bartering": 2.0, "salvage": 2.0, "blacksmithing": 3.5}


def skill_chance_new(level, m):
    return max(FLOOR, min(1.0, curve(level, SKILL_SOFT) * m))


def skill_uses_to_reach_new(L, m):
    return sum(1.0 / skill_chance_new(l, m) for l in range(int(L)))


def skill_uses_to_reach_old(L, m, cap=20_000_000):
    """Old model: rank = (uses * m)/25 for m < 1, else uses/25. Chance carries m."""
    gains, u, step = 0.0, 0, 1
    while u < cap:
        adj = u * m if m < 1.0 else u
        gains += min(1.0, curve(adj / UPR, SKILL_SOFT) * m) * step
        u += step
        if gains >= L:
            return float(u)
    return float("inf")


def skill_old_ceiling(m, cap=20_000_000):
    gains, u, step = 0.0, 0, 20
    while u < cap:
        adj = u * m if m < 1.0 else u
        gains += min(1.0, curve(adj / UPR, SKILL_SOFT) * m) * step
        u += step
    return gains


print("%-16s %-7s %-14s %-12s %-12s" % ("skill", "m", "OLD ceiling", "old->L=20", "new->L=20"))
for s, m in SKILL_M.items():
    ceil = skill_old_ceiling(m)
    old20 = skill_uses_to_reach_old(20, m)
    new20 = skill_uses_to_reach_new(20, m)
    print("%-16s %-7.2f %-14.0f %-12s %-12.0f"
          % (s, m, ceil, "never" if math.isinf(old20) else "%.0f" % old20, new20))

print()
print("Solved skill multipliers to hold the old pace at level L")
print("%-16s %-8s %8s %8s %8s" % ("skill", "shipped", "L=10", "L=20", "L=30"))
for s, m in SKILL_M.items():
    row = "%-16s %-8.2f" % (s, m)
    for L in (10, 20, 30):
        old = skill_uses_to_reach_old(L, m)
        if math.isinf(old) or old <= 0:
            row += " %8s" % "-"
            continue
        row += " %8.2f" % (skill_uses_to_reach_new(L, 1.0) / old)
    print(row)
