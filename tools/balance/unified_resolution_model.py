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
from dataclasses import dataclass, field
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
ITEM_STATE_PATH = ROOT / "internal" / "items" / "items.go"
ITEM_SPEC_PATH = ROOT / "internal" / "items" / "itemspec.go"

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


def _discover_ranged_capacity():
    """Return the current projectile capacity asserted from the live item model."""
    state_source = ITEM_STATE_PATH.read_text(encoding="utf-8")
    spec_source = ITEM_SPEC_PATH.read_text(encoding="utf-8")
    assert re.search(r"\bLoaded\s+bool\s+`yaml:\"loaded,omitempty\"`", state_source), (
        "ranged state is no longer the asserted one-projectile Loaded bool"
    )
    assert not re.search(r"\b(?:Ammo|Magazine|Projectile)Capacity\b", state_source + spec_source), (
        "a ranged capacity field now exists; extend the capacity discovery model"
    )
    return 1


RANGED_WEAPON_CAPACITY = _discover_ranged_capacity()


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
    if modifier < 0:
        raise ValueError("action cost modifier cannot be negative")
    multiplier = skill_cost_multiplier(skill)
    if physical:
        multiplier *= encumbrance_multiplier(load)
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
    attempted: float = 0.0
    admitted: float = 0.0
    admitted_by_action: dict = field(default_factory=dict)
    zero_round: int = 0

    def commit(self, amount, full, action=""):
        self.attempted += amount
        debt = self.carry + amount
        whole = math.floor(debt + 1e-12)
        if full and whole > self.current:
            return "refused"
        self.carry = debt - whole
        charged = min(self.current, whole)
        self.current -= charged
        self.admitted += charged
        if action:
            self.admitted_by_action[action] = self.admitted_by_action.get(action, 0.0) + charged
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
        for event in events.get(round_number, ()):
            if isinstance(event[0], str):
                name, amount, full = event[:3]
                state_path = "->".join(event[3:])
            else:
                name = ""
                amount, full = event
                state_path = ""
            status = trace.commit(amount, full, name)
            if name:
                label = f"{name}:{status}"
                if state_path:
                    label += f"[{state_path}]"
                statuses.append(label)
            else:
                statuses.append(status)
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


def selected_action_equivalents(package, fixture):
    """Price every selected base in ordinary swings for the same skill rank."""
    specifications = (
        ("special", "stamina", "unarmed-combat", package.special, True, 1.0),
        ("shoot", "stamina", "ranged-combat", package.shoot, True, 1.0),
        ("reload", "stamina", "ranged-combat", package.reload, True, 1.0),
        ("sneak", "stamina", "skullduggery", package.sneak, True, 1.0),
        ("rhetoric", "conviction", "rhetoric", package.rhetoric, False, 1.0),
        (
            "grapple-controller", "stamina", "unarmed-combat",
            package.grapple, True, SHIPPED["GrappleControllerCostMultiplier"],
        ),
        (
            "grapple-controlled", "stamina", "unarmed-combat",
            package.grapple, True, SHIPPED["GrappleControlledCostMultiplier"],
        ),
    )
    rows = []
    for action, pool, skill_tag, base, physical, modifier in specifications:
        skill = fixture.skill(skill_tag)
        cost = action_cost(
            base, TYPICAL_LOAD if physical else 0.0, skill,
            physical=physical, modifier=modifier,
        )
        swing = ordinary_swing_cost(fixture, TYPICAL_LOAD, skill)
        rows.append({
            "action": action,
            "pool": pool,
            "governing_skill": skill_tag,
            "cost": cost,
            "swing_equivalent": cost / swing,
        })
    return tuple(rows)


def _rounds_every(interval, start=1, end=TRACE_ROUNDS):
    # A shipped zero cooldown means no delay: at most one attempt per round.
    return range(start, end + 1, max(1, interval))


def _ranged_action_schedule(cooldown=None, capacity=None, rounds=TRACE_ROUNDS):
    """Build the one authoritative shoot/reload cadence from live constraints."""
    cooldown = int(SHIPPED["SpecialMoveCooldown"] if cooldown is None else cooldown)
    capacity = RANGED_WEAPON_CAPACITY if capacity is None else capacity
    assert cooldown >= 1
    assert capacity >= 1

    schedule = []
    loaded = capacity
    reload_ready = 1
    shoot_ready = 1
    for round_number in range(1, rounds + 1):
        if loaded > 0 and round_number >= shoot_ready:
            schedule.append((round_number, "shoot"))
            loaded -= 1
        elif loaded == 0 and round_number >= reload_ready:
            schedule.append((round_number, "reload"))
            loaded = capacity
            reload_ready = round_number + cooldown
            shoot_ready = round_number + 1
    return tuple(schedule)


def ranged_event_sequence(package, fixture, cooldown=None, capacity=None):
    ranged_mult = action_cost(1.0, TYPICAL_LOAD, fixture.skill("ranged-combat"))
    costs = {
        "shoot": package.shoot * ranged_mult,
        "reload": package.reload * ranged_mult,
    }
    events = {}
    for round_number, action in _ranged_action_schedule(cooldown, capacity):
        events[round_number] = [(action, costs[action], True)]
    return events


def special_combat_events(package, fixture):
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
    events = {}
    for round_number in range(1, TRACE_ROUNDS + 1):
        events[round_number] = [
            ("attack", attack, False),
            ("defence", defense, False),
        ]
        if round_number in special_rounds:
            events[round_number].append(("special", special, True))
    return events


def rhetoric_event_sequence(package, fixture):
    amount = action_cost(
        package.rhetoric, 0.0, fixture.skill("rhetoric"), physical=False,
    )
    return {
        round_number: [("rhetoric", amount, True)]
        for round_number in _rounds_every(int(SHIPPED["SpecialMoveCooldown"]))
    }


def sneak_event_sequence(package, fixture):
    amount = action_cost(
        package.sneak, TYPICAL_LOAD, fixture.skill("skullduggery"),
    )
    return {
        round_number: [(
            "sneak-detected", amount, True,
            "Visible", "Concealing", "Visible",
        )]
        for round_number in _rounds_every(int(SHIPPED["SneakFailCooldown"]))
    }


def _sequence_attempted(events):
    return sum(event[1] for round_events in events.values() for event in round_events)


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

    special_trace, _ = run_trace(
        fixture, "stamina", special_combat_events(package, fixture),
        TRACE_ROUNDS, combat=True,
    )
    special = special_trace.admitted_by_action.get("special", 0.0)

    ranged_trace, _ = run_trace(
        fixture, "stamina", ranged_event_sequence(package, fixture),
        TRACE_ROUNDS, combat=True,
    )
    ranged = (
        ranged_trace.admitted_by_action.get("shoot", 0.0)
        + ranged_trace.admitted_by_action.get("reload", 0.0)
    )

    rhetoric_trace, _ = run_trace(
        fixture, "conviction", rhetoric_event_sequence(package, fixture),
        TRACE_ROUNDS, combat=True,
    )
    rhetoric = rhetoric_trace.admitted_by_action.get("rhetoric", 0.0)

    sneak_trace, _ = run_trace(
        fixture, "stamina", sneak_event_sequence(package, fixture),
        TRACE_ROUNDS, combat=False,
    )
    sneak = sneak_trace.admitted_by_action.get("sneak-detected", 0.0)

    controller_trace, _ = _grapple_trace(
        package, fixture, SHIPPED["GrappleControllerCostMultiplier"],
    )
    controlled_trace, _ = _grapple_trace(
        package, fixture, SHIPPED["GrappleControlledCostMultiplier"],
    )
    grapple = mean((controller_trace.admitted, controlled_trace.admitted))
    return (
        special / usable_stamina,
        ranged / usable_stamina,
        rhetoric / usable_conviction,
        sneak / usable_stamina,
        grapple / usable_stamina,
    )


def _combat_survives_to_round_20(package, fixture):
    trace, _ = run_trace(
        fixture, "stamina", special_combat_events(package, fixture),
        TRACE_ROUNDS, combat=True,
    )
    return trace.zero_round == 0 or trace.zero_round >= 20


def _package_pressure_score(package):
    return mean(
        ratio
        for fixture in FIXTURES
        for ratio in _u8_trace_ratios(package, fixture)
    )


def _grapple_trace(package, fixture, role_multiplier):
    amount = action_cost(
        package.grapple * role_multiplier,
        TYPICAL_LOAD, fixture.skill("unarmed-combat"),
    )
    events = {round_number: [(amount, False)] for round_number in range(1, GRAPPLE_ROUNDS + 1)}
    return run_trace(fixture, "stamina", events, GRAPPLE_ROUNDS, combat=True)


def _candidate_passes(package):
    for fixture in FIXTURES:
        for skill_tag in ("weapon-combat", "unarmed-combat", "skullduggery"):
            skill = fixture.skill(skill_tag)
            swing = ordinary_swing_cost(fixture, TYPICAL_LOAD, skill)
            special = action_cost(package.special, TYPICAL_LOAD, skill)
            assert package.special > 0
            if not (special > swing and special <= swing * 4):
                return False

        ranged_events = ranged_event_sequence(package, fixture)
        shoot = next(event[1] for events in ranged_events.values() for event in events
                     if event[0] == "shoot")
        reload_cost = next(event[1] for events in ranged_events.values() for event in events
                           if event[0] == "reload")
        if reload_cost > shoot * 0.75 + 1e-12:
            return False

        usable_stamina = usable_pool_max(fixture.stamina_max)
        usable_conviction = usable_pool_max(fixture.conviction_max)
        ranged_trace, _ = run_trace(
            fixture, "stamina", ranged_events, TRACE_ROUNDS, combat=True,
        )
        ranged_ratio = ranged_trace.admitted / usable_stamina
        rhetoric_cost = action_cost(
            package.rhetoric, 0.0, fixture.skill("rhetoric"), physical=False,
        )
        rhetoric_trace, _ = run_trace(
            fixture, "conviction", rhetoric_event_sequence(package, fixture),
            TRACE_ROUNDS, combat=True,
        )
        rhetoric_ratio = rhetoric_trace.admitted / usable_conviction
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
        score = _package_pressure_score(package)
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


def print_u8_model(selected, owner_selected):
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
    print(
        f"  {'owner':<10} {owner_selected.pressure_score:6.2%} "
        + " ".join(f"{_fmt(value):>6}" for value in owner_selected.values())
    )

    print("\n  Owner-selected action costs (ordinary single-swing equivalents):")
    print("  fixture                    action                 pool        skill             cost equivalent")
    for fixture in FIXTURES:
        for row in selected_action_equivalents(owner_selected, fixture):
            print(
                f"  {fixture.name:<26} {row['action']:<22} {row['pool']:<11} "
                f"{row['governing_skill']:<17} {row['cost']:6.2f} "
                f"{row['swing_equivalent']:9.2f}x"
            )

    print("\n  Thirty-round ranged and rhetoric traces at the reservation ceiling:")
    print("  band       fixture                    ranged attempt/admit/end  rhetoric attempt/admit/end")
    displayed = (*zip(("low", "midpoint", "high"), selected), ("owner", owner_selected))
    for label, package in displayed:
        for fixture in FIXTURES:
            ranged_events = ranged_event_sequence(package, fixture)
            ranged_trace, _ = run_trace(
                fixture, "stamina", ranged_events, TRACE_ROUNDS, combat=True,
            )
            rhetoric_trace, _ = run_trace(
                fixture, "conviction", rhetoric_event_sequence(package, fixture),
                TRACE_ROUNDS, combat=True,
            )
            print(
                f"  {label:<10} {fixture.name:<26} "
                f"{ranged_trace.attempted:6.1f}/{ranged_trace.admitted:5.1f}/{ranged_trace.current:<4} "
                f"{rhetoric_trace.attempted:9.1f}/{rhetoric_trace.admitted:5.1f}/{rhetoric_trace.current:<4}"
            )

    print("\n  Source-derived ranged cycle (shipped capacity/cooldown):")
    schedule = _ranged_action_schedule()
    print("  round    " + " ".join(f"{round_number:>6}" for round_number, _ in schedule))
    print("  action   " + " ".join(f"{action:>6}" for _, action in schedule))

    print("\n  Rhetoric attempted pressure across reservation bands (fixture range):")
    print("  band       reserved    pressure range")
    for label, package in displayed:
        for reserved_ratio in (0.0, SHIPPED["PoolReservationCapPct"] / 2, SHIPPED["PoolReservationCapPct"]):
            ratios = []
            for fixture in FIXTURES:
                usable = fixture.conviction_max - math.floor(fixture.conviction_max * reserved_ratio)
                attempted = _sequence_attempted(rhetoric_event_sequence(package, fixture))
                ratios.append(attempted / usable)
            print(
                f"  {label:<10} {reserved_ratio:7.0%}    "
                f"{min(ratios):6.2%} .. {max(ratios):6.2%}"
            )

    exact_traces = (
        (
            "combined special / attack / defence", "stamina",
            special_combat_events(owner_selected, novice), True,
        ),
        (
            "detected sneak with awareness reset", "stamina",
            sneak_event_sequence(owner_selected, novice), False,
        ),
        (
            "rhetoric cadence and recovery", "conviction",
            rhetoric_event_sequence(owner_selected, novice), True,
        ),
    )
    for title, pool, events, combat in exact_traces:
        _, rows = run_trace(novice, pool, events, TRACE_ROUNDS, combat=combat)
        print(f"\n  Owner-selected novice thirty-round {title} trace:")
        print("  round  pool  regen  resolved action/state")
        for round_number, current, regen, status in rows:
            print(f"  {round_number:>5} {current:>5} {regen:>6}  {status}")

    print("\n  Ten-round novice grapple pools at typical load (controller/controlled):")
    print("  band       r1      r2      r3      r4      r5      r6      r7      r8      r9      r10")
    for label, package in displayed:
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
    for label, package in displayed:
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
    for label, package in displayed:
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


def assert_review_ranged_cadence_is_source_driven():
    """Changing the shipped cooldown must change ranged pressure everywhere."""
    package = CandidatePackage(4.0, 2.0, 1.0, 2.5, 4.0, 2.0)
    fixture = FIXTURES[0]
    original = SHIPPED["SpecialMoveCooldown"]
    try:
        baseline = _u8_trace_ratios(package, fixture)[1]
        SHIPPED["SpecialMoveCooldown"] = original + 1
        changed = _u8_trace_ratios(package, fixture)[1]
    finally:
        SHIPPED["SpecialMoveCooldown"] = original
    assert not math.isclose(baseline, changed), (
        "ranged pressure ignored a shipped SpecialMoveCooldown change"
    )


def assert_review_admission_accounting():
    """Refusal is atomic; partial payment writes off debt and records admission."""
    refused = PoolTrace(current=0, carry=0.25)
    before = (refused.current, refused.carry)
    assert refused.commit(1.5, full=True) == "refused"
    assert (refused.current, refused.carry) == before
    assert getattr(refused, "attempted", None) == 1.5
    assert getattr(refused, "admitted", None) == 0.0

    partial = PoolTrace(current=1, carry=0.25)
    assert partial.commit(2.0, full=False) == "short"
    assert (partial.current, partial.carry) == (0, 0.25)
    assert partial.attempted == 2.0
    assert partial.admitted == 1.0

    # The unpaid whole point is written off. After restoring two points, only
    # the new 0.75 plus the existing 0.25 carry is charged.
    partial.current = 2
    assert partial.commit(0.75, full=True) == "paid"
    assert (partial.current, partial.carry) == (1, 0.0)
    assert partial.admitted == 2.0

    recovered, rows = run_trace(
        FIXTURES[0], "stamina",
        {1: [(2.0, True)], 4: [(2.0, True)]},
        rounds=4, combat=False, initial=0,
    )
    assert rows[0] == (1, 0, 0, "refused")
    assert rows[2] == (3, 8, 8, "idle")
    assert rows[3] == (4, 6, 0, "paid")
    assert (recovered.attempted, recovered.admitted) == (4.0, 2.0)


def assert_review_zero_modifier_is_zero():
    """Explicit zero is a valid authored modifier, not a neutral default."""
    assert action_cost(3.0, 0.5, 25, modifier=0.0) == 0.0
    try:
        action_cost(3.0, 0.5, 25, modifier=-1.0)
    except ValueError:
        pass
    else:
        raise AssertionError("negative action modifier must be rejected")


def assert_review_all_selected_equivalents():
    """Evidence data covers all six bases, including both grapple roles."""
    helper = globals().get("selected_action_equivalents")
    assert callable(helper), "selected-action equivalent model is absent"
    package = CandidatePackage(4.0, 2.0, 1.0, 2.5, 4.0, 2.0)
    rows = helper(package, FIXTURES[0])
    assert tuple(row["action"] for row in rows) == (
        "special", "shoot", "reload", "sneak", "rhetoric",
        "grapple-controller", "grapple-controlled",
    )
    assert tuple(row["pool"] for row in rows) == (
        "stamina", "stamina", "stamina", "stamina", "conviction",
        "stamina", "stamina",
    )
    assert all(row["governing_skill"] for row in rows)
    assert all(row["cost"] >= 0 and row["swing_equivalent"] >= 0 for row in rows)


def assert_review_exact_event_sequences():
    """All exact thirty-round traces share asserted event builders."""
    builders = {
        name: globals().get(name)
        for name in (
            "special_combat_events", "ranged_event_sequence",
            "rhetoric_event_sequence", "sneak_event_sequence",
        )
    }
    assert all(callable(builder) for builder in builders.values()), (
        f"missing exact trace builder(s): "
        f"{[name for name, builder in builders.items() if not callable(builder)]}"
    )

    package = CandidatePackage(4.0, 2.0, 1.0, 2.5, 4.0, 2.0)
    fixture = FIXTURES[0]
    special = builders["special_combat_events"](package, fixture)
    assert tuple(round_number for round_number, events in special.items()
                 if any(event[0] == "special" for event in events)) == (1, 5, 9, 13, 17, 21, 25, 29)
    assert all(tuple(event[0] for event in special[round_number])[:2] == ("attack", "defence")
               for round_number in range(1, TRACE_ROUNDS + 1))

    ranged = builders["ranged_event_sequence"](package, fixture)
    assert tuple((round_number, events[0][0]) for round_number, events in ranged.items()) == (
        (1, "shoot"), (2, "reload"), (3, "shoot"), (6, "reload"),
        (7, "shoot"), (10, "reload"), (11, "shoot"), (14, "reload"),
        (15, "shoot"), (18, "reload"), (19, "shoot"), (22, "reload"),
        (23, "shoot"), (26, "reload"), (27, "shoot"), (30, "reload"),
    )

    rhetoric = builders["rhetoric_event_sequence"](package, fixture)
    assert tuple(rhetoric) == (1, 5, 9, 13, 17, 21, 25, 29)

    sneak = builders["sneak_event_sequence"](package, fixture)
    assert tuple(sneak) == tuple(range(1, TRACE_ROUNDS + 1))
    assert all(events[0][3:] == ("Visible", "Concealing", "Visible")
               for events in sneak.values())

    _, special_rows = run_trace(
        fixture, "stamina", special, TRACE_ROUNDS, combat=True,
    )
    assert tuple(row[1] for row in special_rows) == (
        129, 126, 125, 122, 113, 112, 108, 105, 98, 95,
        92, 90, 81, 78, 77, 74, 65, 63, 60, 57,
        50, 47, 43, 42, 33, 30, 29, 25, 16, 15,
    )
    assert all("attack:paid,defence:paid" in row[3] for row in special_rows)

    _, rhetoric_rows = run_trace(
        fixture, "conviction", rhetoric, TRACE_ROUNDS, combat=True,
    )
    assert tuple(row[1] for row in rhetoric_rows) == (
        134, 134, 138, 138, 134, 138, 138, 138, 138, 138,
        138, 138, 133, 133, 138, 138, 134, 138, 138, 138,
        138, 138, 138, 138, 133, 133, 138, 138, 134, 138,
    )
    assert tuple(row[2] for row in rhetoric_rows) == tuple(
        8 if round_number % 3 == 0 else 0
        for round_number in range(1, TRACE_ROUNDS + 1)
    )

    _, sneak_rows = run_trace(
        fixture, "stamina", sneak, TRACE_ROUNDS, combat=False,
    )
    assert tuple(row[1] for row in sneak_rows) == (
        135, 131, 136, 132, 128, 133, 129, 126, 130, 126,
        123, 127, 124, 120, 124, 121, 117, 122, 118, 114,
        119, 115, 112, 116, 112, 109, 113, 110, 106, 110,
    )
    assert all("[Visible->Concealing->Visible]" in row[3] for row in sneak_rows)


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

    assert_review_ranged_cadence_is_source_driven()
    assert_review_admission_accounting()
    assert_review_zero_modifier_is_zero()
    assert_review_all_selected_equivalents()
    assert_review_exact_event_sequences()
    all_passing_packages, u8_candidate_packages = generate_candidate_packages()
    owner_values = (4.0, 2.0, 1.0, 2.5, 4.0, 2.0)
    owner_package = CandidatePackage(*owner_values)
    assert _candidate_passes(owner_package), "owner-selected package fails a corrected gate"
    owner_selected = CandidatePackage(
        *owner_values, pressure_score=_package_pressure_score(owner_package),
    )
    assert_u8_acceptance(u8_candidate_packages)
    print_u8_model(u8_candidate_packages, owner_selected)
    print(f"\n  {len(all_passing_packages)} packages passed every U8 gate.")
