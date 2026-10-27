package quests

import (
	"strings"
	"testing"
)

// #375 phase 3b slice 4: area, zone, count, distinct, label, explored_line.

func TestValidateAreaAndZone(t *testing.T) {
	for _, ev := range []string{"command", "mob_death", "item_gain", "room_enter", "spell_landed"} {
		for _, set := range []func(*TriggerDef){
			func(td *TriggerDef) { td.Area = "grove" },
			func(td *TriggerDef) { td.Zone = "Thornwall" },
		} {
			q := validQuest(ActionDef{Grant: "9001-start"})
			q.Triggers[0].Event = ev
			set(&q.Triggers[0])
			if err := q.Validate(); err != nil {
				t.Errorf("%s with area/zone refused: %v", ev, err)
			}
		}
	}
	q := validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Area, q.Triggers[0].Zone = "grove", "Thornwall"
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "set area or zone, not both") {
		t.Fatalf("area and zone together must be refused, got %v", err)
	}
}

func TestValidateCount(t *testing.T) {
	for _, n := range []int{0, 2, 3} {
		q := countQuest("mob_death")
		q.Triggers[0].Count = n
		if err := q.Validate(); err != nil {
			t.Errorf("count %d refused: %v", n, err)
		}
	}
	for _, n := range []int{1, -2} {
		q := validQuest(ActionDef{Grant: "9001-start"})
		q.Triggers[0].Count = n
		if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "count must be 2 or more") {
			t.Errorf("count %d: want refusal, got %v", n, err)
		}
	}
	q := validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Area, q.Triggers[0].Distinct, q.Triggers[0].Count = "room_enter", "grove", "all", 3
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "set count or distinct, not both") {
		t.Fatalf("count with distinct must be refused, got %v", err)
	}
}

// countQuest is a quest whose one trigger counts on event: it holds its
// own start step and waits for the end its actions grant.
func countQuest(event string) *Quest {
	return &Quest{
		QuestId: 9001, Name: "Probe",
		Steps: []QuestStep{{Id: "start"}, {Id: "end"}},
		Triggers: []TriggerDef{{
			Event: event, Count: 3,
			Conditions: Conditions{Has: []string{"9001-start"}, Missing: []string{"9001-end"}},
			Actions:    []ActionDef{{Grant: "9001-end"}},
		}},
	}
}

// A partial count marks the event handled, and on these three events a
// handled event means the quest took the command: the gate refuses the move
// in silence, look and read print nothing, give keeps the item unconsumed.
// So count is refused there.
func TestValidateCountRefusedWhereAMatchSwallowsTheCommand(t *testing.T) {
	for _, ev := range []string{"room_gate", "room_interact", "item_give"} {
		q := countQuest(ev)
		err := q.Validate()
		if err == nil || !strings.Contains(err.Error(), "count on a "+ev+" trigger") {
			t.Errorf("%s: want the count refused, got %v", ev, err)
		}
	}
	for _, ev := range []string{"mob_death", "command", "item_gain", "spell_landed"} {
		if err := countQuest(ev).Validate(); err != nil {
			t.Errorf("count on %s refused: %v", ev, err)
		}
	}
}

// A count belongs to its own step: the trigger holds a step of its own quest
// (that step keys the count) and waits for the token it grants, so it
// counts on that step alone.
func TestValidateCountNeedsItsOwnStep(t *testing.T) {
	for _, c := range []struct {
		name         string
		has, missing []string
		want         string
	}{
		{"no has", nil, []string{"9001-end"}, "count needs a has naming a step of quest 9001"},
		{"has of another quest", []string{"77-start"}, []string{"9001-end"}, "count needs a has naming a step of quest 9001"},
		{"no missing", []string{"9001-start"}, nil, `count needs a missing naming the token its actions grant ("9001-end")`},
		{"missing another token", []string{"9001-start"}, []string{"77-end"}, `count needs a missing naming the token its actions grant ("9001-end")`},
	} {
		q := countQuest("mob_death")
		q.Triggers[0].Conditions = Conditions{Has: c.has, Missing: c.missing}
		if err := q.Validate(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want %q, got %v", c.name, c.want, err)
		}
	}
	q := countQuest("mob_death")
	q.Triggers[0].Actions = []ActionDef{{SetFlag: &QuestFlagAction{Key: "k", Value: "v"}}}
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "count needs an action that grants a step") {
		t.Errorf("a count that grants nothing: got %v", err)
	}
}

// CountStep is the latest own-quest step the trigger holds, by step order.
func TestCountStepAndKey(t *testing.T) {
	q := &Quest{QuestId: 53, Steps: []QuestStep{{Id: "start"}, {Id: "hunt"}, {Id: "end"}}}
	td := TriggerDef{Conditions: Conditions{Has: []string{"53-hunt", "7-end", "53-start"}}}
	if got := q.CountStep(td); got != "hunt" {
		t.Fatalf("CountStep = %q, want hunt", got)
	}
	if got := q.CountStep(TriggerDef{Conditions: Conditions{Has: []string{"7-end"}}}); got != "" {
		t.Fatalf("CountStep with no own-quest step = %q, want empty", got)
	}
	if got := CountKey(53, "hunt", 2); got != "53-hunt-t2" {
		t.Fatalf("CountKey = %q", got)
	}
}

// "<label>: N of M" fits 80 columns with the log's indent at 40 characters.
func TestValidateLabelLength(t *testing.T) {
	q := countQuest("mob_death")
	q.Triggers[0].Label = strings.Repeat("a", 40)
	if err := q.Validate(); err != nil {
		t.Fatalf("a 40-character label refused: %v", err)
	}
	q.Triggers[0].Label = strings.Repeat("a", 41)
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "40 characters or fewer") {
		t.Fatalf("a 41-character label must be refused, got %v", err)
	}
}

func TestValidateDistinct(t *testing.T) {
	for _, d := range []string{"all", "1", "12"} {
		q := validQuest(ActionDef{Grant: "9001-start"})
		q.Triggers[0].Event, q.Triggers[0].Area, q.Triggers[0].Distinct = "room_enter", "grove", d
		if err := q.Validate(); err != nil {
			t.Errorf("distinct %q refused: %v", d, err)
		}
	}
	for _, c := range []struct {
		event, area, zone, distinct, want string
	}{
		{"room_enter", "grove", "", "0", `distinct "0" must be all or a whole number of 1 or more`},
		{"room_enter", "grove", "", "most", `distinct "most" must be all or a whole number of 1 or more`},
		{"room_enter", "grove", "", "-1", `distinct "-1" must be all or a whole number of 1 or more`},
		{"mob_death", "grove", "", "all", "distinct needs a room_enter trigger"},
		{"room_enter", "", "", "all", "distinct needs an area"},
		{"room_enter", "", "Thornwall", "all", "distinct needs an area"},
	} {
		q := validQuest(ActionDef{Grant: "9001-start"})
		q.Triggers[0].Event, q.Triggers[0].Area, q.Triggers[0].Zone, q.Triggers[0].Distinct = c.event, c.area, c.zone, c.distinct
		if err := q.Validate(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: want %q, got %v", c, c.want, err)
		}
	}
}

func TestValidateLabelAndExploredLine(t *testing.T) {
	q := countQuest("mob_death")
	q.Triggers[0].Count, q.Triggers[0].Label = 3, "Roots gathered"
	if err := q.Validate(); err != nil {
		t.Fatalf("label with count refused: %v", err)
	}
	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Event, q.Triggers[0].Area, q.Triggers[0].Distinct = "room_enter", "grove", "all"
	q.Triggers[0].Label, q.Triggers[0].ExploredLine = "Rooms explored", "You know this grove well."
	if err := q.Validate(); err != nil {
		t.Fatalf("label and explored_line with distinct refused: %v", err)
	}
	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Label = "Progress"
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "label needs count or distinct") {
		t.Fatalf("a label alone must be refused, got %v", err)
	}
	q = countQuest("mob_death")
	q.Triggers[0].ExploredLine = "You know it."
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "explored_line needs distinct") {
		t.Fatalf("explored_line without distinct must be refused, got %v", err)
	}
}
