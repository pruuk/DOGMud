package usercommands

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/position"
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
		attackerLines := events.DrainQueuedMessagesForTest(1)
		defenderLines := events.DrainQueuedMessagesForTest(2)
		roomLines := events.DrainQueuedMessagesForTest(3)
		require.Len(t, attackerLines, 1, "defended taunt must replace the generic attacker hit line")
		require.Len(t, defenderLines, 1, "defended taunt must replace the generic defender hit line")
		require.Len(t, roomLines, 1, "defended taunt must replace the generic room hit line")
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
		attackerLines := events.DrainQueuedMessagesForTest(attacker.UserId)
		roomLines := events.DrainQueuedMessagesForTest(observer.UserId)
		require.Len(t, attackerLines, 1, "defended taunt must replace the generic attacker hit line")
		require.Len(t, roomLines, 1, "defended taunt must replace the generic room hit line")
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

// deterministicTauntContest returns a channel-contest runner whose normalized
// attack margin is exactly normMargin sigma (attack-positive; negative is a
// defence win by that many sigma), with explicit roll z-scores so fumbles and
// defence crits fire only when asked for.
func deterministicTauntContest(normMargin, atkZ, defZ float64) func(float64, []contest.Entry) contest.Result {
	return func(atkScore float64, entries []contest.Entry) contest.Result {
		stdDev := dice.StdDevFor(atkScore)
		if stdDev <= 0 {
			stdDev = 15
		}
		margin := normMargin * stdDev * math.Sqrt2
		result := contest.Result{
			Contested: true,
			Success:   margin > 0,
			Margin:    margin,
			AttackRoll: dice.RollResult{
				Value: atkScore + atkZ*stdDev, Mean: atkScore,
				StdDev: stdDev, ZScore: atkZ,
			},
		}
		if len(entries) > 0 {
			result.Winner = entries[0].Name
			result.DefenseRoll = dice.RollResult{
				Value: atkScore + atkZ*stdDev - margin, Mean: entries[0].Score,
				StdDev: stdDev, ZScore: defZ,
			}
		}
		return result
	}
}

// TestPlayerTauntMidCombatAndGrappledAlwaysNarrates closes the U6b playtest
// cell the live sessions could not observe (every eligible mid-combat target
// died within a round or hid): the REAL actions.ExecuteTaunt — no stubbed
// action seam — driven through the player wrapper while already in combat,
// and again while grappled, must put at least one line in front of the
// player for EVERY contest outcome. The events pump swallows listener panics
// in production (internal/events/listeners.go), so a panicking taunt path
// would be silent live; this test calls the wrapper directly, so any panic
// fails it loudly.
func TestPlayerTauntMidCombatAndGrappledAlwaysNarrates(t *testing.T) {
	outcomes := []struct {
		name                   string
		normMargin, atkZ, defZ float64
		wantSubstring          string
	}{
		{"attack_win", 0.5, 0.5, -0.5, "taunt"},
		{"defended_partial", -0.5, 0.5, 0.5, "defy attacker"},
		{"defensive_crit", -4, 0.5, 2.5, "defy attacker"},
		{"fumble", -1, -2.5, 0.5, ""},
	}
	for _, grappled := range []bool{false, true} {
		stance := "mid_combat_standing"
		if grappled {
			stance = "mid_combat_grappled"
		}
		for _, tc := range outcomes {
			t.Run(stance+"/"+tc.name, func(t *testing.T) {
				cleanup := seedAllRegistries()
				defer cleanup()
				restoreMessages := seedTauntRuntimeMessages(t)
				defer restoreMessages()
				cfg := configs.GetConfig()
				cfg.Balance.ContestFloor = 0
				cfg.Balance.MinAttackCritChance = 0
				cfg.Balance.MinDefenseCritChance = 0
				configs.SetConfigForTest(t, cfg)
				restoreContest := combat.SetChannelAttackContestRunnerForTest(
					deterministicTauntContest(tc.normMargin, tc.atkZ, tc.defZ))
				t.Cleanup(restoreContest)

				attacker := users.GetByUserId(1)
				room := rooms.LoadRoom(1)
				target := mobs.GetInstance(100)
				require.NotNil(t, target)
				target.Character.MobInstanceId = target.InstanceId

				// Mid-combat: aggro already points at the live, visible mob,
				// exactly the state the playtest could never hold long enough
				// to observe.
				attacker.Character.SetAggro(0, target.InstanceId, characters.DefaultAttack)
				if grappled {
					setCombatPositionParallel(attacker.Character, position.Clinch)
				}
				events.DrainQueuedMessagesForTest(attacker.UserId)

				handled, err := Taunt("", attacker, room, 0)
				require.NoError(t, err)
				require.True(t, handled)

				lines := events.DrainQueuedMessagesForTest(attacker.UserId)
				require.NotEmpty(t, lines,
					"a %s taunt (%s) left the player with ZERO output — the silent-taunt defect",
					tc.name, stance)
				if tc.wantSubstring != "" {
					joined := strings.ToLower(strings.Join(lines, "\n"))
					require.Contains(t, joined, tc.wantSubstring,
						"the %s outcome (%s) must narrate its result to the taunter", tc.name, stance)
				}
			})
		}
	}
}
