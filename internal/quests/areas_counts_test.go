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
		q := validQuest(ActionDef{Grant: "9001-start"})
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
	q := validQuest(ActionDef{Grant: "9001-start"})
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
	q = validQuest(ActionDef{Grant: "9001-start"})
	q.Triggers[0].Count, q.Triggers[0].ExploredLine = 3, "You know it."
	if err := q.Validate(); err == nil || !strings.Contains(err.Error(), "explored_line needs distinct") {
		t.Fatalf("explored_line without distinct must be refused, got %v", err)
	}
}
