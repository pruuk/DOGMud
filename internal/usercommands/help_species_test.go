package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// `help species` renders help/species.template over the species list, but
// GetHelpContents only built that list when the topic was `races`. Since the
// race-to-species rename, keywords.yaml aliases race and races TO species, so
// the check never matched and the per-species block always rendered empty
// (found by the conditions slice 3 player smoke playtest, 2026-09-15). The block
// is also the only player-facing use of the conditionname template function.
func TestGetHelpContents_SpeciesRendersSpeciesDetail(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	defer conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		4401: {ConditionId: 4401, Name: "Probe Resilience", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 1},
	})()
	defer species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "Probe Human", Selectable: true, ConditionIds: []int{4401}},
	})()
	useDogmudTemplates(t)

	out, err := GetHelpContents("species", 80)
	require.NoError(t, err)
	assert.Contains(t, out, "Probe Human", "help species must list the selectable species")
	assert.Contains(t, out, "Probe Resilience", "the species' condition must render through conditionname")

	// The unarmed attack shows its name only: the empty "()" that followed it
	// (the damage dice, empty for every species) is gone (plan 4, decision 5).
	unarmed := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Unarmed:") {
			unarmed = line
		}
	}
	require.NotEmpty(t, unarmed, "help species must show an Unarmed row")
	assert.NotContains(t, unarmed, "()", "the Unarmed row must not print an empty ()")
	assert.NotContains(t, unarmed, "(<", "the Unarmed row must not print the damage brackets")
}
