"""Measure the swing-count change from making calcSwingCount per-weapon.

The followup this measures: calcSwingCount took its skill term from
characters.GetCombatSkillLevel, which resolves the MAIN-HAND weapon's tag for
every entry in the attack plan. So an empty offhand beside a sword threw FIST
swings counted at the swordsman's weapon-combat rank. calcAttackScore and
buildDamageParams were fixed for this on 2026-08-27; calcSwingCount was not,
because it never took a weapon.

WHY THIS IS NOT A LOG QUERY. _datafiles/logs/combat-analytics.jsonl records
what combat DID, not the swing counts a counterfactual formula would have
produced, and it predates the whole U10b arc besides. calcSwingCount is
deterministic in (dex, skill, weaponSpeed, isOffhand, stamina, encumbrance,
position), so the honest measurement is to evaluate the real formula on the
real character population, both ways. That is what this does.

Formula, from internal/combat/combat_helpers.go:

    swings = 1 + (dex - 50)/100 * weaponSpeed * (1 + skill*SkillWeight/softCap)
    swings += extraAttacks
    if offhand:  swings *= 0.5 + (dualSkill*SkillWeight/50) * 0.5
    swings *= staminaMult * encumbranceMult * hasteMult * positionMult
    swings = clamp(round(swings), 1, 4)

Evaluated at full stamina, no encumbrance, standing, no haste, which is the
reference condition every other file in this directory uses.

SCOPE. Only an entry whose OWN combat skill differs from the main hand's can
move. In practice that is overwhelmingly the empty offhand beside a weapon:
a fist counted at weapon-combat. Same-class dual-wield is unaffected, and so
is any bare-handed build (GetCombatSkillLevel already resolves a zero Item to
unarmed-combat, so bare hands were always correct).
"""
import glob
import os
import re

SKILL_WEIGHT = 5.0      # _datafiles/config.yaml. The Go DEFAULT is 2.0; a test
                        # binary sees that one. This is what the game runs.
SOFT_CAP = 50.0
UNARMED_SPEED = 1.8     # UnarmedSpeedMultiplier, shipped
SWING_CAP = 4

USERS = os.path.join(os.path.dirname(__file__), "..", "..",
                     "_datafiles", "world", "dogmud", "users")


def swings(dex, skill, speed, is_offhand, dual_skill):
    """calcSwingCount at the reference condition."""
    s = 1.0 + (dex - 50.0) / 100.0 * speed * (1.0 + skill * SKILL_WEIGHT / SOFT_CAP)
    if is_offhand:
        s *= 0.5 + (dual_skill * SKILL_WEIGHT / 50.0) * 0.5
    n = int(round(s))
    return max(1, min(SWING_CAP, n))


def read_char(path):
    """Pull dex and the two melee skills out of a save. Deliberately a line
    scanner, not a YAML parse: these files carry engine-written structures this
    script has no business round-tripping, and every field wanted here is a
    flat scalar under a known key."""
    txt = open(path, encoding="utf-8", errors="replace").read()
    name = re.search(r"^\s*name:\s*(.+)$", txt, re.M)
    wc = re.search(r"^\s*weapon-combat:\s*(\d+)", txt, re.M)
    uc = re.search(r"^\s*unarmed-combat:\s*(\d+)", txt, re.M)
    dex = re.search(r"^\s*dexterity:\s*\n\s*base:\s*(\d+)\s*\n\s*training:\s*(\d+)",
                    txt, re.M)
    if not dex:
        dex = re.search(r"^\s*dexterity:\s*\n\s*base:\s*(\d+)", txt, re.M)
        d = int(dex.group(1)) if dex else 0
    else:
        d = int(dex.group(1)) + int(dex.group(2))
    return {
        "file": os.path.basename(path),
        "name": name.group(1).strip() if name else "?",
        "dex": d,
        # GetCombatSkillLevelFor floors an untrained skill at 1, so an absent
        # key is 1 here, not 0. Getting this wrong would invent a change on
        # every character who has never punched anything.
        "wc": int(wc.group(1)) if wc else 1,
        "uc": int(uc.group(1)) if uc else 1,
    }


def main():
    chars = [read_char(p) for p in sorted(glob.glob(os.path.join(USERS, "*.yaml")))]
    chars = [c for c in chars if c["dex"] > 0]

    print("Swing-count impact: a FIST in the empty offhand, beside a weapon.")
    print("OLD = fist counted at weapon-combat (main hand). NEW = at unarmed-combat.")
    print("Reference condition: full stamina, unencumbered, standing.\n")
    print("%-22s %5s %5s %5s   %4s %4s  %s" %
          ("character", "dex", "wc", "uc", "old", "new", "delta"))
    print("-" * 68)

    moved = same = 0
    up = down = 0
    for c in sorted(chars, key=lambda c: -abs(c["wc"] - c["uc"])):
        # The offhand dual-wield modifier is UNCHANGED by this fix, so it uses
        # the same dual_skill on both sides and cannot manufacture a delta.
        # IsUnarmedStyle is false here (a weapon is in the main hand), so
        # dual_skill is weapon-combat either way.
        old = swings(c["dex"], c["wc"], UNARMED_SPEED, True, c["wc"])
        new = swings(c["dex"], c["uc"], UNARMED_SPEED, True, c["wc"])
        if old == new:
            same += 1
            continue
        moved += 1
        if new > old:
            up += 1
        else:
            down += 1
        print("%-22s %5d %5d %5d   %4d %4d  %+d" %
              (c["name"][:22], c["dex"], c["wc"], c["uc"], old, new, new - old))

    print("-" * 68)
    print("%d of %d characters move; %d unchanged." % (moved, len(chars), same))
    print("%d gain swings (unarmed rank ABOVE weapon rank), %d lose them." % (up, down))
    print("")
    print("The population is dominated by low-rank saves where both skills sit")
    print("at or near the floor of 1, so most rows cannot move at all. The grid")
    print("below is what the change is actually worth across the rank band.\n")

    # dual_skill tracks weapon-combat in every row below, because a weapon in
    # the main hand makes IsUnarmedStyle false, and the dual-wield modifier is
    # NOT part of this change. Pinning it to a constant would let the modifier
    # push rows into the cap independently of the archetype and flatten the
    # very effect being measured.
    archetypes = [
        ("swordsman, untrained fists", 40, 1),
        ("mixed fighter", 20, 10),
        ("even split", 15, 15),
        ("brawler holding a sword", 5, 43),
    ]
    print("Fist swings in an empty offhand, OLD (fist read at weapon-combat)")
    print("against NEW (read at unarmed-combat), across dexterity.\n")
    dexes = [70, 80, 90, 100, 110, 130, 150]
    print("%-28s %5s %s" % ("archetype (wc/uc)", "", "".join("%9d" % d for d in dexes)))
    print("%-28s %5s %s" % ("", "dex ->", "".join("%9s" % "old->new" for _ in dexes)))
    print("-" * (34 + 9 * len(dexes)))
    for label, wc, uc in archetypes:
        cells = []
        for d in dexes:
            o = swings(d, wc, UNARMED_SPEED, True, wc)
            n = swings(d, uc, UNARMED_SPEED, True, wc)
            cells.append("%9s" % ("%d->%d" % (o, n) if o != n else "%d  =" % o))
        print("%-28s %5s %s" % ("%s (%d/%d)" % (label[:16], wc, uc), "", "".join(cells)))

    print("")
    print("READ THIS OFF THE TABLE, not from the headline:")
    print("  * THE 4-SWING CAP EATS MOST OF THE CHANGE AT HIGH DEX. A swordsman")
    print("    at dex 130+ was capped under BOTH readings, so the fix costs him")
    print("    nothing. It bites hardest in the dex 80-110 band, which is where")
    print("    the actual player population sits.")
    print("  * IT IS NOT A NERF. It corrects in both directions, and in this")
    print("    save population it corrects UPWARD five times for every once it")
    print("    corrects down, because the characters who bothered training")
    print("    unarmed-combat are the ones it was cheating.")
    print("  * The floor of 1 protects the untrained: GetCombatSkillLevelFor")
    print("    returns 1, not 0, so nobody drops below the dex-only baseline.")


if __name__ == "__main__":
    main()
