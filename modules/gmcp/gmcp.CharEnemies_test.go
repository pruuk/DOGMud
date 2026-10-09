package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Char.Enemies rides the same sight gate as the fight prompt's {target}
// token (userrecord.prompt.go promptTargetSight / messaging.ReaderSight),
// per owner ruling 4 (2026-09-20): GMCP must not undo the darkness work by
// handing a modern client the enemy's name and a live HP bar while the room
// text reads "Something slashes you!".
//
// charEnemiesFixture seeds one room, one fighting mob (Grave Wight, 30/50
// hp), and one viewer, then asks GetCharNode for Char.Enemies. blind selects
// whether the viewer is Blinded before the call.
func charEnemiesFixture(t *testing.T, blind bool) []GMCPCharModule_Enemy {
	t.Helper()

	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"default": {BiomeId: "default"},
	}))

	// Lamp pins the room fully lit regardless of the ambient test round.
	// Since graded lighting plan 3a Task 8, Room.LightLevel() reads the real
	// celestial term at whatever round util.GetRoundCount() holds, which a
	// bare unpinned round reads as shapes tier, not full -- and this fixture
	// is reused by the blind-viewer lane below, whose expectations do not
	// depend on room light at all (a blinded observer sees nothing regardless).
	room := &rooms.Room{RoomId: 9700, Zone: "TestZone", Biome: "default", Lamp: rooms.LampPtr(90)}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9700: room},
		map[string]*rooms.ZoneConfig{
			"TestZone": {Name: "TestZone", RoomId: 9700, RoomIds: map[int]struct{}{9700: {}}},
		},
	))

	enemy := &mobs.Mob{MobId: 1, InstanceId: 501, Zone: "TestZone"}
	enemy.Character.Name = "Grave Wight"
	enemy.Character.Health = 30
	enemy.Character.HealthMax.Value = 50
	enemy.Character.CombatPhase = combatphase.NewMachine()
	// Fighting a player (userId 9701, the viewer) is what GetMobs(FindFighting)
	// requires to surface this mob at all -- a mob merely standing in the room
	// is not an "enemy" row.
	enemy.Character.SetAggro(9701, 0, characters.DefaultAttack)

	t.Cleanup(mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Zone: "TestZone"}},
		map[int]*mobs.Mob{501: enemy},
	))
	room.AddMob(501)

	viewer := users.NewTestUser(9701, "viewer", "Viewer", 97701)
	viewer.Character.RoomId = 9700
	viewer.Character.Perception = perception.NewMachine()
	if blind {
		require.NoError(t, viewer.Character.Perception.TransitionTo(
			perception.Blinded, state.TransitionReason{Trigger: "gmcp_char_enemies_test"}))
	}
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9701: viewer}))
	room.AddPlayer(9701)

	g := &GMCPCharModule{}
	data, moduleName := g.GetCharNode(viewer, `Char.Enemies`)
	require.Equal(t, `Char.Enemies`, moduleName)

	enemies, ok := data.([]GMCPCharModule_Enemy)
	require.True(t, ok, "Char.Enemies payload must be []GMCPCharModule_Enemy, got %T", data)
	return enemies
}

func TestCharEnemies_SightedViewerSeesNameAndHp(t *testing.T) {
	enemies := charEnemiesFixture(t, false)
	require.Len(t, enemies, 1, "the fighting mob must appear in the row")

	e := enemies[0]
	require.Equal(t, "Grave Wight", e.Name, "a sighted viewer's payload must be unchanged")
	require.Equal(t, 30, e.Hp)
	require.Equal(t, 50, e.MaxHp)
}

func TestCharEnemies_BlindViewerGetsNoIdentityOrHp(t *testing.T) {
	enemies := charEnemiesFixture(t, true)
	require.Len(t, enemies, 1,
		"the row must stay -- dropping it would tell a scripted client the fight ended")

	e := enemies[0]
	require.Equal(t, "something", e.Name, "must match the prompt's and the combat lines' wording exactly (#455)")
	require.NotEqual(t, "Grave Wight", e.Name, "a blind viewer's payload must name no mob")
	require.Equal(t, 0, e.Hp, "a blind viewer must carry no readable hp")
	require.Equal(t, 0, e.MaxHp, "a blind viewer must carry no readable hp_max")
}
