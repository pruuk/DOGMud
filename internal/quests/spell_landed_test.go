package quests

import (
	"strings"
	"testing"
)

func TestValidateSpellLandedTriggers(t *testing.T) {
	q := validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Effect, q.Triggers[0].Target = "spell_landed", "heal", TargetMob
	if err := q.Validate(); err != nil {
		t.Fatalf("spell_landed with effect and target refused: %v", err)
	}

	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Noun = "spell_landed", "heal"
	if err := q.Validate(); err != nil {
		t.Fatalf("spell_landed with only a spell noun refused: %v", err)
	}

	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Effect = "heal" // the helper's trigger is a command trigger
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "only spell_landed has an effect") {
		t.Fatalf("an effect on a command trigger must be refused, got %v", err)
	}

	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Target = "mob_death", TargetMob
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "only command and spell_landed have a target") {
		t.Fatalf("a target on mob_death must be refused, got %v", err)
	}
}
