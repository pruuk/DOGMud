package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #428 review: corpse salvage named the corpse to a salvager who makes out
// shapes only, and its room line wrapped the corpse in a mobname tag, which
// Anonymize turned into "kneels over the a figure". Both lines now read the
// corpse through rooms.Corpse.NameAt / ObservedName with the dead one's name
// hidden, so a shapes reader reads "corpse of a figure".

const salvageSightMobId = 98341

func salvageSightScene(t *testing.T, biome string) (*users.UserRecord, *rooms.Room) {
	t.Helper()
	salvager, room := drinkSightScene(t, biome)
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
		salvageSightMobId: {MobId: salvageSightMobId, Groups: []string{"animal"}},
	}, map[int]*mobs.Mob{}))
	wolf := rooms.Corpse{MobId: salvageSightMobId}
	wolf.Character.Name = "Grey Wolf"
	room.Corpses = []rooms.Corpse{wolf}
	return salvager, room
}

func TestSalvageCorpse_ShapesReadersDoNotReadTheCorpseName(t *testing.T) {
	salvager, room := salvageSightScene(t, "cave")
	require.True(t, salvager.Character.Conditions.AddCondition(drinkSightInfraredId, true))

	result := Salvage(&UserActor{User: salvager, Room: room}, SalvageOptions{TargetCorpse: true})
	require.True(t, result.RollHappened, "fixture must reach the salvage roll: %q", result.Reason)

	own := drinkSightPlain(events.DrainQueuedMessagesForTest(drinkSightDrinkerId))
	watcher := drinkSightPlain(events.DrainQueuedMessagesForTest(drinkSightInfraId))
	require.NotEmpty(t, own)
	for _, l := range own {
		require.NotContains(t, l, "Wolf", "the salvager read the name: %v", own)
	}
	require.Contains(t, own[0], "salvage the corpse of a figure", "got %v", own)
	require.Equal(t, []string{"A figure kneels over the corpse of a figure and works it for salvage."}, watcher)
}

func TestSalvageCorpse_ClearReadersReadTheNames(t *testing.T) {
	salvager, room := salvageSightScene(t, "city")

	result := Salvage(&UserActor{User: salvager, Room: room}, SalvageOptions{TargetCorpse: true})
	require.True(t, result.RollHappened, "fixture must reach the salvage roll: %q", result.Reason)

	own := drinkSightPlain(events.DrainQueuedMessagesForTest(drinkSightDrinkerId))
	watcher := drinkSightPlain(events.DrainQueuedMessagesForTest(drinkSightInfraId))
	require.NotEmpty(t, own)
	require.Contains(t, own[0], "salvage the Grey Wolf corpse", "got %v", own)
	require.Equal(t, []string{"Drinker kneels over the corpse of Grey Wolf and works it for salvage."}, watcher)
}

// A salvager below clear sight whose corpse vanished is told so without the
// dead mob's name, which the salvager may never have seen.
func TestSalvageCorpse_VanishedCorpseUnnamedInTheDark(t *testing.T) {
	const mobId = 98342
	goblin := &mobs.Mob{}
	goblin.Character.Name = "goblin"
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{mobId: goblin}, map[int]*mobs.Mob{}))

	room := newSalvageTestRoom(t, 9406) // unlit, no corpses present
	user := newSalvageFakeActor(t, "SalvageTester4", room, true, 4)
	_ = Salvage(user, SalvageOptions{TargetCorpse: true, TargetCorpseMobId: mobId, TargetCorpseRoundCreated: 1})

	require.Len(t, user.sent, 1)
	require.NotContains(t, user.sent[0], "goblin")
	require.Contains(t, user.sent[0], "You can no longer find the corpse you were working on.")
}
