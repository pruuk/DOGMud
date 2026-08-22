"""U10b-0 Phase D revision 3: solve progression multipliers on MEASURED rates.

Revisions 1 and 2 both failed blind adversarial review because uses/hour were
ASSERTED. Every rate here is either measured from the engine's own combat
analytics, computed closed-form from the shipped constants, or an explicit
owner ruling. Each is labelled.

FRAMING (owner): "roughly balance on time given a concerted effort to grind X
or Y". So each track is solved assuming the player CONCENTRATES on that track.
That is what resolves the shared-cooldown problem: a rhetoric-grinder spends
the whole special-move budget on taunt, a caster spends it all on casting.
They are alternatives, not a sum.

MEASURED INPUTS
  clean-hit rate 0.5752      96,723 events over 272 server runs
                             (_datafiles/logs/combat-analytics.jsonl).
                             PRE-ARC: predates U6b/U9/U10 and Phases A/B/C.
                             Owner accepted this with the caveat stated.
  defence share              dodge 77.0% / parry 15.1% / block 8.0% of
                             DefenseUsed. Renormalised to dodge/parry for a
                             no-shield build (block needs a shield, which
                             suppresses the offhand fist -- they are exclusive).
  forage success             saturates >97% by Search rank ~20
                             (tools/balance/forage_rate.py), so forage is
                             cooldown-bound at its 150/hr ceiling, a CONSTANT.

OWNER RULINGS
  engagement                 combat 10%, search/forage 100%,
                             craft/salvage/barter 40% (gather THEN craft),
                             manifestation gated to ~the raise+assess rate.
  targets                    3 points/hr for combat tracks, 4 for the rest.
  look/consider              UNHOOKED from perception (see plan Task 1).
  conjure                    gets its OWN cooldown key (see plan Task 3).

STRUCTURAL FACTS (docs/superpowers/specs/2026-08-22-progression-faucet-census.md)
  "special-move" is ONE shared 4-round key across 18 verbs, so at 10%
  engagement the whole special-move family shares 22.5 uses/hr.
  Difficulty bonus uses the MEAN, not the median: chance is LINEAR in it.
"""
import math

# ── shipped config (config.yaml at HEAD, NOT Go defaults) ────────────────────
BASE = 0.12          # BaseProgressionChance
RATE = 2.25          # StatProgressionRate (stat rolls only)
D_BELOW = 3.0        # ProgressionDecayBelowCap
D_ABOVE = 2.0        # ProgressionDecayAboveCap
SOFT = 50.0          # SkillSoftCap / StatProgressionSoftCap
ROUND_SECONDS = 4
SPECIAL_MOVE_CD = 4  # rounds

ROUNDS_PER_HOUR = 3600 / ROUND_SECONDS          # 900
REF_RANK = 25

# ── measured ────────────────────────────────────────────────────────────────
CLEAN_HIT = 0.5752
DODGE_SHARE, PARRY_SHARE = 0.770, 0.151         # of DefenseUsed; block excluded
_d = DODGE_SHARE + PARRY_SHARE
DODGE, PARRY = DODGE_SHARE / _d, PARRY_SHARE / _d

# mid-profile swing counts from calcSwingCount (dex 118, wc 18, SkillWeight 5.0,
# longsword speed 0.7 -> 2 swings; fist at UnarmedSpeed 1.8 x dualWield 1.4 -> capped 4)
SWINGS_MAIN, SWINGS_FIST = 2, 4

# WeaponHits.CleanHit is OR-aggregated across a weapon's swings (combat.go:474)
P_ENTRY_MAIN = 1 - (1 - CLEAN_HIT) ** SWINGS_MAIN
P_ENTRY_FIST = 1 - (1 - CLEAN_HIT) ** SWINGS_FIST

# ── owner engagement rulings ────────────────────────────────────────────────
ENG_COMBAT, ENG_GATHER, ENG_CRAFT = 0.10, 1.00, 0.40

combat_rounds = ROUNDS_PER_HOUR * ENG_COMBAT               # 90
special_move_budget = combat_rounds / SPECIAL_MOVE_CD      # 22.5, SHARED
crafts_hr = (ROUNDS_PER_HOUR * ENG_CRAFT) / 5.0            # median time_rounds 5
forage_hr = ROUNDS_PER_HOUR / 6.0                          # hardcoded 6-round cd

# ── difficulty bonuses: MEAN (chance is linear in the bonus) ────────────────
CRAFT_BONUS = 1.4724      # 126 recipes
SPELL_BONUS = 1.2780      # 59 spells
MANI_BONUS = 1.3393       # 14 manifestation spells


def curve(rank):
    if rank <= 0:
        return BASE
    r = rank / SOFT
    if rank <= SOFT:
        return BASE * math.exp(-D_BELOW * r)
    return BASE * math.exp(-D_BELOW) * math.exp(-D_ABOVE * (r - 1.0))


# ── per engaged combat round, for a 1H + empty offhand build ────────────────
# emitAttackerStatGain fires ONCE per exchange, outside the WeaponHits loop, and
# is the ONLY strength source; per-hit events use GetSkillPrimaryStat, which is
# dexterity for BOTH combat skills. The offhand fist adds ZERO strength.
# AwardDefenceProgression: dodge -> unarmed +1, dex +2; parry -> weapon +1,
# dex +2, str +1. One defence type per attacker per round.
str_pr = 1.0 + PARRY * 1.0
dex_pr = 1.0 + P_ENTRY_MAIN + P_ENTRY_FIST + 2.0
wc_pr = P_ENTRY_MAIN + PARRY
uc_pr = P_ENTRY_FIST + DODGE

# ── conjure gate: match the raise+assess rate ───────────────────────────────
# ~2 manifestation uses per corpse (one assess, one raise); corpses/hr = kills/hr.
# Kill rate is the one number the failed measurement session owed, so this ships
# as a CONFIG KNOB with a defensible default rather than a hardcoded literal.
CONJURE_CD_ROUNDS = 36          # -> 25 casts/hr, midpoint of the 18-36/hr band
mani_hr = ROUNDS_PER_HOUR / CONJURE_CD_ROUNDS

TRACKS = [
    # (name, is_skill, uses/hr, bonus, note)
    ("weapon-combat",  True,  combat_rounds * wc_pr,  1.0, "measured clean-hit"),
    ("unarmed-combat", True,  combat_rounds * uc_pr,  1.0, "measured; fist+dodge"),
    ("ranged-combat",  True,  special_move_budget,    1.0, "reload is on special-move"),
    ("spellcasting",   True,  special_move_budget,    SPELL_BONUS, "shared budget"),
    ("rhetoric",       True,  special_move_budget,    1.0, "shared budget"),
    ("manifestation",  True,  mani_hr,                MANI_BONUS, "conjure cd (new)"),
    ("skullduggery",   True,  180.0,                  1.0, "shadow cd 5 rounds"),
    ("search",         True,  forage_hr,              1.0, "forage cd, 100% eng"),
    ("bartering",      True,  crafts_hr,              1.0, "NOT time-bound; see plan"),
    ("salvage",        True,  crafts_hr,              1.0, "no craft bonus"),
    ("blacksmithing",  True,  crafts_hr,              CRAFT_BONUS, ""),
    ("alchemy",        True,  crafts_hr,              CRAFT_BONUS, ""),
    ("tailoring",      True,  crafts_hr,              CRAFT_BONUS, ""),
    ("cooking",        True,  crafts_hr,              CRAFT_BONUS, ""),
    ("jewelcrafting",  True,  crafts_hr,              CRAFT_BONUS, ""),
    ("enchanting",     True,  crafts_hr,              CRAFT_BONUS, ""),
    ("strength",       False, combat_rounds * str_pr, 1.0, "attacker + parry only"),
    ("dexterity",      False, combat_rounds * dex_pr, 1.0, "both entries + defence"),
    # A stat must be solved on the same "concerted effort" basis as a skill:
    # the dominant path a player actually grinds, NOT the sum of every feeder.
    # Summing alternatives inflates the rate and collapses the multiplier.
    #
    # perception, AFTER unhooking look/consider. The realistic concerted hour is
    # craft-led: the owner's 40% craft engagement already means the other 60% is
    # spent GATHERING, which is foraging. So one hour yields forage at 60%
    # engagement PLUS one perception craft (alchemy / cooking / enchanting /
    # salvage all map to perception).
    ("perception",     False, (ROUNDS_PER_HOUR * (1 - ENG_CRAFT)) / 6.0 + crafts_hr,
                                                      1.0, "forage+craft hour"),
    ("willpower",      False, special_move_budget,    1.0, "spellcasting primary"),
    # charisma has NO direct player faucet; it is the primary stat behind
    # rhetoric, bartering and manifestation. A face/merchant build grinds
    # bartering and taunts in the same hour; manifestation is a different build.
    ("charisma",       False, crafts_hr + special_move_budget,
                                                      1.0, "barter+rhetoric hour"),
]

SHIPPED = {
    "weapon-combat": 0.23, "unarmed-combat": 0.23, "ranged-combat": 0.5,
    "spellcasting": 0.63, "rhetoric": 0.58, "manifestation": 0.38,
    "skullduggery": 2.0, "search": 2.0, "bartering": 2.0, "salvage": 2.0,
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


def main():
    print("U10b-0 Phase D revision 3 -- solved on MEASURED rates")
    print("clean-hit %.4f (96,723 events, pre-arc) | defence dodge %.1f%% / parry %.1f%%"
          % (CLEAN_HIT, 100 * DODGE, 100 * PARRY))
    print("engagement: combat %.0f%%, gather %.0f%%, craft %.0f%%   (owner)"
          % (100 * ENG_COMBAT, 100 * ENG_GATHER, 100 * ENG_CRAFT))
    print("special-move SHARED budget: %.1f uses/hr across 18 verbs" % special_move_budget)
    print("targets: %d pts/hr combat, %d other, at rank %d\n" % (3, 4, REF_RANK))

    print("%-16s %8s %7s %7s %8s %8s %7s  %s"
          % ("track", "uses/hr", "bonus", "target", "shipped", "solved", "change", "basis"))
    solved = {}
    for name, is_skill, uses, bonus, note in TRACKS:
        tgt = 3.0 if name in COMBAT_TRACKS else 4.0
        m = solve(uses, is_skill, bonus, tgt)
        solved[name] = m
        star = "*" if name in CONFIG_ABSENT else " "
        print("%-16s %8.1f %7.3f %7.0f %8.2f %8.2f %6.1fx%s %s"
              % (name, uses, bonus, tgt, SHIPPED[name], m, m / SHIPPED[name], star, note))

    print("\n* no config.yaml entry today -- D must ADD the key, not edit it.")
    print("  Skills ALSO have a Go-side map (internal/skills/skills.go); stats do not.")
    print("  Update BOTH or test binaries keep the old value.\n")

    print("Per engaged combat round (measured):")
    print("  strength %.3f   dexterity %.3f   weapon-combat %.3f   unarmed-combat %.3f"
          % (str_pr, dex_pr, wc_pr, uc_pr))
    print("  P(weapon entry clean) %.3f over %d swings; P(fist entry clean) %.3f over %d"
          % (P_ENTRY_MAIN, SWINGS_MAIN, P_ENTRY_FIST, SWINGS_FIST))
    print("  -> unarmed-combat earns %.2fx the uses weapon-combat does."
          % (uc_pr / wc_pr))
    print("     That is why their multipliers must DIFFER; equal ones favour the fist.\n")

    print("Drift from the rank-%d anchor:" % REF_RANK)
    row = "  "
    for rank in (0, 10, 25, 40, 50, 60):
        row += "rank %-3d %.2fx   " % (rank, curve(rank) / curve(REF_RANK))
    print(row)
    print("  -> a fresh character progresses at %.1fx the target." % (curve(0) / curve(REF_RANK)))

    c0 = BASE * solved["blacksmithing"] * CRAFT_BONUS
    print("\nSanity: a craft skill's rank-0 chance is %.3f%s"
          % (c0, "  (CLAMPED at 1.0 -- see plan)" if c0 > 1.0 else "  (under the clamp)"))


if __name__ == "__main__":
    main()
