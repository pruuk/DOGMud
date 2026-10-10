package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Condition ids clear of the fixture and of the narration conditions (7001-7007).
const (
	quietConditionId  = 7101 // no authored text at all
	hushedConditionId = 7102 // secret, with authored text that must never show
	scaledConditionId = 7103 // no authored text, applied with a duration multiplier
)

func seedNoticeConditions() func() {
	return conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		quietConditionId:  {ConditionId: quietConditionId, Name: "Test Quiet", RoundInterval: 5, TriggerCount: 3},
		hushedConditionId: {ConditionId: hushedConditionId, Name: "Test Hushed", Secret: true, RoundInterval: 5, TriggerCount: 3, StartUserText: "You should never read this.", EndUserText: "Nor this."},
	})
}

func TestConditionNotice_HolderReadsTheGenericStartLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeConditions()
	defer restore()
	drainPlain(1)
	drainPlain(2)

	assert.Equal(t, events.Continue, ApplyConditions(events.Condition{UserId: 1, ConditionId: quietConditionId}))
	assert.Equal(t, 1, countContaining(drainPlain(1), "Test Quiet takes effect."))
	assert.Equal(t, 0, countContaining(drainPlain(2), "Test Quiet"), "no room line was authored, so the room hears nothing")
}

func TestConditionNotice_HolderReadsTheGenericEndLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeConditions()
	defer restore()
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(quietConditionId, false))
	expire(t, holder.Character.Conditions.List, quietConditionId)
	drainPlain(1)

	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(1), "Test Quiet has expired."))
}

func TestConditionNotice_SecretConditionIsSilentAtBothEnds(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeConditions()
	defer restore()
	holder := users.GetByUserId(1)
	drainPlain(1)

	ApplyConditions(events.Condition{UserId: 1, ConditionId: hushedConditionId})
	assert.Empty(t, drainPlain(1), "a secret condition's authored start text must not be sent")

	expire(t, holder.Character.Conditions.List, hushedConditionId)
	PruneConditions(events.NewTurn{TurnNumber: 1})
	assert.Empty(t, drainPlain(1), "a secret condition's authored end text must not be sent")
}

// TestConditionNotice_ScaledEventStillNarratesTheStart pins the delivery path for a
// condition whose duration is scaled. Potion potency and crafting skill scale a
// condition's duration, and the drink path used to do that by calling
// Character.AddConditionScaled directly, which queues nothing: Purging Weakness
// landed in play with no line at all. The multiplier now rides on the event,
// so a scaled application takes the same one door as an unscaled one and the
// holder reads the start notice.
func TestConditionNotice_ScaledEventStillNarratesTheStart(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		scaledConditionId: {ConditionId: scaledConditionId, Name: "Test Scaled", RoundInterval: 1, TriggerCount: 10},
	})
	defer restore()
	drainPlain(1)

	assert.Equal(t, events.Continue, ApplyConditions(events.Condition{UserId: 1, ConditionId: scaledConditionId, DurationMult: 0.5}))
	assert.Equal(t, 1, countContaining(drainPlain(1), "Test Scaled takes effect."),
		"a scaled application must still narrate the start; this is the defect the playtest found")

	holder := users.GetByUserId(1)
	require.NotNil(t, holder)
	var triggersLeft int
	var found bool
	for _, b := range holder.Character.Conditions.List {
		if b.ConditionId == scaledConditionId {
			triggersLeft, found = b.TriggersLeft, true
		}
	}
	require.True(t, found, "the condition must actually be held after the event")
	assert.Equal(t, 5, triggersLeft, "the multiplier must survive the trip through the event")
}

// TestConditionNotice_MagnitudeEventAppliesSilently pins the event-path door for a
// former combat condition: Character.AddConditionMagnitude is what every condition
// site calls synchronously, but the event carries Triggers/Magnitude too, for
// a future caller (a spell or item) that wants the start notice through the
// queue instead. This only pins that the exact trigger count and the
// magnitude-derived effect both survive the trip through ApplyConditions.
func TestConditionNotice_MagnitudeEventAppliesSilently(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionRecordsForTest()()

	assert.Equal(t, events.Continue, ApplyConditions(events.Condition{UserId: 1, ConditionId: conditions.ConditionIdConvictionWard, Triggers: 7, Magnitude: 9, Source: "test"}))

	holder := users.GetByUserId(1)
	require.NotNil(t, holder)
	assert.Equal(t, 7, holder.Character.Conditions.TriggersLeft(conditions.ConditionIdConvictionWard))
	assert.Equal(t, float64(9), holder.Character.Conditions.Effect(conditions.EffectMitigationFlat))
}

// Condition ids for the immunity pair, clear of the notice fixtures above.
const (
	immunityNoticeConditionId = 7104 // poison-immunity, the Stone Stomach shape
	venomNoticeConditionId    = 7105 // poison, authored start text for both audiences
)

// A refused add must not narrate. An immune player taking a serpent or
// arachnid crit (species critconditionids carry condition 39) read "You feel venom
// seeping into your bloodstream!" and the room read that it took hold, for a
// condition that never landed: the add's bool was discarded and the notice was
// gated on wasAlreadyActive alone.
func TestConditionNotice_ARefusedPoisonConditionNarratesNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		immunityNoticeConditionId: {ConditionId: immunityNoticeConditionId, Name: "Test Stone Stomach", RoundInterval: 1, TriggerCount: 5,
			Flags: []conditions.Flag{conditions.PoisonImmunity}, StartUserText: "Nothing could turn your stomach now.", EndUserText: "Your stomach is ordinary again."},
		venomNoticeConditionId: {ConditionId: venomNoticeConditionId, Name: "Test Venom", RoundInterval: 1, TriggerCount: 5,
			Flags: []conditions.Flag{conditions.Poison}, StartUserText: "You feel venom seeping into your bloodstream!",
			StartRoomText: "{actee} winces as venom takes hold.", EndUserText: "The venom subsides."},
	})
	defer restore()
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Conditions.AddCondition(immunityNoticeConditionId, false))
	drainPlain(1)
	drainPlain(2)

	assert.Equal(t, events.Continue, ApplyConditions(events.Condition{UserId: 1, ConditionId: venomNoticeConditionId}))
	assert.False(t, holder.Character.HasCondition(venomNoticeConditionId), "the poison condition never landed")
	assert.Equal(t, 0, countContaining(drainPlain(1), "venom"), "the immune holder reads nothing")
	assert.Equal(t, 0, countContaining(drainPlain(2), "venom"), "and the room sees nothing take hold")

	// The scaled path is the same primitive and must refuse the same way.
	assert.Equal(t, events.Continue, ApplyConditions(events.Condition{UserId: 1, ConditionId: venomNoticeConditionId, DurationMult: 0.5}))
	assert.False(t, holder.Character.HasCondition(venomNoticeConditionId))
	assert.Equal(t, 0, countContaining(drainPlain(1), "venom"))
	assert.Equal(t, 0, countContaining(drainPlain(2), "venom"))
}

// The other half of the same rule, for the poison tick record rather than an
// ordinary condition: a mob's dot cast at an immune player added nothing and still
// narrated "afflicts you!" to the victim and the affliction to the room,
// because AddConditionMagnitude returns an error for the call site to test.
func TestConditionNotice_ARefusedPoisonedRecordNarratesNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		immunityNoticeConditionId: {ConditionId: immunityNoticeConditionId, Name: "Test Stone Stomach", RoundInterval: 1, TriggerCount: 5,
			Flags: []conditions.Flag{conditions.PoisonImmunity}, StartUserText: "Nothing could turn your stomach now.", EndUserText: "Your stomach is ordinary again."},
	})
	defer restore()
	defer conditions.SeedConditionRecordsForTest()()
	original := runSpellChannelAttack
	runSpellChannelAttack = func(messaging.RoomVisibility, combatvocab.Attack, combat.AttackSide, *characters.Character, *characters.Character) combat.ChannelDefenceResult {
		return spellContestAttackWin()
	}
	t.Cleanup(func() { runSpellChannelAttack = original })

	target := users.GetByUserId(1)
	require.True(t, target.Character.Conditions.AddCondition(immunityNoticeConditionId, false))
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "test-blight", Name: "Blight", AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle, EffectType: "dot"}
	resolveMobSpellAgainstPlayer(mobs.GetInstance(100), target, rooms.LoadRoom(1), spell, combat.AttackSide{}, 10)

	assert.False(t, target.Character.HasCondition(conditions.ConditionIdPoisoned), "the record was refused")
	assert.Equal(t, 0, countContaining(drainPlain(1), "afflicts you"), "the immune victim reads nothing")
	assert.Equal(t, 0, countContaining(drainPlain(2), "afflicts"), "and the room is told nothing either")
}
