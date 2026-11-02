package quests

import (
	"strings"
	"testing"
)

func TestValidateGainReasons(t *testing.T) {
	gain := func(reason string) *Quest {
		q := validQuest(ActionDef{Grant: "9001-start"})
		q.Triggers[0].Event, q.Triggers[0].Reason = "item_gain", reason
		return q
	}
	for _, reason := range TriggerGainReasons {
		if err := gain(reason).Validate(); err != nil {
			t.Fatalf("reason %q refused: %v", reason, err)
		}
	}
	if err := gain("stolen").Validate(); err == nil || !strings.Contains(err.Error(), `invalid reason "stolen"`) {
		t.Fatalf("a reason no gain carries must be refused, got %v", err)
	}
	q := gain("steal")
	q.Triggers[0].Event = "command"
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "only item_gain has a reason") {
		t.Fatalf("a reason on a command trigger must be refused, got %v", err)
	}
}

func TestValidateCommandTargets(t *testing.T) {
	for _, kind := range TriggerTargetKinds {
		q := validQuest(ActionDef{Grant: "9001-start"})
		q.Triggers[0].Target = kind
		if err := q.Validate(); err != nil {
			t.Fatalf("target %q refused: %v", kind, err)
		}
	}
	q := validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Target = TargetNone
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), `target "none"`) {
		t.Fatalf("target none is an event value, not a filter; got %v", err)
	}
	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Target = "boar"
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), `target "boar"`) {
		t.Fatalf("target boar must be refused, got %v", err)
	}
	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Target = "mob_death", TargetMob
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "only command and spell_landed have a target") {
		t.Fatalf("a target on a mob_death trigger must be refused, got %v", err)
	}
}
