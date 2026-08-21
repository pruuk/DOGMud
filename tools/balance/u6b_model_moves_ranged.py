"""U6b modelling gate, group B: special moves at x5, new crit tier, ranged,
defence-set width (spec 2026-08-19-u6b-finish-the-flip-design.md, s7 items 3-6).

Self-contained Monte Carlo mirror of the Go code paths:

  internal/contest/contest.go   Run / RunWithFloors
      - ONE attack roll Normal(A, s), s = max(1.0, A * RollSpread)
      - every defence rolled Normal(D_i, s) with the ATTACKER's s
        (defences are conditionally independent GIVEN the attack roll;
         we sample jointly, which models the correlation exactly)
      - best defence = smallest attack-positive margin  (X - max Y_i)
      - RunWithFloors: with prob ContestFloor the OUTCOME flips and the
        margin becomes a +-1 sentinel (Floored=true)
  internal/combat/skill_moves.go  ExecuteSkillMove (BEFORE)
      - attackerScore = AttackStat + AttackSkill (x1), ONE scalar defender
      - damage = RollStat(ApplyMitigation(raw)) * defenceDamageMultiplier
      - NO crit
  internal/combat/defence_multiplier.go  defenceDamageMultiplier
      - success -> 1.0; floored loss -> 0.5; defensive crit -> 0.0;
        else 1.0 - DefenceMitigation(-margin/(s*sqrt2))
        = clamp(0.5 - 0.25 * defz, 0, 0.5)
  internal/combat/margin_crit.go / crit_floor.go
      - crit: margin/(s*sqrt2) >= bar; floored sentinel cannot margin-crit
      - ApplyCritFloor: 1% promotion of wins (MinAttack/DefenseCritChance)
  internal/combat/combat_helpers.go calcCritThreshold (dynamic bar variant)
      - 2.0 - 0.05*(atkCombatSkill - defCombatSkill), floor 1.5
  internal/combat/crit_damage.go CritOrMitigatedDamage
      - crit: raw * (CritDamageBase + CritDamagePerSkill*rank), NO mitigation
      - non-crit: ApplyMitigation(raw)
  internal/actions/combat_fire.go rangedDefenseScore (BEFORE ranged)
      - Dex + GetCombatSkillLevel (x1) + flat 15 if shield offhand
  internal/characters/combat.go GetDefenseScoreFor (AFTER defence sets)
      - dodge  = Dex + unarmed*W                (x DodgeEffectiveness 0.95)
      - parry  = Dex + weapon*W + parryRating   (x ParryEffectiveness 0.97)
      - block  = (Str+Dex)/2 + weapon*W + blockRating (x BlockEff 1.05)

Shipped config (git show HEAD:_datafiles/config.yaml, 2026-08-19):
  RollSpread 0.15, SkillWeight 5.0, ContestFloor 0.125,
  Dodge/Parry/Block effectiveness 0.95/0.97/1.05,
  MinAttackCritChance 0.01, MinDefenseCritChance 0.01,
  CritDamageBase 2.0, CritDamagePerSkill 0.05, RangedShieldDefenseBonus 15.

Method: Monte Carlo, numpy, fixed seed 42, N = 2,000,000 samples per cell.
"""

import numpy as np

# ---- shipped config -------------------------------------------------------
ROLL_SPREAD = 0.15
SKILL_WEIGHT = 5.0
CONTEST_FLOOR = 0.125
EFF = {"dodge": 0.95, "parry": 0.97, "block": 1.05}
MIN_ATK_CRIT = 0.01
MIN_DEF_CRIT = 0.01
CRIT_DMG_BASE = 2.0
CRIT_DMG_PER_SKILL = 0.05
RANGED_SHIELD_BONUS = 15.0
CONTEST_CRIT_THRESHOLD = 2.0  # margin_crit.go const; also the defensive bar
SQRT2 = np.sqrt(2.0)

N = 2_000_000
RNG_SEED = 42

# Representative gear (median-ish of shipped items): parryrating ~5, shield
# blockrating ~15. Mobs are modelled gearless (dodge only) unless noted.
PARRY_RATING = 5.0
BLOCK_RATING = 15.0

# Mitigation assumptions for the damage-expectation columns (stated, not
# code): trash mob 0% (unarmoured), player defenders 25%.
MITIG_MOB = 0.0
MITIG_PLAYER = 0.25


def std_dev_for(mean):
    # dice.StdDevFor: mean * RollSpread, floor 1.0 (mean < 1 -> 1.0)
    if mean < 1.0:
        return 1.0
    return max(mean * ROLL_SPREAD, 1.0)


def crit_dmg_mult(rank):
    if rank <= 0:
        return CRIT_DMG_BASE
    return CRIT_DMG_BASE + CRIT_DMG_PER_SKILL * rank


def run_cell(A, defs, rng, crit_bar=CONTEST_CRIT_THRESHOLD, crit_enabled=True):
    """One contested attack, ExecuteSkillMove-shaped, N samples.

    Returns dict of per-attempt probabilities and the expected defence damage
    multiplier split into (crit portion, non-crit portion).
    defs: list of fully-modified defence scores (contest.Entry.Score).
    """
    s = std_dev_for(A)
    X = rng.normal(A, s, N)
    Ymax = np.full(N, -np.inf)
    for D in defs:
        Y = rng.normal(D, s, N)
        np.maximum(Ymax, Y, out=Ymax)
    margin = X - Ymax                      # attack-positive, best defence
    win = margin > 0

    flip = rng.random(N) < CONTEST_FLOOR   # RunWithFloors: one flip max
    final_win = np.where(flip, ~win, win)
    floored = flip

    z = margin / (s * SQRT2)               # normalized attack margin

    # --- attack crit (AFTER only) -----------------------------------------
    if crit_enabled:
        atk_crit = final_win & ~floored & (z >= crit_bar)
        # ApplyCritFloor: 1% of wins promoted (incl. floored wins: the
        # sentinel z is ~0 so margin-crit is false, but the floor still rolls)
        promote = rng.random(N) < MIN_ATK_CRIT
        atk_crit = atk_crit | (final_win & promote)
    else:
        atk_crit = np.zeros(N, dtype=bool)

    # --- defence damage multiplier (defenceDamageMultiplier) --------------
    loss = ~final_win
    mult = np.ones(N)
    floored_loss = loss & floored
    mult[floored_loss] = 0.5               # floored save: bare win, no crit
    genuine_loss = loss & ~floored
    defz = -z                              # defence-positive normalized margin
    def_crit = genuine_loss & (defz >= CONTEST_CRIT_THRESHOLD)
    promote_d = rng.random(N) < MIN_DEF_CRIT
    def_crit = def_crit | (genuine_loss & promote_d)
    rolled = genuine_loss & ~def_crit
    mult[rolled] = np.clip(0.5 - 0.25 * defz[rolled], 0.0, 0.5)
    mult[def_crit] = 0.0

    return {
        "p_win_raw": float(np.mean(win)),          # pre-floor
        "p_hit": float(np.mean(final_win)),        # post-floor
        "p_crit": float(np.mean(atk_crit)),        # per attempt
        "p_defended": float(np.mean(loss)),
        "p_def_crit": float(np.mean(def_crit)),
        # expected multiplier on the NON-crit damage path (crit samples
        # excluded; they take the bypass path instead)
        "e_mult_noncrit": float(np.mean(np.where(atk_crit, 0.0, mult))),
        # legacy expected multiplier over everything (BEFORE: no crit)
        "e_mult": float(np.mean(mult)),
    }


def expected_damage(cell, mitig, rank, crit_enabled):
    """Expected damage per attempt in units of RAW (pre-mitigation) damage.

    BEFORE: (1-mitig) * E[mult]        (no crit path exists)
    AFTER : P(crit)*critMult(rank)  +  (1-mitig) * E[mult | not crit]
    (CritOrMitigatedDamage: crit bypasses mitigation, takes no defence mult.)
    """
    if not crit_enabled:
        return (1.0 - mitig) * cell["e_mult"]
    return cell["p_crit"] * crit_dmg_mult(rank) \
        + (1.0 - mitig) * cell["e_mult_noncrit"]


def dyn_bar(atk_skill, def_combat_skill):
    # combat_helpers.go calcCritThreshold, no Accuracy/Blink modelled
    bar = 2.0 - 0.05 * (atk_skill - def_combat_skill)
    return max(bar, 1.5)


# ---- tiers ---------------------------------------------------------------
# All stats at the tier value; all combat skills at the tier skill.
# Meirok: real values from users/3.yaml (Str 115, Dex ~110 effective,
# Per 101, weapon-combat 69, unarmed-combat 57, ranged-combat 1).
TIERS = [
    # name, stat, skill (generic), str, dex, per, weapon, unarmed, ranged
    ("novice",     100,  5, 100, 100, 100,  5,  5,  5),
    ("journeyman", 120, 25, 120, 120, 120, 25, 25, 25),
    ("adept",      135, 40, 135, 135, 135, 40, 40, 40),
    ("meirok",     None, None, 115, 110, 101, 69, 57, 1),
]

MOB_STAT = 90.0
MOB_COMBAT_SKILL = 1      # GetCombatSkillLevel fallback (BEFORE scalar)
MOB_RAW_SKILL = 0         # GetSkillLevel(unarmed) for a mob with no skills
                          # block -> 0, NOT 1 (skills.go:166-180)


def player_defset_melee(strv, dex, weapon, unarmed):
    """AFTER defence set for a special move (ChannelMelee: dodge/parry/block),
    armed with weapon + shield."""
    dodge = (dex + unarmed * SKILL_WEIGHT) * EFF["dodge"]
    parry = (dex + weapon * SKILL_WEIGHT + PARRY_RATING) * EFF["parry"]
    block = ((strv + dex) / 2 + weapon * SKILL_WEIGHT + BLOCK_RATING) * EFF["block"]
    return [dodge, parry, block]


def player_defset_ranged(strv, dex, weapon, unarmed, shield):
    dodge = (dex + unarmed * SKILL_WEIGHT) * EFF["dodge"]
    out = [dodge]
    if shield:
        block = ((strv + dex) / 2 + weapon * SKILL_WEIGHT + BLOCK_RATING) * EFF["block"]
        out.append(block)
    return out


def mob_defset():
    # Gearless mob: dodge only; skill term 0 (GetSkillLevel -> 0).
    return [(MOB_STAT + MOB_RAW_SKILL * SKILL_WEIGHT) * EFF["dodge"]]


def fmt_pct(x):
    return f"{100*x:.1f}%"


def main():
    rng = np.random.default_rng(RNG_SEED)
    out = []
    say = out.append

    # ---- sanity anchors --------------------------------------------------
    say("## Sanity anchors")
    c = run_cell(200.0, [200.0], rng)
    say(f"- parity single-defence P(win) pre-floor = {fmt_pct(c['p_win_raw'])} "
        f"(want 50.0%), post-floor P(hit) = {fmt_pct(c['p_hit'])} (want 50.0%)")
    # parity margin ~ N(0, s*sqrt2): P(z>=2) = 2.28% unconditional
    s = std_dev_for(200.0)
    raw_crit = float(np.mean(rng.normal(200, s, N) - rng.normal(200, s, N)
                             >= 2.0 * s * SQRT2))
    say(f"- parity P(margin-crit) unconditional = {fmt_pct(raw_crit)} "
        f"(want ~2.3%)")
    say(f"- ContestFloor 0.125 clamps every final hit rate to [12.5%, 87.5%].")
    say("")

    # ---- roadmap anchor: 130 vs 101 -> 250 vs 105 ------------------------
    say("## Roadmap anchor (weapon-combat-30 player vs mob)")
    b = run_cell(130.0, [101.0], rng, crit_enabled=False)
    a = run_cell(250.0, [105.0], rng, crit_enabled=True,
                 crit_bar=CONTEST_CRIT_THRESHOLD)
    a_dyn = run_cell(250.0, [105.0], rng, crit_enabled=True,
                     crit_bar=dyn_bar(30, 1))
    say(f"- BEFORE 130 vs 101: P(hit) {fmt_pct(b['p_hit'])} "
        f"(pre-floor {fmt_pct(b['p_win_raw'])}), E[dmg mult] {b['e_mult']:.3f}")
    say(f"- AFTER  250 vs 105: P(hit) {fmt_pct(a['p_hit'])} "
        f"(pre-floor {fmt_pct(a['p_win_raw'])}), "
        f"P(crit) const-2.0 {fmt_pct(a['p_crit'])}, "
        f"dyn-bar({dyn_bar(30,1):.2f}) {fmt_pct(a_dyn['p_crit'])}")
    say("")

    # ---- ITEM 3+4: special moves, player attacker ------------------------
    say("## Items 3+4 - special moves (bash-shape), PLAYER attacker")
    say("")
    say("Attack = Str + weapon-combat; damage rank = weapon-combat.")
    say("Defender BEFORE: one scalar Dex + combatSkill(x1). AFTER: "
        "dodge/parry/block set, skill x5, effectiveness applied. "
        "Mob mitig 0%, player mitig 25% (assumption). E[dmg] in units of "
        "raw pre-mitigation damage; ratio = AFTER/BEFORE compounded "
        "(x5 flip + crit tier together).")
    say("")
    hdr = ("| tier | opponent | A_before vs D | A_after vs D(best) | "
           "P(hit) B->A | P(crit) c2.0 | P(crit) dyn(bar) | "
           "E[dmg] B | E[dmg] A (c2.0/dyn) | ratio (c2.0/dyn) |")
    say(hdr)
    say("|---|---|---|---|---|---|---|---|---|---|")

    for name, statv, skillv, strv, dex, per, weapon, unarmed, ranged in TIERS:
        A_b = strv + weapon * 1.0
        A_a = strv + weapon * SKILL_WEIGHT
        rank = weapon
        # vs trash mob
        Db = [MOB_STAT + MOB_COMBAT_SKILL]
        Da = mob_defset()
        cb = run_cell(A_b, Db, rng, crit_enabled=False)
        bar_d = dyn_bar(weapon, MOB_COMBAT_SKILL)
        ca = run_cell(A_a, Da, rng, crit_enabled=True)
        ca_d = run_cell(A_a, Da, rng, crit_enabled=True, crit_bar=bar_d)
        eb = expected_damage(cb, MITIG_MOB, rank, False)
        ea = expected_damage(ca, MITIG_MOB, rank, True)
        ea_d = expected_damage(ca_d, MITIG_MOB, rank, True)
        say(f"| {name} | trash mob | {A_b:.0f} vs {Db[0]:.0f} | "
            f"{A_a:.0f} vs {max(Da):.1f} | "
            f"{fmt_pct(cb['p_hit'])} -> {fmt_pct(ca['p_hit'])} | "
            f"{fmt_pct(ca['p_crit'])} | {fmt_pct(ca_d['p_crit'])} ({bar_d:.2f}) | "
            f"{eb:.3f} | {ea:.3f} / {ea_d:.3f} | "
            f"{ea/eb:.2f}x / {ea_d/eb:.2f}x |")
        # vs same-tier player (armed + shield)
        Db = [dex + weapon * 1.0]
        Da = player_defset_melee(strv, dex, weapon, unarmed)
        cb = run_cell(A_b, Db, rng, crit_enabled=False)
        bar_d = dyn_bar(weapon, weapon)  # parity skill -> 2.0
        ca = run_cell(A_a, Da, rng, crit_enabled=True)
        ca_d = run_cell(A_a, Da, rng, crit_enabled=True, crit_bar=bar_d)
        eb = expected_damage(cb, MITIG_PLAYER, rank, False)
        ea = expected_damage(ca, MITIG_PLAYER, rank, True)
        ea_d = expected_damage(ca_d, MITIG_PLAYER, rank, True)
        say(f"| {name} | same-tier player | {A_b:.0f} vs {Db[0]:.0f} | "
            f"{A_a:.0f} vs {max(Da):.1f} | "
            f"{fmt_pct(cb['p_hit'])} -> {fmt_pct(ca['p_hit'])} | "
            f"{fmt_pct(ca['p_crit'])} | {fmt_pct(ca_d['p_crit'])} ({bar_d:.2f}) | "
            f"{eb:.3f} | {ea:.3f} / {ea_d:.3f} | "
            f"{ea/eb:.2f}x / {ea_d/eb:.2f}x |")
    say("")

    # ratio sweep, abstract (scale-invariant in A for fixed RollSpread)
    say("### Abstract single-defence ratio sweep (D = r x A; scale-invariant)")
    say("")
    say("| r | P(hit) post-floor | P(crit@2.0) | E[def mult] |")
    say("|---|---|---|---|")
    for r in [0.5, 0.8, 1.0, 1.2, 1.5, 2.0]:
        c = run_cell(200.0, [200.0 * r], rng, crit_enabled=True)
        say(f"| {r:.1f} | {fmt_pct(c['p_hit'])} | {fmt_pct(c['p_crit'])} | "
            f"{c['e_mult']:.3f} |")
    say("")

    # ---- ITEM 4b: beast moves, MOB attacker ------------------------------
    say("## Item 4b - beast move (pounce-shape), MOB attacker vs player tiers")
    say("")
    say("Mob attack = Dex 90 + GetSkillLevel(unarmed)=0 (NOT the combat-skill-1")
    say("fallback; callers pass GetSkillLevel, skills.go returns 0 for a mob")
    say("with no skills block). So the mob attack score is 90 BEFORE and "
        "AFTER - x5 of zero is zero. Damage rank 0 -> critMult = 2.0 base.")
    say("")
    say("| defender tier | D before (scalar) | D after (best of 3) | "
        "P(hit) B->A | P(crit) c2.0 | P(crit) dyn(bar) | E[dmg] ratio A/B (c2.0) |")
    say("|---|---|---|---|---|---|---|")
    A_mob = MOB_STAT + MOB_RAW_SKILL  # 90, both regimes
    for name, statv, skillv, strv, dex, per, weapon, unarmed, ranged in TIERS:
        Db = [dex + weapon * 1.0]
        Da = player_defset_melee(strv, dex, weapon, unarmed)
        cb = run_cell(A_mob, Db, rng, crit_enabled=False)
        bar_d = dyn_bar(MOB_COMBAT_SKILL, weapon)  # rises: 2.0+0.05*(K-1)
        ca = run_cell(A_mob, Da, rng, crit_enabled=True)
        ca_d = run_cell(A_mob, Da, rng, crit_enabled=True, crit_bar=bar_d)
        eb = expected_damage(cb, MITIG_PLAYER, 0, False)
        ea = expected_damage(ca, MITIG_PLAYER, 0, True)
        say(f"| {name} | {Db[0]:.0f} | {max(Da):.1f} | "
            f"{fmt_pct(cb['p_hit'])} -> {fmt_pct(ca['p_hit'])} | "
            f"{fmt_pct(ca['p_crit'])} | {fmt_pct(ca_d['p_crit'])} ({bar_d:.2f}) | "
            f"{ea/eb:.2f}x |")
    say("")

    # ---- ITEM 5: ranged --------------------------------------------------
    say("## Item 5 - ranged (`shoot`), same-tier attacker vs defender")
    say("")
    say("Attack = Per + ranged-combat (x1 before, x5 after; "
        "RangedShotScale 1.0, weapon mult 1.0 assumed). BEFORE defender = "
        "Dex + combatSkill(x1) + 15 flat if shield. AFTER = dodge "
        "(+ block only WITH shield, blockrating 15). Mitig 25%.")
    say("")
    say("| tier | shield? | D before | D after (best) | P(defended) B->A | "
        "E[def mult] B->A | E[dmg] B->A | shield worth AFTER |")
    say("|---|---|---|---|---|---|---|---|")
    for name, statv, skillv, strv, dex, per, weapon, unarmed, ranged in TIERS:
        A_b = per + ranged * 1.0
        A_a = per + ranged * SKILL_WEIGHT
        rank = ranged
        results = {}
        for shield in (False, True):
            Db = [dex + weapon * 1.0 + (RANGED_SHIELD_BONUS if shield else 0.0)]
            Da = player_defset_ranged(strv, dex, weapon, unarmed, shield)
            cb = run_cell(A_b, Db, rng, crit_enabled=False)
            ca = run_cell(A_a, Da, rng, crit_enabled=True)
            eb = expected_damage(cb, MITIG_PLAYER, rank, False)
            ea = expected_damage(ca, MITIG_PLAYER, rank, True)
            results[shield] = (Db[0], max(Da), cb, ca, eb, ea)
        for shield in (False, True):
            Db0, Damax, cb, ca, eb, ea = results[shield]
            tag = "yes" if shield else "no"
            if shield:
                worth = (f"E[dmg] {results[True][5]/results[False][5]-1:+.1%} "
                         f"vs no-shield")
            else:
                worth = "-"
            say(f"| {name} | {tag} | {Db0:.0f} | {Damax:.1f} | "
                f"{fmt_pct(cb['p_defended'])} -> {fmt_pct(ca['p_defended'])} | "
                f"{cb['e_mult']:.3f} -> {ca['e_mult']:.3f} | "
                f"{eb:.3f} -> {ea:.3f} | {worth} |")
    say("")

    # ---- ITEM 6: defence-set width ---------------------------------------
    say("## Item 6 - pure defence-set width (equal scores, no effectiveness)")
    say("")
    say("Same defender capability D = r x A contested as 1, 2, or 3 identical")
    say("entries against ONE attack roll (correlated through it).")
    say("")
    say("| r | width | P(defended) | E[def mult] | dmg vs width-1 |")
    say("|---|---|---|---|---|")
    for r in [0.5, 0.8, 1.0, 1.2, 1.5, 2.0]:
        base_e = None
        for w in (1, 2, 3):
            c = run_cell(200.0, [200.0 * r] * w, rng, crit_enabled=False)
            e = c["e_mult"]
            if w == 1:
                base_e = e
            say(f"| {r:.1f} | {w} | {fmt_pct(c['p_defended'])} | {e:.3f} | "
                f"{e/base_e-1:+.1%} |")
    say("")

    print("\n".join(out))


if __name__ == "__main__":
    main()
