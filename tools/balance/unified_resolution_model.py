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
import itertools
import math
import re
from dataclasses import dataclass
from pathlib import Path
from statistics import mean

PHI = lambda z: 0.5 * (1.0 + math.erf(z / math.sqrt(2.0)))
phi = lambda z: math.exp(-0.5 * z * z) / math.sqrt(2 * math.pi)

ROLL_SPREAD = 0.15
CRIT_Z = 2.0
CRIT_BASE = 2.0          # 5.11g, shipped
CRIT_PER_SKILL = 0.05    # 5.11g, shipped

STEPS = 4000
Z_LIM = 6.0

ROOT = Path(__file__).resolve().parents[2]
CONFIG_PATH = ROOT / "_datafiles" / "config.yaml"
VETERAN_PROFILE_PATH = ROOT / "tools" / "playtest" / "profiles" / "veteran.yaml"

PHYSICAL_CANDIDATES = (0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 2.5, 3.0, 3.5, 4.0)
RHETORIC_CANDIDATES = (1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 8.0, 10.0, 12.0)
GRAPPLE_CANDIDATES = (0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 2.5, 3.0, 4.0, 5.0)
LOAD_RATIOS = (0.0, 0.5, 0.75, 1.0)
TYPICAL_LOAD = 0.5
TRACE_ROUNDS = 30
GRAPPLE_ROUNDS = 10


def _yaml_scalar(path, key, missing=None):
    """Read one scalar from shipped YAML without adding a PyYAML dependency."""
    pattern = re.compile(rf"^\s*{re.escape(key)}:\s*([^\s#]+)")
    matches = []
    for line in path.read_text(encoding="utf-8").splitlines():
        match = pattern.match(line)
        if match:
            matches.append(match.group(1))
    if not matches and missing is not None:
        return float(missing)
    assert len(matches) == 1, f"expected one shipped {key}, found {len(matches)}"
    return float(matches[0])


def _load_shipped_values():
    keys = (
        "AttackBaseStaminaCost", "AttackCostModifier",
        "DefenceBaseStaminaCost", "DodgeCostModifier",
        "ParryCostModifier", "BlockCostModifier",
        "CostSkillMultAtZero", "CostSkillMultAtMid", "CostSkillMultAtCap",
        "CostSkillMidRank", "CostSkillCapRank",
        "CostEncumbranceKnee", "CostEncumbranceKneeMult",
        "CostEncumbranceMax", "CostTotalMultiplierMax",
        "SpecialMoveCooldown", "SneakFailCooldown",
        "GrappleStaminaCostPerRound", "GrappleControllerCostMultiplier",
        "GrappleControlledCostMultiplier",
        "PlayerStaminaRegenPct", "PlayerConvictionRegenPct",
        "StaminaBase", "StaminaPerStrength", "StaminaPerVitality",
        "StaminaPerWillpower", "ConvictionBase",
        "ConvictionPerCharisma", "ConvictionPerWillpower",
        "PoolReservationCapPct",
    )
    values = {key: _yaml_scalar(CONFIG_PATH, key) for key in keys if key != "SneakFailCooldown"}
    # SneakFailCooldown is absent from shipped YAML. ConfigInt therefore loads
    # as zero, and validateCombat deliberately defaults only negative values;
    # zero is the effective shipped "no retry delay" value.
    values["SneakFailCooldown"] = _yaml_scalar(CONFIG_PATH, "SneakFailCooldown", missing=0)
    return values


SHIPPED = _load_shipped_values()


def _profile_stats_and_skills(path):
    """Read the first character's persisted stats and skills from a profile."""
    stats = {}
    skills = {}
    section = None
    stat_name = None
    for line in path.read_text(encoding="utf-8").splitlines():
        if line == "  stats:":
            section = "stats"
            continue
        if line == "  skills:":
            section = "skills"
            continue
        if section and line.startswith("  ") and not line.startswith("    "):
            section = None
            stat_name = None
        if section == "stats":
            stat_match = re.match(r"^    ([a-z-]+):$", line)
            if stat_match:
                stat_name = stat_match.group(1)
                stats[stat_name] = {"base": 0, "training": 0}
                continue
            value_match = re.match(r"^      (base|training):\s*(-?\d+)", line)
            if value_match and stat_name:
                stats[stat_name][value_match.group(1)] = int(value_match.group(2))
        elif section == "skills":
            skill_match = re.match(r"^    ([a-z-]+):\s*(-?\d+)", line)
            if skill_match:
                skills[skill_match.group(1)] = int(skill_match.group(2))

    expected_stats = {
        "strength", "dexterity", "perception", "vitality", "willpower", "charisma"
    }
    assert set(stats) == expected_stats, f"profile stats changed: {sorted(stats)}"
    needed_skills = {
        "ranged-combat", "rhetoric", "skullduggery",
        "unarmed-combat", "weapon-combat",
    }
    assert needed_skills <= set(skills), "veteran profile is missing a U8 governing skill"
    # StatInfo.Recalculate is Value = Base + Training + Mods. This tracked band
    # deliberately uses the persisted base + training values; equipment Mods
    # are mutable playtest kit, not part of the anonymized character band.
    final_stats = {name: values["base"] + values["training"] for name, values in stats.items()}
    return final_stats, skills


@dataclass(frozen=True)
class Fixture:
    name: str
    stats: dict
    skills: dict

    def skill(self, tag):
        return self.skills[tag]

    @property
    def stamina_max(self):
        return int(
            SHIPPED["StaminaBase"]
            + self.stats["strength"] * SHIPPED["StaminaPerStrength"]
            + self.stats["vitality"] * SHIPPED["StaminaPerVitality"]
            + self.stats["willpower"] * SHIPPED["StaminaPerWillpower"]
        )

    @property
    def conviction_max(self):
        return int(
            SHIPPED["ConvictionBase"]
            + self.stats["charisma"] * SHIPPED["ConvictionPerCharisma"]
            + self.stats["willpower"] * SHIPPED["ConvictionPerWillpower"]
        )


def _synthetic_fixture(name, stat, skill):
    stats = {
        key: stat for key in
        ("strength", "dexterity", "perception", "vitality", "willpower", "charisma")
    }
    skills = {
        key: skill for key in
        ("ranged-combat", "rhetoric", "skullduggery", "unarmed-combat", "weapon-combat")
    }
    return Fixture(name, stats, skills)


LIVE_STATS, LIVE_SKILLS = _profile_stats_and_skills(VETERAN_PROFILE_PATH)
FIXTURES = (
    _synthetic_fixture("Novice", 100, 5),
    _synthetic_fixture("Mid-skill", 110, 25),
    _synthetic_fixture("Veteran", 136, 69),
    _synthetic_fixture("Synthetic high", 175, 100),
    Fixture("Anonymized live band", LIVE_STATS, LIVE_SKILLS),
)


def skill_cost_multiplier(rank):
    rank = max(0, rank)
    zero = SHIPPED["CostSkillMultAtZero"]
    mid = SHIPPED["CostSkillMultAtMid"]
    cap = SHIPPED["CostSkillMultAtCap"]
    mid_rank = int(SHIPPED["CostSkillMidRank"])
    cap_rank = int(SHIPPED["CostSkillCapRank"])
    if rank <= mid_rank:
        return zero + (mid - zero) * rank / mid_rank
    if rank >= cap_rank:
        return cap
    return mid + (cap - mid) * (rank - mid_rank) / (cap_rank - mid_rank)


def encumbrance_multiplier(load):
    load = min(1.0, max(0.0, load))
    knee = SHIPPED["CostEncumbranceKnee"]
    knee_mult = SHIPPED["CostEncumbranceKneeMult"]
    if load <= knee:
        return 1.0 + (knee_mult - 1.0) * load / knee
    maximum = SHIPPED["CostEncumbranceMax"]
    return knee_mult + (maximum - knee_mult) * (load - knee) / (1.0 - knee)


def action_cost(base, load, skill, physical=True, modifier=1.0):
    multiplier = skill_cost_multiplier(skill)
    if physical:
        multiplier *= encumbrance_multiplier(load)
    if modifier > 0:
        multiplier *= modifier
    multiplier = min(multiplier, SHIPPED["CostTotalMultiplierMax"])
    return base * multiplier


def ordinary_swing_cost(character, load, skill):
    """Reference price for exactly one ordinary swing, never a whole round."""
    del character  # The actor matters through load and the supplied governing skill.
    return action_cost(
        SHIPPED["AttackBaseStaminaCost"], load, skill,
        physical=True, modifier=SHIPPED["AttackCostModifier"],
    )


def usable_pool_max(raw_max):
    reserved = math.floor(raw_max * SHIPPED["PoolReservationCapPct"])
    return raw_max - reserved


def regen_per_hook(fixture, pool, combat):
    if pool == "stamina":
        amount = max(1, int(SHIPPED["PlayerStaminaRegenPct"] * fixture.stamina_max))
        if combat:
            amount = max(1, amount // 4)
        return amount
    return max(1, int(SHIPPED["PlayerConvictionRegenPct"] * fixture.conviction_max))


@dataclass
class PoolTrace:
    current: int
    carry: float = 0.0
    gross: float = 0.0
    zero_round: int = 0

    def commit(self, amount, full):
        self.gross += amount
        debt = self.carry + amount
        whole = math.floor(debt + 1e-12)
        if full and whole > self.current:
            return "refused"
        self.carry = debt - whole
        charged = min(self.current, whole)
        self.current -= charged
        if charged < whole:
            return "short"
        return "paid" if whole else "no-charge"


def run_trace(fixture, pool, events, rounds, combat, initial=None):
    raw_max = fixture.stamina_max if pool == "stamina" else fixture.conviction_max
    ceiling = usable_pool_max(raw_max)
    trace = PoolTrace(ceiling if initial is None else initial)
    rows = []
    for round_number in range(1, rounds + 1):
        statuses = []
        for amount, full in events.get(round_number, ()):
            statuses.append(trace.commit(amount, full))
        if trace.current == 0 and not trace.zero_round:
            trace.zero_round = round_number
        regen = 0
        # AutoHeal is a NewRound listener that runs every third round, after
        # combat resolution. This is the live hook cadence, not smoothed regen.
        if round_number % 3 == 0:
            regen = regen_per_hook(fixture, pool, combat)
            trace.current = min(ceiling, trace.current + regen)
        rows.append((round_number, trace.current, regen, ",".join(statuses) or "idle"))
    return trace, rows


@dataclass(frozen=True)
class CandidatePackage:
    special: float
    shoot: float
    reload: float
    sneak: float
    rhetoric: float
    grapple: float
    pressure_score: float = 0.0

    def values(self):
        return (self.special, self.shoot, self.reload, self.sneak, self.rhetoric, self.grapple)


def _rounds_every(interval, start=1, end=TRACE_ROUNDS):
    # A shipped zero cooldown means no delay: at most one attempt per round.
    return range(start, end + 1, max(1, interval))


def _special_skill_costs(fixture, base, load):
    # The family is one Weapon Combat action, ten Unarmed Combat actions, and
    # throw (Skullduggery). Weight the representative trace by those actions.
    return (
        action_cost(base, load, fixture.skill("weapon-combat")),
        *[action_cost(base, load, fixture.skill("unarmed-combat"))] * 10,
        action_cost(base, load, fixture.skill("skullduggery")),
    )


def _u8_trace_ratios(package, fixture):
    usable_stamina = usable_pool_max(fixture.stamina_max)
    usable_conviction = usable_pool_max(fixture.conviction_max)
    special_uses = len(tuple(_rounds_every(int(SHIPPED["SpecialMoveCooldown"]))))
    sneak_uses = len(tuple(_rounds_every(int(SHIPPED["SneakFailCooldown"]))))

    special = special_uses * mean(_special_skill_costs(fixture, package.special, TYPICAL_LOAD))
    ranged_mult = action_cost(1.0, TYPICAL_LOAD, fixture.skill("ranged-combat"))
    # With the current Loaded bool every shipped ranged weapon has capacity 1.
    # Round 1 shoots; rounds 2,6,... reload; the following rounds shoot.
    ranged = 8 * ranged_mult * (package.shoot + package.reload)
    rhetoric = special_uses * action_cost(
        package.rhetoric, 0.0, fixture.skill("rhetoric"), physical=False,
    )
    sneak = sneak_uses * action_cost(
        package.sneak, TYPICAL_LOAD, fixture.skill("skullduggery"),
    )
    grapple_mult = action_cost(1.0, TYPICAL_LOAD, fixture.skill("unarmed-combat"))
    controller = GRAPPLE_ROUNDS * package.grapple * SHIPPED["GrappleControllerCostMultiplier"] * grapple_mult
    controlled = GRAPPLE_ROUNDS * package.grapple * SHIPPED["GrappleControlledCostMultiplier"] * grapple_mult
    grapple = mean((controller, controlled))
    return (
        special / usable_stamina,
        ranged / usable_stamina,
        rhetoric / usable_conviction,
        sneak / usable_stamina,
        grapple / usable_stamina,
    )


def _combat_survives_to_round_20(package, fixture):
    events = {}
    attack = ordinary_swing_cost(
        fixture, TYPICAL_LOAD, fixture.skill("weapon-combat")
    )
    defense = action_cost(
        SHIPPED["DefenceBaseStaminaCost"], TYPICAL_LOAD,
        fixture.skill("unarmed-combat"), modifier=max(
            SHIPPED["DodgeCostModifier"], SHIPPED["ParryCostModifier"],
            SHIPPED["BlockCostModifier"],
        ),
    )
    special = max(_special_skill_costs(fixture, package.special, TYPICAL_LOAD))
    special_rounds = set(_rounds_every(int(SHIPPED["SpecialMoveCooldown"])))
    for round_number in range(1, TRACE_ROUNDS + 1):
        events[round_number] = [(attack, False), (defense, False)]
        if round_number in special_rounds:
            events[round_number].append((special, True))
    trace, _ = run_trace(fixture, "stamina", events, TRACE_ROUNDS, combat=True)
    return trace.zero_round == 0 or trace.zero_round >= 20


def _grapple_trace(package, fixture, role_multiplier):
    amount = action_cost(
        package.grapple * role_multiplier,
        TYPICAL_LOAD, fixture.skill("unarmed-combat"),
    )
    events = {round_number: [(amount, False)] for round_number in range(1, GRAPPLE_ROUNDS + 1)}
    return run_trace(fixture, "stamina", events, GRAPPLE_ROUNDS, combat=True)


def _candidate_passes(package):
    cooldown = int(SHIPPED["SpecialMoveCooldown"])
    special_uses = len(tuple(_rounds_every(cooldown)))
    for fixture in FIXTURES:
        for skill_tag in ("weapon-combat", "unarmed-combat", "skullduggery"):
            skill = fixture.skill(skill_tag)
            swing = ordinary_swing_cost(fixture, TYPICAL_LOAD, skill)
            special = action_cost(package.special, TYPICAL_LOAD, skill)
            assert package.special > 0
            if not (special > swing and special <= swing * 4):
                return False

        ranged_mult = action_cost(1.0, TYPICAL_LOAD, fixture.skill("ranged-combat"))
        shoot = package.shoot * ranged_mult
        reload_cost = package.reload * ranged_mult
        if reload_cost > shoot * 0.75 + 1e-12:
            return False

        usable_stamina = usable_pool_max(fixture.stamina_max)
        usable_conviction = usable_pool_max(fixture.conviction_max)
        ranged_ratio = 8 * (shoot + reload_cost) / usable_stamina
        rhetoric_cost = action_cost(
            package.rhetoric, 0.0, fixture.skill("rhetoric"), physical=False,
        )
        rhetoric_ratio = special_uses * rhetoric_cost / usable_conviction
        if not (0.05 <= ranged_ratio <= 0.35 and 0.05 <= rhetoric_ratio <= 0.35):
            return False

        voluntary = (
            *(_special_skill_costs(fixture, package.special, TYPICAL_LOAD)),
            shoot, reload_cost,
            action_cost(package.sneak, TYPICAL_LOAD, fixture.skill("skullduggery")),
            rhetoric_cost,
        )
        if any(cost > 0.25 * usable_stamina for cost in voluntary[:-1]):
            return False
        if rhetoric_cost > 0.25 * usable_conviction:
            return False
        if not _combat_survives_to_round_20(package, fixture):
            return False

        for load in LOAD_RATIOS:
            controller = action_cost(
                package.grapple * SHIPPED["GrappleControllerCostMultiplier"],
                load, fixture.skill("unarmed-combat"),
            )
            controlled = action_cost(
                package.grapple * SHIPPED["GrappleControlledCostMultiplier"],
                load, fixture.skill("unarmed-combat"),
            )
            configured_ratio = (
                SHIPPED["GrappleControlledCostMultiplier"]
                / SHIPPED["GrappleControllerCostMultiplier"]
            )
            if not controlled > controller:
                return False
            if abs(controlled / controller - configured_ratio) > configured_ratio * 0.01:
                return False

        for role in (
            SHIPPED["GrappleControllerCostMultiplier"],
            SHIPPED["GrappleControlledCostMultiplier"],
        ):
            trace, _ = _grapple_trace(package, fixture, role)
            if trace.zero_round and trace.zero_round < GRAPPLE_ROUNDS:
                return False

    novice = FIXTURES[0]
    laden_multiplier = min(
        encumbrance_multiplier(1.0) * skill_cost_multiplier(novice.skill("unarmed-combat")),
        SHIPPED["CostTotalMultiplierMax"],
    )
    if laden_multiplier > SHIPPED["CostTotalMultiplierMax"]:
        return False
    return True


def generate_candidate_packages():
    passing = []
    for values in itertools.product(
        PHYSICAL_CANDIDATES, PHYSICAL_CANDIDATES, PHYSICAL_CANDIDATES,
        PHYSICAL_CANDIDATES, RHETORIC_CANDIDATES, GRAPPLE_CANDIDATES,
    ):
        package = CandidatePackage(*values)
        if not _candidate_passes(package):
            continue
        score = mean(
            ratio
            for fixture in FIXTURES
            for ratio in _u8_trace_ratios(package, fixture)
        )
        passing.append(CandidatePackage(*values, pressure_score=score))
    passing.sort(key=lambda p: (p.pressure_score, *p.values()))
    assert passing, "U8 candidate grid has no package satisfying every gate"
    low = passing[0]
    high = passing[-1]
    target = (low.pressure_score + high.pressure_score) / 2.0
    midpoint = min(passing, key=lambda p: (abs(p.pressure_score - target), p.pressure_score, *p.values()))
    return passing, (low, midpoint, high)


def _fmt(value):
    return f"{value:.2f}".rstrip("0").rstrip(".")


def _full_pay_transition(fixture, pool, amount):
    raw_max = fixture.stamina_max if pool == "stamina" else fixture.conviction_max
    ceiling = usable_pool_max(raw_max)

    affordable = PoolTrace(ceiling)
    affordable_status = affordable.commit(amount, full=True)

    exhausted = PoolTrace(0)
    exhausted_status = exhausted.commit(amount, full=True)

    recovered = PoolTrace(0)
    recovered_round = 0
    recovered_status = "refused"
    for round_number in range(1, TRACE_ROUNDS + 1):
        if round_number % 3 == 0:
            recovered.current = min(
                ceiling,
                recovered.current + regen_per_hook(fixture, pool, combat=False),
            )
        recovered_status = recovered.commit(amount, full=True)
        if recovered_status != "refused":
            recovered_round = round_number
            break
    assert recovered_round, f"{pool} did not recover enough for a modelled action"
    return (
        affordable_status, affordable.current,
        exhausted_status, exhausted.current,
        recovered_round, recovered_status, recovered.current,
    )


def print_u8_model(selected):
    novice = FIXTURES[0]
    print("\n=== U8 action-cost candidate model ===")
    print("Shipped source values:")
    print(
        f"  cooldown={int(SHIPPED['SpecialMoveCooldown'])}, "
        f"sneak-fail={int(SHIPPED['SneakFailCooldown'])}, "
        f"reservation cap={SHIPPED['PoolReservationCapPct']:.0%}, "
        f"stamina/conviction regen={SHIPPED['PlayerStaminaRegenPct']:.0%}/"
        f"{SHIPPED['PlayerConvictionRegenPct']:.0%} per AutoHeal hook"
    )
    print("\n  Candidate packages (low / midpoint / high pressure):")
    print("  band       score   special shoot reload sneak rhetoric grapple")
    for label, package in zip(("low", "midpoint", "high"), selected):
        print(
            f"  {label:<10} {package.pressure_score:6.2%} "
            + " ".join(f"{_fmt(value):>6}" for value in package.values())
        )

    print("\n  Typical-load special cost (ordinary single-swing equivalents):")
    print("  band       fixture                    swing  special  equivalent")
    for label, package in zip(("low", "midpoint", "high"), selected):
        for fixture in FIXTURES:
            skill = fixture.skill("unarmed-combat")
            swing = ordinary_swing_cost(fixture, TYPICAL_LOAD, skill)
            special = action_cost(package.special, TYPICAL_LOAD, skill)
            print(
                f"  {label:<10} {fixture.name:<26} {swing:6.2f} "
                f"{special:8.2f} {special / swing:10.2f}x"
            )

    print("\n  Thirty-round ranged and rhetoric traces at the reservation ceiling:")
    print("  band       fixture                    ranged gross/end     rhetoric gross/end")
    for label, package in zip(("low", "midpoint", "high"), selected):
        for fixture in FIXTURES:
            ranged_mult = action_cost(1.0, TYPICAL_LOAD, fixture.skill("ranged-combat"))
            ranged_events = {}
            for round_number in range(1, TRACE_ROUNDS + 1):
                if round_number == 1 or (round_number >= 3 and (round_number - 3) % 4 == 0):
                    ranged_events[round_number] = [(package.shoot * ranged_mult, True)]
                elif round_number >= 2 and (round_number - 2) % 4 == 0:
                    ranged_events[round_number] = [(package.reload * ranged_mult, True)]
            ranged_trace, _ = run_trace(
                fixture, "stamina", ranged_events, TRACE_ROUNDS, combat=True,
            )
            rhetoric_amount = action_cost(
                package.rhetoric, 0.0, fixture.skill("rhetoric"), physical=False,
            )
            rhetoric_events = {
                round_number: [(rhetoric_amount, True)]
                for round_number in _rounds_every(int(SHIPPED["SpecialMoveCooldown"]))
            }
            rhetoric_trace, _ = run_trace(
                fixture, "conviction", rhetoric_events, TRACE_ROUNDS, combat=True,
            )
            print(
                f"  {label:<10} {fixture.name:<26} "
                f"{ranged_trace.gross:6.1f}/{ranged_trace.current:<4} "
                f"{rhetoric_trace.gross:9.1f}/{rhetoric_trace.current:<4}"
            )

    print("\n  Current one-projectile ranged cycle (all shipped ranged weapons):")
    print("  round      1  2  3  6  7 10 11 14 15 18 19 22 23 26 27 30")
    print("  action   shoot reload shoot reload shoot reload shoot reload shoot reload shoot reload shoot reload shoot reload")

    print("\n  Rhetoric gross pressure across reservation bands (fixture range):")
    print("  band       reserved    pressure range")
    for label, package in zip(("low", "midpoint", "high"), selected):
        for reserved_ratio in (0.0, SHIPPED["PoolReservationCapPct"] / 2, SHIPPED["PoolReservationCapPct"]):
            ratios = []
            for fixture in FIXTURES:
                usable = fixture.conviction_max - math.floor(fixture.conviction_max * reserved_ratio)
                cost = action_cost(package.rhetoric, 0.0, fixture.skill("rhetoric"), physical=False)
                ratios.append(8 * cost / usable)
            print(
                f"  {label:<10} {reserved_ratio:7.0%}    "
                f"{min(ratios):6.2%} .. {max(ratios):6.2%}"
            )

    print("\n  Ten-round novice grapple pools at typical load (controller/controlled):")
    print("  band       r1      r2      r3      r4      r5      r6      r7      r8      r9      r10")
    for label, package in zip(("low", "midpoint", "high"), selected):
        controller_trace, controller_rows = _grapple_trace(
            package, novice, SHIPPED["GrappleControllerCostMultiplier"],
        )
        controlled_trace, controlled_rows = _grapple_trace(
            package, novice, SHIPPED["GrappleControlledCostMultiplier"],
        )
        del controller_trace, controlled_trace
        pairs = [
            f"{controller[1]}/{controlled[1]}"
            for controller, controlled in zip(controller_rows, controlled_rows)
        ]
        print(f"  {label:<10} " + " ".join(f"{pair:>7}" for pair in pairs))

    print("\n  Full-pay admission transitions (novice, typical load):")
    print("  band       pool        affordable       exhausted       recovered")
    for label, package in zip(("low", "midpoint", "high"), selected):
        amounts = {
            "stamina": action_cost(
                package.special, TYPICAL_LOAD, novice.skill("unarmed-combat")
            ),
            "conviction": action_cost(
                package.rhetoric, 0.0, novice.skill("rhetoric"), physical=False
            ),
        }
        for pool, amount in amounts.items():
            transition = _full_pay_transition(novice, pool, amount)
            print(
                f"  {label:<10} {pool:<10} "
                f"{transition[0]:>8}/{transition[1]:<3} "
                f"{transition[2]:>9}/{transition[3]:<3} "
                f"r{transition[4]:<2} {transition[5]:>8}/{transition[6]:<3}"
            )

    print("\n  Laden-novice product-clamp assertions:")
    for label, package in zip(("low", "midpoint", "high"), selected):
        laden_novice_costs = [
            action_cost(base, 1.0, novice.skill("unarmed-combat"))
            for base in package.values()[:4]
        ]
        product_clamp_bounds = [
            base * SHIPPED["CostTotalMultiplierMax"] for base in package.values()[:4]
        ]
        assert all(cost <= bound + 1e-12 for cost, bound in zip(laden_novice_costs, product_clamp_bounds))
        print(f"  {label:<10} pass (all physical products <= {SHIPPED['CostTotalMultiplierMax']:.1f}x base)")


def assert_u8_acceptance(selected):
    assert len(selected) == 3
    novice = FIXTURES[0]
    for package in selected:
        for fixture in FIXTURES:
            for skill_tag in ("weapon-combat", "unarmed-combat", "skullduggery"):
                skill = fixture.skill(skill_tag)
                swing = ordinary_swing_cost(fixture, TYPICAL_LOAD, skill)
                special_move_cost = action_cost(package.special, TYPICAL_LOAD, skill)
                assert special_move_cost > swing
                assert special_move_cost <= swing * 4
            ranged_mult = action_cost(1.0, TYPICAL_LOAD, fixture.skill("ranged-combat"))
            shoot_cost = package.shoot * ranged_mult
            reload_cost = package.reload * ranged_mult
            assert reload_cost <= shoot_cost * 0.75 + 1e-12

        for load in LOAD_RATIOS:
            controller = action_cost(
                package.grapple * SHIPPED["GrappleControllerCostMultiplier"],
                load, novice.skill("unarmed-combat"),
            )
            controlled = action_cost(
                package.grapple * SHIPPED["GrappleControlledCostMultiplier"],
                load, novice.skill("unarmed-combat"),
            )
            assert controlled > controller

        laden_novice_costs = [
            action_cost(base, 1.0, novice.skill("unarmed-combat"))
            for base in package.values()[:4]
        ]
        product_clamp_bounds = [
            base * SHIPPED["CostTotalMultiplierMax"] for base in package.values()[:4]
        ]
        assert all(cost <= bound + 1e-12 for cost, bound in zip(laden_novice_costs, product_clamp_bounds))


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

    all_passing_packages, u8_candidate_packages = generate_candidate_packages()
    assert_u8_acceptance(u8_candidate_packages)
    print_u8_model(u8_candidate_packages)
    print(f"\n  {len(all_passing_packages)} packages passed every U8 gate.")
