package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 14: "Aliceia flees to the north exit!" names the exit to
// the room, so an onlooker who reads it learns the secret exit; the fleer's
// own knowledge is not this rule's. In the dark the line is not read and
// nothing is learned.
func TestHandlePlayerFlee_OnlookersLearnTheSecretExitTheySeeUsed(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "lit"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			t.Cleanup(keywords.SeedKeywordsForTest()) // the arrival runs Look
			u, watcher, skeleton := users.GetByUserId(1), users.GetByUserId(2), mobs.GetInstance(100)
			room := rooms.LoadRoom(1)
			north := room.Exits["north"]
			north.Secret = true
			room.Exits["north"] = north
			u.Character.LearnSecretExit(1, "north")
			want := messaging.SightFull
			if dark {
				darken(t, 1)
				want = messaging.SightNone
			} else {
				room.Lamp = rooms.LampPtr(60)
			}
			require.Equal(t, want, room.ParticipantSight(2), "fixture: the watcher's sight")
			// The `flee` command never names an exit, and its random pick
			// skips secret exits (Room.GetRandomExitWhere), so the flee is
			// begun with a preferred exit, as the mob's `flee <exit>` is.
			require.NoError(t, u.Character.Validate())
			u.Character.Stamina = 0
			u.Character.SetAggro(0, 100, characters.DefaultAttack)
			u.Character.CombatPhase.OnRoundTick()
			require.True(t, actions.BeginFlee(actions.NewUserActorInRoom(u, room), "north").Accepted)
			require.True(t, u.Character.IsDisengaging(), "fixture did not enter Disengaging")
			drainPlain(2)

			require.True(t, handlePlayerFlee(u, room, u.UserId))
			require.Equal(t, 2, u.Character.RoomId, "fixture: the flee worked")
			read := countContaining(drainPlain(2), "flees to the north exit") > 0
			require.Equal(t, !dark, read, "premise: the watcher read the line only in the light")
			require.Equal(t, !dark, watcher.Character.KnowsSecretExit(1, "north"))
			require.Equal(t, !dark, skeleton.Character.KnowsSecretExit(1, "north"))
		})
	}
}
