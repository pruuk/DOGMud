package usercommands

import (
	"os"
	"path/filepath"
	"testing"

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

// shootUntilItLands fires repeatedly until the target loses health, and fails
// the test if it never does within a generous cap.
//
// 🔴 WHY A SINGLE SHOT IS NOT A SAFE ASSERTION, even at Perception 300 against
// a skeleton with ContestFloor pinned to 0: an ATTACK FUMBLE is
// `hitRoll.ZScore <= -2.0` on the attacker's OWN distribution, so it fires at
// roughly 2.3% no matter how badly the defender is outclassed, and a fumble
// always misses. That is deliberate. resolveDefenseOutcomeInner says so:
//
//	"Fumbles deliberately REMAIN on the self-relative z-score. They share the
//	 architectural quirk crits had, but moving them would change failure rates
//	 nobody asked to change. Explicitly out of scope for chunk 5.11d -- do not
//	 'fix' them in passing."
//
// So "one loaded shot must damage the mob" asserts something the combat model
// does not promise. Diagnosed 2026-08-28 after this reddened CI on two
// consecutive PRs; measured at 1 failure in 120 runs on unmodified master and 2
// in 120 on a branch, and reproduced on a detached master worktree, which is
// what ruled out either PR as the cause.
//
// The retry is the honest fix rather than a workaround: what the test means is
// "a loaded shot CAN damage the mob", and every other assertion (the weapon
// unloading, aggro on both sides, combat memory) holds on a fumble too, so
// those stay exact and are checked by the caller afterwards.
//
// 12 attempts puts a spurious failure at roughly 0.023^12, which is never.
func shootUntilItLands(t *testing.T, user *users.UserRecord, room *rooms.Room, cmd string, healthOf func() int) {
	t.Helper()
	const attempts = 12
	start := healthOf()
	for i := 0; i < attempts; i++ {
		equipBow(user.Character, true) // firing unloads the bow; reload each try
		handled, err := Fire(cmd, user, room, 0)
		if !handled || err != nil {
			t.Fatalf("Fire(%q) attempt %d: handled=%v err=%v", cmd, i+1, handled, err)
		}
		if healthOf() < start {
			return
		}
	}
	t.Fatalf("%d shots all failed to damage the target. A ~2.3%% fumble rate "+
		"cannot explain this; the shot is not landing at all.", attempts)
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

	handled, err := Fire("skeleton", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	assert.Equal(t, 50, mob.Character.Health, "unloaded bow must deal no damage")
	assert.False(t, user.Character.IsInCombat(), "an unfired shot must not set shooter aggro")
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
	mob.Character.EndAggro()
	user.Character.EndAggro()

	// Retried: a single shot can fumble (~2.3%, self-relative). See
	// shootUntilItLands. Every OTHER assertion below is exact and holds on a
	// fumble too, so only the damage check needed the retry.
	shootUntilItLands(t, user, room, "skeleton", func() int { return mob.Character.Health })

	assert.Less(t, mob.Character.Health, 100000, "a loaded same-room shot must damage the mob")
	assert.False(t, user.Character.Equipment.Weapon.Loaded, "firing must unload the weapon")

	require.True(t, user.Character.IsInCombat(), "same-room shot must set shooter aggro")
	assert.Equal(t, 100, user.Character.CurrentCombatTarget().MobInstanceId)

	require.True(t, mob.Character.IsInCombat(), "mob must retaliate with aggro on the shooter")
	assert.Equal(t, user.UserId, mob.Character.CurrentCombatTarget().UserId)
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
	user.Character.EndAggro()
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

	// Retried for the same self-relative fumble reason as the same-room test.
	shootUntilItLands(t, user, room, "skeleton north", func() int { return target.Character.Health })

	assert.Less(t, target.Character.Health, 100000, "cross-room shot must damage the adjacent-room mob")
	assert.False(t, user.Character.IsInCombat(), "cross-room shot must NOT set shooter aggro (one-shot model)")

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

	if _, err := Fire("beggar", user, room, 0); err != nil {
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
	user.Character.EndAggro()
	equipBow(user.Character, true)

	victim := users.GetByUserId(2)
	require.NotNil(t, victim)
	victim.Character.Health = 500
	victim.Character.HealthMax.Value = 500
	victim.Character.EndAggro()

	handled, err := Fire("Bobrick", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	assert.Equal(t, 500, victim.Character.Health, "a no-PvP shot must not damage the victim")
	assert.False(t, user.Character.IsInCombat(), "a blocked PvP shot must not set shooter aggro")
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
	user.Character.EndAggro()
	equipBow(user.Character, true)

	mob := mobs.GetInstance(100)
	require.NotNil(t, mob)
	mob.Character.Health = 100000
	mob.Character.HealthMax.Value = 100000
	mob.Character.EndAggro()

	handled, err := Fire("skeleton", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	require.True(t, user.Character.IsInCombat(), "opening same-room shot must set shooter aggro before firing")
	assert.Equal(t, 1, user.Character.RoundsWaiting(), "opening shot must consume the attacker's combat round")
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
	user.Character.EndAggro()
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

	handled, err := Fire("merchant", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	assert.Equal(t, 500, target.Character.Health, "a non-combatant must take no damage")
	assert.False(t, user.Character.IsInCombat(), "a refused shot must roll back the speculative opening-shot aggro")
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
	user.Character.EndAggro()
	equipBow(user.Character, true)

	handled, err := Fire("Aliceia", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)

	assert.False(t, user.Character.IsInCombat(), "shooting yourself must not start combat")
	assert.True(t, user.Character.Equipment.Weapon.Loaded, "a self-target shot must not fire")
}

// TestShoot_NoWeapon_Message: with no ranged weapon equipped, the command
// reports the missing weapon and fires nothing.
func TestShoot_NoWeapon(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	user, room := getTestUserAndRoom(t)
	user.Character.Equipment.Weapon = items.Item{} // no weapon

	handled, err := Fire("skeleton", user, room, 0)
	assert.True(t, handled)
	assert.NoError(t, err)
	assert.False(t, user.Character.IsInCombat())
}

// rangedWrapperCycleDelta captures one FIRE, which is now the whole cycle:
// firing resolves the shot and chambers the next round in the same action.
// The fields that used to describe a separate reload phase are gone with it.
type rangedWrapperCycleDelta struct {
	fireDebit         int
	ammoAfterFire     int
	roundsAfterFire   int
	loadedAfterFire   bool
	cooldownAfterFire int
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
		user.Character.EndAggro()
		user.Character.Cooldowns = nil
		target := mobs.GetInstance(100)
		require.NotNil(t, target)
		target.Character.Stats.Dexterity.ValueAdj = 1_000_000
		target.Character.Health = 100_000
		target.Character.HealthMax.Value = 100_000

		require.NoError(t, func() error { _, err := Fire("skeleton", user, room, 0); return err }())
		require.True(t, user.Character.IsInCombat())
		return rangedWrapperCycleDelta{
			fireDebit:         50 - user.Character.Stamina,
			ammoAfterFire:     user.Character.Items[1].Uses,
			roundsAfterFire:   user.Character.RoundsWaiting(),
			loadedAfterFire:   user.Character.Equipment.Weapon.Loaded,
			cooldownAfterFire: user.Character.Cooldowns["special-move"],
		}
	}

	mobCycle := func(t *testing.T) rangedWrapperCycleDelta {
		cleanup := seedAllRegistries()
		defer cleanup()
		pinRangedCostEvidence(t)

		mob, room := getRangedTestMobAndRoom(t)
		prepareRangedCostCycle(&mob.Character, true, 50)
		mob.Character.SetAggro(1, 0, characters.DefaultAttack)
		mob.Character.Cooldowns = nil
		target := users.GetByUserId(1)
		require.NotNil(t, target)
		target.Character.Stats.Dexterity.ValueAdj = 1_000_000
		target.Character.Health = 100_000
		target.Character.HealthMax.Value = 100_000

		require.NoError(t, func() error { _, err := mobcommands.Fire("Aliceia", mob, room); return err }())
		return rangedWrapperCycleDelta{
			fireDebit:         50 - mob.Character.Stamina,
			ammoAfterFire:     mob.Character.Items[1].Uses,
			roundsAfterFire:   mob.Character.RoundsWaiting(),
			loadedAfterFire:   mob.Character.Equipment.Weapon.Loaded,
			cooldownAfterFire: mob.Character.Cooldowns["special-move"],
		}
	}

	player := playerCycle(t)
	mob := mobCycle(t)

	// PARITY IS THE POINT. Both wrappers call actions.ExecuteFire, so the fold
	// reaches mobs by construction -- but "by construction" is exactly the claim
	// that stops being true when someone adds a wrapper-level shortcut, and a mob
	// that silently stops chambering just stops shooting with nothing in any log.
	require.Equal(t, player, mob, "player and mob must fire identically")

	// The whole cycle is ONE action now, and it costs what the two commands cost
	// together before: 2 for the shot plus 2 for the chambering.
	assert.Equal(t, 4, player.fireDebit)
	assert.Equal(t, 1, player.roundsAfterFire)

	// Fired AND chambered: one projectile spent, weapon ready again.
	assert.Equal(t, 19, player.ammoAfterFire)
	assert.True(t, player.loadedAfterFire, "firing must leave the weapon ready")

	// One claim, made by the shot itself rather than by a follow-up command.
	assert.Greater(t, player.cooldownAfterFire, 0)
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
		user.Character.EndAggro()
		user.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
		target := mobs.GetInstance(100)
		require.NotNil(t, target)
		target.Character.Health = 500
		events.DrainQueuedMessagesForTest(user.UserId)

		_, err := Fire("skeleton", user, room, 0)
		require.NoError(t, err)

		assert.Equal(t, 0, user.Character.Stamina)
		assert.True(t, user.Character.Equipment.Weapon.Loaded)
		assert.Equal(t, 20, user.Character.Items[1].Uses)
		assert.False(t, user.Character.IsInCombat())
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
		mob.Character.SetAggro(1, 0, characters.DefaultAttack)
		mob.Character.Cooldowns = characters.Cooldowns{"special-move": 3}
		target := users.GetByUserId(1)
		require.NotNil(t, target)
		target.Character.Health = 500
		events.DrainQueuedMessagesForTest(target.UserId)

		_, err := mobcommands.Fire("Aliceia", mob, room)
		require.NoError(t, err)

		assert.Equal(t, 0, mob.Character.Stamina)
		assert.True(t, mob.Character.Equipment.Weapon.Loaded)
		assert.Equal(t, 20, mob.Character.Items[1].Uses)
		assert.Equal(t, 0, mob.Character.RoundsWaiting())
		assert.Equal(t, 3, mob.Character.Cooldowns["special-move"])
		assert.Equal(t, 500, target.Character.Health)
		assert.Empty(t, events.DrainQueuedMessagesForTest(target.UserId), "mob refusal must stay silent")
	})
}

// TestReloadRefusal_PlayerAndMobWrappersAreAtomic is DELETED, not moved: the
// player and mob `reload` commands it drove no longer exist, because firing
// chambers its own next round. The property it guarded -- an unaffordable
// action refuses atomically, loudly for a player and silently for a mob -- is
// covered for the surviving command by
// TestShootRefusal_PlayerAndMobWrappersAreAtomic above.

// The stale-reload wrapper test and its staleReloadPlayerActor helper are
// DELETED, not moved. They drove the retired player `reload` command through
// the executeReloadAction seam, and both the command and the seam are gone now
// that firing chambers its own next round.
//
// The property they guarded -- an admission that goes stale mid-action keeps
// the one payment and mutates nothing else -- still has coverage at the layer
// that owns it: TestReload_StaleSecondaryStateKeepsSingleAdmission in
// internal/actions/combat_reload_test.go, which exercises chamberNextRound
// directly.
