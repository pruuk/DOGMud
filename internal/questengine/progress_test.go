package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

func noCounts(string) int { return 0 }

func TestStepProgress_Count(t *testing.T) {
	e := NewEngine()
	q := huntQuest()
	q.Triggers[0].Label = "Wolves culled"
	e.RegisterQuest(q)
	counts := map[string]int{"53-start-t0": 2}

	label, have, need, ok := e.StepProgress(53, "start", newFullMockPlayer(100), func(k string) int { return counts[k] })
	assert.True(t, ok)
	assert.Equal(t, "Wolves culled", label)
	assert.Equal(t, 2, have)
	assert.Equal(t, 3, need)
}

// The progress line reads the count of the trigger that leads out of the
// step (its own-quest has step), by that trigger's key: a later trigger's
// count on another step, and a trigger that only also holds the step, are
// not this step's progress.
func TestStepProgress_ReadsTheTriggerOutOfTheStep(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(&QuestDef{
		QuestId: 53, Name: "Hunt",
		Steps: []QuestStep{{Id: "start"}, {Id: "hunt"}, {Id: "end"}},
		Triggers: []TriggerDef{
			{
				Event: "mob_death", Mob: 1, Count: 4,
				Conditions: Conditions{Has: []string{"53-start", "53-hunt"}, Missing: []string{"53-end"}},
				Actions:    []ActionDef{{Grant: "53-end"}},
			},
			{
				Event: "mob_death", Mob: 2, Count: 3, Label: "Boars",
				Conditions: Conditions{Has: []string{"53-start"}, Missing: []string{"53-hunt"}},
				Actions:    []ActionDef{{Grant: "53-hunt"}},
			},
		},
	})
	counts := map[string]int{"53-hunt-t0": 1, "53-start-t1": 2}
	read := func(k string) int { return counts[k] }

	label, have, need, ok := e.StepProgress(53, "start", newFullMockPlayer(100), read)
	assert.True(t, ok)
	assert.Equal(t, "Boars", label)
	assert.Equal(t, 2, have)
	assert.Equal(t, 3, need)

	label, have, need, ok = e.StepProgress(53, "hunt", newFullMockPlayer(100), read)
	assert.True(t, ok)
	assert.Equal(t, "Progress", label)
	assert.Equal(t, 1, have)
	assert.Equal(t, 4, need)
}

func TestStepProgress_LabelDefaultsToProgress(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(huntQuest())
	label, have, need, ok := e.StepProgress(53, "start", newFullMockPlayer(100), noCounts)
	assert.True(t, ok)
	assert.Equal(t, "Progress", label)
	assert.Equal(t, 0, have)
	assert.Equal(t, 3, need)
}

func TestStepProgress_NoneForAPlainStepOrAnotherStep(t *testing.T) {
	e := NewEngine()
	q := huntQuest()
	e.RegisterQuest(q)
	_, _, _, ok := e.StepProgress(53, "end", newFullMockPlayer(100), noCounts)
	assert.False(t, ok, "no count trigger is reached from the end step")

	q.Triggers[0].Count = 0
	_, _, _, ok = e.StepProgress(53, "start", newFullMockPlayer(100), noCounts)
	assert.False(t, ok, "a plain trigger has no progress line")

	_, _, _, ok = e.StepProgress(999, "start", newFullMockPlayer(100), noCounts)
	assert.False(t, ok, "unknown quest")
}

func TestStepProgress_Distinct(t *testing.T) {
	installGrove(t)
	for _, c := range []struct {
		distinct   string
		mapped     []int
		have, need int
	}{
		{"all", []int{5311}, 1, 3},
		{"2", []int{5311}, 1, 2},
		{"2", []int{5311, 5312, 5313}, 2, 2}, // never more than it needs
	} {
		e := NewEngine()
		e.RegisterQuest(exploreQuest(c.distinct, ""))
		label, have, need, ok := e.StepProgress(340, "explore", newMapPlayer(c.mapped...), noCounts)
		assert.True(t, ok, c.distinct)
		assert.Equal(t, "Progress", label)
		assert.Equal(t, c.have, have, c.distinct)
		assert.Equal(t, c.need, need, c.distinct)
	}
}

func TestStepProgress_DistinctNeedsAMap(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(exploreQuest("all", ""))
	_, _, _, ok := e.StepProgress(340, "explore", newFullMockPlayer(100), noCounts)
	assert.False(t, ok)
}

func TestProgressLine(t *testing.T) {
	assert.Equal(t, "Progress: 2 of 5", ProgressLine("Progress", 2, 5))
}

// UserStepProgress reads a real character: its saved counts and its map.
func TestUserStepProgress_ReadsTheCharacter(t *testing.T) {
	installGrove(t)
	e := NewEngine()
	e.RegisterQuest(huntQuest())
	e.RegisterQuest(exploreQuest("all", ""))
	u := users.NewTestUser(9761, "progress", "Progress", 97610)
	u.Character.QuestStepCounts = map[string]int{"53-start-t0": 1}
	u.Character.VisitedRooms = map[string][]int{"Pothole Coulee": {5311, 5313}}

	assert.Equal(t, "Progress: 1 of 3", e.UserStepProgress(u, 53, "start"))
	assert.Equal(t, "Progress: 2 of 3", e.UserStepProgress(u, 340, "explore"))
	assert.Equal(t, "", e.UserStepProgress(u, 340, "start"))
}
