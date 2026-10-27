package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCastTargets_SelfCastIsOneSelfTargetWithTheSpellNoun(t *testing.T) {
	got := castTargets(7, actions.CastResult{TargetUserIds: []int{7}}, "mend")

	require.Len(t, got, 1)
	assert.Equal(t, quests.TargetSelf, got[0].Kind)
	assert.Equal(t, "mend", got[0].Noun)
}

func TestCastTargets_AnotherPlayerIsAPlayerTarget(t *testing.T) {
	got := castTargets(7, actions.CastResult{TargetUserIds: []int{2}}, "mend")

	require.Len(t, got, 1)
	assert.Equal(t, quests.TargetPlayer, got[0].Kind)
	assert.Equal(t, "mend", got[0].Noun)
}

func TestCastTargets_NoCreatureIsOneNoneTargetWithTheSpellNoun(t *testing.T) {
	got := castTargets(7, actions.CastResult{}, "mend")

	require.Len(t, got, 1)
	assert.Equal(t, quests.TargetNone, got[0].Kind)
	assert.Equal(t, "mend", got[0].Noun)
}
