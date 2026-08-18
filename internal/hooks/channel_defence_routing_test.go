package hooks

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestChannelDefenceRoutingTestsDoNotLoadGlobalItemRegistries(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	parsed, err := parser.ParseFile(token.NewFileSet(), here, nil, 0)
	require.NoError(t, err)
	loadCalls := 0
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "LoadDataFiles" {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "items" {
			loadCalls++
		}
		return true
	})
	require.Zero(t, loadCalls, "hooks tests must not replace the global item and attack-message registries")
}

func TestPlayerSpellKnockdownUsesDeterministicDefenceSeam(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(here), "spell_resolution.go"), nil, 0)
	require.NoError(t, err)
	var knockdown *ast.FuncDecl
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "applyMobEffect_knockdown" {
			knockdown = fn
			break
		}
	}
	require.NotNil(t, knockdown)
	seamCalls, directCalls := 0, 0
	ast.Inspect(knockdown.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			if fn.Name == "runPlayerSpellDefence" {
				seamCalls++
			}
		case *ast.SelectorExpr:
			if pkg, ok := fn.X.(*ast.Ident); ok && pkg.Name == "combat" && fn.Sel.Name == "ResolveChannelDefence" {
				directCalls++
			}
		}
		return true
	})
	require.Equal(t, 1, seamCalls, "player knockdown dispatch must resolve defence through its package seam once")
	require.Zero(t, directCalls, "direct canonical resolver calls make dispatch tests stochastic")
}

func seedChannelRoutingMessages(t *testing.T) func() {
	t.Helper()
	mk := func(band string) items.DefenseOptions {
		messages := func(audience string) items.MessageOptions {
			result := make(items.MessageOptions, 5)
			truth := ""
			if band == "heavy" {
				truth = " no damage taken; momentum remains"
			}
			for i := range result {
				result[i] = items.ItemMessage(fmt.Sprintf("%s-%s-index=%d%s {attacker} resists-with {defender} via {attack}", band, audience, i, truth))
			}
			return result
		}
		return items.DefenseOptions{Together: items.DefenseTogetherMessages{
			ToDefender: messages("defender"),
			ToAttacker: messages("attacker"),
			ToRoom:     messages("room"),
		}}
	}
	return items.SeedDefenseMessagesForTest(map[items.DefenseType]*items.DefenseMessageGroup{
		items.DefenseQuell: {
			OptionId: items.DefenseQuell,
			Options: items.DefenseIntensity{
				items.Weak: mk("weak"), items.Normal: mk("normal"), items.Heavy: mk("heavy"),
			},
		},
	})
}

func drainChannelRoutingQueues(userIDs ...int) map[int][]string {
	result := make(map[int][]string, len(userIDs))
	for _, userID := range userIDs {
		result[userID] = events.DrainQueuedMessagesForTest(userID)
	}
	return result
}

func TestSpellChannelDefenceRuntimeQueuesCoverActorOrientations(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	attackerUser := users.GetByUserId(1)
	defenderUser := users.GetByUserId(2)
	observer := users.NewTestUser(3, "observer", "Orin", 1003)
	observer.Character.RoomId = 1
	restoreUsers := users.SeedUsersForTest(map[int]*users.UserRecord{
		1: attackerUser, 2: defenderUser, 3: observer,
	})
	defer restoreUsers()
	room := rooms.LoadRoom(1)
	room.AddPlayer(observer.UserId)

	userAttacker := attackerUser.Character.GetPlayerName(attackerUser.UserId).String()
	userDefender := defenderUser.Character.GetPlayerName(defenderUser.UserId).String()
	mobAttacker := `<ansi fg="mobname">Howler #1</ansi>`
	mobDefender := `<ansi fg="mobname">Skeleton #2</ansi>`
	tests := []struct {
		name                   string
		identities             combat.ChannelDefenceIdentities
		attacker, defender     *users.UserRecord
		attackerID, defenderID int
	}{
		{"player_player", combat.ChannelDefenceIdentities{Attacker: userAttacker, Defender: userDefender}, attackerUser, defenderUser, 1, 2},
		{"player_mob", combat.ChannelDefenceIdentities{Attacker: userAttacker, Defender: mobDefender}, attackerUser, nil, 1, 0},
		{"mob_player", combat.ChannelDefenceIdentities{Attacker: mobAttacker, Defender: userDefender}, nil, defenderUser, 0, 2},
		{"mob_mob", combat.ChannelDefenceIdentities{Attacker: mobAttacker, Defender: mobDefender}, nil, nil, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			drainChannelRoutingQueues(1, 2, 3)
			sendSpellChannelDefenceMessages(room, messaging.CategorySpellMental,
				combat.ChannelDefenceResult{DefenceType: "quell", Defended: true, NormalizedDefenceMargin: 0.75, DamageMultiplier: 0.2},
				tc.identities.Attacker, tc.identities.Defender, "Mind Fog", tc.attacker, tc.defender, 3)
			queues := drainChannelRoutingQueues(1, 2, 3)

			if tc.attackerID != 0 {
				require.Len(t, queues[tc.attackerID], 1)
				require.Contains(t, strings.ToLower(queues[tc.attackerID][0]), "normal-attacker-index=3")
			}
			if tc.defenderID != 0 {
				require.Len(t, queues[tc.defenderID], 1)
				require.Contains(t, strings.ToLower(queues[tc.defenderID][0]), "normal-defender-index=3")
			}
			for userID, lines := range queues {
				if userID == tc.attackerID || userID == tc.defenderID {
					continue
				}
				require.Len(t, lines, 1)
				require.Contains(t, strings.ToLower(lines[0]), "normal-room-index=3")
			}
			for _, lines := range queues {
				for _, line := range lines {
					require.Contains(t, line, tc.identities.Attacker+" resists-with "+tc.identities.Defender,
						"attacker and defender tokens must retain their actor orientation")
				}
			}
		})
	}
}

func TestSpellChannelDefenceRuntimeQueuesUseOutcomeAndStaySilentOnAttackWin(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	room := rooms.LoadRoom(1)
	attacker := users.GetByUserId(1)
	defender := users.GetByUserId(2)
	identities := combat.ChannelDefenceIdentities{
		Attacker: attacker.Character.GetPlayerName(attacker.UserId).String(),
		Defender: defender.Character.GetPlayerName(defender.UserId).String(),
	}

	for _, tc := range []struct {
		name string
		out  combat.ChannelDefenceResult
		want string
	}{
		{"partial", combat.ChannelDefenceResult{DefenceType: "quell", Defended: true, NormalizedDefenceMargin: 0.1, DamageMultiplier: 0.4}, "weak-"},
		{"defensive_crit", combat.ChannelDefenceResult{DefenceType: "quell", Defended: true, DefensiveCrit: true, DamageMultiplier: 0}, "heavy-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			drainChannelRoutingQueues(1, 2)
			sendSpellChannelDefenceMessages(room, messaging.CategorySpellMental, tc.out,
				identities.Attacker, identities.Defender, "Mind Fog", attacker, defender, 2)
			for userID, lines := range drainChannelRoutingQueues(1, 2) {
				require.Len(t, lines, 1, "user %d", userID)
				require.Contains(t, strings.ToLower(lines[0]), tc.want)
				require.Contains(t, lines[0], "index=2")
			}
		})
	}

	drainChannelRoutingQueues(1, 2)
	sendSpellChannelDefenceMessages(room, messaging.CategorySpellMental,
		combat.ChannelDefenceResult{DefenceType: "quell", Defended: false, DefensiveCrit: true, DamageMultiplier: 1},
		identities.Attacker, identities.Defender, "Mind Fog", attacker, defender, 2)
	for userID, lines := range drainChannelRoutingQueues(1, 2) {
		if len(lines) != 0 {
			t.Fatalf("attack win narrated false defence success to user %d: %s", userID, strings.Join(lines, " | "))
		}
	}
}

func TestSpellChannelDefencePreservesComputedDuplicateMobIdentity(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restoreMessages := seedChannelRoutingMessages(t)
	defer restoreMessages()

	room := rooms.LoadRoom(1)
	attacker := users.GetByUserId(1)
	first := mobs.GetInstance(100)
	require.NotNil(t, first)
	first.Character.MobInstanceId = first.InstanceId
	secondValue := *first
	second := &secondValue
	second.InstanceId = 101
	second.Character.MobInstanceId = second.InstanceId
	second.Character.Name = first.Character.Name
	specValue := *first
	restoreMobs := mobs.SeedMobsForTest(map[int]*mobs.Mob{int(first.MobId): &specValue}, map[int]*mobs.Mob{
		first.InstanceId:  first,
		second.InstanceId: second,
	})
	defer restoreMobs()
	room.AddMob(second.InstanceId)

	defenderIdentity := spellDefenceIdentity(&second.Character, nil, room)
	require.Contains(t, defenderIdentity, "Skeleton #2")
	drainChannelRoutingQueues(1, 2)
	sendSpellChannelDefenceMessages(room, messaging.CategorySpellMental,
		combat.ChannelDefenceResult{DefenceType: "quell", Defended: true, NormalizedDefenceMargin: 0.1, DamageMultiplier: 0.4},
		spellDefenceIdentity(attacker.Character, attacker, room), defenderIdentity, "Mind Fog", attacker, nil, 0)

	for userID, lines := range drainChannelRoutingQueues(1, 2) {
		require.Len(t, lines, 1, "user %d", userID)
		require.Contains(t, lines[0], "Skeleton #2")
	}
}

func TestResolveSpellDispatchPreservesSingleAndAreaKnockdownAfterDefensiveCrit(t *testing.T) {
	for _, spellType := range []spells.SpellType{spells.HarmSingle, spells.HarmArea} {
		t.Run(string(spellType), func(t *testing.T) {
			cleanup := seedAllRegistries()
			defer cleanup()
			restoreMessages := seedChannelRoutingMessages(t)
			defer restoreMessages()
			originalContest := runPlayerSpellContest
			runPlayerSpellContest = func(float64, []contest.Entry) contest.Result {
				return contest.Result{
					AttackRoll:  dice.RollResult{Value: 101, Mean: 100, StdDev: 15},
					DefenseRoll: dice.RollResult{Value: 100, Mean: 100, StdDev: 15},
					Margin:      1, Contested: true, Success: true,
				}
			}
			t.Cleanup(func() { runPlayerSpellContest = originalContest })
			originalDefence := runPlayerSpellDefence
			runPlayerSpellDefence = func(combat.AttackChannel, *characters.Character, *characters.Character) combat.ChannelDefenceResult {
				return combat.ChannelDefenceResult{
					DefenceType: characters.DefenseQuell, Defended: true,
					DefensiveCrit: true, DamageMultiplier: 0,
				}
			}
			t.Cleanup(func() { runPlayerSpellDefence = originalDefence })

			room := rooms.LoadRoom(1)
			attacker := users.GetByUserId(1)
			observer := users.GetByUserId(2)
			target := mobs.GetInstance(100)
			target.Character.MobInstanceId = target.InstanceId
			target.Character.Health = 100
			spell := &spells.SpellData{
				SpellId: "force-wave", Name: "Force Wave", Type: spellType,
				EffectType: "knockdown", TargetDefenseType: "mental", EffectMagnitude: 20,
			}
			attacker.Character.Stats.Willpower.ValueAdj = 1000
			attacker.Character.Skills = map[string]int{"spellcasting": 0}
			target.Character.Stats.Willpower.ValueAdj = 1
			target.Character.Skills = map[string]int{"spellcasting": 0}
			require.True(t, target.Character.IsStanding(), "fixture must begin standing")
			drainChannelRoutingQueues(attacker.UserId, observer.UserId)

			casting := activity.CastingData{SpellId: spell.SpellId}
			if spellType == spells.HarmSingle {
				casting.TargetMobInstanceIds = []int{target.InstanceId}
			}
			resolveSpell(attacker, casting, spell, room)

			require.Equal(t, 100, target.Character.Health, "defensive crit must reduce damage to zero")
			require.True(t, target.Character.IsSupine() || target.Character.IsProne(),
				"zero-damage defensive crit must preserve the binary knockdown; state=%v", target.Character.Position.State())
			attackerLines := drainChannelRoutingQueues(attacker.UserId)[attacker.UserId]
			require.Len(t, attackerLines, 1)
			require.Contains(t, strings.ToLower(attackerLines[0]), "heavy-attacker")
			require.Contains(t, strings.ToLower(attackerLines[0]), "no damage taken")
			observerLines := drainChannelRoutingQueues(observer.UserId)[observer.UserId]
			require.Len(t, observerLines, 2)
			require.Contains(t, strings.ToLower(observerLines[0]), "no damage taken")
			require.Contains(t, strings.ToLower(observerLines[0]), "momentum remains")
			require.Contains(t, strings.ToLower(observerLines[1]), "knocks")
			require.Contains(t, strings.ToLower(observerLines[1]), "ground")
		})
	}
}
