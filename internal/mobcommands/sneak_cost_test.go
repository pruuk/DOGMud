package mobcommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSneakMobCostRefusalIsSilentAndPreservesState(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)
	mob.Character.Stamina = 0
	mob.Character.Aggro = nil
	mob.Character.Cooldowns = characters.Cooldowns{
		skills.Skullduggery.String("sneak"): 4,
		"other":                             -2,
	}
	mob.Character.Skills = map[string]int{string(skills.Skullduggery): 0}
	mob.Character.SkillUseCount = map[string]int{}
	mob.Character.StatUseCount = map[string]int{}
	mob.Character.AttacksThisRound = 7
	mob.Character.DefensesThisRound = 8
	mob.Character.LastTargetFoundRound = 9

	cooldownsBefore := mob.Character.GetAllCooldowns()
	attacksBefore := mob.Character.AttacksThisRound
	defensesBefore := mob.Character.DefensesThisRound
	targetRoundBefore := mob.Character.LastTargetFoundRound
	events.DrainQueuedMessagesForTest(0)

	handled, err := Sneak("", mob, room)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Empty(t, events.DrainQueuedMessagesForTest(0), "mob cost refusal must stay silent")
	assert.Equal(t, awareness.Visible, mob.Character.Awareness.State())
	assert.False(t, mob.Character.IsHidden())
	assert.Nil(t, mob.Character.GetMiscData("sneaking"))
	assert.Equal(t, cooldownsBefore, mob.Character.GetAllCooldowns())
	assert.Zero(t, mob.Character.Stamina)
	assert.Zero(t, mob.Character.GetSkillUseCount(string(skills.Skullduggery)),
		"refusal must not call OnSkillUse")
	assert.Zero(t, mob.Character.StatUseCount["dexterity"])
	assert.Equal(t, attacksBefore, mob.Character.AttacksThisRound)
	assert.Equal(t, defensesBefore, mob.Character.DefensesThisRound)
	assert.Equal(t, targetRoundBefore, mob.Character.LastTargetFoundRound)

	// A refused 2.75 quote must not advance fractional carry. Two later
	// rank-zero admissions charge 2 then 3, rather than 3 then 3.
	mob.Character.Stamina = 100
	charged := make([]int, 0, 2)
	for range 2 {
		quote := mob.Character.QuoteActionCost(characters.ActionCostRequest{
			Action: costs.ActionSneak, Pool: characters.PoolStamina,
			Base: float64(configs.GetBalanceConfig().SneakBaseStaminaCost), Modifier: 1, Units: 1,
		})
		charged = append(charged, mob.Character.CommitCost(quote, characters.CostFullOrRefuse).Charged)
	}
	assert.Equal(t, []int{2, 3}, charged)
}
