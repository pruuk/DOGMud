package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func runKills(t *testing.T, questId string, start string, mobs ...int) (*fullMockPlayer, *fullMockActionContext) {
	t.Helper()
	e := NewEngine()
	e.RegisterQuest(loadTier2Quest(t, questId))
	p := newFullMockPlayer(100)
	p.quests[start] = true
	ctx := newFullMockActionContext(1, p)
	for _, m := range mobs {
		e.Notify("mob_death", EventDetails{UserId: 1, MobId: m, RoomId: 100}, p, ctx)
	}
	return p, ctx
}

// Tier 2 ruling: the hedgerow bandit first, then the Reach-Touched.
func TestQuest61InOrder(t *testing.T) {
	p, _ := runKills(t, "61", "61-start", 9173, 9178)
	assert.True(t, p.HasQuest("61-bandit"))
	assert.True(t, p.HasQuest("61-reach"))
}

func TestQuest61ReachFirstGrantsNothingAndSaysSo(t *testing.T) {
	p, ctx := runKills(t, "61", "61-start", 9178)
	assert.False(t, p.HasQuest("61-bandit"))
	assert.False(t, p.HasQuest("61-reach"))
	assert.Len(t, ctx.sentTexts, 1)
	assert.Contains(t, ctx.sentTexts[0], "Pemberton wanted the bandit")

	// The bandit, then the Reach-Touched again, still finishes.
	e := NewEngine()
	e.RegisterQuest(loadTier2Quest(t, "61"))
	e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9173, RoomId: 100}, p, ctx)
	e.Notify("mob_death", EventDetails{UserId: 1, MobId: 9178, RoomId: 100}, p, ctx)
	assert.True(t, p.HasQuest("61-reach"))
}
