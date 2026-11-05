package quests

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// A look through an exit fires the room_gate that would refuse the step, so its
// actions run on a look. Anything beyond text would pay out for looking at a
// door, so the file is refused.
func gateQuest(t *testing.T, action string) *Quest {
	t.Helper()
	src := `
questid: 901
name: Gate Fixture
steps:
  - id: start
triggers:
  - event: room_gate
    room: 5
    conditions:
      missing: ["901-start"]
    actions:
` + action
	var q Quest
	if err := yaml.Unmarshal([]byte(src), &q); err != nil {
		t.Fatal(err)
	}
	return &q
}

func TestValidate_ARoomGateIsTextOnly(t *testing.T) {
	for _, ok := range []string{
		`      - actor: "The door does not give."`,
		`      - observer: "{actor} tries the door."`,
	} {
		if err := gateQuest(t, ok).Validate(); err != nil {
			t.Errorf("%s is text: %v", ok, err)
		}
	}
	for _, bad := range []string{
		`      - grant: "901-start"`,
		`      - give_gold: 5`,
		`      - give_item: 7`,
		`      - teleport: 3`,
		"      - actor: \"Fine.\"\n      - give_mutation: true",
	} {
		err := gateQuest(t, bad).Validate()
		if err == nil || !strings.Contains(err.Error(), "only actor and observer text") {
			t.Errorf("%s: got %v, want it refused", bad, err)
		}
	}
}

// The rule is the gate's alone: any other event may still grant.
func TestValidate_TextOnlyIsNotAskedOfOtherEvents(t *testing.T) {
	q := gateQuest(t, `      - grant: "901-start"`)
	q.Triggers[0].Event = "room_enter"
	q.Triggers[0].Conditions.Missing = []string{"901-start"}
	if err := q.Validate(); err != nil {
		t.Errorf("a room_enter may grant: %v", err)
	}
}
