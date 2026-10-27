package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/assert"
)

// seedTargetQuest: step 1 needs a throw at mob 9112, step 2 any throw.
func seedTargetQuest() *Engine {
	q := &QuestDef{
		QuestId: 300,
		Name:    "Target Test Quest",
		Steps: []QuestStep{
			{Id: "300-start", Description: "Begin"},
			{Id: "300-boar", Description: "Throw at the boar"},
			{Id: "300-any", Description: "Throw at anything"},
		},
		Triggers: []TriggerDef{
			{
				Event: "command", Command: "throw", Target: quests.TargetMob, Mob: 9112,
				Conditions: Conditions{Has: []string{"300-start"}, Missing: []string{"300-boar"}},
				Actions:    []ActionDef{{Grant: "300-boar"}},
			},
			{
				Event: "command", Command: "throw",
				Conditions: Conditions{Has: []string{"300-boar"}, Missing: []string{"300-any"}},
				Actions:    []ActionDef{{Grant: "300-any"}},
			},
		},
	}
	e := NewEngine()
	e.RegisterQuest(q)
	return e
}

func TestTargetFilterNeedsTheNamedMob(t *testing.T) {
	e := seedTargetQuest()
	player := newFullMockPlayer(100)
	player.quests["300-start"] = true

	// A throw at another mob does not count.
	r := e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 7}, player, newFullMockActionContext(1, player))
	assert.False(t, player.HasQuest("300-boar"), "wrong mob advanced the step: %+v", r)

	// A throw at nothing does not count.
	e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetNone}, player, newFullMockActionContext(1, player))
	assert.False(t, player.HasQuest("300-boar"))

	// A throw at the boar does.
	e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetMob, MobId: 9112}, player, newFullMockActionContext(1, player))
	assert.True(t, player.HasQuest("300-boar"))
}

func TestTargetFilterKindMustMatch(t *testing.T) {
	e := seedTargetQuest()
	player := newFullMockPlayer(100)
	player.quests["300-start"] = true

	// MobId set but the kind is item (sell's shopkeeper rides along on an item target).
	e.Notify("command", EventDetails{UserId: 1, Command: "throw", TargetKind: quests.TargetItem, MobId: 9112}, player, newFullMockActionContext(1, player))
	assert.False(t, player.HasQuest("300-boar"))
}
