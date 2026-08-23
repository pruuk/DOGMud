"""U10b-0 Phase D: solve progression multipliers on PLAY TIME.

A use is not a comparable unit across activities: an hour of combat is hundreds
of swings, an hour of crafting is tens of crafts. This converts each track to
uses/hour from the real timing constants, then solves

    m = target / (uses_per_hour * curve(rank) * RATE_if_stat * bonus)

REVISION 2 (2026-08-22), after blind adversarial review. Revision 1 was wrong in
four measured ways, all corrected below and each marked [R2].

VERIFIED TIMING
  Timing.RoundSeconds = 4                     -> 900 rounds/hour
  recipe time_rounds: mode 4, median 5        -> 18 crafts/hour at 10% engagement
  search cooldown "2 rounds" (actions/search.go:53)  [R2] -> ceiling 450/hour

VERIFIED PER-ROUND COMBAT ACCOUNTING (NewRound_DoCombat_unified.go:659-680,
combat/defence_multiplier.go:158-173, combat/combat_helpers.go:282-285)
  emitAttackerStatGain          : strength +1, dexterity +1   (per exchange)
  per clean WeaponHits entry    : that weapon's skill +1, and its primary stat +1
  [R2] an empty offhand supplies a FIST, so a 1H attacker produces TWO
       WeaponHits entries: weapon-combat AND unarmed-combat
  [R2] AwardDefenceProgression calls OnSkillUse(skill) -- which itself fires
       OnStatUse(primary) -- and THEN OnStatUse(stat). Since weapon-combat and
       unarmed-combat both map to dexterity:
         dodge  -> unarmed-combat +1, dexterity +2
         parry  -> weapon-combat  +1, dexterity +2, strength +1
         block  -> weapon-combat  +1, dexterity +1, strength +1
       and it fires once per defence type per ATTACKER per round.

[R2] DIFFICULTY BONUS on the SKILL roll only (the stat roll inside
OnSkillUseScaled always passes 1.0):
  craftBonus = 1 + skill_minimum * 0.02   -> median 1.40 over 126 recipes
  spellBonus = 1 + difficulty     * 0.01  -> median 1.25 over 59 spells
                                             median 1.35 over the 14 manifestation spells
  salvage does NOT get it (actions/salvage.go calls bare OnSkillUse)
"""

import math

BASE, RATE, D_BELOW, D_ABOVE, SOFT = 0.12, 2.25, 3.0, 2.0, 50.0
ROUNDS_PER_HOUR = 3600 / 4
ENGAGEMENT = 0.10          # owner ruling: travel, finding mobs, gathering mats
REF_RANK = 25


def curve(rank):
    if rank <= 0:
        return BASE
    r = rank / SOFT
    if rank <= SOFT:
        return BASE * math.exp(-D_BELOW * r)
    return BASE * math.exp(-D_BELOW) * math.exp(-D_ABOVE * (r - 1.0))


combat_rounds = ROUNDS_PER_HOUR * ENGAGEMENT      # 90
crafts_hr = (ROUNDS_PER_HOUR * ENGAGEMENT) / 5    # 18

# [R2] Per engaged combat round for a 1H-weapon character, p(clean hit) ~ 0.5,
# one defence registering per round, defences mixed evenly across dodge/parry/block.
P_CLEAN = 0.5
str_per_round = 1 + (2 / 3)          # attacker +1; parry and block each add +1
dex_per_round = 1 + P_CLEAN * 2 + (5 / 3)   # attacker; two weapon entries; defences
wc_per_round = P_CLEAN + (2 / 3)     # main weapon hit; parry and block
uc_per_round = P_CLEAN + (1 / 3)     # offhand fist; dodge

TRACKS = [
    # (name, is_skill, uses/hour, difficulty bonus)
    ("weapon-combat",  True,  combat_rounds * wc_per_round, 1.0),
    ("unarmed-combat", True,  combat_rounds * uc_per_round, 1.0),
    ("ranged-combat",  True,  combat_rounds * 0.5, 1.0),
    # [R2] casting is CAST-TIME bound at 10% engagement, not CP-bound: 90 engaged
    # rounds / median 2 waitrounds = 45, well under the CP sustain of ~79.
    ("spellcasting",   True,  combat_rounds / 2, 1.25),
    # [R2] rhetoric has no conversation faucet -- taunt and the defy defence only.
    ("rhetoric",       True,  combat_rounds, 1.0),
    # [R2] manifestation: raise/conjure at 4 waitrounds inside engaged rounds,
    # plus assess (free, 6-round cooldown, corpse-bound).
    ("manifestation",  True,  combat_rounds / 4 + 15, 1.35),
    # [R2] skullduggery has no search faucet: steal, plant, shadow, defuse,
    # surprise_attack, throw. Opportunity-bound.
    ("skullduggery",   True,  100, 1.0),
    # [R2] search is cooldown-bound at 2 rounds over the WHOLE hour.
    ("search",         True,  ROUNDS_PER_HOUR / 2, 1.0),
    ("bartering",      True,  100, 1.0),
    ("salvage",        True,  crafts_hr, 1.0),          # [R2] no craftBonus
    ("blacksmithing",  True,  crafts_hr, 1.40),
    ("alchemy",        True,  crafts_hr, 1.40),
    ("tailoring",      True,  crafts_hr, 1.40),
    ("cooking",        True,  crafts_hr, 1.40),
    ("jewelcrafting",  True,  crafts_hr, 1.40),
    ("enchanting",     True,  crafts_hr, 1.40),
    ("strength",       False, combat_rounds * str_per_round, 1.0),
    ("dexterity",      False, combat_rounds * dex_per_round, 1.0),
    ("perception",     False, ROUNDS_PER_HOUR / 2, 1.0),   # search is its dominant faucet
    ("willpower",      False, combat_rounds / 2, 1.0),
    ("charisma",       False, 100, 1.0),
]

# "shipped" = what production actually reads. [R2] ranged-combat, skullduggery
# and search have NO config.yaml entry, so the Go map in internal/skills is
# authoritative for them; the rest come from config.yaml at HEAD.
SHIPPED = {
    "weapon-combat": 0.23, "unarmed-combat": 0.23,
    "ranged-combat": 0.5,      # Go map only
    "spellcasting": 0.63, "rhetoric": 0.58, "manifestation": 0.38,
    "skullduggery": 2.0,       # Go map only
    "search": 2.0,             # Go map only
    "bartering": 2.0, "salvage": 2.0,
    "blacksmithing": 3.5, "alchemy": 3.5, "tailoring": 3.5,
    "cooking": 3.5, "jewelcrafting": 3.5, "enchanting": 3.5,
    "strength": 0.20, "dexterity": 0.15, "perception": 1.00,
    "willpower": 1.00, "charisma": 0.22,
}
CONFIG_ABSENT = {"ranged-combat", "skullduggery", "search"}

COMBAT_TRACKS = {"weapon-combat", "unarmed-combat", "ranged-combat",
                 "spellcasting", "rhetoric", "strength", "dexterity", "willpower"}


def solve(uses, is_skill, bonus, target, rank=REF_RANK):
    return target / (uses * curve(rank) * (1.0 if is_skill else RATE) * bonus)


print("Owner targets: 3 pts/hr for combat tracks, 4 for the rest, at rank %d." % REF_RANK)
print("%-16s %-9s %-7s %-7s %-9s %-9s %s" %
      ("track", "uses/hr", "bonus", "target", "shipped", "solved", "change"))
solved = {}
for name, is_skill, uses, bonus in TRACKS:
    tgt = 3.0 if name in COMBAT_TRACKS else 4.0
    m = solve(uses, is_skill, bonus, tgt)
    solved[name] = m
    flag = " *" if name in CONFIG_ABSENT else ""
    print("%-16s %-9.0f %-7.2f %-7.0f %-9.2f %-9.2f %.1fx%s"
          % (name, uses, bonus, tgt, SHIPPED[name], m, m / SHIPPED[name], flag))
print()
print("* no config.yaml entry today -- D2 must ADD the key, not edit it.")
print()

# [R2] the anchor drifts by exp(D_BELOW/SOFT * 10) = 1.822x per 10 ranks, not 1.5x.
print("Drift away from the rank-%d anchor (multiple of target):" % REF_RANK)
row = "  "
for rank in (0, 10, 25, 40, 50, 60):
    row += "rank %-3d %.2fx   " % (rank, curve(rank) / curve(REF_RANK))
print(row)
print("  -> a fresh character progresses at %.1fx the target; %.3fx per 10 ranks."
      % (curve(0) / curve(REF_RANK), math.exp(D_BELOW / SOFT * 10)))
print()
print("[R2] At rank 0 a craft skill's chance is %.3f, CLAMPED to 1.0 by"
      % (BASE * solved["blacksmithing"] * 1.40))
print("     progression.go, so the first craft always levels. The clamp wastes")
print("     part of the multiplier at the low end where new players live.")
