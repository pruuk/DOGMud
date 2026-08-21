#!/usr/bin/env python3
"""U6b modelling gate, group A: the spell and taunt hit-gate collapse.

Models §7 items 1-2 of docs/superpowers/specs/2026-08-19-u6b-finish-the-flip-design.md.

Method: deterministic NUMERIC INTEGRATION (midpoint rule over the attacker's
self-relative z, closed-form normal conditionals for the defender's roll),
mirroring the STEPS/Z_LIM idiom of tools/balance/unified_resolution_model.py.
No Monte Carlo, no sampling noise.

The model mirrors the CODE, not the spec prose:

  * contest.Run: attack ~ N(a, s), each defence ~ N(d, s) with s = 0.15*a
    (the ATTACKER's stdDev on both sides). Margin M = A - D, attack-positive.
  * contest.RunWithFloors (via combat.RunContest, ContestFloor = 0.125):
    the outcome is FLIPPED with probability 0.125 (one draw), and a flipped
    outcome carries a +-1 sentinel margin (raw units), which normalizes to
    ~0 sigma -> a floored hit cannot be a ROLLED crit.
  * Fumble is checked FIRST in both spell_resolution.go and combat_taunt.go:
    AttackRoll.ZScore <= -2.0 aborts the cast/taunt (backfire / self-damage)
    even when the contest was won. Self-relative, so a flat ~2.275% per attempt.
  * AttackContestCrit = ContestCrit(margin/(s*sqrt2) >= 2.0) then a 1%
    promotion floor (MinAttackCritChance). NOTE: unlike melee's
    applyCritFloors, the spell/taunt sites have NO res.Floored guard, so a
    floored win can still be floor-promoted to a crit at 1%. Modelled as coded.
  * defenceDamageMultiplier: attack win -> 1.0; floored save -> 0.5;
    DefenseContestCrit (dm >= 2.0, then 1% MinDefenseCritChance promotion,
    only on non-floored losses) -> 0.0; rolled defensive win -> 0.5 - 0.25*dm
    where dm = -M/(s*sqrt2) in [0, 2).
  * BEFORE spell: gate = CalcSpellAttack(stat, skill) = stat + skill*5*3
    vs spellDefenseValue = defender RAW Willpower (skill x0). Crit from the
    gate margin. Quell (ResolveChannelDefence, ChannelSpellMental) runs ONLY
    on non-crit hits: ChannelAttackScore = Wil + spellcasting*5 vs
    quell = (Wil_def + spellcasting_def*5) * QuellEffectiveness(1.0).
  * BEFORE taunt: gate = (Cha + rhet*5) * convMult vs Wil_def + rhet_def*5.
    Crit from the gate margin. Defy runs only on non-crit hits:
    Cha + rhet*5 (NO convMult - code omission the spec notes) vs
    (Wil_def + rhet_def*5) * DefyEffectiveness(1.0).
  * AFTER (collapsed): ONE floored contest, attacker stat + skill*5 vs the
    quell/defy entry; fumble stays self-relative and first; crit from THIS
    margin; defensive crit negates; a rolled defensive win deals PARTIAL
    damage (0-50%) per defenceDamageMultiplier - "fizzle becomes an ordinary
    defence outcome". Taunt keeps convMult on the surviving score (spec 4.1).

Shipped values (verified against `git show HEAD:_datafiles/config.yaml`):
RollSpread 0.15, SkillWeight 5.0, SpellAttackSkillFactor 3, ContestFloor 0.125,
QuellEffectiveness 1.0, DefyEffectiveness 1.0, MinAttackCritChance 0.01,
MinDefenseCritChance 0.01, ConvictionPenaltyMax 0.28, ResourcePenaltyCurve 2.0.
ContestCritThreshold 2.0 is a Go const (margin_crit.go).

Because s scales with the attacker score, every contest outcome depends only on
the ratio r = d/a; scenario scores reduce to ratios before integration.
"""

import math

# ---- shipped constants (see docstring for provenance) ----
SPREAD = 0.15
CONTEST_FLOOR = 0.125
CRIT_T = 2.0
ATK_CRIT_FLOOR = 0.01     # MinAttackCritChance
DEF_CRIT_FLOOR = 0.01     # MinDefenseCritChance
SKILL_WEIGHT = 5.0
SPELL_ATK_SKILL_FACTOR = 3
QUELL_EFF = 1.0
DEFY_EFF = 1.0
CONV_PENALTY_MAX = 0.28
RESOURCE_CURVE = 2.0

SQRT2 = math.sqrt(2.0)
Z_LO, Z_HI = -8.0, 8.0
STEPS = 8000  # dz = 0.002; the fumble boundary z=-2 falls exactly on a cell edge

PHI = lambda z: 0.5 * (1.0 + math.erf(z / SQRT2))
phi = lambda z: math.exp(-0.5 * z * z) / math.sqrt(2.0 * math.pi)


def conv_mult(cp_ratio):
    """combat.ResourceMultiplier for the conviction pool."""
    if cp_ratio >= 1.0:
        return 1.0
    return 1.0 - CONV_PENALTY_MAX * (1.0 - cp_ratio) ** RESOURCE_CURVE


_cache = {}


def contest(ratio, fumble_gate=True, floor=CONTEST_FLOOR,
            atk_floor=ATK_CRIT_FLOOR, def_floor=DEF_CRIT_FLOOR):
    """Full outcome distribution of ONE floored contest at defender/attacker
    score ratio `ratio`, in collapsed (U6-melee-like) semantics.

    Returns dict with per-attempt probabilities:
      p_fumble   attacker self-relative z <= -2 (only when fumble_gate)
      p_succ     attack success (damage multiplier 1.0): genuine unflipped win
                 + floor-rescued loss
      p_crit     attacker crit (rolled crit surviving the flip, + 1% promotion
                 of every remaining success incl. floored rescues, as coded)
      p_fsave    floored save (genuine win flipped): multiplier 0.5
      p_defcrit  defensive crit, full negation (rolled dm>=2 + 1% promotion of
                 rolled partial saves; never on floored outcomes)
      p_partial  rolled defensive win, partial damage 0-50%
      e_mult     expected damage multiplier per attempt (fumble counts as 0)
    Outcome identities: p_fumble + p_succ + p_fsave + p_defcrit + p_partial = 1.
    """
    key = (round(ratio, 10), fumble_gate, floor, atk_floor, def_floor)
    if key in _cache:
        return _cache[key]

    delta = (1.0 - ratio) / SPREAD  # (a - d) / sigma
    dz = (Z_HI - Z_LO) / STEPS
    out = dict(p_fumble=0.0, p_succ=0.0, p_crit=0.0, p_fsave=0.0,
               p_defcrit=0.0, p_partial=0.0, e_mult=0.0)

    for i in range(STEPS):
        z = Z_LO + (i + 0.5) * dz
        w = phi(z) * dz
        if fumble_gate and z <= -2.0:
            out['p_fumble'] += w
            continue

        L = z + delta                      # defender z_d < L  <=>  genuine win
        pw = PHI(L)
        pc_roll = PHI(L - CRIT_T * SQRT2)  # rolled crit: M >= 2*s*sqrt2

        succ = pw * (1.0 - floor) + (1.0 - pw) * floor
        # crit: rolled crit surviving the flip, then the 1% floor promotion on
        # every other success (spell/taunt sites lack melee's Floored guard).
        crit = pc_roll * (1.0 - floor) + atk_floor * (succ - pc_roll * (1.0 - floor))

        fsave = pw * floor                 # genuine win flipped -> mult 0.5

        # genuine loss, unflipped, splits by defender margin dm = (z_d - L)/sqrt2
        p_dcrit_roll = 1.0 - PHI(L + CRIT_T * SQRT2)     # dm >= 2
        m_part = PHI(L + CRIT_T * SQRT2) - PHI(L)        # 0 <= dm < 2
        # E[z_d * 1{a<z_d<b}] = phi(a) - phi(b)
        e_zd = phi(L) - phi(L + CRIT_T * SQRT2)
        e_dm_mass = (e_zd - L * m_part) / SQRT2          # E[dm * 1{partial}]
        e_mult_partial_mass = 0.5 * m_part - 0.25 * e_dm_mass

        dcrit = (1.0 - floor) * (p_dcrit_roll + def_floor * m_part)
        partial = (1.0 - floor) * (1.0 - def_floor) * m_part

        out['p_succ'] += w * succ
        out['p_crit'] += w * crit
        out['p_fsave'] += w * fsave
        out['p_defcrit'] += w * dcrit
        out['p_partial'] += w * partial
        out['e_mult'] += w * (succ * 1.0 + fsave * 0.5 +
                              (1.0 - floor) * (1.0 - def_floor) * e_mult_partial_mass)

    _cache[key] = out
    return out


# ---------------- channel compositions ----------------

def spell_before(gate_ratio, quell_ratio):
    """Today's two-contest spell shape. Hit = gate success; crit from the gate
    margin; quell only on non-crit hits, scaling damage by its multiplier."""
    g = contest(gate_ratio, fumble_gate=True)
    q = contest(quell_ratio, fumble_gate=False)
    p_hit = g['p_succ']
    p_crit = g['p_crit']
    p_noncrit = p_hit - p_crit
    return dict(
        p_hit=p_hit, p_crit=p_crit,
        p_defcrit=p_noncrit * q['p_defcrit'],
        e_mult=p_crit * 1.0 + p_noncrit * q['e_mult'],
        p_fumble=g['p_fumble'],
    )


def collapsed(ratio):
    """AFTER shape for both channels: one floored contest, fumble-first,
    crit from its margin, defence outcomes per defenceDamageMultiplier."""
    c = contest(ratio, fumble_gate=True)
    return dict(p_hit=c['p_succ'], p_crit=c['p_crit'],
                p_defcrit=c['p_defcrit'], e_mult=c['e_mult'],
                p_fumble=c['p_fumble'], p_partial=c['p_partial'],
                p_fsave=c['p_fsave'])


def taunt_before(atk_base, def_score, cp_ratio):
    """Today's two-contest taunt. Gate attacker carries convMult; defy attacker
    does NOT (code omission); defender score identical in both contests."""
    cm = conv_mult(cp_ratio)
    gate_ratio = def_score / (atk_base * cm)
    defy_ratio = (def_score * DEFY_EFF) / atk_base
    g = contest(gate_ratio, fumble_gate=True)
    d = contest(defy_ratio, fumble_gate=False)
    p_hit = g['p_succ']
    p_crit = g['p_crit']
    p_noncrit = p_hit - p_crit
    return dict(p_hit=p_hit, p_crit=p_crit,
                p_defcrit=p_noncrit * d['p_defcrit'],
                e_mult=p_crit * 1.0 + p_noncrit * d['e_mult'],
                p_fumble=g['p_fumble'])


def taunt_after(atk_base, def_score, cp_ratio):
    cm = conv_mult(cp_ratio)
    return collapsed((def_score * DEFY_EFF) / (atk_base * cm))


# ---------------- fixtures ----------------

# Standard tiers (stat, skill). Meirok from _datafiles/world/dogmud/users/3.yaml:
# Wil 148 (113 base + 35 training), Cha 123 (93 + 30), spellcasting 52, rhetoric 55.
SPELL_TIERS = [
    ('novice',     100, 5),
    ('journeyman', 120, 25),
    ('adept',      135, 40),
    ('Meirok',     148, 52),
]
TAUNT_TIERS = [
    ('novice',     100, 5),
    ('journeyman', 120, 25),
    ('adept',      135, 40),
    ('Meirok',     123, 55),
]

TRASH_STAT, TRASH_SKILL = 90, 1   # all mobs carry combat skill 1
RATIOS = [0.5, 0.8, 1.0, 1.2, 1.5, 2.0]


def spell_gate_score(stat, skill):
    # CalcSpellAttack: stat + round(skill*SkillWeight)*SpellAttackSkillFactor
    return stat + round(skill * SKILL_WEIGHT) * SPELL_ATK_SKILL_FACTOR


def chan_score(stat, skill):
    return stat + skill * SKILL_WEIGHT


# ---------------- sanity anchors ----------------

def anchors():
    raw = contest(1.0, fumble_gate=False, floor=0.0, atk_floor=0.0, def_floor=0.0)
    assert abs(raw['p_succ'] - 0.5) < 1e-4, raw['p_succ']
    # parity rolled-crit anchor: P(M >= 2*s*sqrt2) = 1 - PHI(2) = 2.275% of rolls
    assert abs(raw['p_crit'] - (1.0 - PHI(2.0))) < 5e-4, raw['p_crit']
    # arc anchor: per contested WIN that is ~4.55%, per ROLL ~2.275%
    fl = contest(1.0, fumble_gate=False)
    # floors at parity leave p_succ at 0.5 (symmetric flip)
    assert abs(fl['p_succ'] - 0.5) < 1e-4, fl['p_succ']
    print('ANCHORS OK: parity raw P(win)=%.4f, rolled crit/roll=%.4f (target 0.02275),'
          ' crit/win=%.4f, floored parity P(succ)=%.4f'
          % (raw['p_succ'], raw['p_crit'], raw['p_crit'] / raw['p_succ'], fl['p_succ']))


# ---------------- report ----------------

def pct(x):
    return '%5.1f%%' % (100.0 * x)


def row(label, b, a):
    def rat(x, y):
        return ('%4.2fx' % (x / y)) if y > 1e-9 else '  inf'
    print('%-26s | %s %s %s %5.3f | %s %s %s %5.3f | %s %s %s %s' % (
        label,
        pct(b['p_hit']), pct(b['p_crit']), pct(b['p_defcrit']), b['e_mult'],
        pct(a['p_hit']), pct(a['p_crit']), pct(a['p_defcrit']), a['e_mult'],
        rat(a['p_hit'], b['p_hit']), rat(a['p_crit'], b['p_crit']),
        rat(a['p_defcrit'], b['p_defcrit']) if b['p_defcrit'] > 1e-9 else '  new',
        rat(a['e_mult'], b['e_mult'])))


def header(title):
    print()
    print('=== %s ===' % title)
    print('%-26s | %s | %s | after/before' % (
        '', 'BEFORE  hit  crit  dcrit  E[mult]', 'AFTER   hit  crit  dcrit  E[mult]'))


def spell_tables():
    header('SPELL: real defenders (trash mob 90/skill1; mirror = same-tier caster)')
    for name, stat, skill in SPELL_TIERS:
        ga = spell_gate_score(stat, skill)
        ca = chan_score(stat, skill)
        # trash mob: gate vs raw Wil 90, quell/after vs (90 + 1*5)*1.0
        tq = chan_score(TRASH_STAT, TRASH_SKILL) * QUELL_EFF
        b = spell_before(TRASH_STAT / ga, tq / ca)
        a = collapsed(tq / ca)
        row('%s vs trash' % name, b, a)
    for name, stat, skill in SPELL_TIERS:
        ga = spell_gate_score(stat, skill)
        ca = chan_score(stat, skill)
        mq = chan_score(stat, skill) * QUELL_EFF
        b = spell_before(stat / ga, mq / ca)     # mirror: gate d = defender raw Wil
        a = collapsed(mq / ca)
        row('%s vs mirror' % name, b, a)

    header('SPELL: vs plain non-caster (Wil 100, spellcasting 0)')
    for name, stat, skill in SPELL_TIERS:
        ga = spell_gate_score(stat, skill)
        ca = chan_score(stat, skill)
        b = spell_before(100.0 / ga, 100.0 * QUELL_EFF / ca)
        a = collapsed(100.0 * QUELL_EFF / ca)
        row('%s vs plain-100' % name, b, a)

    header('SPELL: trash mob (90/skill1) CASTING AT each tier (PvM reverse direction)')
    mob_gate = spell_gate_score(TRASH_STAT, TRASH_SKILL)   # 90 + 5*3 = 105
    mob_chan = chan_score(TRASH_STAT, TRASH_SKILL)         # 95
    for name, stat, skill in SPELL_TIERS:
        dq = chan_score(stat, skill) * QUELL_EFF
        b = spell_before(stat / mob_gate, dq / mob_chan)   # gate vs player raw Wil
        a = collapsed(dq / mob_chan)
        row('mob vs %s' % name, b, a)

    header('SPELL: per-contest ratio sweep (defender score = r x attacker score '
           'in EVERY contest; tier-independent)')
    for r in RATIOS:
        b = spell_before(r, r)
        a = collapsed(r)
        row('ratio %.1f' % r, b, a)

    print()
    print('--- SPELL gate reality check (BEFORE): defender raw Wil needed to pull '
          'the gate to 50% ---')
    for name, stat, skill in SPELL_TIERS:
        ga = spell_gate_score(stat, skill)
        print('  %-11s gate score %4d -> defender needs raw Wil ~%4d for a 50%% gate'
              % (name, ga, ga))

    print()
    print('--- SPELL crossover: defender Wil=100; minimal spellcasting rank where '
          'AFTER P(hit) < 50%% ---')
    for name, stat, skill in SPELL_TIERS:
        ca = chan_score(stat, skill)
        found = None
        for sc in range(0, 400):
            d = chan_score(100, sc) * QUELL_EFF
            if collapsed(d / ca)['p_hit'] < 0.5:
                found = sc
                break
        parity_sc = (ca - 100.0) / SKILL_WEIGHT
        print('  %-11s attack %4.0f: AFTER P(hit)<50%% at defender spellcasting %s '
              '(score-parity at rank %.1f); BEFORE the gate needed defender raw Wil %d'
              % (name, ca, found, parity_sc, spell_gate_score(stat, skill)))


def taunt_tables():
    for cp, lbl in [(1.0, 'full CP'),
                    (0.5, '50%% CP (convMult %.3f)' % conv_mult(0.5))]:
        header('TAUNT at %s: real defenders' % lbl)
        for name, stat, skill in TAUNT_TIERS:
            atk = chan_score(stat, skill)
            d_trash = chan_score(TRASH_STAT, TRASH_SKILL)
            row('%s vs trash' % name,
                taunt_before(atk, d_trash, cp), taunt_after(atk, d_trash, cp))
        # mirror: defender Wil = tier's Wil-analog. Use the SPELL tier stat as Wil
        # (novice 100 / jman 120 / adept 135 / Meirok 148) + tier rhetoric x5.
        for (name, cha, rhet), (_, wil, _) in zip(TAUNT_TIERS, SPELL_TIERS):
            atk = chan_score(cha, rhet)
            d = chan_score(wil, rhet)
            row('%s vs mirror' % name,
                taunt_before(atk, d, cp), taunt_after(atk, d, cp))

    header('TAUNT full CP: trash mob HOWLING AT each tier (PvM reverse direction)')
    mob_atk = chan_score(TRASH_STAT, TRASH_SKILL)          # 95
    for (name, _cha, rhet), (_, wil, _) in zip(TAUNT_TIERS, SPELL_TIERS):
        d = chan_score(wil, rhet)                          # Wil + rhet*5
        row('mob vs %s' % name,
            taunt_before(mob_atk, d, 1.0), taunt_after(mob_atk, d, 1.0))

    header('TAUNT full CP: per-contest ratio sweep (tier-independent)')
    for r in RATIOS:
        # express as scores so both paths see the same ratio at full CP
        b = taunt_before(100.0, 100.0 * r, 1.0)
        a = taunt_after(100.0, 100.0 * r, 1.0)
        row('ratio %.1f' % r, b, a)

    print()
    print('--- What the double contest costs the taunter today (full CP): E[mult] on a '
          'NON-CRIT hit, i.e. the defy contest alone ---')
    for r in RATIOS:
        d = contest(r, fumble_gate=False)
        print('  defy ratio %.1f: E[mult | non-crit hit] = %.3f  '
              '(P full dmg %.1f%%, P halved-or-worse %.1f%%, P negated %.1f%%)'
              % (r, d['e_mult'], 100 * d['p_succ'],
                 100 * (d['p_fsave'] + d['p_partial']), 100 * d['p_defcrit']))


CRIT_DMG_BASE = 2.0        # CritDamageBase (shipped)
CRIT_DMG_PER_SKILL = 0.05  # CritDamagePerSkill (shipped)


def crit_damage_mult(rank):
    """combat.CritDamageMultiplier. NOTE the per-channel rank divergence:
    the spell path passes the RAW spellcasting rank (combat_shared_helpers.go
    passes skillLevel), the taunt path passes the x5-WEIGHTED rhetoric
    (combat_taunt.go passes int(attackerRhetoric) = rhet*SkillWeight)."""
    if rank <= 0:
        return CRIT_DMG_BASE
    return CRIT_DMG_BASE + CRIT_DMG_PER_SKILL * rank


def e_damage(res, crit_mult, mitig):
    """Expected damage per attempt in units of one raw unmitigated hit.
    res['e_mult'] counts crits at 1.0, so lift them out: crits bypass
    mitigation and scale by crit_mult; everything else is mitigated."""
    return res['p_crit'] * crit_mult + (res['e_mult'] - res['p_crit']) * (1.0 - mitig)


def damage_weighted_tables():
    print()
    print('=== DAMAGE-WEIGHTED NET (units: one raw unmitigated hit per cast/taunt) ===')
    print('Crits bypass mitigation and scale by CritDamageMultiplier; the E[mult]')
    print('tables above count a crit as 1.0 and therefore UNDERSTATE the crit-rate')
    print('collapse. Spell crit rank = raw spellcasting; taunt crit rank = rhet*5.')
    for mitig in (0.0, 0.30):
        print()
        print('--- target mitigation %.0f%% ---' % (100 * mitig))
        print('%-30s | before  after  after/before' % ('SPELL (vs trash / vs mirror)'))
        for name, stat, skill in SPELL_TIERS:
            ga, ca = spell_gate_score(stat, skill), chan_score(stat, skill)
            cmult = crit_damage_mult(skill)
            tq = chan_score(TRASH_STAT, TRASH_SKILL) * QUELL_EFF
            bt = e_damage(spell_before(TRASH_STAT / ga, tq / ca), cmult, mitig)
            at = e_damage(collapsed(tq / ca), cmult, mitig)
            bm = e_damage(spell_before(stat / ga, 1.0), cmult, mitig)
            am = e_damage(collapsed(1.0), cmult, mitig)
            print('%-30s | trash %5.2f -> %5.2f (%4.2fx)   mirror %5.2f -> %5.2f (%4.2fx)'
                  % (name, bt, at, at / bt, bm, am, am / bm))
        print('%-30s | before  after  after/before' % ('TAUNT full CP'))
        for (name, cha, rhet), (_, wil, _) in zip(TAUNT_TIERS, SPELL_TIERS):
            atk = chan_score(cha, rhet)
            cmult = crit_damage_mult(int(rhet * SKILL_WEIGHT))
            d_trash = chan_score(TRASH_STAT, TRASH_SKILL)
            bt = e_damage(taunt_before(atk, d_trash, 1.0), cmult, mitig)
            at = e_damage(taunt_after(atk, d_trash, 1.0), cmult, mitig)
            d_m = chan_score(wil, rhet)
            bm = e_damage(taunt_before(atk, d_m, 1.0), cmult, mitig)
            am = e_damage(taunt_after(atk, d_m, 1.0), cmult, mitig)
            print('%-30s | trash %5.2f -> %5.2f (%4.2fx)   mirror %5.2f -> %5.2f (%4.2fx)'
                  % ('%s (critx%.2f)' % (name, cmult), bt, at, at / bt, bm, am, am / bm))


def headline():
    print()
    print('=== HEADLINE: Meirok-tier caster vs parity defender (per-contest parity) ===')
    b = spell_before(1.0, 1.0)
    a = collapsed(1.0)
    print('  spell BEFORE: P(hit) %s  P(crit) %s  E[mult] %.3f' %
          (pct(b['p_hit']), pct(b['p_crit']), b['e_mult']))
    print('  spell AFTER : P(hit) %s  P(crit) %s  E[mult] %.3f' %
          (pct(a['p_hit']), pct(a['p_crit']), a['e_mult']))
    # the REAL Meirok story: mirror match
    stat, skill = 148, 52
    ga, ca = spell_gate_score(stat, skill), chan_score(stat, skill)
    b2 = spell_before(stat / ga, 1.0)
    a2 = collapsed(1.0)
    print('  Meirok vs Meirok-mirror BEFORE: P(hit) %s  P(crit) %s  E[mult] %.3f '
          '(gate ratio %.2f)' % (pct(b2['p_hit']), pct(b2['p_crit']), b2['e_mult'],
                                 stat / ga))
    print('  Meirok vs Meirok-mirror AFTER : P(hit) %s  P(crit) %s  E[mult] %.3f' %
          (pct(a2['p_hit']), pct(a2['p_crit']), a2['e_mult']))


if __name__ == '__main__':
    anchors()
    spell_tables()
    taunt_tables()
    damage_weighted_tables()
    headline()


# ── U6b Task 1 addendum: the SHIPPED crit bar ────────────────────────────────
# The tables above use CRIT_T = 2.0, the constant bar. U6b ships the bar as a
# clamped function of the CHANNEL's skill pair (owner decision 2026-08-19):
#
#   bar = clamp(2.0 - CritBarSkillSlope*(atk_rank - def_rank),
#               CritBarFloor, CritBarCeiling)     # 0.05 / 1.5 / 3.0 shipped
#
# Everything except P(crit) is bar-independent, so only crit columns move.
def shipped_crit_bar(atk_rank, def_rank, slope=0.05, floor=1.5, ceiling=3.0):
    bar = 2.0 - slope * (atk_rank - def_rank)
    if bar < floor:
        bar = floor
    if ceiling > 0 and bar > ceiling:
        bar = ceiling
    return bar


def shipped_bar_deltas():
    """Crit columns for the §5.1 royalty cells under the shipped bar."""
    print('\n== shipped-bar crit deltas (Queen sc1 vs Meirok sc52; bar %.2f) =='
          % shipped_crit_bar(1, 52))
    meirok_quell = 408.0
    for gold, wil in ((300, 329), (500, 545), (1000, 1075), (2000, 2135)):
        a = wil + 5.0
        mu = (a - meirok_quell) / (SPREAD * a * math.sqrt(2.0))
        for name, bar in (('const 2.0', 2.0),
                          ('shipped', shipped_crit_bar(1, 52)),
                          ('uncapped', 2.0 + 0.05 * 51)):
            crit = max(0.0, 1.0 - PHI(bar - mu))
            print('  %5dg  %-9s bar=%.2f  P(crit)=%6.1f%%'
                  % (gold, name, bar, crit * 100.0))


if __name__ == '__main__' and True:
    shipped_bar_deltas()
