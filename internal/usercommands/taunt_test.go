package usercommands

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func seedTauntRuntimeMessages(t *testing.T) func() {
	t.Helper()
	mk := func(band string) items.DefenseOptions {
		messages := func(audience string) items.MessageOptions {
			result := make(items.MessageOptions, 5)
			for i := range result {
				result[i] = items.ItemMessage(fmt.Sprintf("DEFY %s variant=%d %s: {attacker} tests {defender}; no conviction harm, attention may shift", audience, i, band))
			}
			return result
		}
		return items.DefenseOptions{Together: items.DefenseTogetherMessages{
			ToDefender: messages("defender"), ToAttacker: messages("attacker"), ToRoom: messages("room"),
		}}
	}
	return items.SeedDefenseMessagesForTest(map[items.DefenseType]*items.DefenseMessageGroup{
		items.DefenseDefy: {
			OptionId: items.DefenseDefy,
			Options: items.DefenseIntensity{
				items.Weak: mk("weak"), items.Normal: mk("normal"), items.Heavy: mk("heavy"),
			},
		},
	})
}

func defyRuntimeLines(userID int) []string {
	all := events.DrainQueuedMessagesForTest(userID)
	result := make([]string, 0, len(all))
	for _, line := range all {
		if strings.Contains(strings.ToLower(line), "defy ") {
			result = append(result, line)
		}
	}
	return result
}

const expectedDefenceShortageLine = "You mount a desperate response, too spent to bring practiced technique to it."

func exactRuntimeLineCount(userID int, want string) int {
	count := 0
	for _, line := range events.DrainQueuedMessagesForTest(userID) {
		if strings.Contains(line, want) {
			count++
		}
	}
	return count
}

func defyVariant(t *testing.T, line string) string {
	t.Helper()
	lower := strings.ToLower(line)
	start := strings.Index(lower, "variant=")
	require.NotEqual(t, -1, start, line)
	start += len("variant=")
	end := start
	for end < len(lower) && lower[end] >= '0' && lower[end] <= '9' {
		end++
	}
	return lower[start:end]
}

// This structural boundary test catches a wrapper that restores hardcoded
// partial/full branches or renders the structured defy outcome more than once.
func TestTauntRoutesStructuredDefyOutcomeExactlyOnce(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(thisFile), "taunt.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var actionCalls, renderCalls, legacyBranches int
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			if ident, ok := n.Fun.(*ast.Ident); ok {
				switch ident.Name {
				case "executeTauntAction":
					actionCalls++
				case "sendChannelDefenceMessages":
					renderCalls++
				}
			}
		case *ast.SelectorExpr:
			if n.Sel.Name == "Defied" || n.Sel.Name == "FullyDefied" {
				legacyBranches++
			}
		}
		return true
	})
	if actionCalls != 1 {
		t.Fatalf("Taunt executeTauntAction calls = %d, want exactly 1", actionCalls)
	}
	if renderCalls != 1 {
		t.Fatalf("Taunt sendChannelDefenceMessages calls = %d, want exactly 1", renderCalls)
	}
	if legacyBranches != 0 {
		t.Fatalf("Taunt retains %d legacy Defied/FullyDefied branches", legacyBranches)
	}
}

func TestPlayerTauntRuntimeRoutesCanonicalDefyAcrossRealWrappers(t *testing.T) {
	t.Run("player_to_player_shared_triad", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		restoreMessages := seedTauntRuntimeMessages(t)
		defer restoreMessages()
		attacker := users.GetByUserId(1)
		defender := users.GetByUserId(2)
		called := false
		originalAction := executeTauntAction
		executeTauntAction = func(actions.Actor) actions.TauntResult {
			called = true
			return actions.TauntResult{
				Executed: true, Hit: true,
				Target:  actions.AggroTarget{Char: defender.Character, Name: defender.Character.Name, UserId: defender.UserId, Found: true},
				Defence: combat.ChannelDefenceResult{DefenceType: characters.DefenseDefy, Defended: true, DefensiveCrit: true, DamageMultiplier: 0},
			}
		}
		t.Cleanup(func() { executeTauntAction = originalAction })
		observer := users.NewTestUser(3, "orin", "Orin", 1003)
		observer.Character.RoomId = 1
		restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: attacker, 2: defender, 3: observer})
		defer restoreUsers()
		room := rooms.LoadRoom(1)
		room.AddPlayer(observer.UserId)
		for _, id := range []int{1, 2, 3} {
			events.DrainQueuedMessagesForTest(id)
		}

		handled, err := Taunt("bobrick", attacker, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		require.True(t, called, "named target acquisition must reach the action seam")
		attackerLines, defenderLines, roomLines := defyRuntimeLines(1), defyRuntimeLines(2), defyRuntimeLines(3)
		require.Len(t, attackerLines, 1)
		require.Len(t, defenderLines, 1)
		require.Len(t, roomLines, 1)
		require.Contains(t, strings.ToLower(attackerLines[0]), "defy attacker")
		require.Contains(t, strings.ToLower(defenderLines[0]), "defy defender")
		require.Contains(t, strings.ToLower(roomLines[0]), "defy room")
		require.Equal(t, defyVariant(t, attackerLines[0]), defyVariant(t, defenderLines[0]))
		require.Equal(t, defyVariant(t, attackerLines[0]), defyVariant(t, roomLines[0]))
	})

	t.Run("player_to_indexed_mob_preserves_aggro_pull", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		restoreMessages := seedTauntRuntimeMessages(t)
		defer restoreMessages()
		attacker := users.GetByUserId(1)
		observer := users.GetByUserId(2)
		room := rooms.LoadRoom(1)
		target := mobs.GetInstance(100)
		target.Character.MobInstanceId = target.InstanceId
		called := false
		originalAction := executeTauntAction
		executeTauntAction = func(actions.Actor) actions.TauntResult {
			called = true
			return actions.TauntResult{
				Executed: true, Hit: true,
				Target:  actions.AggroTarget{Char: &target.Character, Name: target.Character.Name, MobInstanceId: target.InstanceId, Found: true},
				Defence: combat.ChannelDefenceResult{DefenceType: characters.DefenseDefy, Defended: true, DefensiveCrit: true, DamageMultiplier: 0},
			}
		}
		t.Cleanup(func() { executeTauntAction = originalAction })
		events.DrainQueuedMessagesForTest(attacker.UserId)
		events.DrainQueuedMessagesForTest(observer.UserId)

		handled, err := Taunt("skeleton", attacker, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		require.True(t, called, "named mob acquisition must reach the action seam")
		attackerLines, roomLines := defyRuntimeLines(attacker.UserId), defyRuntimeLines(observer.UserId)
		require.Len(t, attackerLines, 1)
		require.Len(t, roomLines, 1)
		require.Equal(t, defyVariant(t, attackerLines[0]), defyVariant(t, roomLines[0]))
		require.Contains(t, strings.ToLower(attackerLines[0]), "attention may shift")
	})

	t.Run("attack_win_has_no_defy_narration", func(t *testing.T) {
		cleanup := seedAllRegistries()
		defer cleanup()
		restoreMessages := seedTauntRuntimeMessages(t)
		defer restoreMessages()
		attacker := users.GetByUserId(1)
		observer := users.GetByUserId(2)
		room := rooms.LoadRoom(1)
		target := mobs.GetInstance(100)
		originalAction := executeTauntAction
		executeTauntAction = func(actions.Actor) actions.TauntResult {
			return actions.TauntResult{
				Executed: true, Hit: true,
				Target:  actions.AggroTarget{Char: &target.Character, Name: target.Character.Name, MobInstanceId: target.InstanceId, Found: true},
				Defence: combat.ChannelDefenceResult{DamageMultiplier: 1},
			}
		}
		t.Cleanup(func() { executeTauntAction = originalAction })
		events.DrainQueuedMessagesForTest(attacker.UserId)
		events.DrainQueuedMessagesForTest(observer.UserId)
		handled, err := Taunt("skeleton", attacker, room, 0)
		require.NoError(t, err)
		require.True(t, handled)
		require.Empty(t, defyRuntimeLines(attacker.UserId))
		require.Empty(t, defyRuntimeLines(observer.UserId))
	})
}

func TestPlayerTauntShortDefyIsPrivateAndExactlyOnce(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedTauntRuntimeMessages(t)
	defer restoreMessages()
	attacker := users.GetByUserId(1)
	defender := users.GetByUserId(2)
	observer := users.NewTestUser(3, "orin", "Orin", 1003)
	observer.Character.RoomId = 1
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{1: attacker, 2: defender, 3: observer})
	defer restoreUsers()
	room := rooms.LoadRoom(1)
	room.AddPlayer(observer.UserId)
	originalAction := executeTauntAction
	executeTauntAction = func(actions.Actor) actions.TauntResult {
		return actions.TauntResult{
			Executed: true, Hit: true,
			Target: actions.AggroTarget{Char: defender.Character, Name: defender.Character.Name,
				UserId: defender.UserId, Found: true},
			Defence: combat.ChannelDefenceResult{
				DefenceType: characters.DefenseDefy, Defended: true, DamageMultiplier: 0.3,
				Cost: characters.CostCommitResult{Status: characters.CostPartiallyPaid, Pool: characters.PoolConviction},
			},
		}
	}
	t.Cleanup(func() { executeTauntAction = originalAction })
	for _, id := range []int{attacker.UserId, defender.UserId, observer.UserId} {
		events.DrainQueuedMessagesForTest(id)
	}

	handled, err := Taunt("bobrick", attacker, room, 0)
	require.NoError(t, err)
	require.True(t, handled)
	require.Zero(t, exactRuntimeLineCount(attacker.UserId, expectedDefenceShortageLine))
	require.Equal(t, 1, exactRuntimeLineCount(defender.UserId, expectedDefenceShortageLine))
	require.Zero(t, exactRuntimeLineCount(observer.UserId, expectedDefenceShortageLine))
}

func TestPlayerTauntDefenceShortageSilenceCases(t *testing.T) {
	for _, tc := range []struct {
		name string
		out  combat.ChannelDefenceResult
	}{
		{name: "attack_win", out: combat.ChannelDefenceResult{
			DefenceType: characters.DefenseDefy, DamageMultiplier: 1,
			Cost: characters.CostCommitResult{Status: characters.CostPartiallyPaid, Pool: characters.PoolConviction},
		}},
		{name: "affordable_defence", out: combat.ChannelDefenceResult{
			DefenceType: characters.DefenseDefy, Defended: true, DamageMultiplier: 0.3,
			Cost: characters.CostCommitResult{Status: characters.CostPaid, Pool: characters.PoolConviction},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			restoreMessages := seedTauntRuntimeMessages(t)
			defer restoreMessages()
			attacker, defender := users.GetByUserId(1), users.GetByUserId(2)
			room := rooms.LoadRoom(1)
			originalAction := executeTauntAction
			executeTauntAction = func(actions.Actor) actions.TauntResult {
				return actions.TauntResult{Executed: true, Hit: true,
					Target: actions.AggroTarget{Char: defender.Character, Name: defender.Character.Name,
						UserId: defender.UserId, Found: true}, Defence: tc.out}
			}
			t.Cleanup(func() { executeTauntAction = originalAction })
			events.DrainQueuedMessagesForTest(attacker.UserId)
			events.DrainQueuedMessagesForTest(defender.UserId)
			_, err := Taunt("bobrick", attacker, room, 0)
			require.NoError(t, err)
			require.Zero(t, exactRuntimeLineCount(defender.UserId, expectedDefenceShortageLine))
		})
	}
}
