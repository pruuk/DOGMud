package questengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopicHeardNeedsTheTopicAndTheNPC(t *testing.T) {
	e := NewEngine()
	e.RegisterQuest(&QuestDef{
		QuestId: 320, Name: "Ask Fred",
		Steps: []QuestStep{{Id: "320-start"}, {Id: "320-asked"}},
		Triggers: []TriggerDef{{
			Event: "topic_heard", Mob: 9112, Topic: "directions",
			Conditions: Conditions{Has: []string{"320-start"}, Missing: []string{"320-asked"}},
			Actions:    []ActionDef{{Grant: "320-asked"}},
		}},
	})
	p := newFullMockPlayer(100)
	p.quests["320-start"] = true

	e.Notify("topic_heard", EventDetails{UserId: 1, MobId: 9112, Topic: "rumours"}, p, newFullMockActionContext(1, p))
	assert.False(t, p.HasQuest("320-asked"), "another topic of the same NPC")
	e.Notify("topic_heard", EventDetails{UserId: 1, MobId: 7, Topic: "directions"}, p, newFullMockActionContext(1, p))
	assert.False(t, p.HasQuest("320-asked"), "the same key on another NPC")
	e.Notify("topic_heard", EventDetails{UserId: 1, MobId: 9112, Topic: "directions"}, p, newFullMockActionContext(1, p))
	assert.True(t, p.HasQuest("320-asked"))
}

func TestTopicHeardDetails(t *testing.T) {
	assert.Equal(t, EventDetails{UserId: 1, RoomId: 5200, MobId: 9112, Topic: "directions"},
		TopicHeardDetails(1, 5200, 9112, "directions"))
}
