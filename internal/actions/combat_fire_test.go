package actions

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// pinContestFloorOff removes Balance.ContestFloor for the duration of the
// calling test, so a lopsided contest resolves the obvious way every time.
//
// Needed since U6. Before it, the maneuver and spell floors read
// configs.GetBalanceConfig(), which a Go test binary never loads from
// _datafiles/config.yaml, so they measured 0 and "an overwhelming attacker
// always hits" was true for free. U6 routed every contest through
// combat.RunContest, and Balance.Validate replaces a zero ContestFloor with
// 0.125, so the floor is live in every test binary and the hopeless side saves
// on about one attempt in eight. Over an 8-iteration loop that is a two-in-three
// failure, not a rare flake.
//
// configs.SetConfigForTest assigns without validating, which is why the zero
// survives, and it self-registers the restore.
func pinContestFloorOff(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.ContestFloor = 0
	configs.SetConfigForTest(t, c)
}

// fireRangedWeapon builds a loaded-by-default shooting weapon with a known
// damage multiplier so damage-band assertions are deterministic.
func fireRangedWeapon(id int, mult float64, loaded bool) items.Item {
	return items.Item{
		ItemId: id,
		Loaded: loaded,
		Spec: &items.ItemSpec{
			ItemId:           id,
			Name:             "longbow",
			Type:             items.Weapon,
			Subtype:          items.Shooting,
			AmmoTag:          "arrows",
			DamageMultiplier: mult,
			Hands:            2,
		},
	}
}

// fireAttacker builds the shooter: Perception-dominant, Strength-starved, so
// any successful hit / damage proves Perception (not Strength) governs the
// ranged channel.
func fireAttacker() *characters.Character {
	c := characters.New()
	c.Stats.Perception.ValueAdj = 300
	c.Stats.Strength.ValueAdj = 1
	c.Stamina = 100
	c.StaminaMax.Value = 100
	return c
}

func admissionCallPositions(t *testing.T, fset *token.FileSet, body *ast.BlockStmt, action, base string) []token.Pos {
	t.Helper()
	positions := []token.Pos{}
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 4 {
			return true
		}
		fun, ok := call.Fun.(*ast.Ident)
		if !ok || fun.Name != "admitFullCost" {
			return true
		}
		want := []string{"actor", action, "characters.PoolStamina", "float64(cfg." + base + ")"}
		for i := range want {
			if formattedASTNode(t, fset, call.Args[i]) != want[i] {
				return true
			}
		}
		positions = append(positions, call.Pos())
		return true
	})
	return positions
}

// seedFireMobInRoom seeds rooms 1 (exit north→2) and 2, plus a single mob
// instance (500, "Skeleton") in defenderRoomId with the given Dexterity.
// Returns the instance id and a cleanup. A fresh call gives a fresh
// defender — important because ExecuteSkillMove applies real HP loss.
func seedFireMobInRoom(t *testing.T, defenderRoomId int, defenderDex int) (int, func()) {
	t.Helper()

	mobSpecs := map[int]*mobs.Mob{
		1: {MobId: 1, Zone: "TestZone", Character: characters.Character{Name: "Skeleton"}},
	}

	var defChar characters.Character
	defChar.Name = "Skeleton"
	defChar.RoomId = defenderRoomId
	defChar.Health = 100000
	defChar.Buffs = buffs.New()
	defChar.Cooldowns = map[string]int{}
	defChar.Stats.Dexterity.ValueAdj = defenderDex

	mobInstances := map[int]*mobs.Mob{
		500: {MobId: 1, InstanceId: 500, HomeRoomId: defenderRoomId, Character: defChar},
	}
	cleanupMobs := mobs.SeedMobsForTest(mobSpecs, mobInstances)

	room1 := &rooms.Room{
		RoomId: 1, Zone: "TestZone", Title: "Room One", Biome: "city",
		Exits: map[string]exit.RoomExit{"north": {RoomId: 2}},
	}
	room2 := &rooms.Room{RoomId: 2, Zone: "TestZone", Title: "Room Two", Biome: "city"}
	cleanupRooms := rooms.SeedRoomsForTest(
		map[int]*rooms.Room{1: room1, 2: room2},
		map[string]*rooms.ZoneConfig{
			"TestZone": {Name: "TestZone", RoomId: 1, RoomIds: map[int]struct{}{1: {}, 2: {}}},
		},
	)

	if defenderRoomId == 1 {
		room1.AddMob(500)
		rooms.MarkRoomOccupancy(1, 0, 1)
	} else {
		room2.AddMob(500)
		rooms.MarkRoomOccupancy(2, 0, 1)
	}

	return 500, func() {
		cleanupRooms()
		cleanupMobs()
	}
}

// ---------------------------------------------------------------------------
// 1. No ranged weapon → NoWeapon
// ---------------------------------------------------------------------------

func TestFire_NoRangedWeapon(t *testing.T) {
	char := fireAttacker()
	char.Equipment.Weapon = reloadMeleeWeapon(2)
	actor := newStubActor(char, newTestRoom())

	res := ExecuteFire(actor, "skeleton")

	assert.True(t, res.NoWeapon, "expected NoWeapon with only a melee weapon")
	assert.False(t, res.Executed)
}

// ---------------------------------------------------------------------------
// 2. Unloaded ranged weapon → NotLoaded (WeaponName set)
// ---------------------------------------------------------------------------

func TestFire_Unloaded(t *testing.T) {
	char := fireAttacker()
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, false)
	actor := newStubActor(char, newTestRoom())

	res := ExecuteFire(actor, "skeleton")

	assert.True(t, res.NotLoaded, "expected NotLoaded for an unloaded ranged weapon")
	assert.NotEmpty(t, res.WeaponName, "NotLoaded result should name the weapon")
	assert.False(t, res.Executed)
}

// ---------------------------------------------------------------------------
// 3. Loaded + same-room mob target → Executed, unloads, Perception governs
// ---------------------------------------------------------------------------

// "Perception governs" is a property of the SCORE the seam sends to the
// contest, so assert on that score through a deterministic runner rather than
// on stochastic win rates. The old form looped 8 live-dice shots and asserted
// every one hit; the self-relative fumble (~2.3% per shot, independent of any
// stat gap) failed roughly one run in six.
func TestFire_SameRoomMob_PerceptionGoverns(t *testing.T) {
	// Fresh defender — ExecuteSkillMove mutates HP.
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	// Intercept the ONE contest: capture the attack score and hand back a
	// deterministic ordinary attack win (normalized margin 0.5 — no crit at
	// any bar, no fumble, no floor in play).
	calls := 0
	var gotAtkScore float64
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		calls++
		gotAtkScore = atkScore
		return tauntDeterministicRunner(t, 0.5, 0.5, -0.5)(atkScore, entries)
	})
	defer restore()

	char := fireAttacker()
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
	actor := newStubActor(char, rooms.LoadRoom(1))

	res := ExecuteFire(actor, "skeleton")

	require.True(t, res.Executed, "expected Executed")
	assert.True(t, res.IsTargetMob, "target should be a mob")
	assert.Equal(t, 500, res.TargetMobInstanceId)
	assert.False(t, res.CrossRoom, "same-room shot")
	// Writeback: the equipment slot itself is now unloaded.
	assert.False(t, char.Equipment.Weapon.Loaded,
		"weapon must be unloaded after firing")

	// THE assertion: the score the seam contested is Perception (300) + ranged
	// rank × SkillWeight, times the shared situational layer (Task 17 — the
	// shot's own stamina admission leaves the shooter fractionally below full),
	// and NOT a score built on the Strength floor (1).
	require.Equal(t, 1, calls, "ExecuteFire must run exactly ONE contest")
	cfg := configs.GetBalanceConfig()
	wantAtk := (float64(char.GetEffectivePerception()) +
		float64(char.GetSkillLevel(skills.RangedCombat))*float64(cfg.SkillWeight)) *
		combat.SituationalAttackMult(char, combat.ChannelRanged)
	require.InDelta(t, wantAtk, gotAtkScore, 1e-9,
		"the contested attack score must be governed by Perception")

	// The deterministic win landed, and damage drove off Perception (mean ~90
	// at these knobs), not the Strength floor: P(roll ≤ 1) is ~1e-11.
	assert.True(t, res.MoveResult.Hit, "the deterministic attack win must land")
	assert.Greater(t, res.MoveResult.Damage, 1,
		"damage should reflect Perception, not collapse to the Str floor")
}

// (Section 4 retired by U6b Task 8: it pinned the deleted folded defence
// scalar's flat shield bonus. Its replacement contract — a shield contributes
// a real block CONTEST entry and no flat addend anywhere in the score path —
// is pinned by combat_fire_seam_test.go.)

// ---------------------------------------------------------------------------
// 5. Cross-room: loaded + valid exit + adjacent-room target → CrossRoom
// ---------------------------------------------------------------------------

func TestFire_CrossRoomMob(t *testing.T) {
	_, cleanup := seedFireMobInRoom(t, 2, 1) // mob lives in room 2
	defer cleanup()

	char := fireAttacker()
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
	actor := newStubActor(char, rooms.LoadRoom(1))

	res := ExecuteFire(actor, "skeleton north")

	require.True(t, res.Executed, "expected a cross-room shot to execute")
	assert.True(t, res.CrossRoom, "expected CrossRoom true")
	assert.Equal(t, "north", res.ExitName, "expected the exit name carried through")
	assert.Equal(t, 2, res.TargetRoomId, "target should be resolved in room 2")
	assert.Equal(t, 500, res.TargetMobInstanceId)
	assert.False(t, char.Equipment.Weapon.Loaded, "weapon must be unloaded after firing")
}

// TestFire_UnseenTargetIsRejectedBeforeAdmission guards the target-visibility
// gate on both supported lines of fire. A hidden occupant remains present in
// Room.FindByName, so name resolution alone is not enough: the shot must be
// refused before stamina, aggro, ammunition, health, or round state changes.
func TestFire_UnseenTargetIsRejectedBeforeAdmission(t *testing.T) {
	// The three rejection reasons report through THREE different flags, because
	// they need three different sentences. A hidden target really is unfindable;
	// a blinded shooter and an unlit room both have the target right in front of
	// them and would be misinformed by "Could not find your target."
	for _, tc := range []struct {
		name           string
		defenderRoomId int
		rest           string
		hideTarget     bool
		blindShooter   bool
		wantNoTarget   bool
		wantBlinded    bool
	}{
		{name: "same room hidden target", defenderRoomId: 1, rest: "skeleton", hideTarget: true, wantNoTarget: true},
		{name: "cross room hidden target", defenderRoomId: 2, rest: "skeleton north", hideTarget: true, wantNoTarget: true},
		{name: "same room blinded shooter", defenderRoomId: 1, rest: "skeleton", blindShooter: true, wantBlinded: true},
		{name: "cross room blinded shooter", defenderRoomId: 2, rest: "skeleton north", blindShooter: true, wantBlinded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, cleanup := seedFireMobInRoom(t, tc.defenderRoomId, 1)
			defer cleanup()

			target := mobs.GetInstance(500)
			require.NotNil(t, target)
			if tc.hideTarget {
				addHiddenBuff(&target.Character)
			}

			char := fireAttacker()
			char.Stamina = 10
			char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
			if tc.blindShooter {
				char.AddCondition(characters.ConditionBlinded, 3, 1, "test")
			}
			actor := newStubActor(char, rooms.LoadRoom(1))
			healthBefore := target.Character.Health

			res := ExecuteFire(actor, tc.rest)

			require.Equal(t, tc.wantNoTarget, res.NoTarget, "NoTarget flag")
			require.Equal(t, tc.wantBlinded, res.Blinded, "Blinded flag")
			assert.False(t, res.TooDarkToAim, "neither case is a lighting problem")
			assert.False(t, res.Executed)
			assert.Equal(t, characters.CostCommitResult{}, res.Cost)
			assert.Equal(t, 10, char.Stamina)
			assert.True(t, char.Equipment.Weapon.Loaded)
			assert.Nil(t, char.Aggro)
			assert.Equal(t, healthBefore, target.Character.Health)
			assert.Equal(t, 0, char.Cooldowns["special-move"])
		})
	}
}

// ---------------------------------------------------------------------------
// 6. Fire does NOT consume the special-move cooldown
// ---------------------------------------------------------------------------

func TestFire_DoesNotConsumeSpecialMoveCooldown(t *testing.T) {
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	char := fireAttacker()
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
	actor := newStubActor(char, rooms.LoadRoom(1))

	res := ExecuteFire(actor, "skeleton")
	require.True(t, res.Executed)

	// Reload owns the cooldown; firing must leave it free.
	assert.True(t, char.Cooldowns.Try("special-move", "5 rounds"),
		"firing must NOT consume the special-move cooldown")
}

// TestFire_RefusedCostIsAtomic catches admission being omitted or moved after
// unloading, round consumption, ammunition mutation, or ranged resolution.
func TestFire_RefusedCostIsAtomic(t *testing.T) {
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	char := fireAttacker()
	char.Stamina = 0
	char.StaminaMax.Value = 100
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
	char.Items = []items.Item{reloadAmmoBundle(3, "arrows", 20)}
	char.Aggro = &characters.Aggro{MobInstanceId: 500}
	char.Cooldowns = characters.Cooldowns{"special-move": 3, "other": 7}
	targetMob := mobs.GetInstance(500)
	require.NotNil(t, targetMob)
	healthBefore := targetMob.Character.Health
	cooldownsBefore := maps.Clone(char.Cooldowns)

	res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton")

	require.Equal(t, characters.CostRefused, res.Cost.Status)
	assert.False(t, res.Executed)
	assert.Equal(t, 0, char.Stamina)
	assert.True(t, char.Equipment.Weapon.Loaded)
	require.Len(t, char.Items, 1)
	assert.Equal(t, 20, char.Items[0].Uses)
	assert.Equal(t, 0, char.Aggro.RoundsWaiting)
	assert.Equal(t, healthBefore, targetMob.Character.Health)
	assert.Equal(t, cooldownsBefore, char.Cooldowns)
}

// TestFire_AffordableMissPaysUnloadsAndConsumesRound catches a valid miss
// becoming free, retaining its projectile, consuming a special cooldown, or
// failing to consume the shooter's combat round.
func TestFire_AffordableMissPaysUnloadsAndConsumesRound(t *testing.T) {
	pinContestFloorOff(t)
	_, cleanup := seedFireMobInRoom(t, 1, 1_000_000)
	defer cleanup()

	char := fireAttacker()
	char.Stats.Perception.ValueAdj = 1
	char.Stats.Strength.ValueAdj = 100
	char.Skills[string(skills.RangedCombat)] = 25
	char.Stamina = 10
	char.StaminaMax.Value = 100
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
	char.Items = []items.Item{reloadAmmoBundle(3, "arrows", 20)}
	char.Aggro = &characters.Aggro{MobInstanceId: 500}
	char.Cooldowns = characters.Cooldowns{"special-move": 3}
	targetMob := mobs.GetInstance(500)
	require.NotNil(t, targetMob)
	healthBefore := targetMob.Character.Health

	res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton")

	require.True(t, res.Executed)
	require.Equal(t, characters.CostPaid, res.Cost.Status)
	assert.Equal(t, 2, res.Cost.Charged)
	assert.Equal(t, 8, char.Stamina)
	assert.False(t, res.MoveResult.Hit)
	assert.Zero(t, res.MoveResult.Damage)
	assert.Equal(t, healthBefore, targetMob.Character.Health)
	assert.False(t, char.Equipment.Weapon.Loaded)
	assert.Equal(t, 20, char.Items[0].Uses)
	assert.Equal(t, 1, char.Aggro.RoundsWaiting)
	assert.Equal(t, characters.Cooldowns{"special-move": 3}, char.Cooldowns)
}

// TestFire_StaleWeaponAfterAdmissionDoesNotUnloadReplacement catches a paid
// shot acting on a different equipped item after the read-only weapon gate.
func TestFire_StaleWeaponAfterAdmissionDoesNotUnloadReplacement(t *testing.T) {
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	char := fireAttacker()
	char.Stats.Strength.ValueAdj = 100
	char.Skills[string(skills.RangedCombat)] = 25
	char.Stamina = 10
	char.Equipment.Weapon = fireRangedWeapon(1, 1, true)
	char.Aggro = &characters.Aggro{MobInstanceId: 500}
	target := mobs.GetInstance(500)
	require.NotNil(t, target)
	healthBefore := target.Character.Health
	actor := &staleRangedSecondaryActor{Actor: newStubActor(char, rooms.LoadRoom(1))}
	actor.onAdmission = func(c *characters.Character) {
		c.Equipment.Weapon = fireRangedWeapon(2, 1, true)
	}

	res := ExecuteFire(actor, "skeleton")

	require.Equal(t, characters.CostPaid, res.Cost.Status)
	assert.Equal(t, 2, res.Cost.Charged)
	assert.Equal(t, 8, char.Stamina)
	assert.False(t, res.Executed)
	assert.Equal(t, 2, char.Equipment.Weapon.ItemId)
	assert.True(t, char.Equipment.Weapon.Loaded)
	assert.Equal(t, 0, char.Aggro.RoundsWaiting)
	assert.Equal(t, healthBefore, target.Character.Health)
}

// TestFireAdmissionOrdering parses ExecuteFire's AST so a source reformat
// cannot satisfy the guard. It catches any special-cooldown API use and proves
// the exact configured admission precedes unload, resolution, and round use.
func TestFireAdmissionOrdering(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filepath.Join(filepath.Dir(thisFile), "combat_fire.go"), nil, 0)
	require.NoError(t, err)

	var body *ast.BlockStmt
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "ExecuteFire" {
			body = fn.Body
			break
		}
	}
	require.NotNil(t, body)

	admit := admissionCallPositions(t, fset, body, "costs.ActionShoot", "ShootBaseStaminaCost")
	require.Len(t, admit, 1)
	roomVisibility := exactCallPositions(t, fset, body,
		"messaging.CanSeeClearly(char, targetRoom)", false)
	hiddenTarget := exactCallPositions(t, fset, body, "defChar.IsHidden()", false)
	require.Len(t, roomVisibility, 1)
	require.Len(t, hiddenTarget, 1)
	resolve := exactCallPositions(t, fset, body, "combat.ExecuteSkillMove", true)
	round := exactCallPositions(t, fset, body, "RecordAndWait", true)
	require.Len(t, resolve, 1)
	require.Len(t, round, 1)

	unloads := []token.Pos{}
	cooldownCalls := []string{}
	cooldownPos := []token.Pos{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			if len(n.Lhs) == 1 && len(n.Rhs) == 1 &&
				formattedASTNode(t, fset, n.Lhs[0]) == "weapon.Loaded" &&
				formattedASTNode(t, fset, n.Rhs[0]) == "false" {
				unloads = append(unloads, n.Pos())
			}
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok && strings.Contains(sel.Sel.Name, "Cooldown") {
				cooldownCalls = append(cooldownCalls, formattedASTNode(t, fset, n))
				cooldownPos = append(cooldownPos, n.Pos())
			}
		case *ast.SelectorExpr:
			if formattedASTNode(t, fset, n) == "char.Cooldowns" {
				cooldownCalls = append(cooldownCalls, "char.Cooldowns")
				cooldownPos = append(cooldownPos, n.Pos())
			}
		}
		return true
	})
	require.Len(t, unloads, 1)

	// U10d narrowed this from "shoot must neither query nor mutate any
	// cooldown" to EXACTLY ONE claim, the same-room surprise opener's. An
	// ORDINARY shot still touches no timer (pinned behaviourally by
	// TestFire_DoesNotConsumeSpecialMoveCooldown and
	// TestFireSurprise_OrdinaryShotDoesNotBurnTheCooldown); what this guard
	// keeps is that there is no SECOND cooldown site and that the one claim
	// cannot fire before admission, which would let a refused shot burn the
	// shared special-move timer.
	require.Len(t, cooldownCalls, 1, "exactly one cooldown site: the U10d surprise opener")
	assert.True(t, strings.HasPrefix(cooldownCalls[0], `char.TryCooldown("special-move"`),
		"the surprise opener must CLAIM (not merely read) the shared special-move "+
			"timer, got %q", cooldownCalls[0])
	assert.Contains(t, cooldownCalls[0], "cfg.SpecialMoveCooldown",
		"the claim must be sized by the shared knob, not a literal, got %q", cooldownCalls[0])
	assert.Less(t, int(roomVisibility[0]), int(admit[0]))
	assert.Less(t, int(hiddenTarget[0]), int(admit[0]))
	assert.Less(t, int(admit[0]), int(cooldownPos[0]),
		"the surprise claim must never precede admission")
	assert.Less(t, int(cooldownPos[0]), int(resolve[0]))
	assert.Less(t, int(admit[0]), int(unloads[0]))
	assert.Less(t, int(unloads[0]), int(resolve[0]))
	assert.Less(t, int(admit[0]), int(round[0]))
}

// TestFireReload_MidSkillEvidenceCycle catches either action using the wrong
// registry key/base, bypassing fractional carry, or charging more than once.
// The fixture is Task 1's novice actor at exactly 50% load: the independent
// evidence quotes are 2.88 shoot + 1.44 reload, one four-point whole debit.
func TestFireReload_NoviceEvidenceCycle(t *testing.T) {
	pinContestFloorOff(t)
	cfg := configs.GetConfig()
	cfg.Balance.ShootBaseStaminaCost = 2
	cfg.Balance.ReloadBaseStaminaCost = 1
	cfg.Balance.CarryCapacityMultiplier = 0.65
	cfg.Balance.CostEncumbranceKnee = 0.75
	cfg.Balance.CostEncumbranceKneeMult = 1.5
	cfg.Balance.CostSkillMidRank = 25
	cfg.Balance.CostSkillMultAtMid = 1
	configs.SetConfigForTest(t, cfg)

	_, cleanup := seedFireMobInRoom(t, 1, 1_000_000)
	defer cleanup()

	newFixture := func() *characters.Character {
		char := fireAttacker()
		char.Stats.Perception.ValueAdj = 1
		char.Stats.Strength.ValueAdj = 100
		char.Skills[string(skills.RangedCombat)] = 5
		char.Stamina = 50
		char.StaminaMax.Value = 405
		char.Equipment.Weapon = fireRangedWeapon(1, 1, true)
		char.Items = []items.Item{
			{ItemId: 9, Spec: &items.ItemSpec{ItemId: 9, Name: "ballast", Weight: 32.5}},
			reloadAmmoBundle(3, "arrows", 20),
		}
		char.Aggro = &characters.Aggro{MobInstanceId: 500}
		return char
	}

	// Independent shared-contract quotes establish the expected whole debit.
	quoted := newFixture()
	shootQuote := quoted.QuoteActionCost(characters.ActionCostRequest{
		Action: costs.ActionShoot, Pool: characters.PoolStamina, Base: 2, Modifier: 1, Units: 1,
	})
	shootCost := quoted.CommitCost(shootQuote, characters.CostFullOrRefuse)
	reloadQuote := quoted.QuoteActionCost(characters.ActionCostRequest{
		Action: costs.ActionReload, Pool: characters.PoolStamina, Base: 1, Modifier: 1, Units: 1,
	})
	reloadCost := quoted.CommitCost(reloadQuote, characters.CostFullOrRefuse)
	require.Equal(t, 4, shootCost.Charged+reloadCost.Charged)

	char := newFixture()
	actor := newStubActor(char, rooms.LoadRoom(1))
	fire := ExecuteFire(actor, "skeleton")
	reload := ExecuteReload(actor)

	require.True(t, fire.Executed)
	require.True(t, reload.Loaded)
	assert.Equal(t, shootCost, fire.Cost)
	assert.Equal(t, reloadCost, reload.Cost)
	assert.Equal(t, 46, char.Stamina)
	assert.Equal(t, 19, char.Items[1].Uses)
	assert.True(t, char.Equipment.Weapon.Loaded)
	shootRaw := costs.Calc(costs.Input{
		Base: 2, Carried: 32.5, Capacity: 65, Physical: true, SkillRank: 5, HasSkill: true, Modifier: 1,
	})
	reloadRaw := costs.Calc(costs.Input{
		Base: 1, Carried: 32.5, Capacity: 65, Physical: true, SkillRank: 5, HasSkill: true, Modifier: 1,
	})
	assert.Equal(t, 2.88, math.Round(shootRaw*100)/100)
	assert.Equal(t, 1.44, math.Round(reloadRaw*100)/100)
}

// ---------------------------------------------------------------------------
// 7. Charmed mob target → IsCharmed, not executed, weapon stays loaded
// ---------------------------------------------------------------------------

func TestFire_CharmedMob_NotExecuted(t *testing.T) {
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	// stubActor charms by key 0 (GetUserId()==0, GetMobInstanceId()==0).
	mobs.GetInstance(500).Character.Charm(0, 10, "")

	char := fireAttacker()
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
	actor := newStubActor(char, rooms.LoadRoom(1))

	res := ExecuteFire(actor, "skeleton")

	assert.True(t, res.IsCharmed, "expected IsCharmed for a charmed mob")
	assert.False(t, res.Executed, "must not fire on a charmed mob")
	assert.True(t, char.Equipment.Weapon.Loaded, "weapon must stay loaded — no shot wasted")
}

// ---------------------------------------------------------------------------
// 8. Non-combatant mob target → IsNonCombatant, weapon stays loaded
// ---------------------------------------------------------------------------

func TestFire_NonCombatantMob_NotExecuted(t *testing.T) {
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	mobs.GetInstance(500).Character.NonCombatant = true

	char := fireAttacker()
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
	actor := newStubActor(char, rooms.LoadRoom(1))

	res := ExecuteFire(actor, "skeleton")

	assert.True(t, res.IsNonCombatant, "expected IsNonCombatant for a non_combatant mob")
	assert.False(t, res.Executed, "must not fire on a non-combatant mob")
	assert.True(t, char.Equipment.Weapon.Loaded, "weapon must stay loaded — no shot wasted")
}

// ---------------------------------------------------------------------------
// Balance pin: arbalest baseline raw damage band
// ---------------------------------------------------------------------------

// Spec balance target: arbalest (mult 7.0) at stat 100 / rank 0 must produce
// raw damage in the 180-220 band BEFORE mitigation:
// 100 × SkillMultiplier(0)=1.0 × 7.0 × ChannelScale(0.30) = 210.
func TestRangedShotRawDamage_BalanceBand(t *testing.T) {
	raw := combat.CalcRawDamage(100, 0, 7.0, combat.ChannelPhysical)
	if raw < 180 || raw > 220 {
		t.Errorf("arbalest baseline raw %v outside 180-220 spec band", raw)
	}
}

// TestFire_DarkRoomIsRejectedAsLightingNotAsMissingTarget covers the case the
// unseen-target matrix above does not: the shooter can see fine, but the room
// is unlit.
//
// This gate was introduced with U8 and originally shared NoTarget with the
// hidden-target case, so the player was told "Could not find your target." for
// a target standing in front of them, in every unlit room in the world. Melee
// has no equivalent gate, so `attack skeleton` worked in the same room where
// `shoot skeleton` claimed the skeleton was not there.
func TestFire_DarkRoomIsRejectedAsLightingNotAsMissingTarget(t *testing.T) {
	_, cleanup := seedFireMobInRoom(t, 1, 1)
	defer cleanup()

	// DarkArea without LitArea drives GetVisibility() to 0, which is what
	// messaging.CanSeeClearly reads.
	biomeCleanup := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", Name: "Cave", Symbol: ".", DarkArea: true, MovementCost: 1},
	})
	defer biomeCleanup()
	rooms.LoadRoom(1).Biome = "cave"

	char := fireAttacker()
	char.Stamina = 10
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
	actor := newStubActor(char, rooms.LoadRoom(1))

	res := ExecuteFire(actor, "skeleton")

	require.True(t, res.TooDarkToAim, "an unlit room is a lighting refusal")
	assert.False(t, res.NoTarget, "the target is present and correctly named")
	assert.False(t, res.Blinded, "the shooter's eyes are fine")
	assert.False(t, res.Executed)
	// Refused before admission: nothing is spent and nothing is committed.
	assert.Equal(t, characters.CostCommitResult{}, res.Cost)
	assert.Equal(t, 10, char.Stamina)
	assert.True(t, char.Equipment.Weapon.Loaded)
	assert.Nil(t, char.Aggro)
}
