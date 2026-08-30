package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The whole point of CritSource is to distinguish a crit that WON its roll from
// one that bypassed the roll. If every crit came back "rolled", the field would
// answer nothing and the instrumentation would be worse than useless -- it would
// look like evidence.
func TestCritSourceDistinguishesForcedFromRolled(t *testing.T) {
	events := []CombatEvent{
		{Hit: true, Crit: true, CritSource: CritSourceRolled},
		{Hit: true, Crit: true, CritSource: CritSourceRolled},
		{Hit: true, Crit: true, CritSource: CritSourceSleeping},
		{Hit: true, Crit: true, CritSource: CritSourceOnWin},
		{Hit: true, Crit: false},
	}

	s := computeSummary(events)

	assert.Equal(t, 4, s.Crits)
	assert.Equal(t, map[string]int{
		CritSourceRolled:   2,
		CritSourceSleeping: 1,
		CritSourceOnWin:    1,
	}, s.CritsBySource)
}

// A crit arriving with no label means some path sets Crit without passing a
// labelled site. That must be visible, not silently counted as "rolled" -- the
// question being asked is precisely whether crits are bypassing the roll.
func TestCritSourceSurfacesUnlabelledCrits(t *testing.T) {
	s := computeSummary([]CombatEvent{
		{Hit: true, Crit: true, CritSource: CritSourceRolled},
		{Hit: true, Crit: true}, // no source set
	})

	assert.Equal(t, 2, s.Crits)
	assert.Equal(t, 1, s.CritsBySource["unlabelled"],
		"an unlabelled crit must be reported as such, so a missed instrumentation "+
			"site cannot masquerade as a rolled crit")
}

// No crits must not produce a misleading empty map in the JSON.
func TestCritSourceOmittedWhenNoCrits(t *testing.T) {
	s := computeSummary([]CombatEvent{{Hit: true}, {Hit: false}})
	assert.Zero(t, s.Crits)
	assert.Nil(t, s.CritsBySource)
}
