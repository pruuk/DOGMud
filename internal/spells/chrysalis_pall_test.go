package spells

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Chrysalis Pall (lighting plan 5d, Rule 4, ruling D9) as shipped: mental,
// single, willpower, cost 50, three wait rounds, condition 131, and found by
// discovery at spellcasting 25 (the shipped and Go-default
// SpellDiscoverySkillPerDifficulty is 1.0), never taught.
func TestShippedChrysalisPallIsDiscoveredAtSpellcasting25(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/spells/chrysalis-pall.yaml")
	require.NoError(t, err)
	var sp SpellData
	require.NoError(t, yaml.Unmarshal(data, &sp))
	require.Equal(t, "chrysalis-pall", sp.SpellId, "read the wrong file, so this test proves nothing")

	assert.Equal(t, "Chrysalis Pall", sp.Name)
	assert.Equal(t, []string{"pall"}, sp.Aliases)
	assert.True(t, sp.HasSchool(SchoolMental))
	assert.Equal(t, 50, sp.Cost)
	assert.Equal(t, 3, sp.WaitRounds)
	assert.Equal(t, 25, sp.Difficulty)
	assert.Equal(t, []int{131}, sp.ConditionIds)

	restore := allSpells
	t.Cleanup(func() { allSpells = restore })
	allSpells = map[string]*SpellData{sp.SpellId: &sp}
	assert.NotContains(t, GetEligibleSpells(map[string]int{}, 24, SchoolMental), "chrysalis-pall",
		"spellcasting 24 must not discover the pall")
	assert.Contains(t, GetEligibleSpells(map[string]int{}, 25, SchoolMental), "chrysalis-pall",
		"spellcasting 25 discovers the pall")
}
