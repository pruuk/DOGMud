"""U6b modelling gate, items 7-9: counter frequency, the unowned contest
family at x5, and defence cost load.

Method: NUMERIC INTEGRATION (closed-form normal CDFs for single-defender
contests; a 6001-point z-grid over the attacker's roll for best-of-N defence
sets), cross-checked by fixed-seed Monte Carlo (2,000,000 samples/cell) on
anchor cells. Sanity anchors asserted: parity pre-floor win 50%, parity
pre-floor crit ~2.28%.

Every formula is mirrored from source, not prose (verified 2026-08-19):
  contest        internal/contest/contest.go        Run + RunWithFloors
                 - one attack roll, every defence rolled with the ATTACKER's
                   stdDev; best defence = smallest attack-positive margin
                 - floor flips the outcome with prob f, stamps +-1 sentinel
                   margin (near-zero z -> a floored outcome can never crit,
                   and in grapple drift it lands in the Hold band)
  stdDev         internal/dice/dice.go:435          mean * RollSpread (0.15)
  crit           internal/combat/margin_crit.go     margin/(stdDev*sqrt2) >= 2.0
  def crit floor internal/combat/crit_floor.go      MinDefenseCritChance 0.01,
                 promotion blocked on fumble and on floored outcomes (melee
                 applyCritFloors); channel path promotes via DefenseContestCrit
                 inside defenceDamageMultiplier, floored handled before it
  melee fumble   combat_helpers.go:965 self z <= -2.0, branch returns FIRST,
                 so a fumbled swing cannot be defensively crit
  defence scores characters/combat.go:312 GetDefenseScoreFor
                 dodge Dex+unarmed*W, parry Dex+weapon*W(+rating),
                 block (Str+Dex)/2+weapon*W(+rating); effectiveness
                 dodge .95 / parry .97 / block 1.05 (config)
  riposte        hooks/combat_shared_helpers.go:241
                 CalcRawDamage(Str, combatSkill, 0.5, Physical), mitigated,
                 PARRY-crit only today; dodge crit -> auto-trip, block crit ->
                 auto-bash (different damage). This model prices every
                 defensive crit at riposte value - stated approximation.
  damage         combat/damage_pipeline.go CalcRawDamage =
                 stat * SkillMultiplier(rank) * itemMult * scale * global
                 SkillMultiplier = 1 + 2*sqrt(min(rank,50)/50)
  defence mult   combat/defence_multiplier.go DefenceMitigation:
                 bare win 0.5 rising linearly to 1.0 at margin 2.0;
                 floored save exactly 0.5; def crit 0.0
  flee           combat/flee.go  Dex + Skullduggery*25 vs Dex + Unarmed*25,
                 fleer prone/supine 0.5x, per-blocker RunContest
  grapple init   combat/grapple.go AttemptGrapple Dex + combatSkill*1 both,
                 defender prone 0.3x, attacker prone 0.5x, crit-failure on
                 SELF z <= -2 on a failed attempt
  drift          hooks/Position_GrappleTick.go
                 PRE-U6b-Task-14 live code (what item8_drift's 'current'
                 variant models): score = 0.7*Str + 0.3*Dex + coef*Unarmed
                 (2.2 aggr / 2.0 def); z = res.Margin / res.AttackRoll.StdDev
                 missing sqrt(2), so the live z was inflated ~41%.
                 FIXED by Task 14: coef -> SkillWeight both sides, aggressor
                 edge -> GrappleAggressorDriftBonus (whole-score multiplier,
                 solved by item8_drift_solve_aggressor_bonus), z divided by
                 stdDev*sqrt(2). Routed through combat.RunContest, so the
                 12.5% floor flip applies and its sentinel z lands in Hold
                 (kept deliberately, documented at the z site).
                 tiers (state/position/outcomes.go:22): hold<0.5, 1-step<1.0,
                 2-step<2.0, 3-step>=2.0; negative side degrade/reversal/escape
  submission     combat/submission.go Str + Unarmed*SubSkillWeight(1.5) vs
                 Str + Vit + Unarmed*1.5; tier from SELF z (crit>=2, bad<-1)
  throw          usercommands/throw.go:271,299-302
                 atk Dex + Skullduggery*W  vs  def Dex + Per*W*0.5,
                 binary full damage CalcRawDamage(Dex, skull, itemMult),
                 fumble self z <= -2 detonates in hand
  steal          actions/steal.go:98-104,171
                 atk (Dex + (1+2*sqrt(min(r,50)/50))*25) * StealSkillMultiplier
                 (shipped absent -> Go default 1.0), +25 hidden;
                 def RAW Perception, no skill
  costs          internal/costs Calc = base * enc * skillmult * modifier,
                 product-clamped at 6.0; defence base 1.0 shipped, modifiers
                 dodge 1.25 / parry 1.10 / block 1.15 (NOT base 2 - the prompt
                 said base 2; shipped DefenceBaseStaminaCost is 1.0)

Shipped values read via unified_resolution_model.SHIPPED (from config.yaml).
Reuses tools/balance/unified_resolution_model.py machinery (action_cost,
PoolTrace, regen_per_hook, ranged schedule) WITHOUT modifying it.
"""
import math
import random
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import unified_resolution_model as urm  # noqa: E402

PHI = lambda z: 0.5 * (1.0 + math.erf(z / math.sqrt(2.0)))
SQRT2 = math.sqrt(2.0)

ROLL_SPREAD = 0.15
W = 5.0                    # SkillWeight, shipped
FLOOR = 0.125              # ContestFloor, shipped
CRIT_T = 2.0               # ContestCritThreshold, Go const
MIN_DEF_CRIT = 0.01        # MinDefenseCritChance, shipped
MIN_ATK_CRIT = 0.01        # MinAttackCritChance, shipped
MELEE_SCALE = 0.52         # MeleeDamageScale, shipped
GLOBAL_MULT = 0.5          # GlobalDamageMultiplier, shipped
PHYS_MIT_CAP = 0.75
CRIT_DMG_BASE = 2.0        # CritDamageBase, shipped
CRIT_DMG_PER_SKILL = 0.05  # CritDamagePerSkill, shipped
EFF = {"dodge": 0.95, "parry": 0.97, "block": 1.05}

# grid for best-of-N integration over the attacker's own z
GRID_N = 6001
GRID_LIM = 6.0
_ZS = [-GRID_LIM + 2 * GRID_LIM * i / (GRID_N - 1) for i in range(GRID_N)]
_DZ = 2 * GRID_LIM / (GRID_N - 1)
_PDF = [math.exp(-0.5 * z * z) / math.sqrt(2 * math.pi) for z in _ZS]


def skill_dmg_mult(rank):
    r = min(max(rank, 0), 50)
    return 1.0 + 2.0 * math.sqrt(r / 50.0)


def raw_damage(stat, rank, item_mult):
    return stat * skill_dmg_mult(rank) * item_mult * MELEE_SCALE * GLOBAL_MULT


def mitigated(raw, mit):
    return raw * (1.0 - min(max(mit, 0.0), PHYS_MIT_CAP))


def riposte_damage(def_str, def_combat_skill, atk_mit=0.0):
    return mitigated(raw_damage(def_str, def_combat_skill, 0.5), atk_mit)


# ---------------------------------------------------------------- contests

def single(a, d):
    """Closed-form single-defender contest. Returns dict of pre/post-floor
    probabilities. Both rolls use the ATTACKER's stdDev."""
    sd = a * ROLL_SPREAD if a >= 1 else 1.0
    m0 = (a - d) / (sd * SQRT2)          # mean of normalized attack margin
    p_win_roll = PHI(m0)
    p_atk_crit_roll = 1.0 - PHI(CRIT_T - m0)
    p_def_win_roll = 1.0 - p_win_roll
    p_def_crit_roll = PHI(-CRIT_T - m0)
    p_win = FLOOR + (1 - 2 * FLOOR) * p_win_roll
    # channel path: floored outcomes cannot crit; 1% floor promotes rolled
    # non-crit wins on each side (crit_floor.go)
    p_atk_crit = (1 - FLOOR) * (p_atk_crit_roll +
                                MIN_ATK_CRIT * (p_win_roll - p_atk_crit_roll))
    p_def_crit = (1 - FLOOR) * (p_def_crit_roll +
                                MIN_DEF_CRIT * (p_def_win_roll - p_def_crit_roll))
    return dict(sd=sd, m0=m0, p_win_roll=p_win_roll, p_win=p_win,
                p_atk_crit=p_atk_crit, p_def_crit=p_def_crit,
                p_def_win_roll=p_def_win_roll, p_def_crit_roll=p_def_crit_roll)


def best_of_n(a, defs, fumble_gate=True):
    """Best-of-N defence contest via numeric integration over the attacker's
    self z. defs: list of fully-modified defence scores. Returns
    (p_def_win_roll, p_def_crit_roll, p_def_win_nofumble_roll) pre-floor."""
    sd = a * ROLL_SPREAD if a >= 1 else 1.0
    t = CRIT_T * sd * SQRT2
    p_win = p_crit = p_win_nf = 0.0
    for z, pdf in zip(_ZS, _PDF):
        atk_val = a + sd * z
        prod_win = 1.0
        prod_crit = 1.0
        for d in defs:
            prod_win *= PHI((atk_val - d) / sd)          # P(D_i < A)
            prod_crit *= PHI((atk_val + t - d) / sd)     # P(D_i < A + t)
        w = pdf * _DZ
        p_def_win_here = 1.0 - prod_win
        p_def_crit_here = 1.0 - prod_crit
        p_win += w * p_def_win_here
        if (not fumble_gate) or z > -2.0:
            p_crit += w * p_def_crit_here
            p_win_nf += w * p_def_win_here
    return p_win, p_crit, p_win_nf


def def_crit_final(a, defs, fumble_gate=True):
    """Post-floor defensive-crit probability per swing (melee shape:
    fumble branch returns first; floored outcomes neither crit nor promote)."""
    p_win, p_crit, p_win_nf = best_of_n(a, defs, fumble_gate)
    return (1 - FLOOR) * (p_crit + MIN_DEF_CRIT * (p_win_nf - p_crit))


def expected_damage_multiplier(a, defs):
    """E[damage multiplier] under the U6 defence-multiplier curve (channel
    path shape, single or multi defence): attack win 1.0 (+crit bypass
    handled by caller), floored save 0.5, def win 0.5..0 by margin, def
    crit 0. Pre-crit-scaling; integration over attacker z."""
    sd = a * ROLL_SPREAD if a >= 1 else 1.0
    # integrate joint: for def-win outcomes we need the margin distribution of
    # the best defence. Use MC-free approach: for each attacker z, the best
    # defence margin CDF is known; integrate the multiplier over it.
    e_mult_roll = 0.0
    p_win_roll = 0.0
    m_steps = 400
    for z, pdf in zip(_ZS, _PDF):
        atk_val = a + sd * z
        w = pdf * _DZ
        # P(all defences below A) -> attack win, mult 1.0
        prod = 1.0
        for d in defs:
            prod *= PHI((atk_val - d) / sd)
        p_win_roll += w * prod
        e_mult_roll += w * prod * 1.0
        # defence win: margin_def = maxD - A in (0, inf). mult(m) with
        # m_norm = m/(sd*sqrt2): 0.5-0.25*m_norm for m_norm in (0,2), 0 above.
        for j in range(m_steps):
            mn_lo = 2.0 * j / m_steps
            mn_hi = 2.0 * (j + 1) / m_steps
            lo = atk_val + mn_lo * sd * SQRT2
            hi = atk_val + mn_hi * sd * SQRT2
            prod_lo = 1.0
            prod_hi = 1.0
            for d in defs:
                prod_lo *= PHI((lo - d) / sd)
                prod_hi *= PHI((hi - d) / sd)
            p_band = prod_hi - prod_lo   # P(maxD in (lo,hi))
            mn_mid = 0.5 * (mn_lo + mn_hi)
            mult = 1.0 - (0.5 + 0.5 * (mn_mid / CRIT_T))  # 0.5 -> 0.0
            e_mult_roll += w * p_band * mult
        # m_norm >= 2: def crit, mult 0 - contributes nothing
    # floors: with prob f the outcome flips. flipped loss->win: full mult 1.0.
    # flipped win->loss: floored save, mult 0.5.
    e_mult = ((1 - FLOOR) * e_mult_roll
              + FLOOR * (1.0 - p_win_roll) * 1.0
              + FLOOR * p_win_roll * 0.5)
    return e_mult, p_win_roll


# ---------------------------------------------------------------- fixtures

class Tier:
    def __init__(self, name, stat=None, skill=None, stats=None, skills=None):
        self.name = name
        if stats is None:
            stats = {k: stat for k in ("strength", "dexterity", "perception",
                                       "vitality", "willpower", "charisma")}
            skills = {k: skill for k in ("weapon-combat", "unarmed-combat",
                                         "skullduggery", "ranged-combat",
                                         "rhetoric", "spellcasting")}
        self.stats = stats
        self.skills = skills

    def s(self, k):
        return self.stats[k]

    def k(self, k):
        return self.skills[k]

    def melee_attack(self):
        return self.s("dexterity") + self.k("weapon-combat") * W

    def defence_set(self, shield=True):
        dodge = (self.s("dexterity") + self.k("unarmed-combat") * W) * EFF["dodge"]
        parry = (self.s("dexterity") + self.k("weapon-combat") * W) * EFF["parry"]
        block = ((self.s("strength") + self.s("dexterity")) / 2.0
                 + self.k("weapon-combat") * W) * EFF["block"]
        return [dodge, parry, block] if shield else [dodge, parry]

    def dodge_only(self):
        return [(self.s("dexterity") + self.k("unarmed-combat") * W) * EFF["dodge"]]

    def ranged_def_set(self, shield=True):
        dodge = (self.s("dexterity") + self.k("unarmed-combat") * W) * EFF["dodge"]
        block = ((self.s("strength") + self.s("dexterity")) / 2.0
                 + self.k("weapon-combat") * W) * EFF["block"]
        return [dodge, block] if shield else [dodge]


NOVICE = Tier("novice", 100, 5)
JOURNEY = Tier("journeyman", 120, 25)
ADEPT = Tier("adept", 135, 40)
# Meirok from _datafiles/world/dogmud/users/3.yaml (base+training); Dex uses
# the memory-verified effective 110 (98 base + 12 gear). Other stats base.
MEIROK = Tier("meirok",
              stats=dict(strength=115, dexterity=110, perception=101,
                         vitality=104, willpower=113, charisma=93),
              skills={"weapon-combat": 69, "unarmed-combat": 57,
                      "skullduggery": 50, "ranged-combat": 1,
                      "rhetoric": 55, "spellcasting": 52})
TRASH = Tier("trash mob", 90, 1)
TIERS = (NOVICE, JOURNEY, ADEPT, MEIROK)
RATIOS = (0.5, 0.8, 1.2, 1.5, 2.0)


def scaled_opponent(tier, r):
    return Tier(f"{r:g}x", stats={k: v * r for k, v in tier.stats.items()},
                skills={k: v * r for k, v in tier.skills.items()})


# ---------------------------------------------------------------- MC check

def mc_check_best_of_n(a, defs, n=2_000_000, seed=1337):
    rng = random.Random(seed)
    sd = a * ROLL_SPREAD
    t = CRIT_T * sd * SQRT2
    crit = win = 0
    for _ in range(n):
        za = rng.normalvariate(0, 1)
        atk = a + sd * za
        maxd = max(d + sd * rng.normalvariate(0, 1) for d in defs)
        if maxd > atk:
            win += 1
            if za > -2.0 and maxd >= atk + t:
                crit += 1
    return win / n, crit / n


# ---------------------------------------------------------------- sections

def sec(title):
    print("\n" + "=" * 78)
    print(title)
    print("=" * 78)


def sanity():
    sec("SANITY ANCHORS")
    c = single(300.0, 300.0)
    print(f"parity pre-floor win        {c['p_win_roll']*100:6.2f}%  (want 50.00)")
    print(f"parity pre-floor atk crit   {(1.0-PHI(2.0))*100:6.3f}% "
          f"(want ~2.275; model raw {c['p_atk_crit']/(1-FLOOR)*100:5.3f}% incl 1% floor promo)")
    assert abs(c['p_win_roll'] - 0.5) < 1e-9
    assert abs((1.0 - PHI(2.0)) - 0.02275) < 0.0001
    # MC cross-check of one best-of-3 cell: Meirok defending a trash swing
    a = TRASH.melee_attack()
    defs = MEIROK.defence_set()
    p_win_i, p_crit_i, _ = best_of_n(a, defs)
    p_win_mc, p_crit_mc = mc_check_best_of_n(a, defs)
    print(f"MC cross-check (2M, seed 1337) meirok-def vs trash-atk: "
          f"defwin int {p_win_i:.4f} mc {p_win_mc:.4f} | "
          f"defcrit int {p_crit_i:.4f} mc {p_crit_mc:.4f}")
    assert abs(p_win_i - p_win_mc) < 0.002 and abs(p_crit_i - p_crit_mc) < 0.002
    # parity best-of-1 MC
    p_win_i, p_crit_i, _ = best_of_n(300.0, [300.0])
    p_win_mc, p_crit_mc = mc_check_best_of_n(300.0, [300.0])
    print(f"MC cross-check parity single: defwin int {p_win_i:.4f} mc {p_win_mc:.4f}"
          f" | defcrit int {p_crit_i:.4f} mc {p_crit_mc:.4f}")
    assert abs(p_win_i - p_win_mc) < 0.002 and abs(p_crit_i - p_crit_mc) < 0.002


def item7_counters():
    sec("ITEM 7 - COUNTER FREQUENCY AND WORTH (defensive crit -> counter)")
    print("Defensive crit bar 2.0 sigma on the normalized defence margin; floor")
    print("promotion 1% of non-crit defensive wins; fumbled swings excluded;")
    print("floored outcomes excluded. Counter priced at riposte value =")
    print("CalcRawDamage(defStr, defCombatSkill, 0.5, Physical) (parry-crit's")
    print("riposte; dodge->auto-trip and block->auto-bash differ - stated approx).")
    print("\n--- P(defensive crit) per SWING + expected counter damage/round")
    print("    (attacker swings once/round; scale linearly for faster weapons)")
    hdr = f"{'defender':12s}{'attacker':16s}{'atkScore':>9s}{'P(defWin)':>10s}{'P(defCrit)':>11s}{'riposte':>9s}{'ctr/rd':>8s}"
    print(hdr)
    for defender in TIERS:
        defs = defender.defence_set(shield=True)
        rip = riposte_damage(defender.s("strength"), defender.k("weapon-combat"))
        opponents = [TRASH, Tier("parity", defender.s("dexterity"), defender.k("weapon-combat"))]
        opponents += [scaled_opponent(defender, r) for r in RATIOS]
        for atk in opponents:
            a = atk.melee_attack()
            p_win, p_crit, p_win_nf = best_of_n(a, defs)
            p_crit_f = (1 - FLOOR) * (p_crit + MIN_DEF_CRIT * (p_win_nf - p_crit))
            p_win_f = (1 - FLOOR) * p_win + FLOOR * (1 - p_win)
            print(f"{defender.name:12s}{atk.name:16s}{a:9.0f}{p_win_f*100:9.1f}%"
                  f"{p_crit_f*100:10.2f}%{rip:9.1f}{p_crit_f*rip:8.2f}")
        print()

    print("--- mob-vs-player both ways (trash mob: dodge-only defence set, skill 1)")
    print(f"{'attacker':14s}{'defender':14s}{'P(defCrit)':>11s}{'riposte':>9s}{'ctr dmg/rd':>11s}")
    for player in TIERS:
        # player attacks trash mob: mob's counter chance against the player
        a = player.melee_attack()
        p_win, p_crit, p_win_nf = best_of_n(a, TRASH.dodge_only())
        pc = (1 - FLOOR) * (p_crit + MIN_DEF_CRIT * (p_win_nf - p_crit))
        rip_mob = riposte_damage(TRASH.s("strength"), TRASH.k("weapon-combat"))
        print(f"{player.name:14s}{'trash mob':14s}{pc*100:10.3f}%{rip_mob:9.1f}{pc*rip_mob:11.3f}")
        # trash mob attacks player
        a = TRASH.melee_attack()
        p_win, p_crit, p_win_nf = best_of_n(a, player.defence_set())
        pc = (1 - FLOOR) * (p_crit + MIN_DEF_CRIT * (p_win_nf - p_crit))
        rip = riposte_damage(player.s("strength"), player.k("weapon-combat"))
        print(f"{'trash mob':14s}{player.name:14s}{pc*100:10.3f}%{rip:9.1f}{pc*rip:11.3f}")

    print("\n--- CROSS-ROOM IMMUNITY: counter damage a kiting shooter avoids per")
    print("    SHOT by staying adjacent (post-U6b same-room shot earns a melee")
    print("    counter on a defensive crit; cross-room earns none by design).")
    print("    Shot cadence: 1 shot / 4 rounds steady state (capacity 1, cd 4).")
    print(f"{'shooter':12s}{'target':12s}{'shotScore':>10s}{'P(defCrit)':>11s}{'ctr avoided/shot':>17s}{'shot dmg':>9s}{'ratio':>7s}")
    for shooter in (NOVICE, JOURNEY, ADEPT):
        a = shooter.s("perception") + shooter.k("ranged-combat") * W
        shot = mitigated(raw_damage(shooter.s("perception"),
                                    shooter.k("ranged-combat"), 1.0), 0.0)
        for target in (TRASH, Tier("parity", shooter.s("perception"), shooter.k("ranged-combat")),
                       scaled_opponent(shooter, 1.5), scaled_opponent(shooter, 2.0)):
            # target defends shot with dodge only (mob-shaped) - conservative
            defs = target.dodge_only()
            p_win, p_crit, p_win_nf = best_of_n(a, defs, fumble_gate=False)
            pc = (1 - FLOOR) * (p_crit + MIN_DEF_CRIT * (p_win_nf - p_crit))
            ctr = pc * riposte_damage(target.s("strength"),
                                      target.k("weapon-combat"))
            ratio = ctr / shot if shot else 0.0
            print(f"{shooter.name:12s}{target.name:12s}{a:10.0f}{pc*100:10.2f}%"
                  f"{ctr:17.2f}{shot:9.1f}{ratio:7.2f}")
        print()


def item8_family():
    sec("ITEM 8 - THE UNOWNED FAMILY AT x5")

    print("--- FLEE: Dex + Skullduggery*25 vs Dex + Unarmed*25  ->  x5 both")
    print(f"{'fleer':12s}{'blocker':14s}{'P(flee) BEFORE':>15s}{'P(flee) AFTER':>15s}{'delta':>8s}")
    cells = []
    for fleer in TIERS:
        blockers = [TRASH, Tier("parity", fleer.s("dexterity"), fleer.k("skullduggery"))]
        blockers += [scaled_opponent(fleer, r) for r in (0.5, 1.5, 2.0)]
        for b in blockers:
            a_b = fleer.s("dexterity") + fleer.k("skullduggery") * 25
            d_b = b.s("dexterity") + b.k("unarmed-combat") * 25
            a_a = fleer.s("dexterity") + fleer.k("skullduggery") * W
            d_a = b.s("dexterity") + b.k("unarmed-combat") * W
            pb = single(a_b, d_b)["p_win"]
            pa = single(a_a, d_a)["p_win"]
            cells.append((fleer.name, b.name, pb, pa))
            print(f"{fleer.name:12s}{b.name:14s}{pb*100:14.1f}%{pa*100:14.1f}%"
                  f"{(pa-pb)*100:+7.1f}")
        print()
    # headline pair: novice fleeing a Meirok-class veteran blocker
    a_b = NOVICE.s("dexterity") + NOVICE.k("skullduggery") * 25
    d_b = MEIROK.s("dexterity") + MEIROK.k("unarmed-combat") * 25
    a_a = NOVICE.s("dexterity") + NOVICE.k("skullduggery") * W
    d_a = MEIROK.s("dexterity") + MEIROK.k("unarmed-combat") * W
    print(f"novice fleeing veteran(meirok):  BEFORE {single(a_b, d_b)['p_win']*100:.1f}%"
          f"  AFTER {single(a_a, d_a)['p_win']*100:.1f}%"
          f"   (scores {a_b:.0f}v{d_b:.0f} -> {a_a:.0f}v{d_a:.0f})")
    a_b = MEIROK.s("dexterity") + MEIROK.k("skullduggery") * 25
    d_b = TRASH.s("dexterity") + TRASH.k("unarmed-combat") * 25
    a_a = MEIROK.s("dexterity") + MEIROK.k("skullduggery") * W
    d_a = TRASH.s("dexterity") + TRASH.k("unarmed-combat") * W
    print(f"veteran(meirok) fleeing trash:   BEFORE {single(a_b, d_b)['p_win']*100:.1f}%"
          f"  AFTER {single(a_a, d_a)['p_win']*100:.1f}%"
          f"   (scores {a_b:.0f}v{d_b:.0f} -> {a_a:.0f}v{d_a:.0f})")

    print("\n--- GRAPPLE INITIATION: Dex + combatSkill*1 both  ->  x5 both")
    print("    (standing vs standing; prone mods 0.3/0.5 unchanged here)")
    print(f"{'attacker':12s}{'defender':14s}{'P BEFORE':>9s}{'P AFTER':>9s}"
          f"{'critFail BEFORE':>16s}{'atkFumble AFTER':>16s}")
    for atk in TIERS:
        for d in (TRASH, Tier("parity", atk.s("dexterity"), atk.k("unarmed-combat")),
                  scaled_opponent(atk, 1.5)):
            a_b = atk.s("dexterity") + atk.k("unarmed-combat") * 1
            d_b = d.s("dexterity") + d.k("unarmed-combat") * 1
            a_a = atk.s("dexterity") + atk.k("unarmed-combat") * W
            d_a = d.s("dexterity") + d.k("unarmed-combat") * W
            cb = single(a_b, d_b)
            ca = single(a_a, d_a)
            # BEFORE crit-failure: failed AND self z <= -2. Self z and margin
            # correlate; P(z<=-2 & loss) ~= P(z<=-2)*P(loss|z<=-2)~=P(z<=-2)
            # when losing is likely at z=-2; exact via integration:
            sd = a_b * ROLL_SPREAD
            p_cf = 0.0
            for z, pdf in zip(_ZS, _PDF):
                if z > -2.0:
                    continue
                p_loss = 1.0 - PHI((a_b + sd * z - d_b) / sd)
                p_cf += pdf * _DZ * p_loss
            p_cf *= (1 - FLOOR)  # floored-to-win removes the failure
            # AFTER: fumble from self z stays (spec 4.4.5 self-relative)
            print(f"{atk.name:12s}{d.name:14s}{cb['p_win']*100:8.1f}%"
                  f"{ca['p_win']*100:8.1f}%{p_cf*100:15.2f}%{2.28:15.2f}%")
        print()

    print("--- SUBMISSION: Str + Unarmed*1.5 vs Str + Vit + Unarmed*1.5 -> x5 both")
    print(f"{'attacker':12s}{'defender':14s}{'P BEFORE':>9s}{'P AFTER':>9s}{'crit BEFORE':>12s}{'crit AFTER':>11s}")
    for atk in TIERS:
        for d in (TRASH, Tier("parity", atk.s("strength"), atk.k("unarmed-combat")),
                  scaled_opponent(atk, 1.5)):
            a_b = atk.s("strength") + atk.k("unarmed-combat") * 1.5
            d_b = d.s("strength") + d.s("vitality") + d.k("unarmed-combat") * 1.5
            a_a = atk.s("strength") + atk.k("unarmed-combat") * W
            d_a = d.s("strength") + d.s("vitality") + d.k("unarmed-combat") * W
            cb = single(a_b, d_b)
            ca = single(a_a, d_a)
            # BEFORE crit: success AND self z >= 2 (exact integration)
            sd = a_b * ROLL_SPREAD
            p_cb = 0.0
            for z, pdf in zip(_ZS, _PDF):
                if z < 2.0:
                    continue
                p_cb += pdf * _DZ * PHI((a_b + sd * z - d_b) / sd)
            p_cb *= (1 - FLOOR)
            print(f"{atk.name:12s}{d.name:14s}{cb['p_win']*100:8.1f}%"
                  f"{ca['p_win']*100:8.1f}%{p_cb*100:11.2f}%{ca['p_atk_crit']*100:10.2f}%")
        print()

    print("--- THROW (firebomb, item mult 0.85): expected damage per grenade per target")
    print("    BEFORE: binary vs Dex+Per*2.5, no crit tier, no partial")
    print("    AFTER : dodge defence set, crit tier, margin-scaled partial damage")
    print(f"{'thrower':12s}{'target':14s}{'P(hit) BEF':>11s}{'E[dmg] BEF':>11s}"
          f"{'E[mult] AFT':>12s}{'P(crit) AFT':>12s}{'E[dmg] AFT':>11s}{'ctr/thrown':>11s}")
    for atk in TIERS:
        a = atk.s("dexterity") + atk.k("skullduggery") * W
        base_dmg = mitigated(raw_damage(atk.s("dexterity"),
                                        atk.k("skullduggery"), 0.85), 0.0)
        crit_dmg = raw_damage(atk.s("dexterity"), atk.k("skullduggery"), 0.85) \
            * (CRIT_DMG_BASE + CRIT_DMG_PER_SKILL * min(atk.k("skullduggery"), 50))
        for tgt in (TRASH, scaled_opponent(atk, 1.0), scaled_opponent(atk, 1.5)):
            d_bef = tgt.s("dexterity") + tgt.s("perception") * W * 0.5
            p_bef = single(a, d_bef)["p_win"]
            defs = tgt.dodge_only()
            e_mult, p_win_roll = expected_damage_multiplier(a, defs)
            c = single(a, defs[0])
            # E[dmg] AFTER: split crit vs non-crit attacker wins
            p_atk_crit = c["p_atk_crit"]
            e_after = (e_mult - p_atk_crit * 1.0) * base_dmg + p_atk_crit * crit_dmg
            # counter risk: thrower same room -> def crit earns melee counter
            pc = (1 - FLOOR) * (c["p_def_crit_roll"]
                                + MIN_DEF_CRIT * (c["p_def_win_roll"] - c["p_def_crit_roll"]))
            ctr = pc * riposte_damage(tgt.s("strength"), tgt.k("weapon-combat"))
            print(f"{atk.name:12s}{tgt.name:14s}{p_bef*100:10.1f}%"
                  f"{p_bef*base_dmg:11.1f}{e_mult:12.3f}{p_atk_crit*100:11.2f}%"
                  f"{e_after:11.1f}{ctr:11.2f}")
        print()
    print("    (throw fumble self z<=-2 ~2.28%: detonates in hand BEFORE; spec")
    print("     keeps self-relative fumble AFTER - unchanged rate)")

    print("--- STEAL: (Dex + sqrtcurve*25)*1.0 vs raw Perception  ->  x5 both")
    print("    AFTER assumes defender Perception + skill*5 (skill=combat skill;")
    print("    the spec does not name the defender's skill - ASSUMPTION)")
    print(f"{'thief':12s}{'mark':14s}{'atk BEF':>8s}{'def BEF':>8s}{'P BEF':>8s}{'P AFT':>8s}")
    for thief in TIERS:
        for mark in (TRASH, Tier("parity", thief.s("dexterity"), thief.k("skullduggery")),
                     scaled_opponent(thief, 1.5)):
            a_b = (thief.s("dexterity")
                   + skill_dmg_mult(thief.k("skullduggery")) * 25.0) * 1.0
            d_b = mark.s("perception")
            a_a = thief.s("dexterity") + thief.k("skullduggery") * W
            d_a = mark.s("perception") + mark.k("unarmed-combat") * W
            print(f"{thief.name:12s}{mark.name:14s}{a_b:8.0f}{d_b:8.0f}"
                  f"{single(a_b, d_b)['p_win']*100:7.1f}%"
                  f"{single(a_a, d_a)['p_win']*100:7.1f}%")
        print()


def drift_dist(a, d, use_sqrt2):
    """Grapple-drift outcome distribution. z_live = Margin/sd (no sqrt2) or
    Margin/(sd*sqrt2) fixed. Margin ~ N(a-d, sd*sqrt2). Floor flips 12.5% of
    outcomes; the sentinel margin +-1 normalizes to ~0 -> Hold."""
    sd = a * ROLL_SPREAD
    denom = sd * SQRT2 if use_sqrt2 else sd
    mu = (a - d) / denom
    s = (sd * SQRT2) / denom  # std of z_live: 1.0 fixed, sqrt2 live
    def p_between(lo, hi):
        return PHI((hi - mu) / s) - PHI((lo - mu) / s)
    probs = {
        "hold": p_between(-0.5, 0.5),
        "adv1": p_between(0.5, 1.0), "adv2": p_between(1.0, 2.0),
        "adv3": 1.0 - PHI((2.0 - mu) / s),
        "degrade": p_between(-1.0, -0.5), "reversal": p_between(-2.0, -1.0),
        "escape": PHI((-2.0 - mu) / s),
    }
    out = {k: (1 - FLOOR) * v for k, v in probs.items()}
    out["hold"] += FLOOR  # floored sentinel z ~ 0 -> Hold band
    return out


def item8_drift():
    print("--- GRAPPLE DRIFT: 0.7*Str+0.3*Dex + coef*Unarmed, coef 2.2/2.0")
    print("    z tiers: hold<0.5 | 1-step<1.0 | 2-step<2.0 | 3-step>=2.0")
    print("    DEFECT (FIXED by U6b Task 14 in Position_GrappleTick.go):")
    print("    z = Margin/stdDev was missing sqrt(2) -> every z inflated 41%.")
    print("    'current' below models the PRE-fix live code. Floor: 12.5% of")
    print("    rounds forced to Hold (sentinel margin ~0 sigma) - in EVERY")
    print("    variant; kept deliberately (documented at the z site).")
    variants = [
        ("current (2.2/2.0, no sqrt2)", (2.2, 2.0), False),
        ("sqrt2 fix only", (2.2, 2.0), True),
        ("reweight only (5/5, no sqrt2)", (W, W), False),
        ("both (5/5 + sqrt2)", (W, W), True),
    ]
    pairs = [
        ("parity novice (100/5 both)", NOVICE, NOVICE),
        ("parity journeyman (120/25)", JOURNEY, JOURNEY),
        ("adept ctrl vs trash", ADEPT, TRASH),
    ]
    for pname, ctrl, ctrld in pairs:
        print(f"\n  pair: {pname} (controller = aggressor)")
        print(f"  {'variant':30s}{'hold':>7s}{'adv1':>7s}{'adv2':>7s}{'adv3':>7s}"
              f"{'degr':>7s}{'rev':>7s}{'esc':>7s}{'P(move)':>8s}{'E[steps]':>9s}")
        for vname, (ca, cd), fix in variants:
            a = 0.7 * ctrl.s("strength") + 0.3 * ctrl.s("dexterity") + ca * ctrl.k("unarmed-combat")
            d = 0.7 * ctrld.s("strength") + 0.3 * ctrld.s("dexterity") + cd * ctrld.k("unarmed-combat")
            p = drift_dist(a, d, fix)
            move = 1.0 - p["hold"]
            esteps = (p["adv1"] + 2 * p["adv2"] + 3 * p["adv3"]
                      - p["degrade"] - 2 * p["reversal"] - 3 * p["escape"])
            print(f"  {vname:30s}" + "".join(
                f"{p[k]*100:6.1f}%" for k in ("hold", "adv1", "adv2", "adv3",
                                              "degrade", "reversal", "escape"))
                + f"{move*100:7.1f}%{esteps:9.3f}")


def _drift_esteps(p):
    return (p["adv1"] + 2 * p["adv2"] + 3 * p["adv3"]
            - p["degrade"] - 2 * p["reversal"] - 3 * p["escape"])


def item8_drift_solve_aggressor_bonus():
    """U6b Task 14 solve: the reweight to SkillWeight (5/5) deletes the
    accidental 2.2-vs-2.0 aggressor edge (parity E[drift] -> exactly 0).
    Solve for GrappleAggressorDriftBonus B, a multiplier on the aggressor's
    WHOLE drift score, such that under the fixed maths (5/5 + sqrt2) parity
    E[drift] equals the old live parity value. Note the attacker's stdDev
    scales with B too (stdDev = score * RollSpread), so the parity mean z is
    (1 - 1/B)/(RollSpread*sqrt2): scale-free. One B restores the same
    E[drift] at EVERY parity tier."""
    print("--- GRAPPLE DRIFT AGGRESSOR-BONUS SOLVE (U6b Task 14)")
    base = (0.7 * JOURNEY.s("strength") + 0.3 * JOURNEY.s("dexterity")
            + W * JOURNEY.k("unarmed-combat"))
    a_old = (0.7 * JOURNEY.s("strength") + 0.3 * JOURNEY.s("dexterity")
             + 2.2 * JOURNEY.k("unarmed-combat"))
    d_old = (0.7 * JOURNEY.s("strength") + 0.3 * JOURNEY.s("dexterity")
             + 2.0 * JOURNEY.k("unarmed-combat"))
    target = _drift_esteps(drift_dist(a_old, d_old, False))
    print(f"    target: parity journeyman E[drift] under live maths "
          f"(2.2/2.0, no sqrt2) = {target:+.4f} steps/round")
    lo, hi = 1.0, 1.5
    for _ in range(80):
        mid = (lo + hi) / 2.0
        if _drift_esteps(drift_dist(mid * base, base, True)) > target:
            hi = mid
        else:
            lo = mid
    b = (lo + hi) / 2.0
    print(f"    solved GrappleAggressorDriftBonus = {b:.4f} "
          f"(ship rounded 1.038)")
    for name, tier in (("novice", NOVICE), ("journeyman", JOURNEY),
                       ("adept", ADEPT)):
        pb = (0.7 * tier.s("strength") + 0.3 * tier.s("dexterity")
              + W * tier.k("unarmed-combat"))
        e = _drift_esteps(drift_dist(1.038 * pb, pb, True))
        print(f"    parity {name:11s} @ shipped 1.038: E[drift] = "
              f"{e:+.4f} steps/round")
    print("    (multiplicative bonus -> parity mean z is scale-free; the")
    print("     shipped value restores ~+0.196 at every parity tier)")


def _run_my_trace(fixture, events, rounds, ceiling, defence_action="defence"):
    """Local trace reusing urm.PoolTrace/regen_per_hook with an explicit
    ceiling (urm.run_trace hardcodes the max-reservation ceiling)."""
    trace = urm.PoolTrace(ceiling)
    first_short = 0
    for rn in range(1, rounds + 1):
        for name, amount, full in events.get(rn, ()):
            st = trace.commit(amount, full, name)
            if name == defence_action and st in ("short", "refused") and not first_short:
                first_short = rn
        if trace.current == 0 and not trace.zero_round:
            trace.zero_round = rn
        if rn % 3 == 0:
            regen = urm.regen_per_hook(fixture, "stamina", combat=True)
            trace.current = min(ceiling, trace.current + regen)
    return trace, first_short


def item9_costs():
    sec("ITEM 9 - DEFENCE COST LOAD (defending specials/shots now costs)")
    load = urm.TYPICAL_LOAD
    print(f"Load {load}. Defence priced like the model script: base "
          f"{urm.SHIPPED['DefenceBaseStaminaCost']:.1f} x enc x unarmed skill x "
          f"dodge modifier {urm.SHIPPED['DodgeCostModifier']:.2f} (worst case).")
    print("Attack partial-pay 1/round. Regen: in-combat hook every 3rd round")
    print("(PlayerStaminaRegenPct 0.02, quartered in combat). 30-round fight.")
    print("NOTE: shipped DefenceBaseStaminaCost is 1.0, NOT the 2 in the task")
    print("prompt; code wins.\n")

    shot_rounds = [rn for rn, act in urm._ranged_action_schedule() if act == "shoot"]
    print(f"ranged attacker schedule (capacity 1, cooldown "
          f"{int(urm.SHIPPED['SpecialMoveCooldown'])}): shots on rounds {shot_rounds}"
          f" = {len(shot_rounds)} defended shots / 30 rounds\n")

    hdr = (f"{'tier':12s}{'pool':>6s}{'atk/rd':>7s}{'def/rd':>7s}"
           f"{'+spec/rd':>9s}{'+shot/rd':>9s}"
           f"{'end BEF-A':>10s}{'end AFT-A':>10s}{'end BEF-B':>10s}{'end AFT-B':>10s}")
    print(hdr)
    fixtures = {
        "novice": urm.Fixture("novice", NOVICE.stats, NOVICE.skills),
        "journeyman": urm.Fixture("journeyman", JOURNEY.stats, JOURNEY.skills),
        "adept": urm.Fixture("adept", ADEPT.stats, ADEPT.skills),
        "meirok": urm.Fixture("meirok", MEIROK.stats, MEIROK.skills),
    }
    results = {}
    for name, fx in fixtures.items():
        atk_cost = urm.ordinary_swing_cost(fx, load, fx.skill("weapon-combat"))
        def_cost = urm.action_cost(
            urm.SHIPPED["DefenceBaseStaminaCost"], load, fx.skill("unarmed-combat"),
            modifier=max(urm.SHIPPED["DodgeCostModifier"],
                         urm.SHIPPED["ParryCostModifier"],
                         urm.SHIPPED["BlockCostModifier"]))
        pool = fx.stamina_max

        def build(extra_def_rounds):
            ev = {}
            for rn in range(1, urm.TRACE_ROUNDS + 1):
                ev[rn] = [("attack", atk_cost, False), ("defence", def_cost, False)]
                if rn in extra_def_rounds:
                    ev[rn].append(("defence", def_cost, False))
            return ev

        def build_ranged(defended):
            ev = {}
            for rn in range(1, urm.TRACE_ROUNDS + 1):
                ev[rn] = [("attack", atk_cost, False)]
                if defended and rn in shot_rounds:
                    ev[rn].append(("defence", def_cost, False))
            return ev

        spec_rounds = set(range(3, urm.TRACE_ROUNDS + 1, 3))
        tr_ba, _ = _run_my_trace(fx, build(set()), urm.TRACE_ROUNDS, pool)
        tr_aa, _ = _run_my_trace(fx, build(spec_rounds), urm.TRACE_ROUNDS, pool)
        tr_bb, _ = _run_my_trace(fx, build_ranged(False), urm.TRACE_ROUNDS, pool)
        tr_ab, _ = _run_my_trace(fx, build_ranged(True), urm.TRACE_ROUNDS, pool)
        results[name] = (fx, atk_cost, def_cost, pool, tr_ba, tr_aa, tr_bb, tr_ab)
        print(f"{name:12s}{pool:6d}{atk_cost:7.2f}{def_cost:7.2f}"
              f"{def_cost/3:9.2f}{def_cost*len(shot_rounds)/30:9.2f}"
              f"{tr_ba.current:10d}{tr_aa.current:10d}{tr_bb.current:10d}{tr_ab.current:10d}")
    print("\n(end = stamina at round 30 from a FULL, unreserved pool; zero_round")
    print(" 0 everywhere = nobody exhausts. Scenario A: melee + 1 special/3rd")
    print(" round defended. B: full-ranged attacker, shots defended AFTER only.)")

    print("\n--- stress: max-reserved pool (66% reserved - urm's own ceiling) + crushed load 1.0")
    print(f"{'tier':12s}{'usable':>7s}{'def/rd@1.0':>11s}{'end AFT-A':>10s}{'zeroRd':>7s}{'defShortRd':>11s}")
    for name, fx in fixtures.items():
        usable = urm.usable_pool_max(fx.stamina_max)
        atk_cost = urm.ordinary_swing_cost(fx, 1.0, fx.skill("weapon-combat"))
        def_cost = urm.action_cost(
            urm.SHIPPED["DefenceBaseStaminaCost"], 1.0, fx.skill("unarmed-combat"),
            modifier=urm.SHIPPED["DodgeCostModifier"])
        ev = {}
        for rn in range(1, urm.TRACE_ROUNDS + 1):
            ev[rn] = [("attack", atk_cost, False), ("defence", def_cost, False)]
            if rn % 3 == 0:
                ev[rn].append(("defence", def_cost, False))
        tr, short_rn = _run_my_trace(fx, ev, urm.TRACE_ROUNDS, usable)
        print(f"{name:12s}{usable:7d}{def_cost:11.2f}{tr.current:10d}"
              f"{tr.zero_round:7d}{short_rn:11d}")

    print("\n--- ONE ITERATION OF THE FEEDBACK LOOP (U8 skill-strip)")
    print("When the pool cannot cover a defence quote, the skill term is")
    print("stripped: defence score falls by skill*5*effectiveness. vs a parity")
    print("melee attacker:")
    print(f"{'tier':12s}{'P(defWin) full':>15s}{'P(defWin) strip':>16s}"
          f"{'P(defCrit) full':>16s}{'P(defCrit) strip':>17s}{'E[dmgMult] delta':>17s}")
    for tier in TIERS:
        a = tier.melee_attack()
        full = tier.defence_set()
        stripped = [(tier.s("dexterity")) * EFF["dodge"],
                    (tier.s("dexterity")) * EFF["parry"],
                    ((tier.s("strength") + tier.s("dexterity")) / 2.0) * EFF["block"]]
        pw_f, pc_f, pnf_f = best_of_n(a, full)
        pw_s, pc_s, pnf_s = best_of_n(a, stripped)
        pcf = (1 - FLOOR) * (pc_f + MIN_DEF_CRIT * (pnf_f - pc_f))
        pcs = (1 - FLOOR) * (pc_s + MIN_DEF_CRIT * (pnf_s - pc_s))
        em_f, _ = expected_damage_multiplier(a, full)
        em_s, _ = expected_damage_multiplier(a, stripped)
        print(f"{tier.name:12s}{((1-FLOOR)*pw_f+FLOOR*(1-pw_f))*100:14.1f}%"
              f"{((1-FLOOR)*pw_s+FLOOR*(1-pw_s))*100:15.1f}%"
              f"{pcf*100:15.2f}%{pcs*100:16.2f}%{(em_s-em_f):17.3f}")
    print("(E[dmgMult] delta = extra fraction of every incoming swing's damage")
    print(" taken once stripped; the loop: more defended attacks -> more cost ->")
    print(" strip -> weaker defence -> longer fight -> more defended attacks.)")


if __name__ == "__main__":
    print(__doc__)
    sanity()
    item7_counters()
    item8_family()
    item8_drift()
    item9_costs()
    item8_drift_solve_aggressor_bonus()
