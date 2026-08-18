package usercommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crimes"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pinContestFloorOff removes Balance.ContestFloor for the duration of the
// calling test, so a shot from an overwhelming shooter actually lands.
//
// Needed since U6. ExecuteFire resolves through combat.ExecuteSkillMove, which
// used to take the maneuver floor pair; those knobs read
// configs.GetBalanceConfig(), which a Go test binary never loads from
// _datafiles/config.yaml, so they measured 0 and a lopsided shot was a
// certainty for free. U6 routed the move through combat.RunContest, and
// Balance.Validate replaces a zero ContestFloor with 0.125, so the floor is
// live in every test binary and the target now saves on about one run in
// eight. configs.SetConfigForTest assigns without validating, which is why the
// zero survives, and it self-registers the restore.
func pinContestFloorOff(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.ContestFloor = 0
	configs.SetConfigForTest(t, c)
}

// equipBow puts a (optionally loaded) longbow in the user's weapon slot. The
// inline Spec mirrors the actions package's fire test fixture.
func equipBow(c *characters.Character, loaded bool) {
	c.Equipment.Weapon = items.Item{
		ItemId: 70001,
		Loaded: loaded,
		Spec: &items.ItemSpec{
			ItemId:           70001,
			Name:             "longbow",
			Type:             items.Weapon,
			Subtype:          items.Shooting,
			AmmoTag:          "arrows",
			DamageMultiplier: 1.0,
			Hands:            2,
		},
	}
}

func prepareRangedCostCycle(c *characters.Character, loaded bool, stamina int) {
	c.Stats.Strength.ValueAdj = 100
	c.Stats.Perception.ValueAdj = 1
	if c.Skills == nil {
		c.Skills = map[string]int{}
	}
	c.Skills[string(skills.RangedCombat)] = 5
	c.Stamina = stamina
	c.StaminaMax.Value = 405
	c.Equipment.Offhand = items.Item{}
	equipBow(c, loaded)
	c.Items = []items.Item{
		{ItemId: 70003, Spec: &items.ItemSpec{ItemId: 70003, Name: "ballast", Weight: 32.5}},
		{ItemId: 70002, Uses: 20, Spec: &items.ItemSpec{
			ItemId: 70002, Name: "arrows", Type: items.Ammo, AmmoTag: "arrows",
		}},
	}
}

func pinRangedCostEvidence(t *testing.T) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.ContestFloor = 0
	cfg.Balance.ShootBaseStaminaCost = 2
	cfg.Balance.ReloadBaseStaminaCost = 1
	cfg.Balance.CarryCapacityMultiplier = 0.65
	cfg.Balance.CostEncumbranceKnee = 0.75
	cfg.Balance.CostEncumbranceKneeMult = 1.5
	cfg.Balance.CostSkillMidRank = 25
	cfg.Balance.CostSkillMultAtMid = 1
	configs.SetConfigForTest(t, cfg)
}

func getRangedTestMobAndRoom(t *testing.T) (*mobs.Mob, *rooms.Room) {
	t.Helper()
	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	room := rooms.LoadRoom(mob.Character.RoomId)
	require.NotNil(t, room)
	return mob, room
}

// isolateOpinions points the opinion store at a temp dir so test shots that
// bump disposition don't touch real data.
func isolateOpinions(t *testing.T) {
	t.Helper()
	t.Setenv("DOGMUD_OPINIONS_DIR_OVERRIDE", t.TempDir())
	opinions.ClearCache()
}

// TestShoot_UnloadedWeapon_NoDamage: an unloaded bow fires nothing — the mob's
// HP is untouched.
func TestShoot_UnloadedWeapon_NoDamage(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	isolateOpinions(t)

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Perception.ValueAdj = 300
	equipBow(user.Character, false) // unloaded

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Character.Health = 50

	handled, err := Shoot("skeleton", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	assert.Equal(t, 50, mob.Character.Health, "unloaded bow must deal no damage")
	assert.Nil(t, user.Character.Aggro, "an unfired shot must not set shooter aggro")
}

// TestShoot_SameRoomLoaded_DamageAndAggro: a loaded same-room shot drops the
// mob's HP, unloads the weapon, sets shooter aggro on the mob, and sets the
// mob's aggro back on the shooter.
func TestShoot_SameRoomLoaded_DamageAndAggro(t *testing.T) {
	pinContestFloorOff(t)

	cleanup := seedAllRegistries()
	defer cleanup()
	isolateOpinions(t)

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Perception.ValueAdj = 300
	user.Character.Stats.Strength.ValueAdj = 1
	equipBow(user.Character, true)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Character.Health = 100000
	mob.Character.HealthMax.Value = 100000
	mob.Character.Aggro = nil
	user.Character.Aggro = nil

	handled, err := Shoot("skeleton", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	assert.Less(t, mob.Character.Health, 100000, "a loaded same-room shot must damage the mob")
	assert.False(t, user.Character.Equipment.Weapon.Loaded, "firing must unload the weapon")

	require.NotNil(t, user.Character.Aggro, "same-room shot must set shooter aggro")
	assert.Equal(t, 100, user.Character.Aggro.MobInstanceId)

	require.NotNil(t, mob.Character.Aggro, "mob must retaliate with aggro on the shooter")
	assert.Equal(t, user.UserId, mob.Character.Aggro.UserId)
}

// TestShoot_CrossRoomLoaded_NoShooterAggro_MobPursues: a loaded cross-room shot
// damages the adjacent-room mob, leaves the shooter WITHOUT aggro (one-shot
// model), and writes the CombatMemory breadcrumb that drives revenge pursuit.
func TestShoot_CrossRoomLoaded_NoShooterAggro_MobPursues(t *testing.T) {
	pinContestFloorOff(t)

	cleanup := seedAllRegistries()
	defer cleanup()
	isolateOpinions(t)

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Perception.ValueAdj = 300
	user.Character.Stats.Strength.ValueAdj = 1
	user.Character.Aggro = nil
	equipBow(user.Character, true)

	// A fresh mob in the adjacent room (room 2).
	target := &mobs.Mob{
		MobId:      1,
		InstanceId: 400,
		HomeRoomId: 2,
		Character: characters.Character{
			Name:      "Skeleton",
			RoomId:    2,
			Health:    100000,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
		},
	}
	target.Character.HealthMax.Value = 100000
	target.Character.Stats.Dexterity.ValueAdj = 80
	mobs.SetInstanceForTest(400, target)
	defer mobs.SetInstanceForTest(400, nil)
	room2 := rooms.LoadRoom(2)
	require.NotNil(t, room2)
	room2.AddMob(400)
	defer room2.RemoveMob(400)

	handled, err := Shoot("skeleton north", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	assert.Less(t, target.Character.Health, 100000, "cross-room shot must damage the adjacent-room mob")
	assert.Nil(t, user.Character.Aggro, "cross-room shot must NOT set shooter aggro (one-shot model)")

	require.NotNil(t, target.CombatMemory, "cross-room shot must seed CombatMemory for revenge pursuit")
	assert.Equal(t, user.UserId, target.CombatMemory.TargetUserId, "memory must target the shooter")
	assert.Equal(t, user.Character.RoomId, target.CombatMemory.LastSeenRoomId, "memory must point at the shooter's room")
	assert.True(t, target.CombatMemory.Grudge, "pursuit memory must carry a grudge")
}

// TestShoot_RecordsAssaultCrimeOnFactionMob: shooting a faction-aligned mob
// records an assault crime attributing the shooter as the perpetrator —
// mirroring melee's recordAssaultCrime semantics.
func TestShoot_RecordsAssaultCrimeOnFactionMob(t *testing.T) {
	pinContestFloorOff(t)

	cleanup := seedAllRegistries()
	defer cleanup()

	dir := t.TempDir()
	t.Setenv("DOGMUD_FACTIONS_DIR_OVERRIDE", dir)
	t.Setenv("DOGMUD_FACTIONS_REP_DIR_OVERRIDE", t.TempDir())
	t.Setenv("DOGMUD_FACTIONS_CRIMES_DIR_OVERRIDE", t.TempDir())
	t.Setenv("DOGMUD_OPINIONS_DIR_OVERRIDE", t.TempDir())
	if err := os.WriteFile(filepath.Join(dir, "thornwall_citizens.yaml"),
		[]byte(`faction_id: thornwall_citizens
display_name: "Citizens"
description: "x"
default_rep: 0
allies: []
enemies: []
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := factions.LoadAllDefinitions(); err != nil {
		t.Fatal(err)
	}
	factions.ClearCache()
	crimes.ClearCache()
	opinions.ClearCache()

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Perception.ValueAdj = 300
	equipBow(user.Character, true)

	target := &mobs.Mob{
		MobId:      100,
		InstanceId: 410,
		HomeRoomId: 1,
		Groups:     []string{"thornwall_citizens"},
		Character: characters.Character{
			Name:      "city beggar",
			RoomId:    1,
			Health:    100000,
			Buffs:     buffs.New(),
			Cooldowns: map[string]int{},
		},
	}
	target.Character.HealthMax.Value = 100000
	mobs.SetInstanceForTest(410, target)
	defer mobs.SetInstanceForTest(410, nil)
	room.AddMob(410)
	defer room.RemoveMob(410)

	if _, err := Shoot("beggar", user, room, 0); err != nil {
		t.Fatalf("Shoot: %v", err)
	}

	got := crimes.AllForFaction("thornwall_citizens", false)
	require.Len(t, got, 1, "expected exactly one assault crime")
	assert.Equal(t, crimes.KindAssault, got[0].Kind)
	assert.Equal(t, crimes.PerpPlayer, got[0].Perpetrator.Type)
	assert.Equal(t, user.UserId, got[0].Perpetrator.Id)
}

// TestShoot_PvpDisabled_PreFireGate: shooting another player in a no-PvP world
// is blocked BEFORE firing — the victim takes no damage, the weapon stays
// loaded, and the shooter gains no aggro. (Issue 1 — the old post-fire PvP
// check let the damage stick.)
func TestShoot_PvpDisabled_PreFireGate(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	isolateOpinions(t)

	// Disable PvP world-wide; restore to the default afterward so sibling
	// tests still see PvP-permitting defaults.
	if err := configs.AddOverlayOverrides(map[string]any{"GamePlay.PVP": "disabled"}); err != nil {
		t.Fatalf("AddOverlayOverrides: %v", err)
	}
	defer configs.AddOverlayOverrides(map[string]any{"GamePlay.PVP": "enabled"})

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Perception.ValueAdj = 300
	user.Character.Aggro = nil
	equipBow(user.Character, true)

	victim := users.GetByUserId(2)
	require.NotNil(t, victim)
	victim.Character.Health = 500
	victim.Character.HealthMax.Value = 500
	victim.Character.Aggro = nil

	handled, err := Shoot("Bobrick", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	assert.Equal(t, 500, victim.Character.Health, "a no-PvP shot must not damage the victim")
	assert.Nil(t, user.Character.Aggro, "a blocked PvP shot must not set shooter aggro")
	assert.True(t, user.Character.Equipment.Weapon.Loaded, "a blocked shot must not unload the weapon")
}

// TestShoot_OpeningShot_ChargesCombatRound: an out-of-combat same-room opener
// sets the shooter's aggro BEFORE firing so RecordAndWait charges the combat
// round (RoundsWaiting == 1). (Issue 2 — old ordering gave a free opening
// shot + a full melee swing next round.)
func TestShoot_OpeningShot_ChargesCombatRound(t *testing.T) {
	pinContestFloorOff(t)

	cleanup := seedAllRegistries()
	defer cleanup()
	isolateOpinions(t)

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Perception.ValueAdj = 300
	user.Character.Aggro = nil
	equipBow(user.Character, true)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Character.Health = 100000
	mob.Character.HealthMax.Value = 100000
	mob.Character.Aggro = nil

	handled, err := Shoot("skeleton", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	require.NotNil(t, user.Character.Aggro, "opening same-room shot must set shooter aggro before firing")
	assert.Equal(t, 1, user.Character.Aggro.RoundsWaiting, "opening shot must consume the attacker's combat round")
}

// TestShoot_RefusedNonCombatant_NoAggro: a refused shot (non-combatant target)
// rolls the speculative opening-shot aggro back to nil and never fires.
// (Issue 2 early-return path.)
func TestShoot_RefusedNonCombatant_NoAggro(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	isolateOpinions(t)

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Perception.ValueAdj = 300
	user.Character.Aggro = nil
	equipBow(user.Character, true)

	target := &mobs.Mob{
		MobId:      2,
		InstanceId: 420,
		HomeRoomId: 1,
		Character: characters.Character{
			Name:         "Merchant",
			RoomId:       1,
			Health:       500,
			NonCombatant: true,
			Buffs:        buffs.New(),
			Cooldowns:    map[string]int{},
		},
	}
	target.Character.HealthMax.Value = 500
	mobs.SetInstanceForTest(420, target)
	defer mobs.SetInstanceForTest(420, nil)
	room.AddMob(420)
	defer room.RemoveMob(420)

	handled, err := Shoot("merchant", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	assert.Equal(t, 500, target.Character.Health, "a non-combatant must take no damage")
	assert.Nil(t, user.Character.Aggro, "a refused shot must roll back the speculative opening-shot aggro")
	assert.True(t, user.Character.Equipment.Weapon.Loaded, "a refused shot must not unload the weapon")
}

// TestShoot_SelfTarget_Blocked: a name lookup that resolves the shooter is
// rejected before firing. (Issue 3.)
func TestShoot_SelfTarget_Blocked(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	isolateOpinions(t)

	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Perception.ValueAdj = 300
	user.Character.Aggro = nil
	equipBow(user.Character, true)

	handled, err := Shoot("Aliceia", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	assert.Nil(t, user.Character.Aggro, "shooting yourself must not start combat")
	assert.True(t, user.Character.Equipment.Weapon.Loaded, "a self-target shot must not fire")
}

// TestShoot_NoWeapon_Message: with no ranged weapon equipped, the command
// reports the missing weapon and fires nothing.
func TestShoot_NoWeapon(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)
	user.Character.Equipment.Weapon = items.Item{} // no weapon

	handled, err := Shoot("skeleton", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)
	assert.Nil(t, user.Character.Aggro)
}

type rangedWrapperCycleDelta struct {
	shootDebit        int
	reloadDebit       int
	ammoAfterShoot    int
	ammoAfterReload   int
	roundsAfterShoot  int
	loadedAfterShoot  bool
	loadedAfterReload bool
	cooldownAfterFire int
	cooldownAfterLoad int
}

// TestShootReload_PlayerAndMobWrappersShareMechanicalDeltas catches either
// wrapper bypassing the shared actions or charging/mutating in a different
// order. The 50%-load novice fixture is Task 1's 2.88 + 1.44 evidence row,
// which commits as exactly four Stamina with shared fractional carry.
func TestShootReload_PlayerAndMobWrappersShareMechanicalDeltas(t *testing.T) {
	playerCycle := func(t *testing.T) rangedWrapperCycleDelta {
		cleanup := seedAllRegistries()
		defer cleanup()
		pinRangedCostEvidence(t)
		isolateOpinions(t)

		user, room := getTestUserAndRoom(t)
		prepareRangedCostCycle(user.Character, true, 50)
		user.Character.Aggro = nil
		user.Character.Cooldowns = nil
		target := mobs.GetInstance(100)
		require.NotNil(t, target)
		target.Character.Stats.Dexterity.ValueAdj = 1_000_000
		target.Character.Health = 100_000
		target.Character.HealthMax.Value = 100_000

		require.NoError(t, func() error { _, err := Shoot("skeleton", user, room, 0); return err }())
		require.NotNil(t, user.Character.Aggro)
		afterShoot := user.Character.Stamina
		delta := rangedWrapperCycleDelta{
			shootDebit:        50 - afterShoot,
			ammoAfterShoot:    user.Character.Items[1].Uses,
			roundsAfterShoot:  user.Character.Aggro.RoundsWaiting,
			loadedAfterShoot:  user.Character.Equipment.Weapon.Loaded,
			cooldownAfterFire: user.Character.Cooldowns["special-move"],
		}

		require.NoError(t, func() error { _, err := Reload("", user, room, 0); return err }())
		delta.reloadDebit = afterShoot - user.Character.Stamina
		delta.ammoAfterReload = user.Character.Items[1].Uses
		delta.loadedAfterReload = user.Character.Equipment.Weapon.Loaded
		delta.cooldownAfterLoad = user.Character.Cooldowns["special-move"]
		return delta
	}

	mobCycle := func(t *testing.T) rangedWrapperCycleDelta {
		cleanup := seedAllRegistries()
		defer cleanup()
		pinRangedCostEvidence(t)

		mob, room := getRangedTestMobAndRoom(t)
		prepareRangedCostCycle(&mob.Character, true, 50)
		mob.Character.Aggro = &characters.Aggro{UserId: 1}
		mob.Character.Cooldowns = nil
		target := users.GetByUserId(1)
		require.NotNil(t, target)
		target.Character.Stats.Dexterity.ValueAdj = 1_000_000
		target.Character.Health = 100_000
		target.Character.HealthMax.Value = 100_000

		require.NoError(t, func() error { _, err := mobcommands.Shoot("Aliceia", mob, room); return err }())
		afterShoot := mob.Character.Stamina
		delta := rangedWrapperCycleDelta{
			shootDebit:        50 - afterShoot,
			ammoAfterShoot:    mob.Character.Items[1].Uses,
			roundsAfterShoot:  mob.Character.Aggro.RoundsWaiting,
			loadedAfterShoot:  mob.Character.Equipment.Weapon.Loaded,
			cooldownAfterFire: mob.Character.Cooldowns["special-move"],
		}

		require.NoError(t, func() error { _, err := mobcommands.Reload("", mob, room); return err }())
		delta.reloadDebit = afterShoot - mob.Character.Stamina
		delta.ammoAfterReload = mob.Character.Items[1].Uses
		delta.loadedAfterReload = mob.Character.Equipment.Weapon.Loaded
		delta.cooldownAfterLoad = mob.Character.Cooldowns["special-move"]
		return delta
	}

	player := playerCycle(t)
	mob := mobCycle(t)
	require.Equal(t, player, mob)
	assert.Equal(t, 2, player.shootDebit)
	assert.Equal(t, 2, player.reloadDebit)
	assert.Equal(t, 4, player.shootDebit+player.reloadDebit)
	assert.Equal(t, 20, player.ammoAfterShoot)
	assert.Equal(t, 19, player.ammoAfterReload)
	assert.Equal(t, 1, player.roundsAfterShoot)
	assert.False(t, player.loadedAfterShoot)
	assert.True(t, player.loadedAfterReload)
	assert.Zero(t, player.cooldownAfterFire)
	assert.Greater(t, player.cooldownAfterLoad, 0)
}

// TestShootRefusal_PlayerAndMobWrappersAreAtomic catches wrapper-specific
// mechanics on a full-cost refusal. Only the player receives private refusal
// text; both actors preserve weapon, ammo, round, cooldown, and target health.
func TestShootRefusal_PlayerAndMobWrappersAreAtomic(t *testing.T) {
	t.Run("player", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		pinRangedCostEvidence(t)
		isolateOpinions(t)

		user, room := getTestUserAndRoom(t)
		prepareRangedCostCycle(user.Character, true, 0)
		user.Character.Aggro = nil
		user.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
		target := mobs.GetInstance(100)
		require.NotNil(t, target)
		target.Character.Health = 500
		events.DrainQueuedMessagesForTest(user.UserId)

		_, err := Shoot("skeleton", user, room, 0)
		require.NoError(t, err)

		assert.Equal(t, 0, user.Character.Stamina)
		assert.True(t, user.Character.Equipment.Weapon.Loaded)
		assert.Equal(t, 20, user.Character.Items[1].Uses)
		assert.Nil(t, user.Character.Aggro)
		assert.Equal(t, 3, user.Character.Cooldowns["special-move"])
		assert.Equal(t, 500, target.Character.Health)
		assertVoluntaryRefusalOutput(t, events.DrainQueuedMessagesForTest(user.UserId), characters.PoolStamina)
	})

	t.Run("mob", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		pinRangedCostEvidence(t)

		mob, room := getRangedTestMobAndRoom(t)
		prepareRangedCostCycle(&mob.Character, true, 0)
		mob.Character.Aggro = &characters.Aggro{UserId: 1}
		mob.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
		target := users.GetByUserId(1)
		require.NotNil(t, target)
		target.Character.Health = 500
		events.DrainQueuedMessagesForTest(target.UserId)

		_, err := mobcommands.Shoot("Aliceia", mob, room)
		require.NoError(t, err)

		assert.Equal(t, 0, mob.Character.Stamina)
		assert.True(t, mob.Character.Equipment.Weapon.Loaded)
		assert.Equal(t, 20, mob.Character.Items[1].Uses)
		assert.Equal(t, 0, mob.Character.Aggro.RoundsWaiting)
		assert.Equal(t, 3, mob.Character.Cooldowns["special-move"])
		assert.Equal(t, 500, target.Character.Health)
		assert.Empty(t, events.DrainQueuedMessagesForTest(target.UserId), "mob refusal must stay silent")
	})
}

// TestReloadRefusal_PlayerAndMobWrappersAreAtomic catches a cost refusal
// consuming a projectile, loading the weapon, pruning/consuming cooldown, or
// diverging mechanically between player and mob wrappers.
func TestReloadRefusal_PlayerAndMobWrappersAreAtomic(t *testing.T) {
	for _, actorType := range []string{"player", "mob"} {
		t.Run(actorType, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			pinRangedCostEvidence(t)

			if actorType == "player" {
				user, room := getTestUserAndRoom(t)
				prepareRangedCostCycle(user.Character, false, 0)
				user.Character.Cooldowns = characters.Cooldowns{"special-move": -2, "other": 7}
				events.DrainQueuedMessagesForTest(user.UserId)
				_, err := Reload("", user, room, 0)
				require.NoError(t, err)
				assert.False(t, user.Character.Equipment.Weapon.Loaded)
				assert.Equal(t, 20, user.Character.Items[1].Uses)
				assert.Equal(t, characters.Cooldowns{"special-move": -2, "other": 7}, user.Character.Cooldowns)
				assertVoluntaryRefusalOutput(t, events.DrainQueuedMessagesForTest(user.UserId), characters.PoolStamina)
				return
			}

			mob, room := getRangedTestMobAndRoom(t)
			prepareRangedCostCycle(&mob.Character, false, 0)
			mob.Character.Cooldowns = characters.Cooldowns{"special-move": -2, "other": 7}
			target := users.GetByUserId(1)
			require.NotNil(t, target)
			events.DrainQueuedMessagesForTest(target.UserId)
			_, err := mobcommands.Reload("", mob, room)
			require.NoError(t, err)
			assert.False(t, mob.Character.Equipment.Weapon.Loaded)
			assert.Equal(t, 20, mob.Character.Items[1].Uses)
			assert.Equal(t, characters.Cooldowns{"special-move": -2, "other": 7}, mob.Character.Cooldowns)
			assert.Empty(t, events.DrainQueuedMessagesForTest(target.UserId), "mob refusal must stay silent")
		})
	}
}

type staleReloadPlayerActor struct {
	actions.Actor
	characterCalls int
	onAdmission    func(*characters.Character)
}

func (a *staleReloadPlayerActor) GetCharacter() *characters.Character {
	a.characterCalls++
	char := a.Actor.GetCharacter()
	if a.characterCalls == 3 && a.onAdmission != nil {
		a.onAdmission(char)
	}
	return char
}

// TestReloadPaidStalePlayerWrapperHasNoSuccessSideEffects drives the real
// player command wrapper through each paid admission-time invalidation. The
// paid cost remains, but a stale action must never be rendered or published as
// a successful reload and must never advance a reload-gated quest.
func TestReloadPaidStalePlayerWrapperHasNoSuccessSideEffects(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutate   func(*characters.Character)
		assertNo func(*testing.T, *characters.Character)
	}{
		{
			name: "weapon changed",
			mutate: func(char *characters.Character) {
				replacement := char.Equipment.Weapon
				replacement.ItemId++
				spec := *replacement.Spec
				spec.ItemId = replacement.ItemId
				replacement.Spec = &spec
				char.Equipment.Weapon = replacement
			},
			assertNo: func(t *testing.T, char *characters.Character) {
				assert.False(t, char.Equipment.Weapon.Loaded)
				assert.Equal(t, 20, char.Items[1].Uses)
				assert.Zero(t, char.Cooldowns["special-move"])
			},
		},
		{
			name: "ammunition disappeared",
			mutate: func(char *characters.Character) {
				char.Items = char.Items[:1]
			},
			assertNo: func(t *testing.T, char *characters.Character) {
				assert.False(t, char.Equipment.Weapon.Loaded)
				require.Len(t, char.Items, 1)
				assert.Equal(t, 70003, char.Items[0].ItemId)
				assert.Zero(t, char.Cooldowns["special-move"])
			},
		},
		{
			name: "cooldown became busy",
			mutate: func(char *characters.Character) {
				char.Cooldowns["special-move"] = 3
			},
			assertNo: func(t *testing.T, char *characters.Character) {
				assert.False(t, char.Equipment.Weapon.Loaded)
				assert.Equal(t, 20, char.Items[1].Uses)
				assert.Equal(t, 3, char.Cooldowns["special-move"])
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			cleanupQuests := questengine.ResetEngineForTest()
			defer cleanupQuests()
			pinRangedCostEvidence(t)

			user, room := getTestUserAndRoom(t)
			prepareRangedCostCycle(user.Character, false, 50)
			observer := users.GetByUserId(2)
			require.NotNil(t, observer)

			const questFlag = "99001-reloaded"
			questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
				QuestId: 99001,
				Name:    "Stale Reload Wrapper Test",
				Steps:   []questengine.QuestStep{{Id: "start"}},
				Flags: []questengine.QuestFlagDef{{
					Key: "reloaded", Values: []string{"yes"},
				}},
				Triggers: []questengine.TriggerDef{{
					Event:   "command",
					Command: "reload",
					Actions: []questengine.ActionDef{{SetFlag: &questengine.QuestFlagAction{
						Key: questFlag, Value: "yes",
					}}},
				}},
			})

			originalExecute := executeReloadAction
			executeReloadAction = func(actor actions.Actor) actions.ReloadResult {
				return actions.ExecuteReload(&staleReloadPlayerActor{
					Actor:       actor,
					onAdmission: tc.mutate,
				})
			}
			t.Cleanup(func() { executeReloadAction = originalExecute })

			events.DrainQueuedMessagesForTest(user.UserId)
			events.DrainQueuedMessagesForTest(observer.UserId)
			handled, err := Reload("", user, room, 0)
			require.True(t, handled)
			require.NoError(t, err)

			assert.Equal(t, 49, user.Character.Stamina,
				"the one already-paid admission remains charged")
			tc.assertNo(t, user.Character)
			assert.NotContains(t,
				strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "\n"),
				"You ready your")
			assert.NotContains(t,
				strings.Join(events.DrainQueuedMessagesForTest(observer.UserId), "\n"),
				"readies their")
			assert.Empty(t, user.Character.GetQuestFlag(questFlag),
				"a paid stale reload must not advance the reload quest")
		})
	}
}
