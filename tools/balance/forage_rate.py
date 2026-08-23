"""Closed-form forage success rate -> realised perception uses/hour.

Forage is the anchor faucet for perception (owner ruling, from play). Its
success is a plain gaussian check, so the rate is computable exactly rather
than sampled:

    searchScore = Perception.ValueAdj + SkillMultiplier(searchRank) * 25
                                        (actions/skill_helpers.go:87-90)
    roll        = Normal(searchScore, searchScore * RollSpread)
                                        (dice.RollStat -> StdDevFor)
    success     iff roll >= ForageDifficulty[biome]
                                        (forager/forage_core.go:129-130)

Progression fires only on a successful find (actions/forage.go:142), and the
cooldown is a hardcoded "6 rounds" (forage.go:63) -> 150 attempts/hour ceiling
at RoundSeconds=4.

    realised perception uses/hour = 150 * P(success) * engagement

Verified against config.yaml at HEAD. All constants are read from the shipped
values, not Go defaults.
"""
import math

ROLL_SPREAD = 0.15        # GamePlay.RollSpread
SKILL_MULT_BASE = 1.0     # Balance.SkillMultiplierBase
SKILL_MULT_MAX = 3.0      # Balance.SkillMultiplierMax
SKILL_SOFT_CAP = 50.0     # Balance.SkillSoftCap
ROUNDS_PER_HOUR = 3600 / 4
FORAGE_COOLDOWN_ROUNDS = 6            # hardcoded literal, NOT a knob
ATTEMPT_CEILING = ROUNDS_PER_HOUR / FORAGE_COOLDOWN_ROUNDS   # 150

DIFFICULTY = {
    "farmland": 110, "forest": 120, "land": 125, "swamp": 130,
    "shore": 135, "water": 135, "cave": 135, "mountains": 140, "cliffs": 145,
}


def skill_multiplier(rank):
    r = min(rank, SKILL_SOFT_CAP)
    return SKILL_MULT_BASE + (SKILL_MULT_MAX - SKILL_MULT_BASE) * math.sqrt(r / SKILL_SOFT_CAP)


def search_score(perception, search_rank):
    return perception + skill_multiplier(search_rank) * 25.0


def p_success(perception, search_rank, biome):
    score = search_score(perception, search_rank)
    sd = score * ROLL_SPREAD
    if sd <= 0:
        return 0.0
    z = (DIFFICULTY[biome] - score) / sd
    return 0.5 * math.erfc(z / math.sqrt(2.0))


def main():
    print("Forage success probability by (perception, Search rank), per biome")
    print("searchScore = perception + SkillMultiplier(rank)*25; roll ~ N(score, 0.15*score)\n")

    profiles = [
        ("fresh char", 100, 0),
        ("early", 100, 10),
        ("mid", 110, 20),
        ("practised", 120, 35),
        ("soft-capped", 130, 50),
        ("Meirok-ish", 150, 50),
    ]
    biomes = ["farmland", "forest", "land", "swamp", "mountains", "cliffs"]

    hdr = "%-14s %6s %5s %7s" % ("profile", "per", "rank", "score")
    for b in biomes:
        hdr += " %9s" % b[:9]
    print(hdr)
    for name, per, rank in profiles:
        row = "%-14s %6d %5d %7.1f" % (name, per, rank, search_score(per, rank))
        for b in biomes:
            row += " %8.1f%%" % (100 * p_success(per, rank, b))
        print(row)

    print()
    print("Realised perception uses/hour (ceiling %d attempts/hr, forest biome):" % ATTEMPT_CEILING)
    print("%-14s %10s %12s %12s" % ("profile", "P(find)", "@100% eng", "@10% eng"))
    for name, per, rank in profiles:
        p = p_success(per, rank, "forest")
        print("%-14s %9.1f%% %11.1f %12.1f"
              % (name, 100 * p, ATTEMPT_CEILING * p, ATTEMPT_CEILING * p * 0.10))

    print()
    print("Compare the UNGATED perception faucet (look/consider, no cooldown at all):")
    print("  1 command/sec sustained = 3600 uses/hr, ~24x forage's best case.")
    print()
    print("NOTE: forage trains the SEARCH SKILL; perception is the auto-rolled")
    print("primary stat behind it (skills.go SkillPrimaryStats). So one forage")
    print("success advances both, and the skill rank feeds back into searchScore.")


if __name__ == "__main__":
    main()
