package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Buff room lines describe what the room SEES ("A warm glow surrounds Alice"),
// but went out on the audio channel, which is never sight-gated, so blind and
// unsighted observers received them. M2 fixed the same defect for
// cast_room_text; these three buff phases were never touched.

// expire sets a buff's remaining triggers to the pruning threshold, so the next
// PruneBuffs removes it and sends its end text. Deterministic, unlike counting
// ticks.
func expire(t *testing.T, list []*buffs.Buff, buffId int) {
	t.Helper()
	for _, b := range list {
		if b.BuffId == buffId {
			b.TriggersLeft = buffs.TriggersLeftExpired
			return
		}
	}
	t.Fatalf("buff %d not found to expire", buffId)
}

// rawLineContaining returns the first raw (still tagged) line whose plain text
// contains want, or "" if none does.
func rawLineContaining(raw []string, want string) string {
	for _, line := range raw {
		if strings.Contains(plainText(line), want) {
			return line
		}
	}
	return ""
}

func TestBuffStartRoomText_SightedObserverSeesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	drainPlain(2)

	assert.Equal(t, events.Continue, ApplyBuffs(events.Buff{UserId: 1, BuffId: glowBuffId}))
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia glows."))
}

func TestBuffStartRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	drainPlain(2)

	ApplyBuffs(events.Buff{UserId: 1, BuffId: glowBuffId})
	assert.Equal(t, 0, countContaining(drainPlain(2), "glows."),
		"an observer who cannot see must not be told what a buff looks like")
}

func TestBuffStartRoomText_NightVisionSeesItInTheDark(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	require.True(t, users.GetByUserId(2).Character.Buffs.AddBuff(nightEyesBuffId, true))
	drainPlain(2)

	ApplyBuffs(events.Buff{UserId: 1, BuffId: glowBuffId})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia glows."))
}

func TestBuffStartRoomText_MobHolderUsesTheMobTag(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	events.DrainQueuedMessagesForTest(2)

	ApplyBuffs(events.Buff{MobInstanceId: 100, BuffId: glowBuffId})
	line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "Skeleton glows.")
	require.NotEmpty(t, line, "the observer must receive the mob's start text")
	assert.Contains(t, line, `fg="mobname`)
	assert.NotContains(t, line, `fg="username`,
		"a mob holder was tagged with the player colour")
}

func TestBuffTriggerRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	require.True(t, users.GetByUserId(1).Character.Buffs.AddBuff(shiverBuffId, false))
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	assert.Equal(t, 0, countContaining(drainPlain(2), "shivers."))
}

func TestBuffTriggerRoomText_SightedObserverSeesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	require.True(t, users.GetByUserId(1).Character.Buffs.AddBuff(shiverBuffId, false))
	drainPlain(2)

	UserRoundTick(events.NewRound{RoundNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia shivers."))
}

func TestBuffEndRoomText_UnsightedObserverInTheDarkGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Buffs.AddBuff(fadeBuffId, false))
	expire(t, holder.Character.Buffs.List, fadeBuffId)
	drainPlain(2)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 0, countContaining(drainPlain(2), "fades."))
}

func TestBuffEndRoomText_SightedObserverSeesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Buffs.AddBuff(fadeBuffId, false))
	expire(t, holder.Character.Buffs.List, fadeBuffId)
	drainPlain(2)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia fades."))
}

func TestBuffEndRoomText_MobHolderIsVisualAndUsesTheMobTag(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	mob := mobs.GetInstance(100)
	require.True(t, mob.Character.Buffs.AddBuff(fadeBuffId, false))
	expire(t, mob.Character.Buffs.List, fadeBuffId)
	events.DrainQueuedMessagesForTest(2)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	line := rawLineContaining(events.DrainQueuedMessagesForTest(2), "Skeleton fades.")
	require.NotEmpty(t, line)
	assert.Contains(t, line, `fg="mobname`)

	// And the same line is gated by sight.
	require.True(t, mob.Character.Buffs.AddBuff(fadeBuffId, false))
	expire(t, mob.Character.Buffs.List, fadeBuffId)
	darken(t, 1)
	drainPlain(2)
	PruneBuffs(events.NewTurn{TurnNumber: 2})
	assert.Equal(t, 0, countContaining(drainPlain(2), "fades."))
}

// TestMobBuffTriggerRoomText is the D4 guard. The player round tick has always
// sent a triggered buff's trigger_room_text; tickMobBuffs never did, so a mob
// holding a trigger-text buff showed nothing. No mob holder of the shipped
// trigger-text buffs could be staged in a playtest, so this is its only check.
func TestMobBuffTriggerRoomText(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	mob := mobs.GetInstance(100)
	require.True(t, mob.Character.Buffs.AddBuff(shiverBuffId, false))
	events.DrainQueuedMessagesForTest(2)

	tickMobBuffs(mob, 100)
	raw := events.DrainQueuedMessagesForTest(2)
	line := rawLineContaining(raw, "Skeleton shivers.")
	require.NotEmpty(t, line, "a sighted observer must see the mob's trigger text")
	assert.Contains(t, line, `fg="mobname`)
	delivered := 0
	for _, l := range raw {
		if strings.Contains(plainText(l), "Skeleton shivers.") {
			delivered++
		}
	}
	assert.Equal(t, 1, delivered, "the trigger line must arrive exactly once")

	// Sight-gated like every other buff room line.
	darken(t, 1)
	drainPlain(2)
	tickMobBuffs(mob, 100)
	assert.Equal(t, 0, countContaining(drainPlain(2), "shivers."))
}

// A light buff's end line describes the light going out, and the moment a
// light goes out is seen by everyone in the room with working eyes. But the
// light stops counting the instant the buff EXPIRES (Buffs.HasFlag skips
// expired buffs, and expiry happens on the round tick), while its end text is
// sent later, at the turn's prune. So a plain visual send judged sight in a
// room that was already dark, and silenced the line for exactly the people
// who had been seeing by that light. Found by the Task 2 review against
// shipped buff 1, Illumination.

func TestBuffEndRoomText_LightBuffEndIsSeenByItsOwnLight_Player(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Buffs.AddBuff(lanternBuffId, false))
	require.GreaterOrEqual(t, room.GetVisibility(), 1, "the lantern must light the cave, or this test proves nothing")
	expire(t, holder.Character.Buffs.List, lanternBuffId)
	require.Zero(t, room.GetVisibility(), "the light is already out once the buff expires, before any prune")
	drainPlain(2)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Aliceia's light gutters out."))
}

// TestBuffEndRoomText_LightBuffEnd_SleeperStillGetsNothing proves the light
// line is still a SIGHT line, not audio: an observer who cannot see for a
// reason other than darkness is not told.
func TestBuffEndRoomText_LightBuffEnd_SleeperStillGetsNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Buffs.AddBuff(lanternBuffId, false))
	require.True(t, users.GetByUserId(2).Character.Buffs.AddBuff(dozeBuffId, true))
	expire(t, holder.Character.Buffs.List, lanternBuffId)
	drainPlain(2)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 0, countContaining(drainPlain(2), "light gutters out"))
}

func TestBuffEndRoomText_LightBuffEndIsSeenByItsOwnLight_Mob(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationBuffs()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	mob := mobs.GetInstance(100)
	require.True(t, mob.Character.Buffs.AddBuff(lanternBuffId, false))
	require.GreaterOrEqual(t, room.GetVisibility(), 1, "the lantern must light the cave, or this test proves nothing")
	expire(t, mob.Character.Buffs.List, lanternBuffId)
	drainPlain(2)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(2), "Skeleton's light gutters out."))
}
