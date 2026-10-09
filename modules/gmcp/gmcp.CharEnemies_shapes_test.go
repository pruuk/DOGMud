package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const charEnemiesHeatEyesConditionId = 97031

// #455: in a shapes-only fight the enemy row said "an unseen foe" while every
// combat line said "a figure". The row now uses UnseenNoun of the viewer's
// sight. The viewer here carries heat sight in a pitch-dark room, so they
// make out shapes and nothing more.
func TestCharEnemies_ShapesViewerReadsAFigure(t *testing.T) {
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		charEnemiesHeatEyesConditionId: {
			ConditionId: charEnemiesHeatEyesConditionId,
			Name:        "Test Heat Eyes",
			Flags:       []conditions.Flag{conditions.InfraredVision},
			Effects:     map[conditions.EffectKind]conditions.EffectValue{conditions.EffectInfraReach: {Literal: 30}},
		},
	}))
	room := &rooms.Room{RoomId: 9710, Zone: "TestZone", Biome: "cave"}
	t.Cleanup(rooms.SeedRoomsForTest(
		map[int]*rooms.Room{9710: room},
		map[string]*rooms.ZoneConfig{
			"TestZone": {Name: "TestZone", RoomId: 9710, RoomIds: map[int]struct{}{9710: {}}},
		},
	))

	enemy := &mobs.Mob{MobId: 1, InstanceId: 511, Zone: "TestZone"}
	enemy.Character.Name = "Grave Wight"
	enemy.Character.Health = 30
	enemy.Character.HealthMax.Value = 50
	enemy.Character.CombatPhase = combatphase.NewMachine()
	enemy.Character.SetAggro(9711, 0, characters.DefaultAttack)
	t.Cleanup(mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Zone: "TestZone"}},
		map[int]*mobs.Mob{511: enemy},
	))
	room.AddMob(511)

	viewer := users.NewTestUser(9711, "viewer", "Viewer", 97711)
	viewer.Character.RoomId = 9710
	require.True(t, viewer.Character.Conditions.AddCondition(charEnemiesHeatEyesConditionId, true))
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9711: viewer}))
	room.AddPlayer(9711)

	g := &GMCPCharModule{}
	data, _ := g.GetCharNode(viewer, `Char.Enemies`)
	enemies, ok := data.([]GMCPCharModule_Enemy)
	require.True(t, ok)
	require.Len(t, enemies, 1)
	require.Equal(t, "a figure", enemies[0].Name, "a shapes viewer reads what the combat lines say")
	require.Equal(t, 0, enemies[0].Hp, "a shapes viewer must carry no readable hp")
	require.Equal(t, 0, enemies[0].MaxHp)
}
