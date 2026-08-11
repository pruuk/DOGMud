"""5.11a addendum - calibrate the candidate so total damage is preserved,
and decompose WHERE skill's leverage actually comes from."""
import math
from skill_model import (PHI, Tuning, p_hit, SHIPPED, CANDIDATE, MELEE_SCALE,
                         GLOBAL_MULT, SKILL_WEIGHT, ROLL_SPREAD)


def edmg(t, skill, stat, item_mult, def_skill, def_stat, mit, melee_scale):
    atk = stat + skill * SKILL_WEIGHT
    dfn = def_stat + def_skill * SKILL_WEIGHT
    ph = p_hit(atk, dfn)
    raw = stat * t.skill_mult(skill) * item_mult * melee_scale * GLOBAL_MULT
    pc = t.crit_rate(skill - def_skill)
    return ph * ((1 - pc) * raw * (1 - mit) + pc * raw * t.crit_dmg_mult(skill))


REF = dict(stat=100, item_mult=1.0, def_skill=25, def_stat=100, mit=0.40)

# Calibrate: hold E[dmg] at the reference skill 50 equal to shipped.
target = edmg(SHIPPED, 50, melee_scale=MELEE_SCALE, **REF)
cur = edmg(CANDIDATE, 50, melee_scale=MELEE_SCALE, **REF)
comp = MELEE_SCALE * target / cur
print(f"MeleeDamageScale compensation to hold skill-50 damage: "
      f"{MELEE_SCALE:.2f} -> {comp:.2f}\n")

print("=" * 78)
print("E[dmg/swing]: shipped vs candidate-calibrated")
print("=" * 78)
print(f"{'skill':>6} {'shipped':>10} {'calibrated':>11} {'delta':>8}")
for sk in [0, 25, 50, 75, 100]:
    s = edmg(SHIPPED, sk, melee_scale=MELEE_SCALE, **REF)
    c = edmg(CANDIDATE, sk, melee_scale=comp, **REF)
    print(f"{sk:>6} {s:>10.2f} {c:>11.2f} {(c/s-1)*100:>7.1f}%")

# Decompose skill's leverage into hit vs damage-multiplier vs crit.
print()
print("=" * 78)
print("WHERE does skill's leverage come from? (skill 0 -> 100, shipped)")
print("=" * 78)


def parts(t, skill, melee_scale):
    atk = REF["stat"] + skill * SKILL_WEIGHT
    dfn = REF["def_stat"] + REF["def_skill"] * SKILL_WEIGHT
    ph = p_hit(atk, dfn)
    sm = t.skill_mult(skill)
    pc = t.crit_rate(skill - REF["def_skill"])
    critfac = (1 - pc) * (1 - REF["mit"]) + pc * t.crit_dmg_mult(skill)
    return ph, sm, critfac


for name, t, ms in (("shipped", SHIPPED, MELEE_SCALE),
                    ("calibrated", CANDIDATE, comp)):
    h0, m0, c0 = parts(t, 0, ms)
    h1, m1, c1 = parts(t, 100, ms)
    tot = (h1 * m1 * c1) / (h0 * m0 * c0)
    print(f"\n{name}: total {tot:.2f}x")
    print(f"   P(hit)        {h0:.3f} -> {h1:.3f}   = {h1/h0:5.2f}x")
    print(f"   SkillMult     {m0:.3f} -> {m1:.3f}   = {m1/m0:5.2f}x")
    print(f"   crit factor   {c0:.3f} -> {c1:.3f}   = {c1/c0:5.2f}x")
    print(f"   share of leverage: hit {math.log(h1/h0)/math.log(tot)*100:4.0f}%"
          f"  dmg-mult {math.log(m1/m0)/math.log(tot)*100:4.0f}%"
          f"  crit {math.log(c1/c0)/math.log(tot)*100:4.0f}%")

# The grapple-floor conflict.
print()
print("=" * 78)
print("Crit-threshold floor conflict (candidate uses floor 1.0)")
print("=" * 78)
print("calcCritThreshold has TWO floors: 1.5 after the skill term, and 1.0")
print("absolute after position modifiers (grapple controller -0.2/-0.4).")
for sd, lbl in ((25, "skill +25"), (50, "skill +50"), (75, "skill +75")):
    thr = max(2.0 - sd * CANDIDATE.crit_slope, CANDIDATE.crit_floor)
    grap = max(thr - 0.4, 1.0)
    print(f"  {lbl}: threshold {thr:.2f} -> crit {(1-PHI(thr))*100:4.1f}%   "
          f"| +ground-grapple {grap:.2f} -> {(1-PHI(grap))*100:4.1f}%"
          f"{'   <-- grapple buys NOTHING' if abs(grap-thr) < 1e-9 else ''}")
