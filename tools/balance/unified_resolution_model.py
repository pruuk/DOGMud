"""Model the unified-resolution redesign against today's live behaviour.

Answers one question: with damage scaled down 35% and the item mitigation cap
raised, where does damage-per-swing actually land, INCLUDING the 5.11g crit
magnitude change that the earlier +31% throughput figure left out?

Formulas taken from source on 2026-08-12:
  attack score   = stat + skill*SkillWeight          combat_helpers.go:407
  defence score  = stat + skill*SkillWeight          characters/combat.go:317
  normalized z   = margin / (stdDev * sqrt(2))       margin_crit.go:59
  stdDev         = attackScore * RollSpread          dice.StdDevFor
  crit           = z >= 2.0                          margin_crit.go ContestCrit
  crit damage    = raw * (CritDamageBase + CritDamagePerSkill*rank),
                   BYPASSING item mitigation          crit_damage.go
  normal damage  = ApplyMitigation(raw, itemMit, cap) damage_pipeline.go

TODAY: a defensive win means zero damage.
NEW  : a defensive win means margin-scaled mitigation, 50% at a bare win
       rising to 100% at the crit threshold, applied AFTER item mitigation.
       A defensive crit fully negates and fires a counterattack.
"""
import math

PHI = lambda z: 0.5 * (1.0 + math.erf(z / math.sqrt(2.0)))
phi = lambda z: math.exp(-0.5 * z * z) / math.sqrt(2 * math.pi)

ROLL_SPREAD = 0.15
CRIT_Z = 2.0
CRIT_BASE = 2.0          # 5.11g, shipped
CRIT_PER_SKILL = 0.05    # 5.11g, shipped

STEPS = 4000
Z_LIM = 6.0


def _z_grid():
    """Numeric integration grid over the normalized margin z ~ N(0,1)."""
    step = 2 * Z_LIM / STEPS
    for i in range(STEPS):
        z = -Z_LIM + (i + 0.5) * step
        yield z, phi(z) * step


def expected_damage(atk_score, def_score, raw, skill, item_mit, mit_cap,
                    model, crit_mult_on=True):
    """Mean damage per swing, integrating over the opposed-roll outcome.

    model 'today': defensive win -> 0 damage.
    model 'new'  : defensive win -> margin-scaled mitigation 50%..100%.
    """
    std = atk_score * ROLL_SPREAD
    if std <= 0:
        return 0.0
    # Attacker-positive margin in sigmas of the DIFFERENCE.
    mean_z = (atk_score - def_score) / (std * math.sqrt(2.0))

    eff_mit = min(item_mit, mit_cap)
    normal_dmg = raw * (1.0 - eff_mit)
    cmult = (CRIT_BASE + CRIT_PER_SKILL * skill) if crit_mult_on else 1.0
    crit_dmg = raw * cmult          # crits bypass item mitigation entirely

    total = 0.0
    for z, w in _z_grid():
        za = z + mean_z             # attacker's realised normalized margin
        if za >= CRIT_Z:
            total += w * crit_dmg
        elif za >= 0:
            total += w * normal_dmg
        else:
            zd = -za                # defender's margin
            if model == 'today':
                continue            # defensive win = clean miss
            if zd >= CRIT_Z:
                continue            # defensive crit = full negation
            # 50% at a bare win, rising linearly to 100% at the crit threshold
            def_mit = 0.5 + 0.5 * (zd / CRIT_Z)
            total += w * normal_dmg * (1.0 - def_mit)
    return total


def scenario(label, atk_stat, atk_skill, def_stat, def_skill, item_mit,
             raw_today, dmg_scale_new, mit_cap_new, sw=5.0):
    a = atk_stat + atk_skill * sw
    d = def_stat + def_skill * sw

    today = expected_damage(a, d, raw_today, atk_skill, item_mit, 0.75,
                            'today', crit_mult_on=False)
    # New pipeline, no compensation: full crit magnitude + margin mitigation.
    new_raw = expected_damage(a, d, raw_today, atk_skill, item_mit, 0.75, 'new')
    # New pipeline with the proposed compensation package.
    new_tuned = expected_damage(a, d, raw_today * dmg_scale_new, atk_skill,
                                item_mit, mit_cap_new, 'new')

    def pct(x):
        return f"{(x / today - 1.0) * 100:+6.0f}%" if today > 0 else "   n/a"

    print(f"  {label:<34} today {today:6.1f} | new {new_raw:6.1f} {pct(new_raw)}"
          f" | tuned {new_tuned:6.1f} {pct(new_tuned)}")


def run(dmg_scale, mit_cap):
    print(f"\n=== damage x{dmg_scale:.2f}, item mitigation cap {mit_cap:.0%} ===")
    RAW = 100.0
    print("\n  -- lightly armoured targets (item mitigation 20%) --")
    scenario("parity, skill 30 v 30", 100, 30, 100, 30, 0.20, RAW, dmg_scale, mit_cap)
    scenario("player 30 v mob skill 1", 100, 30, 100, 1, 0.20, RAW, dmg_scale, mit_cap)
    scenario("outclassed 10 v 50", 100, 10, 140, 50, 0.20, RAW, dmg_scale, mit_cap)

    print("\n  -- mid armour (declared 60%, under every cap) --")
    scenario("parity, skill 30 v 30", 100, 30, 100, 30, 0.60, RAW, dmg_scale, mit_cap)
    scenario("player 69 v endgame mob", 150, 69, 417, 1, 0.60, RAW, dmg_scale, mit_cap)

    # Mitigation SUMS across slots and top items declare 55-65 each, so a full
    # best-in-slot set blows past any of these caps. Declaring 95% here is what
    # actually exercises the cap lever - passing exactly 0.75 leaves
    # min(declared, cap) pinned at 0.75 and the lever looks like a no-op.
    print("\n  -- best-in-slot armour (declared 95%, CAP IS BINDING) --")
    scenario("parity, skill 30 v 30", 100, 30, 100, 30, 0.95, RAW, dmg_scale, mit_cap)
    scenario("player 30 v mob skill 1", 100, 30, 100, 1, 0.95, RAW, dmg_scale, mit_cap)
    scenario("endgame mob hitting a BIS player", 417, 1, 150, 69, 0.95, RAW, dmg_scale, mit_cap)


if __name__ == '__main__':
    print(__doc__)
    for cap in (0.75, 0.80, 0.85):
        run(0.65, cap)
