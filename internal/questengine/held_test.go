package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// heldQuest: "begin" moves 341 to fetch; gaining the item ends it. Any filter
// the case sets goes on the item_gain trigger.
func heldQuest(mutate func(t *TriggerDef)) *QuestDef {
	gain := TriggerDef{
		Event: "item_gain", Item: 77,
		Conditions: Conditions{Has: []string{"341-fetch"}, Missing: []string{"341-end"}},
		Actions:    []ActionDef{{Grant: "341-end"}},
	}
	if mutate != nil {
		mutate(&gain)
	}
	return &QuestDef{
		QuestId: 341, Name: "Fetch",
		Steps: []QuestStep{{Id: "start"}, {Id: "fetch"}, {Id: "end"}},
		Triggers: []TriggerDef{
			{Event: "command", Command: "begin",
				Conditions: Conditions{Has: []string{"341-start"}, Missing: []string{"341-fetch"}},
				Actions:    []ActionDef{{Grant: "341-fetch"}}},
			gain,
		},
	}
}

func runHeld(q *QuestDef, holds bool) (*fullMockPlayer, *fullMockActionContext) {
	e := NewEngine()
	e.RegisterQuest(q)
	p := newFullMockPlayer(100)
	p.quests["341-start"] = true
	if holds {
		p.items[77] = true
	}
	ctx := newFullMockActionContext(1, p)
	e.Notify("command", EventDetails{UserId: 1, Command: "begin", TargetKind: "none"}, p, ctx)
	return p, ctx
}

func TestHeld_AHeldItemFiresAtStepStart(t *testing.T) {
	p, ctx := runHeld(heldQuest(nil), true)
	assert.True(t, p.HasQuest("341-end"), "the held item satisfies the step as it starts")
	assert.Equal(t, []string{"You already have that."}, ctx.sentTexts,
		"with no item spec loaded the default line names no item")
}

func TestHeld_HeldLineOverridesTheDefault(t *testing.T) {
	p, ctx := runHeld(heldQuest(func(t *TriggerDef) { t.HeldLine = "You kept it safe." }), true)
	assert.True(t, p.HasQuest("341-end"))
	assert.Equal(t, []string{"You kept it safe."}, ctx.sentTexts)
}

func TestHeld_NotHeldDoesNotFire(t *testing.T) {
	p, ctx := runHeld(heldQuest(nil), false)
	assert.True(t, p.HasQuest("341-fetch"))
	assert.False(t, p.HasQuest("341-end"))
	assert.Empty(t, ctx.sentTexts)
}

// A filter asks how or where the item arrived, which a held item cannot answer.
func TestHeld_AFilteredTriggerDoesNotFire(t *testing.T) {
	for name, mutate := range map[string]func(t *TriggerDef){
		"reason": func(t *TriggerDef) { t.Reason = "steal" },
		"room":   func(t *TriggerDef) { t.Room = 5218 },
		"area":   func(t *TriggerDef) { t.Area = "grove" },
		"zone":   func(t *TriggerDef) { t.Zone = "Somewhere" },
		"count":  func(t *TriggerDef) { t.Count = 2 },
	} {
		p, _ := runHeld(heldQuest(mutate), true)
		assert.False(t, p.HasQuest("341-end"), name)
	}
}

// The trigger's own conditions still gate it: a held item with the step not
// reached fires nothing.
func TestHeld_ConditionsGate(t *testing.T) {
	q := heldQuest(func(t *TriggerDef) { t.Conditions.Has = []string{"341-other"} })
	p, _ := runHeld(q, true)
	assert.False(t, p.HasQuest("341-end"))
}
