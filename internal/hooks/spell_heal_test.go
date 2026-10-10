package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func healSpellForParityTest() *spells.SpellData {
	return &spells.SpellData{
		SpellId: "test-mend", Name: "Mend", AttackType: combatvocab.AttackNone,
		DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle,
		EffectType: "heal", EffectMagnitude: 3, BaseFolds: 4, PrimaryStat: "willpower",
		Schools: []string{spells.SchoolVital},
	}
}

// parityHealRounds is the heal's duration on the fixture's equalised caster
// (skill 3, willpower 100) for the Regenerating record a heal spell that
// names no condition lands: its authored 10 triggers scaled by
// (10 + 5 + 1.5) / 15, so 11.
func parityHealRounds() int {
	return healTriggers(conditions.ConditionIdRegenerating, 100, 3)
}

// regenRecord lands the queued heal and returns c's one Regenerating record,
// or fails the test.
func regenRecord(t *testing.T, c *characters.Character) *conditions.Condition {
	t.Helper()
	landQueuedConditions()
	recs := c.GetConditions(conditions.ConditionIdRegenerating)
	require.Len(t, recs, 1, "the heal must leave one Regenerating record")
	return recs[0]
}

// Slice 3b (audit row 3): a creature's heal on a player was contested, fell
// to the default arm and applied nothing.
func TestSpellHeal_ACreatureHealsAPlayer(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	spell := healSpellForParityTest()

	resolveMobSpellAgainstPlayer(f.casterMob, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, &f.casterMob.Character, nil), spell.EffectMagnitude)

	rec := regenRecord(t, f.targetUser.Character)
	assert.Equal(t, 3.0, rec.Magnitude)
	assert.Equal(t, 11, parityHealRounds(), "the fixture's arithmetic")
	assert.Equal(t, parityHealRounds(), rec.TriggersLeft)
	assert.Equal(t, state.ActorRef{MobInstanceId: 100}, rec.Caster, "the heal remembers its caster")
	assert.Equal(t, 1, countContaining(drainPlain(2), "Mend envelops you in healing energy."))
	assert.Equal(t, 1, countContaining(drainPlain(3), "Mend envelops Bobrick in healing light."))
}

// Owner ruling 3: help spells do not crit. The player-to-player heal doubled
// the part of its multiplier above 1x on a crit no cast could reach.
func TestSpellHeal_ACritChangesNothing(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	caster := actions.NewUserActorInRoom(f.casterUser, f.room)
	target := actions.NewUserActorInRoom(f.targetUser, f.room)

	applySpellEffect(newSpellEffectCtx(f.casterUser.Character, caster, target, f.room,
		healSpellForParityTest(), 3, spellContestAttackCrit()))

	assert.Equal(t, 3.0, regenRecord(t, f.targetUser.Character).Magnitude, "a crit must not boost a heal")
}

// A player healing a mob queues events.Healed for the AI companion; nothing
// else does, because the event names a player healer and a mob.
func TestSpellHeal_OnlyAPlayerHealingAMobQueuesHealed(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	spell := healSpellForParityTest()
	events.DrainQueuedHealedForTest(0)

	resolveAgainstMob(f.casterUser, f.targetMob, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)
	healed := events.DrainQueuedHealedForTest(0)
	require.Len(t, healed, 1)
	assert.Equal(t, events.Healed{HealerUserId: 1, MobInstanceId: 101}, healed[0])

	resolveMobSpellAgainstMob(f.casterMob, f.targetMob, f.room, spell,
		spellAttackSideFor(spell, &f.casterMob.Character, nil), spell.EffectMagnitude)
	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)
	assert.Empty(t, events.DrainQueuedHealedForTest(0), "only a player healing a mob is tended")
}

const (
	testGentleHealId = 7231 // heal family, long and gentle
	testStrongHealId = 7232 // heal family, short and strong
	testSalveId      = 7233 // no family, a healthrecovery statmod like the Healing Salve
)

// seedTestHeals adds two heals and a salve to the fixture's registry,
// keeping the fixture's condition 100 and the condition records.
func seedTestHeals(t *testing.T) {
	t.Helper()
	mag := conditions.EffectValue{UsesMagnitude: true}
	heal := func(id, triggers int, name, start string) *conditions.ConditionSpec {
		return &conditions.ConditionSpec{ConditionId: id, Name: name, Family: conditions.FamilyHeal,
			RoundInterval: 1, TriggerCount: triggers,
			Effects:        map[conditions.EffectKind]conditions.EffectValue{conditions.EffectRegenMult: mag},
			StartActorText: start + " settles on {actee}.", StartUserText: start + " settles on you.",
			StartRoomText: start + " settles on {actee_plain}.", EndUserText: start + " fades."}
	}
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		100:              conditions.GetConditionSpec(100),
		testGentleHealId: heal(testGentleHealId, 45, "Test Gentle", "A gentle heal"),
		testStrongHealId: heal(testStrongHealId, 15, "Test Strong", "A strong heal"),
		testSalveId: {ConditionId: testSalveId, Name: "Test Salve", RoundInterval: 1, TriggerCount: 100,
			StatMods: statmods.StatMods{"healthrecovery": 3}, StartUserText: "The salve warms.", EndUserText: "The salve cools."},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
}

func healSpellLanding(conditionId, magnitude int) *spells.SpellData {
	s := healSpellForParityTest()
	s.ConditionIds = []int{conditionId}
	s.EffectMagnitude = magnitude
	return s
}

// R8 and R9: a heal spell lands its own heal, at its own multiplier and its
// own duration scaled by the caster, and its lines are the heal's own.
func TestSpellHeal_LandsTheSpellsOwnHeal(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedTestHeals(t)
	spell := healSpellLanding(testGentleHealId, 2)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), spell.EffectMagnitude)
	landQueuedConditions()

	recs := f.targetUser.Character.GetConditions(testGentleHealId)
	require.Len(t, recs, 1)
	assert.Equal(t, 2.0, recs[0].Magnitude)
	assert.Equal(t, 50, recs[0].TriggersLeft, "45 authored rounds x (10 + 5 + 1.5) / 15, rounded")
	assert.False(t, f.targetUser.Character.HasCondition(conditions.ConditionIdRegenerating))
	assert.Equal(t, []string{"A gentle heal settles on Bobrick."}, drainPlain(1))
	assert.Equal(t, []string{"A gentle heal settles on you."}, drainPlain(2))
	assert.Equal(t, []string{"A gentle heal settles on Bobrick."}, drainPlain(3))
}

// R4 and R6: a second heal spell replaces the first, announced; a salve
// stacks with either.
func TestSpellHeal_ASecondHealReplacesTheFirstAndASalveStacks(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedTestHeals(t)
	c := f.targetUser.Character
	require.NoError(t, c.AddCondition(testSalveId, false))
	gentle, strong := healSpellLanding(testGentleHealId, 2), healSpellLanding(testStrongHealId, 6)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, gentle,
		spellAttackSideFor(gentle, f.casterUser.Character, nil), gentle.EffectMagnitude)
	landQueuedConditions()
	drainPlain(2)
	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, strong,
		spellAttackSideFor(strong, f.casterUser.Character, nil), strong.EffectMagnitude)
	landQueuedConditions()

	assert.False(t, c.HasCondition(testGentleHealId))
	assert.True(t, c.HasCondition(testStrongHealId))
	assert.True(t, c.HasCondition(testSalveId), "a salve is no heal spell, so it stays")
	assert.Equal(t, 6.0, c.Conditions.Effect(conditions.EffectRegenMult), "one heal multiplier, never two multiplied")
	assert.Equal(t, []string{"Your Test Gentle fades as Test Strong takes hold.", "A strong heal settles on you."}, drainPlain(2))
}
