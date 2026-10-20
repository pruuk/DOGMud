package behaviortree

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// The Sentinel (mob 9552) as shipped fires its Ironhorn Warbow (#265). It
// spawns through the real path (Room.Prepare on its vault, room 6371) and its
// real per-mob tree answers a combat round with a shot. Before the fix the bow
// was carried loot, never wielded, with no arrows and no try_fire in the tree,
// so the Sentinel never shot.
func TestTheSentinelFiresItsWarbow(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	// fileloader and ReloadConfig resolve "_datafiles/..." against the CWD,
	// which a shared test binary does not set to this package's directory
	// (dogmud-writing-tests). Anchor on this file and chdir to the repo root.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	require.NoError(t, err)
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoRoot))
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	configs.SetConfigForTest(t, configs.GetConfig())
	require.NoError(t, configs.ReloadConfig())

	t.Cleanup(conditions.SeedConditionsForTest(nil))
	t.Cleanup(species.SeedSpeciesForTest(nil))
	t.Cleanup(items.SeedItemsForTest(nil))
	t.Cleanup(rooms.SeedBiomesForTest(nil))
	conditions.LoadDataFiles()
	species.LoadDataFiles()
	items.LoadDataFiles()
	rooms.LoadBiomeDataFiles()

	dataFiles := string(configs.GetFilePathsConfig().DataFiles)
	highlands, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataFiles + `/mobs/eastern_highlands`)
	require.NoError(t, err)
	sentinelSpec, ok := highlands[9552]
	require.True(t, ok, "mob 9552 (The Sentinel) is not shipped")
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{9552: sentinelSpec}, map[int]*mobs.Mob{}))

	raw, err := os.ReadFile(dataFiles + `/rooms/eastern_highlands/6371.yaml`)
	require.NoError(t, err)
	vault := &rooms.Room{}
	require.NoError(t, yaml.Unmarshal(raw, vault))
	require.Equal(t, 6371, vault.RoomId, "read the wrong room file, so this test proves nothing")
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{6371: vault},
		map[string]*rooms.ZoneConfig{vault.Zone: {Name: vault.Zone, RoomId: 6371, RoomIds: map[int]struct{}{6371: {}}}}))

	vault.Prepare(false)
	mobIds := vault.GetMobs()
	require.Len(t, mobIds, 1, "room 6371 must spawn exactly its Sentinel")
	sentinel := mobs.GetInstance(mobIds[0])
	require.NotNil(t, sentinel)
	t.Cleanup(func() { mobs.SetInstanceForTest(mobIds[0], nil) })
	require.Equal(t, 9552, int(sentinel.MobId))

	// The bow is wielded, chambered at spawn, and backed by matching arrows,
	// which is what firing needs to chamber the next one (chamberNextRound).
	bow := sentinel.Character.Equipment.Weapon
	require.Equal(t, 10046, bow.ItemId, "the Sentinel wields the Ironhorn Warbow")
	require.True(t, bow.IsRangedWeapon())
	require.True(t, bow.Loaded, "an equipped ranged weapon is chambered at spawn")
	require.Equal(t, 60, bow.DropChance, "the warbow keeps the 60% drop chance it had as carried loot")
	arrows := 0
	for _, it := range sentinel.Character.Items {
		if it.GetSpec().Type == items.Ammo && it.GetSpec().AmmoTag == bow.GetSpec().AmmoTag {
			arrows += it.Uses
		}
	}
	require.Positive(t, arrows, "the Sentinel carries arrows for its bow")

	// Its real per-mob tree answers a combat round by firing at its foe.
	treePath := dataFiles + `/behaviors/eastern_highlands/9552-the_sentinel.yaml`
	require.NoError(t, GetEngine().LoadTree(9552, treePath))
	t.Cleanup(func() { GetEngine().EvictTree(9552) })

	target := seedTargetMob(t, 96371, 6371)
	sentinel.Character.SetAggro(0, target.InstanceId, characters.DefaultAttack)
	queuedCmds(sentinel.InstanceId) // clear

	// Its foe brings a torch, as a delver would, so the vault is lit whatever
	// the test clock reads (the unlit case is TestActTryFire_TooDarkToAim_Fails).
	target.Character.Conditions = conditions.New()
	torch := items.New(20096)
	target.Character.Equipment.Light = torch
	for _, id := range torch.GetSpec().WornConditionIds {
		target.Character.Conditions.AddCondition(id, true)
	}
	vault.AddMob(target.InstanceId)
	require.True(t, messaging.CanSeeClearly(&sentinel.Character, vault), "a torch in the vault lets the Sentinel aim")

	require.True(t, TryMobBehavior(sentinel.InstanceId, EventContext{EventType: "mob_combat_round"}),
		"the Sentinel's tree must claim the combat round with a shot")
	require.Contains(t, queuedCmds(sentinel.InstanceId), "shoot Target")

	// The shot itself goes through the shared fire rule and re-chambers from
	// the quiver, so the Sentinel keeps firing while arrows last.
	res := actions.ExecuteFire(&actions.MobActor{Mob: sentinel, Room: vault}, "Target")
	require.False(t, res.NoWeapon || res.NotLoaded || res.NoTarget, "the shot must resolve: %+v", res)
	require.False(t, res.TooDarkToAim || res.Blinded, "the Sentinel must see to aim in its own vault: %+v", res)
	require.True(t, res.Executed, "the shot was taken: %+v", res)
	require.True(t, res.Chambered.Loaded, "firing chambers the next arrow from the quiver")

	// A shot waits for the shared special-move cooldown, players and mobs
	// alike (owner ruling 2026-10-19, #265). While it runs the tree must NOT
	// claim the round, so the round driver falls through to the Sentinel's
	// combat commands and melee swing; once it clears, the Sentinel fires again.
	cooldown := int(configs.GetBalanceConfig().SpecialMoveCooldown)
	require.Positive(t, cooldown)
	held := 0
	for round := 0; round < cooldown+2 && !actions.SpecialMoveReady(&sentinel.Character); round++ {
		require.False(t, TryMobBehavior(sentinel.InstanceId, EventContext{EventType: "mob_combat_round"}),
			"round %d inside the cooldown: the tree must leave the round to melee", round)
		require.Empty(t, queuedCmds(sentinel.InstanceId), "no shot inside the cooldown")
		held++
		sentinel.Character.Cooldowns.RoundTick()
		sentinel.Character.Cooldowns.Prune()
	}
	require.Equal(t, cooldown, held, "the Sentinel holds its bow for exactly the shared cooldown")
	require.True(t, TryMobBehavior(sentinel.InstanceId, EventContext{EventType: "mob_combat_round"}),
		"once the cooldown clears the Sentinel fires again")
	require.Contains(t, queuedCmds(sentinel.InstanceId), "shoot Target")
}
