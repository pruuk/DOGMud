package quests

import (
	"strings"
	"testing"
)

func TestValidateTopicHeardTriggers(t *testing.T) {
	q := validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Mob, q.Triggers[0].Topic = "topic_heard", 9112, "directions"
	if err := q.Validate(); err != nil {
		t.Fatalf("topic_heard with mob and topic refused: %v", err)
	}
	for _, c := range []struct {
		mob   int
		topic string
		want  string
	}{
		{0, "directions", "topic_heard needs a mob"},
		{9112, "", "topic_heard needs a topic"},
	} {
		q := validQuest(ActionDef{Grant: "9001-start"})
		q.Triggers[0].Event, q.Triggers[0].Mob, q.Triggers[0].Topic = "topic_heard", c.mob, c.topic
		if err := q.Validate(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("mob %d topic %q: want %q, got %v", c.mob, c.topic, c.want, err)
		}
	}
	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Topic = "directions" // a command trigger
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "only topic_heard has a topic") {
		t.Fatalf("a topic on a command trigger must be refused, got %v", err)
	}
}
