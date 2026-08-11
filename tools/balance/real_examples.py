"""5.11 - three REAL matchups from live data, shipped vs margin-derived crit.

Sources (read 2026-08-11):
  Meirok            _archive/prod-users/users/24.yaml (stat = base + training)
  Drowned Claws     items/weapons-10000/10036  damage_multiplier 0.45
  Arena Champion    mobs/instance_arena/324, species 1 (human, all base 100)
  Elemental King    mobs/instance_planar_oasis/320, species 40 (magma elemental)
  statpool scaling  ScaleSpawnStatPools: pool = goldPaid * spawnInfo.StatPool(=4)
  archetype fighting: 80% -> Str/Dex/Vit, 20% -> Per/Wil/Cha
  mob combat skill  no skills block -> GetCombatSkillLevel() floors at 1
"""
import math

PHI = lambda z: 0.5 * (1 + math.erf(z / math.sqrt(2)))
RS, SW = 0.15, 2.0
MIN_ATK_HIT = MIN_DEF = 0.15


def fighting_stats(base, training, pool):
    """Distribute a statpool with the 'fighting' archetype weighting."""
    phys, ment = pool * 0.80 / 3.0, pool * 0.20 / 3.0
    out = {}
    for k in ("strength", "dexterity", "vitality"):
        out[k] = base.get(k, 100) + training.get(k, 0) + phys
    for k in ("perception", "willpower", "charisma"):
        out[k] = base.get(k, 100) + training.get(k, 0) + ment
    return out


HUMAN = dict(strength=100, dexterity=100, perception=100,
             vitality=100, willpower=100, charisma=100)
MAGMA = dict(strength=180, dexterity=70, perception=80,
             vitality=180, willpower=60, charisma=80)

newbie_mob = fighting_stats(HUMAN, {}, 15)
champion = fighting_stats(HUMAN, dict(strength=8, dexterity=4,
                                      vitality=6, perception=3), 100 * 4)
king = fighting_stats(MAGMA, dict(strength=10, vitality=10), 325 * 4)

SCEN = [
    # label,                 player dex, str, skill, weaponmult, mob stats, mobskill
    ("EARLY  newbie vs newbie mob", 100, 100, 5, 0.30, newbie_mob, 1),
    ("MID    skill-25 vs arena champion (100g)", 110, 110, 25, 0.45, champion, 1),
    ("LATE   Meirok (wc 69) vs Elemental King (325g)", 110, 136, 69, 0.45, king, 1),
]


def p_hit(atk, dfn):
    s = atk * RS * math.sqrt(2)
    p = PHI((atk - dfn) / s)
    return p * (1 - MIN_DEF) + (1 - p) * MIN_ATK_HIT


def crit_shipped(skill_diff):
    thr = max(2.0 - skill_diff * 0.05, 1.5)
    return 1 - PHI(thr)


def crit_margin(atk, dfn, thr=2.0, floor=0.01):
    s = atk * RS * math.sqrt(2)
    mu = (atk - dfn) / s
    return max(1 - PHI(thr - mu), floor)


print("=" * 92)
print("REAL MATCHUPS - opposed-roll position")
print("=" * 92)
print(f"{'scenario':<46} {'atk':>7} {'def':>7} {'ratio':>7} {'P(hit)':>8}")
rows = []
for lbl, pdex, pstr, psk, wm, mob, msk in SCEN:
    atk = pdex + psk * SW
    dfn = mob["dexterity"] + msk * SW      # dodge = Dex + skill*SkillWeight
    rows.append((lbl, atk, dfn, psk, msk, mob))
    print(f"{lbl:<46} {atk:>7.0f} {dfn:>7.0f} {atk/dfn:>6.2f}x {p_hit(atk,dfn)*100:>7.1f}%")

print()
print("=" * 92)
print("CRIT RATE: shipped (skill-diff threshold) vs margin-derived")
print("=" * 92)
print(f"{'scenario':<46} {'shipped':>9} {'margin':>9} {'change':>10}")
for lbl, atk, dfn, psk, msk, mob in rows:
    cs = crit_shipped(psk - msk)
    cm = crit_margin(atk, dfn)
    print(f"{lbl:<46} {cs*100:>8.1f}% {cm*100:>8.1f}% {('x%.2f' % (cm/cs)):>10}")

print()
print("=" * 92)
print("THE MIRROR - what the MOB crits at (defence side, same rule)")
print("=" * 92)
print(f"{'scenario':<46} {'shipped':>9} {'margin':>9}")
for lbl, atk, dfn, psk, msk, mob in rows:
    cs = crit_shipped(msk - psk)          # mob's skill disadvantage
    cm = crit_margin(dfn, atk)            # mob attacking the player
    print(f"{lbl:<46} {cs*100:>8.1f}% {cm*100:>8.1f}%")

print()
print("=" * 92)
print("WHY - what each side's score is actually made of")
print("=" * 92)
for lbl, atk, dfn, psk, msk, mob in rows:
    print(f"\n{lbl}")
    print(f"   player: Dex {atk - psk*SW:6.0f} + skill {psk:>3} x{SW:.0f} = {psk*SW:6.0f}  -> {atk:6.0f}")
    print(f"   mob:    Dex {mob['dexterity']:6.0f} + skill {msk:>3} x{SW:.0f} = {msk*SW:6.0f}  -> {dfn:6.0f}")
    print(f"   mob Str {mob['strength']:.0f}  Vit {mob['vitality']:.0f}")
