package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const goDepartureLightCond = 9781 // a carried light, literal strength 60

// #456, owner ruling 2026-10-09: a mover is seen by the light they carry on
// their own way out. Aliceia carries the only light in the dark cave (room 2)
// and walks south; Bobrick, left behind in the dark, saw her go by her own
// light, so he reads her name, not the sound.
func TestGo_DepartureIsJudgedByTheMoversOwnLight(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		goDepartureLightCond: {ConditionId: goDepartureLightCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 60}}},
	}))
	room1, cave := rooms.LoadRoom(1), rooms.LoadRoom(2)
	cave.Biome = "cave"
	mover, watcher := users.GetByUserId(1), users.GetByUserId(2)
	for _, u := range []*users.UserRecord{mover, watcher} {
		room1.RemovePlayer(u.UserId)
		u.Character.RoomId = 2
		cave.AddPlayer(u.UserId)
	}
	mover.Character.ActionPoints = 100
	require.True(t, mover.Character.Conditions.AddCondition(goDepartureLightCond, false))
	require.Equal(t, messaging.SightFull, cave.ParticipantSight(watcher.UserId), "fixture: the mover's light lights the cave")
	events.DrainQueuedMessagesForTest(watcher.UserId)

	handled, err := Go("south", mover, cave, 0)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, mover.Character.RoomId, "fixture: the mover left")
	require.Equal(t, messaging.SightNone, cave.ParticipantSight(watcher.UserId), "fixture: the light left with the mover")

	got := hoodTestText(watcher.UserId)
	require.Contains(t, got, "Aliceia", "the watcher saw the mover go by her own light")
	require.NotContains(t, got, "You hear someone leave the room.")
}
