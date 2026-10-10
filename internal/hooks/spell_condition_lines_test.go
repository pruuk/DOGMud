package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Owner ruling R11 at the spell: a condition spell whose condition authors
// its start lines prints those lines and not the spell's generic "takes
// effect" trio, so every audience reads exactly one line.

const (
	spellLineAuthoredId = 7211 // all three start lines
	spellLineSilentId   = 7212 // silent-start
	spellLineHarmId     = 7213 // a harmful condition that names its caster
)

// seedSpellLineConditions adds the two specs to the fixture's registry,
// keeping the fixture's own condition 100 and the condition records.
func seedSpellLineConditions(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		100: conditions.GetConditionSpec(100),
		spellLineAuthoredId: {ConditionId: spellLineAuthoredId, Name: "Test Aura", RoundInterval: 1, TriggerCount: 10,
			StartActorText: "An aura blooms around {actee}.", StartUserText: "An aura blooms around you.",
			StartRoomText: "An aura blooms around {actee_plain}.", EndUserText: "Your aura fades."},
		spellLineSilentId: {ConditionId: spellLineSilentId, Name: "Test Hush", RoundInterval: 1, TriggerCount: 10,
			Flags: []conditions.Flag{conditions.SilentStart}, EndUserText: "The hush lifts."},
		spellLineHarmId: {ConditionId: spellLineHarmId, Name: "Test Fog", RoundInterval: 1, TriggerCount: 10,
			StartActorText: "You fog {actee}'s mind.", StartUserText: "{actor}'s spell fogs your mind.",
			StartRoomText: "{actor}'s spell fogs {actee}'s mind.", EndUserText: "The fog lifts."},
	}))
	t.Cleanup(conditions.SeedConditionRecordsForTest())
}

// applyQueuedConditions runs ApplyConditions over every condition event the
// spell queued for userId, as the event loop would.
func applyQueuedConditions(t *testing.T, userId int) []events.Condition {
	t.Helper()
	queued := events.DrainQueuedConditionsForTest(userId)
	for _, evt := range queued {
		ApplyConditions(evt)
	}
	return queued
}

func auraSpell(conditionId int) *spells.SpellData {
	s := conditionSpellForParityTest()
	s.Name = "Aura"
	s.ConditionIds = []int{conditionId}
	return s
}

func TestSpellConditionLines_CastOnAnotherIsOneLinePerAudience(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineAuthoredId)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	queued := applyQueuedConditions(t, 2)

	require.Len(t, queued, 1)
	assert.Equal(t, state.ActorRef{UserId: 1}, queued[0].Caster, "the spell names its caster on the event")
	assert.Equal(t, []string{"An aura blooms around Bobrick."}, drainPlain(1))
	assert.Equal(t, []string{"An aura blooms around you."}, drainPlain(2))
	assert.Equal(t, []string{"An aura blooms around Bobrick."}, drainPlain(3))
}

func TestSpellConditionLines_SelfCastIsOneLinePerAudience(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineAuthoredId)

	resolveAgainstPlayer(f.casterUser, f.casterUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	applyQueuedConditions(t, 1)

	assert.Equal(t, []string{"An aura blooms around you."}, drainPlain(1))
	assert.Equal(t, []string{"An aura blooms around Aliceia."}, drainPlain(3))
}

// Owner ruling 2026-10-10: a harmful condition spell names its caster to the
// target and the room through {actor} in the condition's own start lines.
func TestSpellConditionLines_AHarmfulSpellNamesItsCaster(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := hexSpellForConditionTest()
	spell.ConditionIds = []int{spellLineHarmId}
	require.True(t, spell.IsHarm())

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	applyQueuedConditions(t, 2)

	assert.Equal(t, []string{"You fog Bobrick's mind."}, drainPlain(1))
	assert.Equal(t, []string{"Aliceia's spell fogs your mind."}, drainPlain(2))
	assert.Equal(t, []string{"Aliceia's spell fogs Bobrick's mind."}, drainPlain(3))
}

// A mob caster's harmful condition names the mob to its player target.
func TestSpellConditionLines_AHarmfulMobSpellNamesTheMob(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := hexSpellForConditionTest()
	spell.ConditionIds = []int{spellLineHarmId}

	resolveMobSpellAgainstPlayer(f.casterMob, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, &f.casterMob.Character, nil), 0)
	applyQueuedConditions(t, 2)

	assert.Equal(t, []string{"Skeleton's spell fogs your mind."}, drainPlain(2))
	assert.Equal(t, []string{"Skeleton's spell fogs Bobrick's mind."}, drainPlain(3))
}

// A mob caster's condition on a player: the player and the room read the
// condition's lines; the mob has no client.
func TestSpellConditionLines_MobCasterOnAPlayer(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineAuthoredId)

	resolveMobSpellAgainstPlayer(f.casterMob, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, &f.casterMob.Character, nil), 0)
	queued := applyQueuedConditions(t, 2)

	require.Len(t, queued, 1)
	assert.Equal(t, state.ActorRef{MobInstanceId: 100}, queued[0].Caster)
	assert.Equal(t, []string{"An aura blooms around you."}, drainPlain(2))
	assert.Equal(t, []string{"An aura blooms around Bobrick."}, drainPlain(3))
}

// The crit marker leaves the dropped trio and rides the caster's line.
func TestSpellConditionLines_CritMarkerMovesToTheCasterLine(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackCrit())
	seedSpellLineConditions(t)
	spell := hexSpellForConditionTest()
	spell.ConditionIds = []int{spellLineAuthoredId}

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	applyQueuedConditions(t, 2)

	assert.Equal(t, []string{"An aura blooms around Bobrick. [CRIT!]"}, drainPlain(1))
	assert.Equal(t, []string{"An aura blooms around you."}, drainPlain(2))
}

// A narrated self-cast that crits: the marker rides the holder's own line,
// once, and the spell's trio is not sent. A help spell is uncontested and
// never crits, so the cast is a contested one.
func TestSpellConditionLines_SelfCastCritMarksTheHolderLineOnce(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackCrit())
	seedSpellLineConditions(t)
	spell := hexSpellForConditionTest()
	spell.ConditionIds = []int{spellLineAuthoredId}

	resolveAgainstPlayer(f.casterUser, f.casterUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	applyQueuedConditions(t, 1)

	assert.Equal(t, []string{"An aura blooms around you. [CRIT!]"}, drainPlain(1))
	assert.Equal(t, []string{"An aura blooms around Aliceia."}, drainPlain(3))
}

// Two casts of the same narrated condition on one holder in one round both
// resolve before either event lands (DoCombat resolves every cast inside the
// one NewRound dispatch; the queue drains after it). Both dropped their trio,
// so the second, landing on a record the first just made, still tells its
// start: every caster, the holder and the room read a line for each cast.
func TestSpellConditionLines_ASameRoundRecastStillTellsItsStart(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineAuthoredId)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	resolveAgainstPlayer(f.watcher, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.watcher.Character, nil), 0)
	queued := applyQueuedConditions(t, 2)

	require.Len(t, queued, 2)
	// Each caster reads its own caster line and the other cast's room line.
	assert.Equal(t, []string{"An aura blooms around Bobrick.", "An aura blooms around Bobrick."}, drainPlain(1))
	assert.Equal(t, []string{"An aura blooms around you.", "An aura blooms around you."}, drainPlain(2))
	assert.Equal(t, []string{"An aura blooms around Bobrick.", "An aura blooms around Bobrick."}, drainPlain(3))
}

// A refresh no spell narrated (a potion, a hazard, a re-cast whose spell
// kept its trio) still tells no start.
func TestSpellConditionLines_ARefreshFromAnotherDoorStaysSilent(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	require.NoError(t, f.targetUser.Character.AddCondition(spellLineAuthoredId, false))

	ApplyConditions(events.Condition{UserId: 2, ConditionId: spellLineAuthoredId, Source: "potion",
		LifeEpoch: f.targetUser.Character.LifeEpoch})

	assert.Empty(t, drainPlain(2))
	assert.Empty(t, drainPlain(3))
}

// A silent-start condition tells nobody, so the spell keeps its trio.
func TestSpellConditionLines_ASilentConditionKeepsTheTrio(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineSilentId)

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	applyQueuedConditions(t, 2)

	assert.Equal(t, []string{"Your Aura takes effect on Bobrick!"}, drainPlain(1))
	assert.Equal(t, []string{"Aliceia's Aura takes effect on you!"}, drainPlain(2))
	assert.Equal(t, []string{"Aliceia's Aura settles over Bobrick."}, drainPlain(3))
}

// A re-cast of a condition already held is a refresh: its start lines are
// suppressed, so the trio tells each audience the spell renewed it.
func TestSpellConditionLines_ARecastOfAnActiveConditionKeepsTheTrio(t *testing.T) {
	f := newSpellParityFixture(t, spellContestAttackWin())
	seedSpellLineConditions(t)
	spell := auraSpell(spellLineAuthoredId)
	require.NoError(t, f.targetUser.Character.AddCondition(spellLineAuthoredId, false))

	resolveAgainstPlayer(f.casterUser, f.targetUser, f.room, spell,
		spellAttackSideFor(spell, f.casterUser.Character, nil), 0)
	applyQueuedConditions(t, 2)

	assert.Equal(t, []string{"Your Aura takes effect on Bobrick!"}, drainPlain(1))
	assert.Equal(t, []string{"Aliceia's Aura takes effect on you!"}, drainPlain(2))
	assert.Equal(t, []string{"Aliceia's Aura settles over Bobrick."}, drainPlain(3))
}
