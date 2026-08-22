"""U10b-0 Phase D: balance progression on PLAY TIME, not on use counts.

A use is not a comparable unit across activities: an hour of combat produces
hundreds of swings, an hour of crafting produces tens of crafts. This model
converts each activity to uses/hour from the real timing constants, then asks
how many points an hour of concerted grinding actually yields.

TIMING FACTS (verified in config / data, not assumed):
  RoundSeconds = 4            -> 900 rounds per hour
  recipe time_rounds          -> mode 4, median 5 (range 2-20)

PER-EVENT ACCOUNTING (verified in NewRound_DoCombat_unified.go /
defence_multiplier.go / progression.go):
  attacking, per exchange : strength +1, dexterity +1  (emitAttackerStatGain)
  per clean weapon hit    : combat skill +1, and its primary stat (dexterity) +1
  defending, per round    : defence skill +1, its stat +1 (parry also strength)
  any skill use           : also fires OnStatUse(primary stat)
  crafting, per craft     : craft skill +1, its primary stat +1
"""

import math

BASE, RATE, D_BELOW, D_ABOVE = 0.12, 2.25, 3.0, 2.0
SOFT = 50.0
FLOOR = 1e-5
ROUNDS_PER_HOUR = 3600 / 4


def curve(rank):
    if rank <= 0:
        return BASE
    r = rank / SOFT
    if rank <= SOFT:
        return BASE * math.exp(-D_BELOW * r)
    return BASE * math.exp(-D_BELOW) * math.exp(-D_ABOVE * (r - 1.0))


def stat_chance(rank, m):
    return max(FLOOR, min(1.0, curve(rank) * RATE * m))


def skill_chance(rank, m):
    # skills do NOT get StatProgressionRate
    return max(FLOOR, min(1.0, curve(rank) * m))


def gains_per_hour(uses, rank, m, is_skill):
    f = skill_chance if is_skill else stat_chance
    return uses * f(rank, m)


# ---- activity profiles: uses per hour, with stated uptime assumptions -------
COMBAT_UPTIME = 0.60      # travel, finding targets, recovery
CRAFT_UPTIME = 0.50       # gathering ingredients, walking to stations
CRAFT_ROUNDS = 5          # median recipe
UTILITY_PER_HOUR = 300    # search/forage style, cooldown-limited

combat_rounds = ROUNDS_PER_HOUR * COMBAT_UPTIME
crafts_per_hour = (ROUNDS_PER_HOUR * CRAFT_UPTIME) / CRAFT_ROUNDS

COMBAT_USES = {
    "strength": combat_rounds * 1,          # attacker stat gain
    "dexterity": combat_rounds * 2,         # attacker stat gain + skill primary
    "weapon-combat": combat_rounds * 1,     # one clean hit per exchange
}
CRAFT_USES = {
    "blacksmithing": crafts_per_hour,
    "strength": crafts_per_hour,            # blacksmithing's primary stat
}
UTILITY_USES = {
    "search": UTILITY_PER_HOUR,
    "perception": UTILITY_PER_HOUR,
}

print("USES PER HOUR OF CONCERTED GRINDING")
print("  combat  : %.0f rounds/hr at %.0f%% uptime" % (combat_rounds, COMBAT_UPTIME * 100))
print("  crafting: %.0f crafts/hr (%d-round recipe, %.0f%% uptime)"
      % (crafts_per_hour, CRAFT_ROUNDS, CRAFT_UPTIME * 100))
print("  utility : %d uses/hr" % UTILITY_PER_HOUR)
print()
print("  combat gives %.0fx more skill uses per hour than crafting"
      % (COMBAT_USES["weapon-combat"] / crafts_per_hour))
print()

SHIPPED = {"strength": 0.20, "dexterity": 0.15, "perception": 1.00,
           "weapon-combat": 0.23, "search": 2.00, "blacksmithing": 3.50}
SOLVED20 = {"strength": 0.33, "dexterity": 0.23, "perception": 1.85,
            "weapon-combat": 0.34, "search": 3.36, "blacksmithing": 6.13}

for label, MULT in (("SHIPPED multipliers", SHIPPED), ("T=20-SOLVED multipliers", SOLVED20)):
    print("=" * 70)
    print("POINTS GAINED PER HOUR -- %s" % label)
    print("=" * 70)
    print("%-16s %-10s %8s %8s %8s %8s" % ("track", "what", "rank 0", "rank 10", "rank 25", "rank 40"))
    rows = [
        ("combat", "strength", COMBAT_USES["strength"], False),
        ("combat", "dexterity", COMBAT_USES["dexterity"], False),
        ("combat", "weapon-combat", COMBAT_USES["weapon-combat"], True),
        ("crafting", "blacksmithing", CRAFT_USES["blacksmithing"], True),
        ("crafting", "strength", CRAFT_USES["strength"], False),
        ("utility", "search", UTILITY_USES["search"], True),
        ("utility", "perception", UTILITY_USES["perception"], False),
    ]
    for track, name, uses, is_skill in rows:
        line = "%-16s %-10s" % (track, name)
        for rank in (0, 10, 25, 40):
            line += " %8.1f" % gains_per_hour(uses, rank, MULT[name], is_skill)
        print(line)
    print()
