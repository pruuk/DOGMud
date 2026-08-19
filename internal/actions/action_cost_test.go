package actions

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

type u8SharedActionFunction struct {
	file string
	name string
}

// u8SharedActionFunctions is intentionally forward-compatible: Task 3 runs
// before later migrations, so the guard checks a function only after it exists.
var u8SharedActionFunctions = []u8SharedActionFunction{
	{file: "combat_bash.go", name: "ExecuteBash"},
	{file: "combat_trip.go", name: "ExecuteTrip"},
	{file: "combat_kick.go", name: "ExecuteKick"},
	{file: "combat_grapple.go", name: "ExecuteGrapple"},
	{file: "combat_hamstring.go", name: "ExecuteHamstring"},
	{file: "combat_rake.go", name: "ExecuteRake"},
	{file: "combat_maul.go", name: "ExecuteMaul"},
	{file: "combat_pounce.go", name: "ExecutePounce"},
	{file: "combat_gore.go", name: "ExecuteGore"},
	{file: "combat_drain.go", name: "ExecuteDrain"},
	{file: "combat_throttle.go", name: "ExecuteThrottle"},
	{file: "combat_fire.go", name: "ExecuteFire"},
	{file: "combat_reload.go", name: "ExecuteReload"},
	{file: "sneak.go", name: "Sneak"},
	{file: "combat_taunt.go", name: "ExecuteTaunt"},
	{file: "combat_rally.go", name: "ExecuteRally"},
	{file: "combat_warcry.go", name: "ExecuteWarcry"},
}

// TestU8SharedActionsUseQuoteCommitInsteadOfLegacyCostCalls catches a future
// shared U8 action migration that charges a pool through a legacy primitive
// instead of the quote/commit admission seam.
func TestU8SharedActionsUseQuoteCommitInsteadOfLegacyCostCalls(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	actionsDir := filepath.Dir(testFile)
	forbidden := map[string]bool{
		"ApplyCost":              true,
		"ApplyCostPartial":       true,
		"ApplyCostFloat":         true,
		"ApplyCostFloatOrRefuse": true,
	}

	for _, target := range u8SharedActionFunctions {
		t.Run(target.name, func(t *testing.T) {
			parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(actionsDir, target.file), nil, 0)
			require.NoError(t, err)

			found := false
			var violations []string
			ast.Inspect(parsed, func(node ast.Node) bool {
				decl, ok := node.(*ast.FuncDecl)
				if !ok || decl.Name.Name != target.name {
					return true
				}
				found = true
				ast.Inspect(decl.Body, func(bodyNode ast.Node) bool {
					call, ok := bodyNode.(*ast.CallExpr)
					if !ok {
						return true
					}
					selector, ok := call.Fun.(*ast.SelectorExpr)
					if ok && forbidden[selector.Sel.Name] {
						violations = append(violations, selector.Sel.Name)
					}
					return true
				})
				return false
			})

			if !found {
				return
			}
			require.Empty(t, violations, "%s must use the shared quote/commit cost seam", target.name)
		})
	}
}

// TestCostRefusalText_UsesPoolAwareProseOnlyForRefusal catches a renderer that
// exposes cost values, confuses pools, or turns a paid/no-charge result into a
// refusal message.
func TestCostRefusalText_UsesPoolAwareProseOnlyForRefusal(t *testing.T) {
	digit := regexp.MustCompile(`\d`)
	for _, tc := range []struct {
		name   string
		result characters.CostCommitResult
		phrase string
	}{
		{
			name:   "stamina",
			result: characters.CostCommitResult{Status: characters.CostRefused, Pool: characters.PoolStamina},
			phrase: "too spent",
		},
		{
			name:   "conviction",
			result: characters.CostCommitResult{Status: characters.CostRefused, Pool: characters.PoolConviction},
			phrase: "cannot muster the resolve",
		},
		{
			name:   "paid",
			result: characters.CostCommitResult{Status: characters.CostPaid, Pool: characters.PoolStamina},
			phrase: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := costRefusalText(tc.result)
			if tc.phrase == "" {
				require.Empty(t, got)
				return
			}
			require.Contains(t, got, tc.phrase)
			require.False(t, digit.MatchString(got), "refusal prose must not expose a number")
		})
	}
}

// TestCostRefusalText_ExportsTheWrapperSeam catches an exported wrapper API
// that bypasses the pool-aware private helper or returns a different result.
func TestCostRefusalText_ExportsTheWrapperSeam(t *testing.T) {
	for _, result := range []characters.CostCommitResult{
		{Status: characters.CostRefused, Pool: characters.PoolStamina},
		{Status: characters.CostRefused, Pool: characters.PoolConviction},
		{Status: characters.CostPaid, Pool: characters.PoolStamina},
	} {
		require.Equal(t, costRefusalText(result), CostRefusalText(result))
	}
}

// TestAdmitFullCost_DoesNotEmitPrivateText catches shared admission code that
// renders a player message itself instead of leaving that decision to the user
// wrapper. The real UserActor routes any such text into the event queue.
func TestAdmitFullCost_DoesNotEmitPrivateText(t *testing.T) {
	const userID = 8113
	user := users.NewTestUser(userID, "cost-test", "Cost Test", 0)
	user.Character.Stamina = 0
	events.DrainQueuedMessagesForTest(userID)

	result := admitFullCost(&UserActor{User: user}, costs.ActionShoot, characters.PoolStamina, 2)

	require.Equal(t, characters.CostRefused, result.Status)
	require.Empty(t, events.DrainQueuedMessagesForTest(userID))
}

// TestAdmitFullCost_PlayerAndMobCommitOneFractionalCharge catches a helper
// that bypasses Character.QuoteActionCost/CommitCost, uses a non-neutral
// modifier, or commits the same voluntary action more than once. The follow-up
// quote exposes the one retained fractional carry without mocking Character.
func TestAdmitFullCost_PlayerAndMobCommitOneFractionalCharge(t *testing.T) {
	playerCharacter := characters.New()
	playerCharacter.Conviction = 10
	playerCharacter.Skills[string(skills.Rhetoric)] = 0

	mobCharacter := *playerCharacter
	player := &UserActor{User: &users.UserRecord{UserId: 11, Character: playerCharacter}}
	mob := &MobActor{Mob: &mobs.Mob{InstanceId: 22, Character: mobCharacter}}

	tests := []struct {
		name  string
		actor Actor
		char  *characters.Character
	}{
		{name: "player", actor: player, char: playerCharacter},
		{name: "mob", actor: mob, char: &mob.Mob.Character},
	}
	results := make([]characters.CostCommitResult, len(tests))
	for i, tc := range tests {
		i, tc := i, tc
		t.Run(tc.name, func(t *testing.T) {
			result := admitFullCost(tc.actor, costs.ActionTaunt, characters.PoolConviction, 1.5)
			results[i] = result

			require.Equal(t, characters.CostPaid, result.Status)
			require.Equal(t, 1, result.Charged)
			require.Equal(t, 9, tc.char.Conviction)

			// Rhetoric rank zero prices the second 0.5 base at 0.55. After
			// exactly one 1.65 admission, the retained 0.65 carry makes the
			// next whole due one. No carry (or two commits) produces a different
			// observable charge/pool result.
			followUp := tc.char.CommitCost(tc.char.QuoteActionCost(characters.ActionCostRequest{
				Action:   costs.ActionTaunt,
				Pool:     characters.PoolConviction,
				Base:     0.5,
				Modifier: 1.0,
				Units:    1,
			}), characters.CostFullOrRefuse)
			require.Equal(t, characters.CostPaid, followUp.Status)
			require.Equal(t, 1, followUp.Charged)
			require.Equal(t, 8, tc.char.Conviction)
		})
	}
	require.Equal(t, results[0].Status, results[1].Status)
	require.Equal(t, results[0].Charged, results[1].Charged)
}
