package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
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
	assert.NotContains(t, line, `fg="username">Skeleton`,
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
