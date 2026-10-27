package questengine_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/assert"
)

// The bridge bumps the key it is given while the player is on that step.
func TestGameBridge_BumpStepCount(t *testing.T) {
	u := newBridgeUser(1)
	u.Character.QuestProgress = map[int]string{53: "hunt"}
	var sc questengine.StepCounter = questengine.NewGameBridge(u, 1)
	n, ok := sc.BumpStepCount(53, "hunt", "53-hunt-t0")
	assert.True(t, ok)
	assert.Equal(t, 1, n)
	n, _ = sc.BumpStepCount(53, "hunt", "53-hunt-t0")
	assert.Equal(t, 2, n)
	assert.Equal(t, 2, u.Character.QuestStepCount("53-hunt-t0"))
}

// A count for one step is not bumped while the player is on another, and a
// quest the player does not hold bumps nothing.
func TestGameBridge_BumpStepCountOnlyOnItsStep(t *testing.T) {
	u := newBridgeUser(1)
	u.Character.QuestProgress = map[int]string{53: "b"}
	b := questengine.NewGameBridge(u, 1)
	_, ok := b.BumpStepCount(53, "a", "53-a-t0")
	assert.False(t, ok)
	_, ok = b.BumpStepCount(54, "start", "54-start-t0")
	assert.False(t, ok)
	assert.Empty(t, u.Character.QuestStepCounts)
}

// Through the engine: a count trigger for step a (that skips step b) whose
// conditions pass while the player is on b does not count there.
func TestCount_TriggerForStepADoesNotCountOnB(t *testing.T) {
	q := &quests.Quest{
		QuestId: 53, Name: "Hunt",
		Steps: []quests.QuestStep{{Id: "start"}, {Id: "a"}, {Id: "b"}, {Id: "end"}},
		Triggers: []quests.TriggerDef{{
			Event: "mob_death", Mob: 9110, Count: 2,
			Conditions: quests.Conditions{Has: []string{"53-a"}, Missing: []string{"53-end"}},
			Actions:    []quests.ActionDef{{Grant: "53-end"}},
		}},
	}
	t.Cleanup(quests.SeedQuestsForTest(q))
	e := questengine.NewEngine()
	e.RegisterQuest(q)
	u := newBridgeUser(1)
	u.Character.QuestProgress = map[int]string{53: "b"}
	b := questengine.NewGameBridge(u, 100)

	r := e.Notify("mob_death", questengine.EventDetails{UserId: 1, MobId: 9110, RoomId: 100}, b, b)
	assert.False(t, r.Handled)
	assert.Empty(t, u.Character.QuestStepCounts, "nothing is counted on step b")

	u.Character.QuestProgress[53] = "a"
	e.Notify("mob_death", questengine.EventDetails{UserId: 1, MobId: 9110, RoomId: 100}, b, b)
	assert.Equal(t, 1, u.Character.QuestStepCount("53-a-t0"))
}
