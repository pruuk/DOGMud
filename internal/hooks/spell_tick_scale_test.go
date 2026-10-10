package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spellTickScaleWandId is a seeded weapon with a spell damage multiplier.
const spellTickScaleWandId = 7130

func seedSpellTickScaleWand(t *testing.T) {
	t.Helper()
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		spellTickScaleWandId: {ItemId: spellTickScaleWandId, Name: "Test Wand", Type: items.Weapon, SpellDamageMultiplier: 1.5},
	}))
}

func TestSpellTickScale_SkillZeroNoWeaponIsTheSkillBase(t *testing.T) {
	caster := characters.New()
	// New seeds every skill at rank 1 (initAllSkills); zero removes it.
	caster.SetSkill("spellcasting", 0)
	assert.Equal(t, combat.SkillMultiplier(0), spellTickScale(caster))
	assert.InDelta(t, 1.0, spellTickScale(caster), 1e-9, "SkillMultiplierBase defaults to 1.0")
	assert.InDelta(t, 1.0, spellTickScale(nil), 1e-9, "no caster is 1.0")
}

func TestSpellTickScale_WeaponMultiplies(t *testing.T) {
	seedSpellTickScaleWand(t)
	caster := characters.New()
	caster.SetSkill("spellcasting", 20)
	bare := spellTickScale(caster)
	caster.Equipment.Weapon = items.Item{ItemId: spellTickScaleWandId}
	assert.InDelta(t, bare*1.5, spellTickScale(caster), 1e-9,
		"an unmutated caster's gear effectiveness is 1.0, so the weapon's 1.5 applies whole")
	assert.Greater(t, bare, 1.0, "precondition: skill 20 scales above the base")
}

// One formula for both: a mob caster used to skip the weapon (self-cast) or
// skip scaling altogether (mob on mob).
func TestSpellTickScale_PlayerAndMobCastersMatch(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	seedSpellTickScaleWand(t)
	player := users.GetByUserId(1).Character
	mob := &mobs.GetInstance(100).Character
	for _, c := range []*characters.Character{player, mob} {
		c.SetSkill("spellcasting", 30)
		c.Equipment.Weapon = items.Item{ItemId: spellTickScaleWandId}
	}
	assert.Equal(t, spellTickScale(player), spellTickScale(mob))
	assert.Greater(t, spellTickScale(mob), combat.SkillMultiplier(30), "the mob gets the weapon too")
}

// Spell-path conditions for applySpellCondition: a tick_pool heal, a plain
// record, and a magnitude light.
const (
	spellPathTickConditionId  = 7131
	spellPathPlainConditionId = 7132
	spellPathGlowConditionId  = 7133
)

func seedSpellPathConditions(t *testing.T) {
	t.Helper()
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		spellPathTickConditionId: {ConditionId: spellPathTickConditionId, Name: "Test Surge",
			RoundInterval: 1, TriggerCount: 5, TickPool: "health", TickPercent: 0.05},
		spellPathPlainConditionId: {ConditionId: spellPathPlainConditionId, Name: "Test Plain",
			RoundInterval: 1, TriggerCount: 4},
		spellPathGlowConditionId: {ConditionId: spellPathGlowConditionId, Name: "Test Glow",
			RoundInterval: 1, TriggerCount: 4,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {UsesMagnitude: true}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
	}))
	events.DrainQueuedMobConditionsForTest(0)
}

// Regression for the whole spell path: a skilled caster's heal-over-time at a
// target that does not hold it yet. On master the event carried no scale and
// the post-queue SetTickAmount found no record, so the target held 0 until
// the round tick's fallback computed it at 1.0; a recast alone got the
// caster's scale.
func TestConditionSpell_FreshTargetHealsAtCasterScale(t *testing.T) {
	seedSpellPathConditions(t)
	caster := users.GetByUserId(1)
	target := users.GetByUserId(2)
	caster.Character.SetSkill("spellcasting", 50)
	seedRacialStats(target, 0)
	target.Character.HealthMax.Base = 200
	_ = target.Character.Validate()
	require.Empty(t, target.Character.GetConditions(spellPathTickConditionId), "precondition: a fresh target")
	scale := combat.SkillMultiplier(50)
	require.Greater(t, scale, 2.0, "precondition: spellcasting 50 scales well above 1")

	spell := &spells.SpellData{SpellId: "test-surge", Name: "Test Surge", EffectType: "condition",
		ConditionIds: []int{spellPathTickConditionId}}
	events.DrainQueuedConditionsForTest(0)
	r := rooms.LoadRoom(1)
	applySpellEffect(newSpellEffectCtx(caster.Character, actions.NewUserActorInRoom(caster, r),
		actions.NewUserActorInRoom(target, r), r, spell, 0, spellContestAttackWin()))

	queued := events.DrainQueuedConditionsForTest(2)
	require.Len(t, queued, 1)
	assert.Equal(t, events.Continue, ApplyConditions(queued[0]))

	want := conditions.ComputeTickAmount(target.Character.HealthMax.Value, 0.05, 0, 0, scale)
	assert.Equal(t, want, heldTickAmount(t, target.Character, spellPathTickConditionId),
		"a first cast heals at the caster's scale")
	assert.NotEqual(t, conditions.ComputeTickAmount(target.Character.HealthMax.Value, 0.05, 0, 0, 1.0), want,
		"precondition: the pool is large enough that the scale shows")
}

// A mob healing itself now gets the same formula, weapon included.
func TestConditionSpell_MobSelfCastHealsAtCasterScale(t *testing.T) {
	seedSpellPathConditions(t)
	seedSpellTickScaleWand(t)
	mob := mobs.GetInstance(100)
	mob.Character.SetSkill("spellcasting", 30)
	mob.Character.Equipment.Weapon = items.Item{ItemId: spellTickScaleWandId}
	mob.Character.HealthMax.Base = 200
	_ = mob.Character.Validate()
	mob.Character.RemoveCondition(spellPathTickConditionId)

	spell := &spells.SpellData{SpellId: "test-surge", Name: "Test Surge", EffectType: "condition",
		ConditionIds: []int{spellPathTickConditionId}}
	r := rooms.LoadRoom(1)
	applySpellEffect(newSpellEffectCtx(&mob.Character, actions.NewMobActorInRoom(mob, r),
		actions.NewMobActorInRoom(mob, r), r, spell, 0, uncontestedSpellResult()))

	queued := events.DrainQueuedMobConditionsForTest(100)
	require.Len(t, queued, 1)
	assert.Equal(t, events.Continue, ApplyConditions(queued[0]))

	scale := combat.SkillMultiplier(30) * 1.5
	want := conditions.ComputeTickAmount(mob.Character.HealthMax.Value, 0.05, 0, 0, scale)
	assert.Equal(t, want, heldTickAmount(t, &mob.Character, spellPathTickConditionId))
}

func TestApplySpellCondition_TickPoolQueuesTheCasterScale(t *testing.T) {
	seedSpellPathConditions(t)
	seedSpellTickScaleWand(t)
	spell := &spells.SpellData{SpellId: "test-surge", PrimaryStat: "willpower"}
	caster := characters.New()
	caster.SetSkill("spellcasting", 40)
	caster.Equipment.Weapon = items.Item{ItemId: spellTickScaleWandId}
	want := spellTickScale(caster)
	require.Greater(t, want, 1.0, "precondition: the caster scale is not the default")

	targets := []struct {
		name  string
		door  spellConditionTarget
		drain func() []events.Condition
	}{
		{"player target", users.GetByUserId(1), func() []events.Condition { return events.DrainQueuedConditionsForTest(1) }},
		{"mob target", mobs.GetInstance(100), func() []events.Condition { return events.DrainQueuedMobConditionsForTest(100) }},
	}
	for _, tg := range targets {
		t.Run(tg.name, func(t *testing.T) {
			applySpellCondition(tg.door, spell, caster, spellPathTickConditionId, state.ActorRef{}, false)
			q := tg.drain()
			require.Len(t, q, 1)
			assert.Equal(t, want, q[0].TickScale, "a tick_pool condition carries the caster's scale")

			applySpellCondition(tg.door, spell, caster, spellPathPlainConditionId, state.ActorRef{}, false)
			q = tg.drain()
			require.Len(t, q, 1)
			assert.Zero(t, q[0].TickScale, "a non-ticking condition carries no scale")

			applySpellCondition(tg.door, spell, caster, spellPathGlowConditionId, state.ActorRef{}, false)
			q = tg.drain()
			require.Len(t, q, 1)
			assert.NotZero(t, q[0].Magnitude, "a magnitude light still queues its magnitude")
			assert.Zero(t, q[0].TickScale)
		})
	}
}
