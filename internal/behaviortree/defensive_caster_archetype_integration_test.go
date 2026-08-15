package behaviortree

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

const defensiveCasterYAML = "../../_datafiles/world/dogmud/behaviors/archetypes/defensive_caster.yaml"

// seedDefensiveCasterSpells seeds the production spells the rewritten
// defensive_caster archetype's four consumers actually carry in their
// spellbooks: chrysalis-cocoon (self_defense, buff 52) / conviction-spike
// and conviction-barrage (harm_single / harm_multi) for the three
// previously-spellbook-less consumers (219/74/321), plus 285 bandit_caster's
// mind-spike / nerve-disruption (harm_single) / mend-wounds (self_heal).
// Values mirror the real spell YAML (base_folds, cost) so ranking behaves
// the way it does in production.
func seedDefensiveCasterSpells(t *testing.T) func() {
	t.Helper()
	return spells.SeedSpellsForTest(map[string]*spells.SpellData{
		// self_defense
		"chrysalis-cocoon": {
			SpellId: "chrysalis-cocoon", Name: "Chrysalis Cocoon",
			Type: spells.HelpSingle, Cost: 60, BaseFolds: 8,
			EffectType: "shield", EffectMagnitude: 125, BuffIds: []int{52},
			Categories: []string{"self_defense"},
		},
		// self_heal
		"mend-wounds": {
			SpellId: "mend-wounds", Name: "Mend Wounds",
			Type: spells.HelpSingle, Cost: 40, BaseFolds: 4,
			EffectType: "heal", EffectMagnitude: 5,
			Categories: []string{"self_heal"},
		},
		// harm_single
		"conviction-spike": {
			SpellId: "conviction-spike", Name: "Conviction Spike",
			Type: spells.HarmSingle, Cost: 50, BaseFolds: 4,
			EffectType: "damage", EffectMagnitude: 180, DamageMultiplier: 0.50,
			Categories: []string{"harm_single"},
		},
		"mind-spike": {
			SpellId: "mind-spike", Name: "Mind Spike",
			Type: spells.HarmSingle, Cost: 20, BaseFolds: 2,
			EffectType: "damage", EffectMagnitude: 100, DamageMultiplier: 0.40,
			Categories: []string{"harm_single"},
		},
		"nerve-disruption": {
			SpellId: "nerve-disruption", Name: "Nerve Disruption",
			Type: spells.HarmSingle, Cost: 40, BaseFolds: 5,
			EffectType: "buff", BuffIds: []int{30},
			Categories: []string{"harm_single"},
		},
		// harm_multi
		"conviction-barrage": {
			SpellId: "conviction-barrage", Name: "Conviction Barrage",
			Type: spells.HarmArea, Cost: 160, BaseFolds: 24,
			EffectType: "damage", EffectMagnitude: 700, DamageMultiplier: 1.60,
			Categories: []string{"harm_multi"},
		},
	})
}

// seedDefensiveCasterMob seeds a mob with BehaviorArchetype:
// "defensive_caster" and the provided spellbook. HP starts at 100/100
// (full). Caller can mutate m.Character.Health for low-HP tests.
func seedDefensiveCasterMob(t *testing.T, instanceId int, spellbook map[string]int) (*mobs.Mob, func()) {
	t.Helper()
	m := &mobs.Mob{
		MobId:             mobs.MobId(500 + instanceId),
		InstanceId:        instanceId,
		BehaviorArchetype: "defensive_caster",
	}
	m.Character.Name = "testdefensivecaster"
	m.Character.Conviction = 500
	m.Character.Health = 100
	m.Character.HealthMax.Value = 100
	m.Character.SpellBook = spellbook
	m.Character.Buffs = buffs.New()
	cleanup := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{500 + instanceId: m},
		map[int]*mobs.Mob{instanceId: m},
	)
	return m, cleanup
}

// TestDefensiveCaster_FullHP_Unbuffed_CastsCocoonFirst verifies the
// goblin_shaman/tunnel_shaman/elemental_queen spellbook shape (219/74/321,
// as content-fixed alongside this rewrite): full HP, no buffs, single
// self_defense candidate (chrysalis-cocoon) wins the self_defense branch
// before harm is ever considered.
func TestDefensiveCaster_FullHP_Unbuffed_CastsCocoonFirst(t *testing.T) {
	defer seedDefensiveCasterSpells(t)()
	LoadArchetypeForTest(t, "defensive_caster", defensiveCasterYAML)

	mob, cleanup := seedDefensiveCasterMob(t, 92001, map[string]int{
		"chrysalis-cocoon":   50,
		"conviction-spike":   50,
		"conviction-barrage": 50,
	})
	defer cleanup()
	defer events.DrainQueuedInputsForTest(mob.InstanceId)

	ok := TryMobBehavior(mob.InstanceId, EventContext{EventType: "mob_combat_round"})
	if !ok {
		t.Fatalf("TryMobBehavior: expected Success, got false")
	}
	DrainAllDelayedActionsForTest(t)

	cmd := events.InspectQueuedInputForTest(mob.InstanceId, "cast ")
	if !strings.HasPrefix(cmd, "cast chrysalis-cocoon") {
		t.Fatalf("full-HP unbuffed caster should cast chrysalis-cocoon (self_defense), got %q", cmd)
	}
}

// TestDefensiveCaster_CocoonActive_SingleEnemy_CastsHarmSingle verifies the
// cast_best_in_category "already active" semantics from
// action_cast_best_in_category.go: spellEffectAlreadyActive skips a
// candidate when ANY of its BuffIds is already present on the caster.
// chrysalis-cocoon carries BuffIds:[52], so seeding buff 52 (not
// ConditionShield, which is a different, EffectType=="shield" check that
// chrysalis-cocoon also has but isn't required to trip this skip) is
// sufficient to remove it as a self_defense candidate. With no other
// self_defense spell known, that branch has no candidates and Fails;
// with a single enemy (no room setup => multiple_enemies Fails), the
// selector reaches harm_single and casts conviction-spike.
func TestDefensiveCaster_CocoonActive_SingleEnemy_CastsHarmSingle(t *testing.T) {
	defer seedDefensiveCasterSpells(t)()
	LoadArchetypeForTest(t, "defensive_caster", defensiveCasterYAML)

	mob, cleanup := seedDefensiveCasterMob(t, 92002, map[string]int{
		"chrysalis-cocoon":   50,
		"conviction-spike":   50,
		"conviction-barrage": 50,
	})
	defer cleanup()
	defer events.DrainQueuedInputsForTest(mob.InstanceId)

	// Buff 52 is what chrysalis-cocoon's cast actually grants; seeding it
	// is what makes spellEffectAlreadyActive skip the spell via the
	// BuffIds branch.
	defer seedBuffOnChar(t, &mob.Character, 52)()

	ok := TryMobBehavior(mob.InstanceId, EventContext{EventType: "mob_combat_round"})
	if !ok {
		t.Fatalf("TryMobBehavior: expected Success, got false")
	}
	DrainAllDelayedActionsForTest(t)

	cmd := events.InspectQueuedInputForTest(mob.InstanceId, "cast ")
	if !strings.HasPrefix(cmd, "cast conviction-spike") {
		t.Fatalf("cocoon-active, single-enemy caster should cast conviction-spike (harm_single), got %q", cmd)
	}
}

// TestDefensiveCaster_BanditCaster285Shape_NoSelfDefense_CastsHarmSingle
// mirrors mob 285 bandit_caster's actual spellbook: no self_defense spell
// known at all, so that branch has no candidates and Fails regardless of
// buff state. Ranking: nerve-disruption (base_folds 5 x cost 40 = 200)
// outranks mind-spike (base_folds 2 x cost 20 = 40), so nerve-disruption
// wins the harm_single branch.
func TestDefensiveCaster_BanditCaster285Shape_NoSelfDefense_CastsHarmSingle(t *testing.T) {
	defer seedDefensiveCasterSpells(t)()
	LoadArchetypeForTest(t, "defensive_caster", defensiveCasterYAML)

	mob, cleanup := seedDefensiveCasterMob(t, 92003, map[string]int{
		"mind-spike":       50,
		"nerve-disruption": 50,
		"mend-wounds":      30,
	})
	defer cleanup()
	defer events.DrainQueuedInputsForTest(mob.InstanceId)

	ok := TryMobBehavior(mob.InstanceId, EventContext{EventType: "mob_combat_round"})
	if !ok {
		t.Fatalf("TryMobBehavior: expected Success, got false")
	}
	DrainAllDelayedActionsForTest(t)

	cmd := events.InspectQueuedInputForTest(mob.InstanceId, "cast ")
	if !strings.HasPrefix(cmd, "cast nerve-disruption") {
		t.Fatalf("285-shape caster with no self_defense spell should cast nerve-disruption (top harm_single), got %q", cmd)
	}
}

// TestDefensiveCaster_LowHP_CastsSelfHeal verifies the new self_heal branch:
// below 40% HP, a caster who knows mend-wounds casts it ahead of any
// defense or harm consideration.
func TestDefensiveCaster_LowHP_CastsSelfHeal(t *testing.T) {
	defer seedDefensiveCasterSpells(t)()
	LoadArchetypeForTest(t, "defensive_caster", defensiveCasterYAML)

	mob, cleanup := seedDefensiveCasterMob(t, 92004, map[string]int{
		"mind-spike":       50,
		"nerve-disruption": 50,
		"mend-wounds":      30,
	})
	defer cleanup()
	defer events.DrainQueuedInputsForTest(mob.InstanceId)

	// Drop HP below 40% (30/100 = 30%).
	mob.Character.Health = 30

	ok := TryMobBehavior(mob.InstanceId, EventContext{EventType: "mob_combat_round"})
	if !ok {
		t.Fatalf("TryMobBehavior: expected Success, got false")
	}
	DrainAllDelayedActionsForTest(t)

	cmd := events.InspectQueuedInputForTest(mob.InstanceId, "cast ")
	if !strings.HasPrefix(cmd, "cast mend-wounds") {
		t.Fatalf("low-HP caster with mend-wounds known should cast mend-wounds first, got %q", cmd)
	}
}

// TestDefensiveCaster_EmptySpellbook_FallsThroughNoWedge pins the pre-fix
// failure mode's replacement behavior: a caster with an empty spellbook (the
// shape 219/74/321 had before this content fix) has no candidates in any
// cast_best_in_category branch. try_special_move also Fails for a mob with
// no Aggro target (the test fixture never sets one), so the whole
// mob_combat_round selector returns Failure and TryMobBehavior returns
// false — legacy melee runs instead of the old tree's silent wedge on an
// unfinishable cocoon cast.
func TestDefensiveCaster_EmptySpellbook_FallsThroughNoWedge(t *testing.T) {
	defer seedDefensiveCasterSpells(t)()
	LoadArchetypeForTest(t, "defensive_caster", defensiveCasterYAML)

	mob, cleanup := seedDefensiveCasterMob(t, 92005, map[string]int{})
	defer cleanup()
	defer events.DrainQueuedInputsForTest(mob.InstanceId)

	ok := TryMobBehavior(mob.InstanceId, EventContext{EventType: "mob_combat_round"})
	if ok {
		t.Fatalf("TryMobBehavior: expected Failure (empty spellbook, no candidates), got Success")
	}
	DrainAllDelayedActionsForTest(t)

	cmd := events.InspectQueuedInputForTest(mob.InstanceId, "cast ")
	if cmd != "" {
		t.Fatalf("empty-spellbook caster should not queue any cast, got %q", cmd)
	}
}
