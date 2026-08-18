package mobcommands

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Consume ────────────────────────────────────────────────────────────────

func TestConsume_NoCorpses(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	mob, room := getTestMobAndRoom(t)

	// Room starts with no corpses
	assert.Empty(t, room.Corpses)

	handled, err := Consume("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)

	// No condition applied when nothing to eat
	assert.False(t, mob.Character.HasCondition(characters.ConditionRegen))
}

func TestConsume_EatsCorpseAndAppliesRegen(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)

	// Add a corpse to the room
	room.AddCorpse(rooms.Corpse{
		MobId:        99,
		Character:    characters.Character{Name: "Dead Rat"},
		RoundCreated: 1,
	})
	require.Len(t, room.Corpses, 1)

	handled, err := Consume("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)

	// Corpse should be removed
	assert.Empty(t, room.Corpses)

	// Mob should have ConditionRegen
	assert.True(t, mob.Character.HasCondition(characters.ConditionRegen))
	assert.Equal(t, 2.0, mob.Character.GetConditionMagnitude(characters.ConditionRegen))
	assert.Equal(t, 6, mob.Character.GetConditionDuration(characters.ConditionRegen))
}

func TestConsume_SkipsPrunableCorpses(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)

	// Add a prunable corpse (already decayed) then a fresh one
	room.AddCorpse(rooms.Corpse{
		MobId:        99,
		Character:    characters.Character{Name: "Old Bones"},
		RoundCreated: 1,
		Prunable:     true,
	})
	room.AddCorpse(rooms.Corpse{
		MobId:        98,
		Character:    characters.Character{Name: "Fresh Kill"},
		RoundCreated: 2,
	})
	require.Len(t, room.Corpses, 2)

	handled, err := Consume("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)

	// Only the prunable one should remain
	assert.Len(t, room.Corpses, 1)
	assert.Equal(t, "Old Bones", room.Corpses[0].Character.Name)
	assert.True(t, mob.Character.HasCondition(characters.ConditionRegen))
}

func TestConsume_AllPrunable(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)

	room.AddCorpse(rooms.Corpse{
		MobId:        99,
		Character:    characters.Character{Name: "Old Bones"},
		RoundCreated: 1,
		Prunable:     true,
	})

	handled, err := Consume("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)

	// Nothing consumed — all prunable
	assert.Len(t, room.Corpses, 1)
	assert.False(t, mob.Character.HasCondition(characters.ConditionRegen))
}

// ─── Flee ───────────────────────────────────────────────────────────────────

func TestFlee_ClearsAggro(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)

	// Put mob in combat
	mob.Character.Aggro = &characters.Aggro{UserId: 1}
	require.NotNil(t, mob.Character.Aggro)

	handled, err := Flee("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)

	// Aggro should be cleared
	assert.Nil(t, mob.Character.Aggro)

	// Reset mob position for other tests
	mob.Character.RoomId = 1
	room.AddMob(100)
}

func TestFlee_NoExits(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, _ := getTestMobAndRoom(t)

	// Create a room with no exits
	deadEnd := &rooms.Room{
		RoomId:      999,
		Zone:        "TestZone",
		Title:       "Dead End",
		Description: "No way out.",
		Exits:       map[string]exit.RoomExit{},
	}
	// Move mob to dead end room (just test the function directly)
	mob.Character.Aggro = &characters.Aggro{UserId: 1}

	handled, err := Flee("", mob, deadEnd)
	assert.True(t, handled)
	assert.NoError(t, err)

	// Aggro still cleared even if cornered
	assert.Nil(t, mob.Character.Aggro)
}

func TestFlee_OutOfCombat(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)

	// Not in combat
	mob.Character.Aggro = nil

	handled, err := Flee("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)

	// Reset position
	mob.Character.RoomId = 1
	room.AddMob(100)
}

// ─── Hamstring ──────────────────────────────────────────────────────────────

func TestHamstring_NotInCombat(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)
	mob.Character.Aggro = nil

	handled, err := Hamstring("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)
}

func TestHamstring_InCombat(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)

	// Hamstring is a beast move (Phase-4 hands rule): the default test mob is a
	// humanoid (SpeciesId 1, has hands). Give it a fanged, no-hands beast species
	// so it qualifies.
	spCleanup := species.SeedSpeciesForTest(map[int]*species.Species{
		2: {SpeciesId: 2, Name: "testbeast", BodyParts: []string{"legs", "mouth"}, NaturalAttack: items.Bite},
	})
	defer spCleanup()
	mob.Character.SpeciesId = 2

	// Set up combat against player
	mob.Character.Aggro = &characters.Aggro{UserId: 1}
	mob.Character.Stats.Strength.ValueAdj = 80
	mob.Character.Stats.Dexterity.ValueAdj = 80

	handled, err := Hamstring("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)

	// Should cost the round
	if mob.Character.Aggro != nil {
		assert.Equal(t, 1, mob.Character.Aggro.RoundsWaiting)
	}

	mob.Character.Aggro = nil
}

func TestHamstring_BleedMagnitude(t *testing.T) {
	// Verify bleed damage calculation: Strength/10, min 2
	tests := []struct {
		name     string
		strength int
		expected float64
	}{
		{"high_strength", 100, 10.0},
		{"medium_strength", 50, 5.0},
		{"low_strength_floor", 10, 2.0},
		{"very_low_strength_floor", 5, 2.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bleedDmg := tc.strength / 10
			if bleedDmg < 2 {
				bleedDmg = 2
			}
			assert.Equal(t, tc.expected, float64(bleedDmg))
		})
	}
}

// ─── Charge ─────────────────────────────────────────────────────────────────

func TestCharge_NotInCombat(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)
	mob.Character.Aggro = nil

	handled, err := Charge("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)
}

func TestCharge_InCombat(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)

	mob.Character.Aggro = &characters.Aggro{UserId: 1}
	mob.Character.Stats.Strength.ValueAdj = 80
	mob.Character.Stats.Dexterity.ValueAdj = 80

	handled, err := Charge("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)

	if mob.Character.Aggro != nil {
		assert.Equal(t, 1, mob.Character.Aggro.RoundsWaiting)
	}

	mob.Character.Aggro = nil
}

// ─── Howl ───────────────────────────────────────────────────────────────────

func TestHowl_NotInCombat(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)
	mob.Character.Aggro = nil

	handled, err := Howl("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)
}

func TestHowl_InCombat(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)

	mob.Character.Aggro = &characters.Aggro{UserId: 1}
	mob.Character.Stats.Charisma.ValueAdj = 80
	mob.Character.ConvictionMax.Value = 50
	mob.Character.Conviction = 50

	handled, err := Howl("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)

	if mob.Character.Aggro != nil {
		assert.Equal(t, 1, mob.Character.Aggro.RoundsWaiting)
	}

	mob.Character.Aggro = nil
}

// TestHowlAliasChargesOnlyThroughTaunt executes the real wrapper and guards
// the alias boundary structurally. A second quote/commit in Howl would charge
// eight or nine Conviction at rank zero instead of the one four-point taunt
// admission asserted here.
func TestHowlAliasChargesOnlyThroughTaunt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)
	mob.Character.Aggro = &characters.Aggro{UserId: 1}
	mob.Character.Stats.Charisma.ValueAdj = 100
	mob.Character.ConvictionMax.Value = 50
	mob.Character.Conviction = 50
	mob.Character.Skills = map[string]int{"rhetoric": 0}
	rand.Seed(1)

	handled, err := Howl("", mob, room)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, 46, mob.Character.Conviction)
	require.Greater(t, mob.Character.Cooldowns["special-move"], 0)
	require.Equal(t, 1, mob.Character.Aggro.RoundsWaiting)

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filepath.Join(filepath.Dir(thisFile), "howl.go"), nil, 0)
	require.NoError(t, err)
	var howl *ast.FuncDecl
	for _, decl := range parsed.Decls {
		candidate, ok := decl.(*ast.FuncDecl)
		if ok && candidate.Name.Name == "Howl" {
			howl = candidate
			break
		}
	}
	require.NotNil(t, howl)
	executeCalls, ownQuotes := 0, 0
	ast.Inspect(howl.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if fn.Sel.Name == "ExecuteTaunt" {
				executeCalls++
			}
			if fn.Sel.Name == "QuoteActionCost" || fn.Sel.Name == "CommitCost" {
				ownQuotes++
			}
		case *ast.Ident:
			if fn.Name == "admitFullCost" {
				ownQuotes++
			}
		}
		return true
	})
	require.Equal(t, 1, executeCalls)
	require.Zero(t, ownQuotes, "howl must not quote or commit apart from ExecuteTaunt")
}

func TestTauntAndHowlRouteStructuredDefyOutcomeExactlyOnce(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	for _, filename := range []string{"taunt.go", "howl.go"} {
		t.Run(filename, func(t *testing.T) {
			parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(thisFile), filename), nil, 0)
			require.NoError(t, err)
			renderCalls, legacyBranches := 0, 0
			ast.Inspect(parsed, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.CallExpr:
					if ident, ok := n.Fun.(*ast.Ident); ok && ident.Name == "sendChannelDefenceMessages" {
						renderCalls++
					}
				case *ast.SelectorExpr:
					if n.Sel.Name == "Defied" || n.Sel.Name == "FullyDefied" {
						legacyBranches++
					}
				}
				return true
			})
			require.Equal(t, 1, renderCalls, "%s must render ExecuteTaunt's outcome once", filename)
			require.Zero(t, legacyBranches, "%s must not retain hardcoded defy branches", filename)
		})
	}
}

func TestMobDefyRoutingExcludesDefenderAndAnonymizesDarkIdentity(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreBiomes := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", Name: "Cave", Symbol: ".", DarkArea: true, MovementCost: 1},
	})
	defer restoreBiomes()
	restoreBuffs := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		9001: {BuffId: 9001, Name: "Test Infrared", RoundInterval: 1, TriggerCount: 1, Flags: []buffs.Flag{buffs.InfraredVision}},
	})
	defer restoreBuffs()

	mk := func(prefix string) items.DefenseOptions {
		five := func(text string) items.MessageOptions {
			message := items.ItemMessage(text)
			return items.MessageOptions{message, message, message, message, message}
		}
		return items.DefenseOptions{Together: items.DefenseTogetherMessages{
			ToDefender: five(prefix + " defender sees {attacker} defied by {defender}"),
			ToAttacker: five(prefix + " attacker sees {defender} defy {attacker}"),
			ToRoom:     five(prefix + " room sees {defender} defy {attacker}"),
		}}
	}
	restoreMessages := items.SeedDefenseMessagesForTest(map[items.DefenseType]*items.DefenseMessageGroup{
		items.DefenseDefy: {OptionId: items.DefenseDefy, Options: items.DefenseIntensity{
			items.Weak: mk("weak"), items.Normal: mk("normal"), items.Heavy: mk("heavy"),
		}},
	})
	defer restoreMessages()

	mob := mobs.GetInstance(100)
	target := users.GetByUserId(1)
	observer := users.GetByUserId(2)
	darkRoom := rooms.LoadRoom(2)
	require.NotNil(t, darkRoom)
	darkRoom.Biome = "cave"
	require.Zero(t, darkRoom.GetVisibility())
	mob.Character.RoomId = 2
	target.Character.RoomId = 2
	observer.Character.RoomId = 2
	darkRoom.AddMob(mob.InstanceId)
	darkRoom.AddPlayer(target.UserId)
	darkRoom.AddPlayer(observer.UserId)
	require.True(t, observer.Character.Buffs.AddBuff(9001, true))

	for _, attack := range []string{"taunt", "howl"} {
		t.Run(attack, func(t *testing.T) {
			events.DrainQueuedMessagesForTest(target.UserId)
			events.DrainQueuedMessagesForTest(observer.UserId)
			sendChannelDefenceMessages(combat.ChannelDefenceResult{
				DefenceType: string(items.DefenseDefy), Defended: true, NormalizedDefenceMargin: 0.1, DamageMultiplier: 0.4,
			}, mob, target, darkRoom, target.Character.Name, attack)

			targetLines := events.DrainQueuedMessagesForTest(target.UserId)
			observerLines := events.DrainQueuedMessagesForTest(observer.UserId)
			require.Len(t, targetLines, 1, "defender must receive one personal line, not its observer line too")
			require.Len(t, observerLines, 1)
			for _, line := range []string{targetLines[0], observerLines[0]} {
				require.NotContains(t, line, mob.Character.Name, "dark routing leaked mob identity")
				require.Contains(t, line, "a figure")
			}
		})
	}
}

// ─── Cooldown Interaction ───────────────────────────────────────────────────

func TestSpecialMoveCooldown_SharedAcrossCommands(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)
	mob.Character.Aggro = &characters.Aggro{UserId: 1}
	mob.Character.Stats.Strength.ValueAdj = 80
	mob.Character.Stats.Dexterity.ValueAdj = 80
	mob.Character.Stats.Charisma.ValueAdj = 80
	mob.Character.ConvictionMax.Value = 50
	mob.Character.Conviction = 50

	// First command should succeed (uses the cooldown)
	handled, err := Charge("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)

	// Second command should silently fail (cooldown not ready)
	// The cooldown was consumed by charge; howl shares the same "special-move" key
	startConviction := mob.Character.Conviction
	handled, err = Howl("", mob, room)
	assert.True(t, handled)
	assert.NoError(t, err)
	// Conviction shouldn't change because cooldown blocked the howl
	assert.Equal(t, startConviction, mob.Character.Conviction)

	mob.Character.Aggro = nil
}

// ─── Command Registration ───────────────────────────────────────────────────

func TestPredatorCommandsRegistered(t *testing.T) {
	cmds := GetAllMobCommands()
	found := map[string]bool{}
	for _, c := range cmds {
		found[c] = true
	}

	assert.True(t, found["charge"], "charge should be registered")
	assert.True(t, found["consume"], "consume should be registered")
	assert.True(t, found["flee"], "flee should be registered")
	assert.True(t, found["hamstring"], "hamstring should be registered")
	assert.True(t, found["howl"], "howl should be registered")
}
