"""Chunk 5.11a - model skill's contribution under shipped vs candidate tuning.

All formulas taken from source on 2026-08-11:
  attackScore   = Dex + skill*SkillWeight            (combat_helpers.go:405)
  dodgeScore    = Dex + skill*SkillWeight            (characters/combat.go:317)
  P(win)        = Phi((atk-def)/(atk*RollSpread*sqrt(2)))   (5.8 analysis)
  hit floors    = MinAttackHitChance / MinDefenseChance = 0.15 (resolveAttack)
  SkillMult     = base + (max-base)*sqrt(min(rank,cap)/cap)  (damage_pipeline.go:23)
  rawDmg        = Str * SkillMult * itemMult * MeleeScale * GlobalMult
  crit thresh   = 2.0 - skillDiff*0.05, floored 1.5   (combat_helpers.go:450)
  crit damage   = BYPASSES MITIGATION (calcHitDamage:998) - no multiplier exists
"""
import math

PHI = lambda z: 0.5 * (1.0 + math.erf(z / math.sqrt(2.0)))

# ---- shipped config (config.yaml, 2026-08-11) ----
ROLL_SPREAD = 0.15
SKILL_WEIGHT = 2.0
MELEE_SCALE = 0.52
GLOBAL_MULT = 0.5
MIN_ATK_HIT = 0.15
MIN_DEF = 0.15
MIT_CAP = 0.75


class Tuning:
    def __init__(self, name, sm_base, sm_max, sm_cap, above_rate,
                 crit_slope, crit_floor, crit_dmg_base, crit_dmg_rate):
        self.name = name
        self.sm_base, self.sm_max, self.sm_cap = sm_base, sm_max, sm_cap
        self.above_rate = above_rate
        self.crit_slope, self.crit_floor = crit_slope, crit_floor
        # crit_dmg: multiplier applied ON TOP of mitigation bypass
        self.crit_dmg_base, self.crit_dmg_rate = crit_dmg_base, crit_dmg_rate

    def skill_mult(self, rank):
        if rank <= 0:
            return self.sm_base
        r = min(rank, self.sm_cap)
        m = self.sm_base + (self.sm_max - self.sm_base) * math.sqrt(r / self.sm_cap)
        if rank > self.sm_cap and self.above_rate > 0:
            m += self.above_rate * math.sqrt((rank - self.sm_cap) / self.sm_cap)
        return m

    def crit_rate(self, skill_diff):
        thr = 2.0 - skill_diff * self.crit_slope
        thr = max(thr, self.crit_floor)
        return 1.0 - PHI(thr)

    def crit_dmg_mult(self, rank):
        # sub-linear so rate x magnitude stays near-linear
        return self.crit_dmg_base + self.crit_dmg_rate * math.sqrt(max(rank, 0) / 50.0)


SHIPPED = Tuning("shipped", 1.0, 3.0, 50, 0.0, 0.05, 1.5, 1.0, 0.0)
# Candidate: skill's DIRECT damage share cut hard; crit rate and crit magnitude
# both widened, each sub-linear.
CANDIDATE = Tuning("candidate", 1.0, 1.5, 50, 0.15, 0.02, 1.0, 1.0, 0.6)


def p_hit(atk, dfn):
    sigma = atk * ROLL_SPREAD * math.sqrt(2.0)
    p_atk_wins = PHI((atk - dfn) / sigma) if sigma > 0 else 0.5
    return p_atk_wins * (1.0 - MIN_DEF) + (1.0 - p_atk_wins) * MIN_ATK_HIT


def expected_damage(t, skill, stat, item_mult, def_skill, def_stat, mit):
    atk = stat + skill * SKILL_WEIGHT
    dfn = def_stat + def_skill * SKILL_WEIGHT
    ph = p_hit(atk, dfn)
    raw = stat * t.skill_mult(skill) * item_mult * MELEE_SCALE * GLOBAL_MULT
    pc = t.crit_rate(skill - def_skill)
    normal = raw * (1.0 - mit)
    crit = raw * t.crit_dmg_mult(skill)          # bypasses mitigation
    return ph * ((1.0 - pc) * normal + pc * crit)


def leverage(t, axis, base):
    """How much does moving ONE axis across its realistic range multiply E[dmg]?"""
    lo = dict(base)
    hi = dict(base)
    if axis == "skill":
        lo["skill"], hi["skill"] = 0, 100
    elif axis == "gear":
        lo["item_mult"], hi["item_mult"] = 0.40, 1.60
        lo["mit"], hi["mit"] = base["mit"], base["mit"]
    elif axis == "stats":
        lo["stat"], hi["stat"] = 100, 200
    return expected_damage(t, **hi) / expected_damage(t, **lo)


BASE = dict(skill=25, stat=100, item_mult=1.0, def_skill=25, def_stat=100, mit=0.40)

print("=" * 78)
print("SkillMultiplier curve")
print("=" * 78)
print(f"{'rank':>6} {'shipped':>10} {'candidate':>10}")
for r in [0, 10, 25, 50, 75, 100]:
    print(f"{r:>6} {SHIPPED.skill_mult(r):>10.3f} {CANDIDATE.skill_mult(r):>10.3f}")

print()
print("=" * 78)
print("Crit rate by skill advantage over the defender")
print("=" * 78)
print(f"{'skillDiff':>10} {'shipped':>10} {'candidate':>10}")
for d in [0, 10, 25, 50, 75]:
    print(f"{d:>10} {SHIPPED.crit_rate(d)*100:>9.1f}% {CANDIDATE.crit_rate(d)*100:>9.1f}%")

print()
print("=" * 78)
print("E[damage per swing] vs an EQUAL-skill defender (skill 25, stat 100, 40% mit)")
print("=" * 78)
print(f"{'skill':>6} {'shipped':>10} {'candidate':>10}   {'ship rel':>9} {'cand rel':>9}")
s0 = expected_damage(SHIPPED, 0, 100, 1.0, 25, 100, 0.40)
c0 = expected_damage(CANDIDATE, 0, 100, 1.0, 25, 100, 0.40)
for sk in [0, 25, 50, 75, 100]:
    s = expected_damage(SHIPPED, sk, 100, 1.0, 25, 100, 0.40)
    c = expected_damage(CANDIDATE, sk, 100, 1.0, 25, 100, 0.40)
    print(f"{sk:>6} {s:>10.2f} {c:>10.2f}   {s/s0:>8.2f}x {c/c0:>8.2f}x")

print()
print("=" * 78)
print("LEVERAGE - the acceptance test. Want Skill > Gear > Stats.")
print("=" * 78)
print("  skill 0->100 | gear 0.40->1.60 weapon mult | stats 100->200")
print()
for t in (SHIPPED, CANDIDATE):
    ls = leverage(t, "skill", BASE)
    lg = leverage(t, "gear", BASE)
    lt = leverage(t, "stats", BASE)
    order = " > ".join(n for n, _ in sorted(
        [("Skill", ls), ("Gear", lg), ("Stats", lt)], key=lambda kv: -kv[1]))
    ok = "PASS" if ls > lg > lt else "FAIL"
    print(f"{t.name:>10}:  Skill {ls:5.2f}x   Gear {lg:5.2f}x   Stats {lt:5.2f}x"
          f"   -> {order}   [{ok}]")

print()
print("=" * 78)
print("Quadratic check: crit contribution as skill climbs (candidate)")
print("=" * 78)
print(f"{'skill':>6} {'P(crit)':>9} {'critMult':>9} {'product':>9} {'vs linear':>10}")
p0 = CANDIDATE.crit_rate(0) * CANDIDATE.crit_dmg_mult(0)
for sk in [0, 25, 50, 75, 100]:
    pc = CANDIDATE.crit_rate(sk - 25)
    cm = CANDIDATE.crit_dmg_mult(sk)
    prod = pc * cm
    print(f"{sk:>6} {pc*100:>8.1f}% {cm:>9.2f} {prod:>9.4f} {prod/p0:>9.2f}x")
