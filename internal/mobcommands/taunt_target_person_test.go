package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #262: a mob's taunt told its player target "a feeble jab at their
// resolve" about the player's own resolve.
func TestMobTauntTriad_TargetReadsYourResolve(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(combat.SeedTauntMessagesForTest(map[combat.TauntIntensity]*combat.TauntMessages{
		combat.TauntHit: {
			ToAttacker: []string{`You sneer at <ansi fg="{acteetype}">{actee}</ansi>! ({damage})`},
			ToDefender: []string{`<ansi fg="{actortype}">{actor}</ansi> sneers at you! ({damage})`},
			ToRoom:     []string{`<ansi fg="{actortype}">{actor}</ansi> sneers at <ansi fg="{acteetype}">{actee}</ansi>!`},
		},
	}))
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	target := users.GetByUserId(1)
	require.NotNil(t, target)
	room := rooms.LoadRoom(mob.Character.RoomId)
	require.NotNil(t, room)
	oldLamp := room.Lamp
	room.Lamp = rooms.LampPtr(90)
	t.Cleanup(func() { room.Lamp = oldLamp })
	events.DrainQueuedMessagesForTest(target.UserId)

	said := sendMobTauntTriad(combat.TauntHit,
		"a feeble jab at their resolve", "a feeble jab at your resolve",
		messaging.CategoryTauntSuccess, mob, target.Character.Name, target, room)
	require.True(t, said)

	lines := events.DrainQueuedMessagesForTest(target.UserId)
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], "a feeble jab at your resolve")
	require.NotContains(t, lines[0], "their resolve")
}
