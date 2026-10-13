package quests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// strongboxQuest is quest 14's strongbox as it shipped (#495): the trigger that
// grants the ledger, then one that says the ledger was already taken.
func strongboxQuest(alreadyFirst bool) *Quest {
	open := TriggerDef{
		Event: "room_interact", Room: 498, Noun: "strongbox",
		Conditions: Conditions{Missing: []string{"14-evidence"}},
		Actions:    []ActionDef{{GiveItem: 40036}, {Grant: "14-evidence"}, {SendText: "You take the ledger."}},
	}
	already := TriggerDef{
		Event: "room_interact", Room: 498, Noun: "strongbox",
		Conditions: Conditions{Has: []string{"14-evidence"}},
		Actions:    []ActionDef{{SendText: "You have already taken the ledger."}},
	}
	q := &Quest{QuestId: 14, Name: "The Undertow", Steps: []QuestStep{{Id: "evidence", Hint: "h"}}}
	if alreadyFirst {
		q.Triggers = []TriggerDef{already, open}
	} else {
		q.Triggers = []TriggerDef{open, already}
	}
	return q
}

func TestValidate_RefusesAGrantALaterTriggerRequiresInTheSameCommand(t *testing.T) {
	err := strongboxQuest(false).Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `trigger 0 grants "14-evidence", which trigger 1`)
}

func TestValidate_AcceptsTheAlreadyDoneTriggerFirst(t *testing.T) {
	assert.NoError(t, strongboxQuest(true).Validate())
}

func TestSameCommandChains_OtherPairs(t *testing.T) {
	cases := []struct {
		name    string
		actions []ActionDef
		later   Conditions
		want    string
	}{
		{"give then has_item", []ActionDef{{GiveItem: 7}}, Conditions{HasItem: 7}, "gives item 7"},
		{"consume then missing_item", []ActionDef{{ConsumeItem: 7}}, Conditions{MissingItem: 7}, "consumes item 7"},
		{"flag then has_flag", []ActionDef{{SetFlag: &QuestFlagAction{Key: "1-k", Value: "v"}}},
			Conditions{HasFlag: map[string]string{"1-k": "v"}}, "sets flag 1-k=v"},
		{"grant then missing is fine", []ActionDef{{Grant: "1-a"}}, Conditions{Missing: []string{"1-a"}}, ""},
		{"other flag value is fine", []ActionDef{{SetFlag: &QuestFlagAction{Key: "1-k", Value: "v"}}},
			Conditions{HasFlag: map[string]string{"1-k": "w"}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sameCommandEnables(c.actions, c.later)
			if c.want == "" {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, []string{c.want}, got)
		})
	}
}

func TestTriggersCanShareAnEvent(t *testing.T) {
	look := &TriggerDef{Event: "command_issued", Command: "look"}
	lookGuide := &TriggerDef{Event: "command_issued", Command: "look", Noun: "guide"}
	lookDoor := &TriggerDef{Event: "command_issued", Command: "look", Noun: "door"}
	room1 := &TriggerDef{Event: "room_interact", Room: 1, Noun: "box"}
	room2 := &TriggerDef{Event: "room_interact", Room: 2, Noun: "box"}

	assert.True(t, triggersCanShareAnEvent(look, lookGuide), "an open noun matches any noun")
	assert.False(t, triggersCanShareAnEvent(lookGuide, lookDoor))
	assert.False(t, triggersCanShareAnEvent(room1, room2))
}
