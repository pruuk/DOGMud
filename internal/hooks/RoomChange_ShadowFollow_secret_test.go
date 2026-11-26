package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

// shadowSecretNorth makes shadowRoomA's north exit (to shadowRoomB) secret
// and teaches it to the quarry of kind k, who walks through it.
func (s *shadowScene) shadowSecretNorth(k shadowKind) {
	a := rooms.LoadRoom(shadowRoomA)
	north := a.Exits["north"]
	north.Secret = true
	a.Exits["north"] = north
	if k == shadowPlayer {
		s.quarryUser.Character.LearnSecretExit(shadowRoomA, "north")
	} else {
		s.quarryMob.Character.LearnSecretExit(shadowRoomA, "north")
	}
}

// #781, owner rulings 2 and 13 (shadowing is following): a shadower behind
// its quarry through a secret exit learns it and goes on, when it is a player,
// a mob charmed by the player it shadows, or a mob shadowing a mob. Any other
// mob shadowing a player ends the shadow quietly at the exit: no step, no
// cooldown, no line to anyone.
func TestRoomChangeShadowFollow_SecretExit(t *testing.T) {
	cases := []struct {
		name            string
		quarry, sly     shadowKind
		charmed, follow bool
	}{
		{name: "player shadows a player: learns and follows", quarry: shadowPlayer, sly: shadowPlayer, follow: true},
		{name: "player shadows a mob: learns and follows", quarry: shadowMob, sly: shadowPlayer, follow: true},
		{name: "mob shadows a mob: learns and follows", quarry: shadowMob, sly: shadowMob, follow: true},
		{name: "mob charmed by the player it shadows: learns and follows", quarry: shadowPlayer, sly: shadowMob, charmed: true, follow: true},
		{name: "mob shadows a player: the shadow ends quietly", quarry: shadowPlayer, sly: shadowMob},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newShadowScene(t)
			s.shadowSecretNorth(tc.quarry)
			s.place(tc.quarry, true, s.roomB)
			s.place(tc.sly, false, rooms.LoadRoom(shadowRoomA))
			c := s.sly(tc.sly)
			if tc.charmed {
				c.Charmed = characters.NewCharm(shadowQuarryUser, -1, "")
			}
			s.shadowTarget(c, tc.quarry, false, true, true)
			events.DrainQueuedMessagesForTest(shadowQuarryUser)
			events.DrainQueuedMessagesForTest(shadowSlyUser)

			RoomChangeShadowFollow(moveEvent(tc.quarry, true, shadowRoomA, shadowRoomB))

			got := s.queued(tc.sly)
			if tc.follow {
				require.Equal(t, []string{"north"}, got, "the shadower steps after its quarry")
				require.True(t, c.KnowsSecretExit(shadowRoomA, "north"), "the shadower learned the exit")
				require.False(t, shadowEnded(c), "the shadow goes on")
				return
			}
			require.Empty(t, got, "the shadower does not step")
			require.False(t, c.KnowsSecretExit(shadowRoomA, "north"))
			require.True(t, shadowEnded(c), "the shadow ended")
			require.Zero(t, shadowCooldown(c), "quietly: no cooldown")
			require.Empty(t, events.DrainQueuedMessagesForTest(shadowQuarryUser), "quietly: no line to the quarry")
		})
	}
}

// #781: the shadower takes an exit its quarry could have used. Here a secret
// "fissure" the quarry never found and a temporary "gap" both lead to
// shadowRoomB; the quarry went by the gap, so the shadower learns nothing.
func TestRoomChangeShadowFollow_TakesTheQuarrysExit(t *testing.T) {
	s := newShadowScene(t)
	a := rooms.LoadRoom(shadowRoomA)
	delete(a.Exits, "north")
	a.Exits["fissure"] = exit.RoomExit{RoomId: shadowRoomB, Secret: true}
	a.ExitsTemp["gap"] = exit.TemporaryRoomExit{RoomId: shadowRoomB, Title: "a narrow gap"}
	s.place(shadowPlayer, true, s.roomB)
	s.place(shadowPlayer, false, a)
	c := s.sly(shadowPlayer)
	s.shadowTarget(c, shadowPlayer, false, true, true)

	RoomChangeShadowFollow(moveEvent(shadowPlayer, true, shadowRoomA, shadowRoomB))

	require.Equal(t, []string{"gap"}, s.queued(shadowPlayer))
	require.False(t, c.KnowsSecretExit(shadowRoomA, "fissure"), "the quarry never used the fissure")
}
