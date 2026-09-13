package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Buff ids clear of the fixture and of the narration buffs (7001-7007).
const (
	quietBuffId  = 7101 // no authored text at all
	hushedBuffId = 7102 // secret, with authored text that must never show
	scaledBuffId = 7103 // no authored text, applied with a duration multiplier
)

func seedNoticeBuffs() func() {
	return buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		quietBuffId:  {BuffId: quietBuffId, Name: "Test Quiet", RoundInterval: 5, TriggerCount: 3},
		hushedBuffId: {BuffId: hushedBuffId, Name: "Test Hushed", Secret: true, RoundInterval: 5, TriggerCount: 3, StartUserText: "You should never read this.", EndUserText: "Nor this."},
	})
}

func TestBuffNotice_HolderReadsTheGenericStartLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeBuffs()
	defer restore()
	drainPlain(1)
	drainPlain(2)

	assert.Equal(t, events.Continue, ApplyBuffs(events.Buff{UserId: 1, BuffId: quietBuffId}))
	assert.Equal(t, 1, countContaining(drainPlain(1), "Test Quiet takes effect."))
	assert.Equal(t, 0, countContaining(drainPlain(2), "Test Quiet"), "no room line was authored, so the room hears nothing")
}

func TestBuffNotice_HolderReadsTheGenericEndLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeBuffs()
	defer restore()
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Buffs.AddBuff(quietBuffId, false))
	expire(t, holder.Character.Buffs.List, quietBuffId)
	drainPlain(1)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(1), "Test Quiet has expired."))
}

func TestBuffNotice_SecretBuffIsSilentAtBothEnds(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeBuffs()
	defer restore()
	holder := users.GetByUserId(1)
	drainPlain(1)

	ApplyBuffs(events.Buff{UserId: 1, BuffId: hushedBuffId})
	assert.Empty(t, drainPlain(1), "a secret buff's authored start text must not be sent")

	expire(t, holder.Character.Buffs.List, hushedBuffId)
	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Empty(t, drainPlain(1), "a secret buff's authored end text must not be sent")
}

// TestBuffNotice_ScaledEventStillNarratesTheStart pins the delivery path for a
// buff whose duration is scaled. Potion potency and crafting skill scale a
// buff's duration, and the drink path used to do that by calling
// Character.AddBuffScaled directly, which queues nothing: Purging Weakness
// landed in play with no line at all. The multiplier now rides on the event,
// so a scaled application takes the same one door as an unscaled one and the
// holder reads the start notice.
func TestBuffNotice_ScaledEventStillNarratesTheStart(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		scaledBuffId: {BuffId: scaledBuffId, Name: "Test Scaled", RoundInterval: 1, TriggerCount: 10},
	})
	defer restore()
	drainPlain(1)

	assert.Equal(t, events.Continue, ApplyBuffs(events.Buff{UserId: 1, BuffId: scaledBuffId, DurationMult: 0.5}))
	assert.Equal(t, 1, countContaining(drainPlain(1), "Test Scaled takes effect."),
		"a scaled application must still narrate the start; this is the defect the playtest found")

	holder := users.GetByUserId(1)
	require.NotNil(t, holder)
	var triggersLeft int
	var found bool
	for _, b := range holder.Character.Buffs.List {
		if b.BuffId == scaledBuffId {
			triggersLeft, found = b.TriggersLeft, true
		}
	}
	require.True(t, found, "the buff must actually be held after the event")
	assert.Equal(t, 5, triggersLeft, "the multiplier must survive the trip through the event")
}

// TestBuffNotice_MagnitudeEventAppliesSilently pins the event-path door for a
// former combat condition: Character.AddBuffMagnitude is what every condition
// site calls synchronously, but the event carries Rounds/Magnitude too, for a
// future caller (a spell or item) that wants the start notice through the
// queue instead. Minor Shield is silent-start, so no start line is expected;
// this only pins that the exact rounds and the magnitude-derived effect both
// survive the trip through ApplyBuffs.
func TestBuffNotice_MagnitudeEventAppliesSilently(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer buffs.SeedConditionRecordsForTest()()

	assert.Equal(t, events.Continue, ApplyBuffs(events.Buff{UserId: 1, BuffId: buffs.BuffIdMinorShield, Rounds: 7, Magnitude: 9, Source: "test"}))

	holder := users.GetByUserId(1)
	require.NotNil(t, holder)
	assert.Equal(t, 7, holder.Character.Buffs.TriggersLeft(buffs.BuffIdMinorShield))
	assert.Equal(t, float64(9), holder.Character.Buffs.Effect(buffs.EffectMitigationFlat))
}

// Buff ids for the immunity pair, clear of the notice fixtures above.
const (
	immunityNoticeBuffId = 7104 // poison-immunity, the Stone Stomach shape
	venomNoticeBuffId    = 7105 // poison, authored start text for both audiences
)

// A refused add must not narrate. An immune player taking a serpent or
// arachnid crit (species critbuffids carry buff 39) read "You feel venom
// seeping into your bloodstream!" and the room read that it took hold, for a
// buff that never landed: the add's bool was discarded and the notice was
// gated on wasAlreadyActive alone.
func TestBuffNotice_ARefusedPoisonBuffNarratesNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		immunityNoticeBuffId: {BuffId: immunityNoticeBuffId, Name: "Test Stone Stomach", RoundInterval: 1, TriggerCount: 5,
			Flags: []buffs.Flag{buffs.PoisonImmunity}, StartUserText: "Nothing could turn your stomach now.", EndUserText: "Your stomach is ordinary again."},
		venomNoticeBuffId: {BuffId: venomNoticeBuffId, Name: "Test Venom", RoundInterval: 1, TriggerCount: 5,
			Flags: []buffs.Flag{buffs.Poison}, StartUserText: "You feel venom seeping into your bloodstream!",
			StartRoomText: "{source} winces as venom takes hold.", EndUserText: "The venom subsides."},
	})
	defer restore()
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Buffs.AddBuff(immunityNoticeBuffId, false))
	drainPlain(1)
	drainPlain(2)

	assert.Equal(t, events.Continue, ApplyBuffs(events.Buff{UserId: 1, BuffId: venomNoticeBuffId}))
	assert.False(t, holder.Character.HasBuff(venomNoticeBuffId), "the poison buff never landed")
	assert.Equal(t, 0, countContaining(drainPlain(1), "venom"), "the immune holder reads nothing")
	assert.Equal(t, 0, countContaining(drainPlain(2), "venom"), "and the room sees nothing take hold")

	// The scaled path is the same primitive and must refuse the same way.
	assert.Equal(t, events.Continue, ApplyBuffs(events.Buff{UserId: 1, BuffId: venomNoticeBuffId, DurationMult: 0.5}))
	assert.False(t, holder.Character.HasBuff(venomNoticeBuffId))
	assert.Equal(t, 0, countContaining(drainPlain(1), "venom"))
	assert.Equal(t, 0, countContaining(drainPlain(2), "venom"))
}

// The other half of the same rule, for the poison tick record rather than an
// ordinary buff: a mob's dot cast at an immune player added nothing and still
// narrated "afflicts you!" to the victim and the affliction to the room,
// because AddBuffMagnitude returns an error for the call site to test.
func TestBuffNotice_ARefusedPoisonedRecordNarratesNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		immunityNoticeBuffId: {BuffId: immunityNoticeBuffId, Name: "Test Stone Stomach", RoundInterval: 1, TriggerCount: 5,
			Flags: []buffs.Flag{buffs.PoisonImmunity}, StartUserText: "Nothing could turn your stomach now.", EndUserText: "Your stomach is ordinary again."},
	})
	defer restore()
	defer buffs.SeedConditionRecordsForTest()()
	original := runSpellChannelAttack
	runSpellChannelAttack = func(combat.AttackChannel, combat.AttackSide, *characters.Character, *characters.Character) combat.ChannelDefenceResult {
		return spellContestAttackWin()
	}
	t.Cleanup(func() { runSpellChannelAttack = original })

	target := users.GetByUserId(1)
	require.True(t, target.Character.Buffs.AddBuff(immunityNoticeBuffId, false))
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "test-blight", Name: "Blight", Type: spells.HarmSingle, EffectType: "dot"}
	resolveMobSpellAgainstPlayer(mobs.GetInstance(100), target, rooms.LoadRoom(1), spell, combat.AttackSide{}, 10)

	assert.False(t, target.Character.HasBuff(buffs.BuffIdPoisoned), "the record was refused")
	assert.Equal(t, 0, countContaining(drainPlain(1), "afflicts you"), "the immune victim reads nothing")
	assert.Equal(t, 0, countContaining(drainPlain(2), "afflicts"), "and the room is told nothing either")
}
