"""Solve multipliers so an hour of concerted grinding yields comparable
progress whatever you grind. Inverse of time_balance.py.

    gains/hour = uses_per_hour * curve(rank) * [RATE if stat] * m
    =>  m = target / (uses_per_hour * curve(rank) * [RATE])
"""

import math

BASE, RATE, D_BELOW, D_ABOVE, SOFT = 0.12, 2.25, 3.0, 2.0, 50.0
ROUNDS_PER_HOUR = 3600 / 4


def curve(rank):
    if rank <= 0:
        return BASE
    r = rank / SOFT
    if rank <= SOFT:
        return BASE * math.exp(-D_BELOW * r)
    return BASE * math.exp(-D_BELOW) * math.exp(-D_ABOVE * (r - 1.0))


# Owner ruling 2026-08-22: ~10%% ENGAGEMENT over a real hour. The rest is
# travelling to find mobs, gathering mats, walking to stations, recovering.
ENGAGEMENT = 0.10
combat_rounds = ROUNDS_PER_HOUR * ENGAGEMENT          # 90 rounds of actual fighting
crafts_hr = (ROUNDS_PER_HOUR * ENGAGEMENT) / 5        # 18 crafts
# Utility is the exception: you search/forage WHILE travelling, which is the
# other 90%% of the hour. Say one attempt every fifth round across the hour.
utility_hr = ROUNDS_PER_HOUR / 5
social_hr = 100          # rhetoric/bartering: conversational, NPC-bound
# Manifestation is trained ONLY by assess (assess.go:86 and :147) -- raise and
# charm do not train it. So it needs a CORPSE and carries a 6-round cooldown,
# which makes it corpse-bound at roughly the kill rate, not cooldown-bound.
# Manifestation IS trained by raise / charm / conjure / summon: the cast path
# picks the skill from the spell school (SchoolManifestation), so a literal grep
# for OnSkillUse("manifestation") misses all 14 of them. Its faucet is those
# casts (CP-bound, ~70/hr at 45 CP) plus assess (free, 6-round cd). Haircut
# because a fielded companion permanently RESERVES conviction, shrinking the
# pool the caster regenerates from.
manifest_hr = 50
# Casting is CP-BOUND over an hour, not cooldown-bound. A mid character
# regenerates ~2700 CP/hr (2%% of a 450 pool every 3 rounds) plus the 450 they
# start with. At a typical 40 CP spell that funds ~79 casts; haircut for CP also
# spent on quell/defy defences.
cast_hr = 60

# (name, is_skill, uses/hour when this is what you are grinding)
TRACKS = [
    ("weapon-combat",  True,  combat_rounds),
    ("unarmed-combat", True,  combat_rounds),
    ("ranged-combat",  True,  combat_rounds * 0.5),   # aimed shots, reload cd
    ("spellcasting",   True,  cast_hr),
    ("rhetoric",       True,  social_hr),
    ("manifestation",  True,  manifest_hr),
    ("skullduggery",   True,  utility_hr),
    ("search",         True,  utility_hr),
    ("bartering",      True,  social_hr),
    ("salvage",        True,  crafts_hr),
    ("blacksmithing",  True,  crafts_hr),
    ("alchemy",        True,  crafts_hr),
    ("tailoring",      True,  crafts_hr),
    ("cooking",        True,  crafts_hr),
    ("jewelcrafting",  True,  crafts_hr),
    ("enchanting",     True,  crafts_hr),
    # stats, by their dominant faucet when grinding that faucet
    ("strength",       False, combat_rounds * 1),
    ("dexterity",      False, combat_rounds * 2),
    ("perception",     False, utility_hr),
    ("willpower",      False, cast_hr),
    ("charisma",       False, social_hr),
]

SHIPPED = {
    "weapon-combat": 0.23, "unarmed-combat": 0.23, "ranged-combat": 0.5,
    "spellcasting": 0.63, "rhetoric": 0.58, "manifestation": 0.38,
    "skullduggery": 2.0, "search": 2.0, "bartering": 2.0, "salvage": 2.0,
    "blacksmithing": 3.5, "alchemy": 3.5, "tailoring": 3.5, "cooking": 3.5,
    "jewelcrafting": 3.5, "enchanting": 3.5,
    "strength": 0.20, "dexterity": 0.15, "perception": 1.00,
    "willpower": 1.00, "charisma": 0.22,
}

REF_RANK = 25


def solve(uses, is_skill, target, rank=REF_RANK):
    denom = uses * curve(rank) * (1.0 if is_skill else RATE)
    return target / denom


# Owner ruling: 3/hr for combat (risk, stamina, time-to-find), 4/hr for the
# rest (crafting spends gold and mats, so a little faster is fair).
COMBAT_TRACKS = {"weapon-combat", "unarmed-combat", "ranged-combat",
                 "spellcasting", "rhetoric", "strength", "dexterity", "willpower"}

print("=" * 78)
print("FINAL: 3/hr for combat tracks, 4/hr for everything else, at rank %d" % REF_RANK)
print("=" * 78)
print("%-16s %-9s %-7s %-9s %-9s %s" % ("track", "uses/hr", "target", "shipped", "solved", "change"))
for name, is_skill, uses in TRACKS:
    tgt = 3.0 if name in COMBAT_TRACKS else 4.0
    m = solve(uses, is_skill, tgt)
    ship = SHIPPED[name]
    print("%-16s %-9.0f %-7.0f %-9.2f %-9.2f %.1fx" % (name, uses, tgt, ship, m, m / ship))
print()

for target in ():
    print("=" * 78)
    print("TARGET: %.0f points per hour at rank %d for whatever you are grinding" % (target, REF_RANK))
    print("=" * 78)
    print("%-16s %-9s %-9s %-9s %s" % ("track", "uses/hr", "shipped", "solved", "change"))
    for name, is_skill, uses in TRACKS:
        m = solve(uses, is_skill, target)
        ship = SHIPPED[name]
        print("%-16s %-9.0f %-9.2f %-9.2f %.1fx" % (name, uses, ship, m, m / ship))
    print()
