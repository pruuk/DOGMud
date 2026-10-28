package questengine

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

func TestDestinations_Findings(t *testing.T) {
	installDestWorld(t, destWorld{spawns: map[int][]int{60: {9}}})
	q := &quests.Quest{QuestId: 701, Name: "Rule", Steps: []quests.QuestStep{
		{Id: "start", Destination: quests.StepDestination{Room: 5}},
		{Id: "lost"}, // unknown
		{Id: "here", Destination: quests.StepDestination{Anywhere: true}}, // mistagged
		{Id: "wolf", Destination: quests.StepDestination{Mob: 60}},        // spawn rooms: fine
		{Id: "ghost", Destination: quests.StepDestination{Mob: 61}},       // no fallback, no spawns
		{Id: "guess"}, // inferred
		{Id: "end"},   // final: skipped
	}, Triggers: []quests.TriggerDef{
		{Event: "room_enter", Room: 7, Conditions: quests.Conditions{Has: []string{"701-here"}}},
		{Event: "mob_death", Mob: 60, Conditions: quests.Conditions{Has: []string{"701-guess"}}},
	}}
	secret := &quests.Quest{QuestId: 702, Name: "S", Secret: true, Steps: []quests.QuestStep{{Id: "x"}, {Id: "end"}}}

	fs, inferred := Destinations([]*quests.Quest{secret, q})
	var keys []string
	for _, f := range fs {
		keys = append(keys, f.Key())
	}
	want := []string{
		"walk-destination 701 step ghost",
		"walk-destination 701 step lost",
		"walk-destination-anywhere 701 step here",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("findings %v, want %v", keys, want)
	}
	if !reflect.DeepEqual(inferred, []string{"quest 701 step guess"}) {
		t.Errorf("inferred %v", inferred)
	}
}
