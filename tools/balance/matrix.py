"""5.11 - full player x enemy matrix under four tunings.

Levers:
  SkillWeight       shipped 2.0  -> candidate 5.0
  spawn StatPool    shipped 4    -> candidate 2   (ScaleSpawnStatPools)

NOTE the asymmetry: defence score is ALSO stat + skill*SkillWeight, and every
mob here has combat skill 1 (no skills block -> GetCombatSkillLevel floors at 1).
So raising SkillWeight lifts the player on offence AND defence while giving the
mobs almost nothing. It is effectively a global player buff, not a neutral knob.
"""
import math

PHI = lambda z: 0.5 * (1 + math.erf(z / math.sqrt(2)))
RS = 0.15
MIN_ATK_HIT = MIN_DEF = 0.15

HUMAN = dict(strength=100, dexterity=100, perception=100,
             vitality=100, willpower=100, charisma=100)
MAGMA = dict(strength=180, dexterity=70, perception=80,
             vitality=180, willpower=60, charisma=80)


def fighting(base, training, pool):
    phys, ment = pool * 0.80 / 3.0, pool * 0.20 / 3.0
    o = {}
    for k in ("strength", "dexterity", "vitality"):
        o[k] = base.get(k, 100) + training.get(k, 0) + phys
    for k in ("perception", "willpower", "charisma"):
        o[k] = base.get(k, 100) + training.get(k, 0) + ment
    return o


PLAYERS = [("newbie   sk 5", 100, 5),
           ("mid      sk 25", 110, 25),
           ("Meirok   sk 69", 110, 69)]


def enemies(mult):
    return [
        ("newbie mob", fighting(HUMAN, {}, 15), 1),
        ("champion 100g", fighting(HUMAN, dict(strength=8, dexterity=4,
                                               vitality=6, perception=3),
                                   100 * mult), 1),
        ("elem king 325g", fighting(MAGMA, dict(strength=10, vitality=10),
                                    325 * mult), 1),
    ]


def p_hit(atk, dfn):
    s = atk * RS * math.sqrt(2)
    p = PHI((atk - dfn) / s)
    return p * (1 - MIN_DEF) + (1 - p) * MIN_ATK_HIT


def crit_margin(atk, dfn, thr=2.0, floor=0.01, cap=0.30):
    s = atk * RS * math.sqrt(2)
    return min(max(1 - PHI(thr - (atk - dfn) / s), floor), cap)


VARIANTS = [("A  SHIPPED      SW 2.0  pool x4", 2.0, 4),
            ("B  skill lever  SW 5.0  pool x4", 5.0, 4),
            ("C  mob nerf     SW 2.0  pool x2", 2.0, 2),
            ("D  both         SW 5.0  pool x2", 5.0, 2)]

for label, SW, mult in VARIANTS:
    ens = enemies(mult)
    print("=" * 94)
    print(label)
    print("=" * 94)
    print(f"{'':<16}" + "".join(f"{e[0]:>26}" for e in ens))
    print(f"{'':<16}" + "".join(f"{'P(hit) ->  <- P(hit)':>26}" for e in ens))
    for pl, pdex, psk in PLAYERS:
        atkp = pdex + psk * SW          # player attack AND player defence
        row = f"{pl:<16}"
        for en, mob, msk in ens:
            atkm = mob["dexterity"] + msk * SW
            ph = p_hit(atkp, atkm)      # player hits mob
            pm = p_hit(atkm, atkp)      # mob hits player
            row += f"{ph*100:>11.1f}% {pm*100:>12.1f}%"
        print(row)
    print()
    # crit picture, margin-derived, cap 30% floor 1%
    print(f"{'':<16}" + "".join(f"{'crit ->     <- crit':>26}" for e in ens))
    for pl, pdex, psk in PLAYERS:
        atkp = pdex + psk * SW
        row = f"{pl:<16}"
        for en, mob, msk in ens:
            atkm = mob["dexterity"] + msk * SW
            row += f"{crit_margin(atkp,atkm)*100:>11.1f}% {crit_margin(atkm,atkp)*100:>12.1f}%"
        print(row)
    print()
