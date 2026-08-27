"""U10b-1 Task 23: re-solve the progression multipliers on the FIRING CONVENTION.

This supersedes u10b_solve_v3.py, which is kept for provenance. Two things
changed under it, and both move the answer:

1. THE MEASURED RATE WAS MISLABELLED. v3's "clean-hit rate 0.5752" is the HIT
   rate. The real clean-hit rate is 0.3856 (37,297 clean + 18,341 deflected =
   55,638 hits over 96,723 events). Fixed in the reader at 987e7e872.

   The correction does not simply lower one input: it SORTS the two numbers
   into different homes. A melee attacker award's `won` is CleanHit (0.3856).
   A special move's `won` is result.Hit (0.5752). v3 used one number for both.

2. THE FIRING CONVENTION CHANGED (this slice). v3 modelled progression as one
   event per weapon ENTRY on a clean hit, plus one per defence type won. The
   convention is now: ONE award per resolved action, for the single
   highest-rolling candidate skill, full weight on a win and
   ProgressionFailureFraction on a loss.

   So a track's rate is no longer uses/hr. It is EFFECTIVE uses/hr:

       eff = uses/hr * (p_win + (1 - p_win) * FRACTION)

   and what this slice changed per site is which of two OLD conventions it is
   being compared against:

       "success-only"  old eff = uses * p_win        (a loss trained nothing)
       "always-full"   old eff = uses * 1.0          (a loss trained fully)

   Sites that were success-only GAIN. Sites that were always-full LOSE. The
   ratio between the two is the multiplier correction this file solves for.

HONESTY ABOUT THE INPUTS. Every row is labelled in the output:
  M = measured from _datafiles/logs/combat-analytics.jsonl (96,723 events).
      PRE-ARC: the log predates U6b/U9/U10 and every phase of U10b. Carried
      forward with that caveat, as v3 did, because it is the only measurement
      that exists.
  D = derived closed-form from shipped constants or from an M row.
  R = owner ruling (engagement, targets).
  J = JUDGEMENT. No measurement basis exists. Confirmed at playtest (Task 25).

  combat-analytics.jsonl is COMBAT-ONLY. Every p_win for search, track,
  forage, salvage, barter, craft and skullduggery is therefore J, not M.
  The single exception is forage, which has its own measurement.
"""
import math

# ── shipped config (_datafiles/config.yaml) ─────────────────────────────────
BASE = 0.12          # BaseProgressionChance
RATE = 2.25          # StatProgressionRate  (STATS ONLY, and NOT the regen channel)
D_BELOW = 3.0        # ProgressionDecayBelowCap
D_ABOVE = 2.0        # ProgressionDecayAboveCap
SOFT = 50.0          # SkillSoftCap / StatProgressionSoftCap
FRACTION = 0.35      # ProgressionFailureFraction   <- the knob this slice added
REGEN_BASE = 0.01    # RegenProgressionBase
REGEN_CURVE = 1.5    # RegenProgressionCurve
ROUND_SECONDS = 4
SPECIAL_MOVE_CD = 4

ROUNDS_PER_HOUR = 3600 / ROUND_SECONDS          # 900
REF_RANK = 25

# ── M: measured ─────────────────────────────────────────────────────────────
CLEAN_HIT = 0.3856      # M. THE CORRECTED RATE. Used where `won` is CleanHit.
HIT = 0.5752            # M. Used where `won` is result.Hit (every special move).
DODGE_SHARE, PARRY_SHARE, BLOCK_SHARE = 0.770, 0.151, 0.080   # M, of DefenseUsed

# D: mid-profile swing counts from calcSwingCount (dex 118, wc 18, SkillWeight
# 5.0, longsword speed 0.7 -> 2 swings; fist at UnarmedSpeed 1.8 x dualWield 1.4
# -> capped 4).
SWINGS_MAIN, SWINGS_FIST = 2, 4

# D: WeaponHits.CleanHit is OR-aggregated across a weapon's swings, and
# attackerCandidates OR-aggregates again across a SKILL's weapons.
P_CLEAN_MAIN = 1 - (1 - CLEAN_HIT) ** SWINGS_MAIN
P_CLEAN_FIST = 1 - (1 - CLEAN_HIT) ** SWINGS_FIST

# ── R: owner engagement rulings (unchanged from v3) ─────────────────────────
ENG_COMBAT, ENG_GATHER, ENG_CRAFT = 0.10, 1.00, 0.40

combat_rounds = ROUNDS_PER_HOUR * ENG_COMBAT               # 90
special_move_budget = combat_rounds / SPECIAL_MOVE_CD      # 22.5, SHARED
crafts_hr = (ROUNDS_PER_HOUR * ENG_CRAFT) / 5.0            # median time_rounds 5
forage_hr = ROUNDS_PER_HOUR / 6.0                          # hardcoded 6-round cd

# D: difficulty bonuses, MEAN (chance is linear in the bonus). Carried from v3.
# Task 13 kept these alive through AwardResolvedScaled; they are NOT dropped.
CRAFT_BONUS = 1.4724      # 126 recipes
SPELL_BONUS = 1.2780      # 59 spells
MANI_BONUS = 1.3393       # 14 manifestation spells


def curve(rank):
    """The standard progression curve. Skills and non-regen stat rolls."""
    if rank <= 0:
        return BASE
    r = rank / SOFT
    if rank <= SOFT:
        return BASE * math.exp(-D_BELOW * r)
    return BASE * math.exp(-D_BELOW) * math.exp(-D_ABOVE * (r - 1.0))


def weight(p_win):
    """Effective value of ONE award that wins with probability p_win.

    A lost award still fires; it is worth FRACTION of a won one because
    ProgressionFailureFraction scales the event multiplier, and chance is
    linear in the multiplier.
    """
    return p_win + (1.0 - p_win) * FRACTION


# ── the combat block, rebuilt for one-award-per-round ───────────────────────
#
# ATTACKER (processAttackerProgression): exactly ONE award per round, for the
# highest-rolling SKILL, with won = did THAT skill's entries clean-hit. v3 paid
# one event per weapon ENTRY and only on a clean hit.
#
# DEFENDER (processDefenderProgression): ONE award per round, for the
# highest-rolling defence, win or lose. v3 paid one per defence type WON, and
# gave a flat +2 dexterity on top.
#
# THE BUILD NOW MATTERS, AND IT DID NOT BEFORE. v3 could model a single
# canonical build (1H + empty offhand) because per-entry awards made
# weapon-combat's rate almost build-independent: 1.09x spread across the three
# builds below. Under Best-of the offhand fist STEALS the single award roughly
# two rounds in three, so weapon-combat's rate now spans 2.48x by build. Solving
# it on one arbitrary build is no longer safe.
#
# The owner's framing settles which one: "roughly balance on time given a
# concerted effort to grind X or Y". Every other track in this file is solved at
# full concentration, so the combat skills are too -- each on the build that
# MAXIMISES it. A player who is not concentrating gets less, which is what the
# concentration framing already implies everywhere else.

P_DEF_WIN = 1.0 - CLEAN_HIT      # D: the defence carried a swing the attack
def_award = weight(P_DEF_WIN)    #    did not cleanly land

# D: which SKILL wins the attacker Best-of in a 1H + fist build. Both hands take
# their SKILL TERM from the same place today -- calcAttackScore reads
# GetCombatSkillLevel, which resolves the MAIN-HAND weapon's tag for every entry
# in the plan, so an offhand fist rolls on a score built from WEAPON-combat's
# rank. That is the known defect owed the moment this slice closes.
#
# Treating the swings as iid then gives the overall maximum to the fist's 4
# draws with probability 4/6.
#
# APPROXIMATION, stated rather than hidden: the hands are not perfectly iid,
# because calcAttackScore subtracts a per-hand dual-wield penalty (ws.penalty)
# that DOES differ between main and offhand. So the fist's draws are centred a
# little lower and 4/6 slightly overstates its share. The shared skill term is
# by far the larger effect, and both go away together when each hand rolls its
# own skill -- at which point this line is the one to re-derive.
SHARE_FIST = SWINGS_FIST / (SWINGS_MAIN + SWINGS_FIST)
SHARE_MAIN = 1.0 - SHARE_FIST

# defence Best-of shares. A shield build can block; the other two cannot, so
# their shares renormalise over dodge and parry.
_ns = DODGE_SHARE + PARRY_SHARE
DODGE_NS, PARRY_NS = DODGE_SHARE / _ns, PARRY_SHARE / _ns

# DefenceSkillAndStat: dodge -> unarmed/dexterity, parry -> weapon/dexterity,
# block -> weapon/STRENGTH. Dexterity IS the primary of both combat skills, so
# dodge and parry pay no separate stat roll; block's strength differs from the
# primary and therefore does.
#
# build -> (weapon-combat, unarmed-combat, dexterity) per engaged round, NEW
BUILDS_NEW = {
    "1H+fist": (SHARE_MAIN * weight(P_CLEAN_MAIN) + PARRY_NS * def_award,
                SHARE_FIST * weight(P_CLEAN_FIST) + DODGE_NS * def_award,
                SHARE_MAIN * weight(P_CLEAN_MAIN) + SHARE_FIST * weight(P_CLEAN_FIST)
                + def_award),
    "2H": (weight(P_CLEAN_MAIN) + PARRY_NS * def_award,
           DODGE_NS * def_award,
           weight(P_CLEAN_MAIN) + def_award),
    "1H+shield": (weight(P_CLEAN_MAIN) + (PARRY_SHARE + BLOCK_SHARE) * def_award,
                  DODGE_SHARE * def_award,
                  weight(P_CLEAN_MAIN) + (DODGE_SHARE + PARRY_SHARE) * def_award),
}
# the same three builds under v3's SHAPE, at the corrected clean-hit rate, so
# the comparison isolates the convention change from the rate correction
BUILDS_OLD = {
    "1H+fist": (P_CLEAN_MAIN + PARRY_NS, P_CLEAN_FIST + DODGE_NS,
                1.0 + P_CLEAN_MAIN + P_CLEAN_FIST + 2.0),
    "2H": (P_CLEAN_MAIN + PARRY_NS, DODGE_NS, 1.0 + P_CLEAN_MAIN + 2.0),
    "1H+shield": (P_CLEAN_MAIN + PARRY_SHARE + BLOCK_SHARE, DODGE_SHARE,
                  1.0 + P_CLEAN_MAIN + 2.0),
}


def at_concentration(table, idx):
    """The rate for a player concentrating on this track: their best build."""
    return max(v[idx] for v in table.values())


wc_pr, v3_wc_pr = at_concentration(BUILDS_NEW, 0), at_concentration(BUILDS_OLD, 0)
uc_pr, v3_uc_pr = at_concentration(BUILDS_NEW, 1), at_concentration(BUILDS_OLD, 1)
dex_pr, v3_dex_pr = at_concentration(BUILDS_NEW, 2), at_concentration(BUILDS_OLD, 2)

# ── strength ────────────────────────────────────────────────────────────────
#
# v3 had strength at 1.164/round because emitAttackerStatGain fired a bare stat
# roll once per exchange. Task 22 DELETED that (a bare stat roll outside the
# firing convention is not progression, owner ruling), which left strength with
# no attack-side faucet. Three faucets replace it, and the concentrating
# strength build is shield + grapple:
#
#   1. block   -- the ONLY defence whose stat differs from the skill primary.
#                 Needs a shield.
#   2. grapple -- Task 22 made unarmed-combat/grapple explicitly strength-stat.
#                 On the shared special-move budget.
#   3. the stamina regen tick -- Task 22 moved this row from
#                 {strength, vitality} to {strength} alone.
#
# Faucets 1 and 2 run on the STANDARD stat channel. Faucet 3 runs on the REGEN
# channel, which has its own base and curve and does NOT multiply by
# StatProgressionRate, so it is converted below before being added.
# The measured 8.0% block share is an ALL-BUILDS figure, and a combatant with
# no shield can never block, so it understates what a shield user does. Divide
# it by the share of combatants who carry one to recover the per-user rate.
SHIELD_CARRY_FRACTION = 0.20        # J: logged combatants carrying a shield
SHIELD_BLOCK_SHARE = BLOCK_SHARE / SHIELD_CARRY_FRACTION
str_block = combat_rounds * SHIELD_BLOCK_SHARE * def_award
str_grapple = special_move_budget * weight(HIT)


def regen_equivalent_uses(depleted_rounds_weighted, rank=REF_RANK):
    """Convert regen-channel ticks into standard-stat-channel equivalent uses.

    regen chance = REGEN_BASE * depletion^REGEN_CURVE * mult * exp(-3r/50)
    standard     = curve(rank) * RATE * mult

    Both carry the SAME per-stat multiplier, so the ratio is multiplier-free and
    the two faucets can be added once expressed in the same unit.
    """
    regen_chance_per_unit = REGEN_BASE * math.exp(-D_BELOW * rank / SOFT)
    return depleted_rounds_weighted * regen_chance_per_unit / (curve(rank) * RATE)


# J: stamina depletion profile over an hour. In combat the pool sits drained;
# for a while afterwards it refills; the rest of the hour it is full and
# regenTickChance returns 0 outright (ratio >= 1.0).
_sp_depleted = (combat_rounds * 0.50 ** REGEN_CURVE +
                combat_rounds * 0.25 ** REGEN_CURVE)
str_regen = regen_equivalent_uses(_sp_depleted)

str_hr = str_block + str_grapple + str_regen
str_old_hr = combat_rounds * 1.164   # v3: emitAttackerStatGain, one bare roll
                                     # per exchange. DELETED by Task 22.

# ── concentration ───────────────────────────────────────────────────────────
#
# RunConcentrationContest fires per DAMAGE INSTANCE while casting, and it was
# success-only: a caster who lost the hold trained nothing at all. Now every
# disruption trains, so this is a large GAIN in event COUNT.
#
# It is NOT the runaway faucet a first pass suggests, because
# ConcentrationDamageThresholdPct (shipped 10) means damage under 10% of the
# pool never rolls. Chip damage generates nothing. Only substantial hits, taken
# while actually mid-cast, contest at all.
CONC_ROLLS_PER_CAST_ROUND = 0.5   # J: hits >= 10% of pool, per round mid-cast
P_CAST_ROUNDS = 0.5               # J: share of combat rounds spent mid-cast
P_CONC_HOLD = 0.35                # J: caster holds, at REF_RANK
conc_hr = combat_rounds * P_CAST_ROUNDS * CONC_ROLLS_PER_CAST_ROUND

# ── the track table ─────────────────────────────────────────────────────────
#
# Each row supplies its NEW effective uses/hr and its OLD effective uses/hr, so
# the table separates two things v3 could not: what THIS SLICE did to the rate
# (new/old) and how far the SHIPPED multiplier already sat from a solve.
#
# old convention per site:
#   success-only  old = uses * p          a loss trained nothing
#   always-full   old = uses * 1.0        a loss trained fully
#   combat        stated explicitly; the shape changed, not just the weight


def succ(uses, p):
    """A site that was success-only: (new, old)."""
    return uses * weight(p), uses * p


def full(uses, p):
    """A site that paid a full event win or lose: (new, old)."""
    return uses * weight(p), uses * 1.0


def mixed(new, old):
    return new, old


_sm = special_move_budget
_crafts_new, _crafts_old = succ(crafts_hr, 0.85)
_spell_new = _sm * weight(HIT) + conc_hr * weight(P_CONC_HOLD)
_spell_old = _sm * HIT + conc_hr * P_CONC_HOLD
_search_new, _search_old = full(forage_hr, 0.97)

# (name, is_skill, new_eff, old_eff, bonus, note)
TRACKS = [
    ("weapon-combat",  True,  *mixed(combat_rounds * wc_pr, combat_rounds * v3_wc_pr),
     1.0, "D Best-of, one award/round"),
    ("unarmed-combat", True,  *mixed(combat_rounds * uc_pr, combat_rounds * v3_uc_pr),
     1.0, "D Best-of, one award/round"),
    ("ranged-combat",  True,  *succ(_sm, HIT),         1.0, "M hit"),
    ("spellcasting",   True,  *mixed(_spell_new, _spell_old),
     SPELL_BONUS, "M hit + J concentration"),
    ("rhetoric",       True,  *succ(_sm, HIT),         1.0, "M hit"),
    ("manifestation",  True,  *succ(ROUNDS_PER_HOUR / 36.0, 0.80),
     MANI_BONUS, "J p; conjure cd"),
    ("skullduggery",   True,  *succ(180.0, 0.50),      1.0, "J p; shadow cd"),
    ("search",         True,  *mixed(_search_new, _search_old),
     1.0, "M forage p; search/track now pay on a loss"),
    ("bartering",      True,  *mixed(crafts_hr, crafts_hr),
     1.0, "buy/sell pass won=true: UNCHANGED"),
    ("salvage",        True,  *succ(crafts_hr, 0.60),  1.0, "J p; no craft bonus"),
    ("blacksmithing",  True,  _crafts_new, _crafts_old, CRAFT_BONUS, "J p"),
    ("alchemy",        True,  _crafts_new, _crafts_old, CRAFT_BONUS, "J p"),
    ("tailoring",      True,  _crafts_new, _crafts_old, CRAFT_BONUS, "J p"),
    ("cooking",        True,  _crafts_new, _crafts_old, CRAFT_BONUS, "J p"),
    ("jewelcrafting",  True,  _crafts_new, _crafts_old, CRAFT_BONUS, "J p"),
    ("enchanting",     True,  _crafts_new, _crafts_old, CRAFT_BONUS, "J p"),

    ("strength",       False, *mixed(str_hr, str_old_hr),
     1.0, "block+grapple+SP regen replaces the deleted roll"),
    ("dexterity",      False, *mixed(combat_rounds * dex_pr, combat_rounds * 4.787),
     1.0, "attacker+defender award primary"),
    ("perception",     False, *mixed(
        (ROUNDS_PER_HOUR * (1 - ENG_CRAFT)) / 6.0 * weight(0.97) + _crafts_new,
        (ROUNDS_PER_HOUR * (1 - ENG_CRAFT)) / 6.0 * 0.97 + _crafts_old),
     1.0, "forage+craft hour"),
    ("willpower",      False, *mixed(_spell_new, _spell_old),
     1.0, "spellcasting primary + concentration"),
    ("charisma",       False, *mixed(crafts_hr + _sm * weight(HIT),
                                     crafts_hr + _sm * HIT),
     1.0, "barter+rhetoric hour"),
]

SHIPPED = {
    "weapon-combat": 1.27, "unarmed-combat": 0.69, "ranged-combat": 4.98,
    "spellcasting": 3.90, "rhetoric": 4.98, "manifestation": 4.46,
    "skullduggery": 0.83, "search": 1.00, "bartering": 2.07, "salvage": 2.07,
    "blacksmithing": 1.41, "alchemy": 1.41, "tailoring": 1.41,
    "cooking": 1.41, "jewelcrafting": 1.41, "enchanting": 1.41,
    # STATS live ONLY in config.yaml (there is no Go-side stat map). These are
    # v3's SOLVED values, which Phase D did ship -- not the pre-D values.
    "strength": 0.48, "dexterity": 0.12, "perception": 0.41,
    "willpower": 2.21, "charisma": 0.70,
}
COMBAT_TRACKS = {"weapon-combat", "unarmed-combat", "ranged-combat",
                 "spellcasting", "rhetoric", "strength", "dexterity", "willpower"}


def solve(eff_uses, is_skill, bonus, target, rank=REF_RANK):
    return target / (eff_uses * curve(rank) * (1.0 if is_skill else RATE) * bonus)


def main():
    print("U10b-1 Task 23 -- re-solved on the FIRING CONVENTION (supersedes v3)")
    print("clean-hit %.4f CORRECTED (v3 used 0.5752, which is the HIT rate) | "
          "hit %.4f | failure fraction %.2f" % (CLEAN_HIT, HIT, FRACTION))
    print("attacker Best-of share: fist %.3f / weapon %.3f  (iid, 4 vs 2 swings)"
          % (SHARE_FIST, SHARE_MAIN))
    print("labels: M measured  D derived  R owner ruling  J JUDGEMENT (playtest)\n")

    print("%-16s %8s %8s %7s %8s %8s %7s  %s" %
          ("track", "old/hr", "new/hr", "slice", "solved", "shipped", "adjust", "note"))
    print("-" * 112)

    solved = {}
    for name, is_skill, new_eff, old_eff, bonus, note in TRACKS:
        target = 3 if name in COMBAT_TRACKS else 4
        m = solve(new_eff, is_skill, bonus, target)
        solved[name] = m
        ship = SHIPPED[name]
        print("%-16s %8.1f %8.1f %6.2fx %8.2f %8.2f %6.2fx  %s" %
              (name, old_eff, new_eff, new_eff / old_eff, m, ship, m / ship, note))

    print("\nCombat, per engaged round, EACH AT ITS CONCENTRATING BUILD:")
    print("  weapon-combat  %.3f  (old shape at the corrected rate %.3f -> %.2fx)"
          % (wc_pr, v3_wc_pr, wc_pr / v3_wc_pr))
    print("  unarmed-combat %.3f  (old shape at the corrected rate %.3f -> %.2fx)"
          % (uc_pr, v3_uc_pr, uc_pr / v3_uc_pr))
    print("  P(clean | weapon entry) %.3f over %d swings; P(clean | fist) %.3f over %d"
          % (P_CLEAN_MAIN, SWINGS_MAIN, P_CLEAN_FIST, SWINGS_FIST))

    print("\nStrength faucets, standard-channel-equivalent uses/hr:")
    print("  block (SHIELD build) %.1f | grapple %.1f | SP regen tick %.2f"
          % (str_block, str_grapple, str_regen))
    print("  A no-shield grappler gets only %.1f, so strength's rate now varies"
          % (str_grapple + str_regen))
    print("  ~%.1fx by build. The SP regen tick is a rounding error: it runs on"
          % (str_hr / (str_grapple + str_regen)))
    print("  RegenProgressionBase %.3f and skips StatProgressionRate entirely."
          % REGEN_BASE)
    print("  GRAPPLE, not the regen tick, is what replaces the deleted roll.")

    print("")
    print("  Per-build spread -- the convention INTRODUCED this, and it is")
    print("  the main thing to watch at playtest:")
    print("    %-11s %8s %8s %8s" % ("build", "weapon", "unarmed", "total"))
    for _b, _v in BUILDS_NEW.items():
        _o = BUILDS_OLD[_b]
        print("    %-11s %8.3f %8.3f %8.3f   (was total %.3f)"
              % (_b, _v[0], _v[1], _v[0] + _v[1], _o[0] + _o[1]))
    print("  weapon-combat now spans %.2fx by build against v3's %.2fx."
          % (max(v[0] for v in BUILDS_NEW.values()) / min(v[0] for v in BUILDS_NEW.values()),
             max(v[0] for v in BUILDS_OLD.values()) / min(v[0] for v in BUILDS_OLD.values())))
    print("  But TOTAL events per round are now nearly FLAT across builds where")
    print("  they used to favour the empty offhand by ~53%. That is the firing")
    print("  convention removing the empty-offhand advantage STRUCTURALLY,")
    print("  which no per-skill multiplier could have done.")

    print("\nVITALITY: this slice took it OFF the stamina regen tick (Task 22")
    print("  moved that row to strength alone), so it has no solved row here.")
    print("  Deliberately NOT retuned, for a sourced reason:")
    print("    tools/balance/u10b_vitality_solve.py puts regen at 0.33/hr")
    print("    against crit-toughen's 2.67/hr, and that was BEFORE the U10b-0")
    print("    damage-taken faucet (PoolProgressionStats) multiplied its events")
    print("    further. Losing one of two regen pools is low single digits of")
    print("    vitality's rate -- well inside the J inputs' noise.")
    print("  That file's own conclusion also says raising the multiplier is the")
    print("  WRONG lever for vitality (at a solved value the rank-0 chance")
    print("  clamps to 100%). Compensating a ~4% cut with the dial would be")
    print("  the exact move it argues against. Left at the shipped 2.2.")

    print("\nDrift from the rank-%d anchor:" % REF_RANK)
    for r in (0, 10, 25, 40, 50, 60):
        print("  rank %-3d %.2fx" % (r, curve(r) / curve(REF_RANK)), end="")
    print("\n")
    return solved


if __name__ == "__main__":
    main()
