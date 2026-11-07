package questengine

import (
	"reflect"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
)

// Owner ruling 16: a mob list with no declared fallback is a finding only
// when none of its mobs has spawn rooms.
func TestDestinations_MobList(t *testing.T) {
	installDestWorld(t, destWorld{spawns: map[int][]int{61: {9}}})
	q := &quests.Quest{QuestId: 703, Name: "List", Steps: []quests.QuestStep{
		{Id: "second", Destination: quests.StepDestination{Mobs: []int{60, 61}}}, // only 61 spawns
		{Id: "none", Destination: quests.StepDestination{Mobs: []int{62, 63}}},   // neither spawns
		{Id: "end"},
	}}
	fs, _ := Destinations([]*quests.Quest{q})
	if len(fs) != 1 || fs[0].Key() != "walk-destination 703 step none" {
		t.Fatalf("want one finding for step none, got %+v", fs)
	}
	if !strings.Contains(fs[0].Msg, "62") || !strings.Contains(fs[0].Msg, "63") {
		t.Errorf("the finding must name every id: %q", fs[0].Msg)
	}
}

func TestDestinations_Findings(t *testing.T) {
	installDestWorld(t, destWorld{spawns: map[int][]int{60: {9}}})
	q := &quests.Quest{QuestId: 701, Name: "Rule", Steps: []quests.QuestStep{
		{Id: "start", Destination: quests.StepDestination{Room: 5}},
		{Id: "lost"}, // unknown
		{Id: "here", Destination: quests.StepDestination{Anywhere: true}},   // mistagged
		{Id: "wolf", Destination: quests.StepDestination{Mobs: []int{60}}},  // spawn rooms: fine
		{Id: "ghost", Destination: quests.StepDestination{Mobs: []int{61}}}, // no fallback, no spawns
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

// A spawn change on a running server is judged before and after: only the
// findings it adds come back (the admin spawninfo commands refuse on them).
func TestSpawnChangeFindings(t *testing.T) {
	installDestWorld(t, destWorld{spawns: map[int][]int{60: {9}, 61: {9, 10}}})
	q := &quests.Quest{QuestId: 704, Name: "Spawns", Steps: []quests.QuestStep{
		{Id: "wolf", Destination: quests.StepDestination{Mobs: []int{60}}}, // only room 9 spawns 60
		{Id: "hunt"}, // inferred from mob 60's trigger
		{Id: "fox", Destination: quests.StepDestination{Mobs: []int{61}}}, // rooms 9 and 10 spawn 61
		{Id: "lost"}, // already a finding
		{Id: "end"},
	}, Triggers: []quests.TriggerDef{
		{Event: "mob_death", Mob: 60, Conditions: quests.Conditions{Has: []string{"704-hunt"}}},
	}}
	qs := []*quests.Quest{q}
	keys := func(room int, mobs ...int) []string {
		var ks []string
		for _, f := range spawnChangeFindings(qs, room, mobs) {
			ks = append(ks, f.Key())
		}
		return ks
	}

	// Clearing room 9 strands 60 (both steps); 61 still spawns in room 10;
	// step lost was a finding before the change, so it is not returned.
	want := []string{"walk-destination 704 step hunt", "walk-destination 704 step wolf"}
	if got := keys(9); !reflect.DeepEqual(got, want) {
		t.Errorf("clear room 9: %v, want %v", got, want)
	}
	// Copying a list that keeps 60 in room 9 adds nothing.
	if got := keys(9, 60, 62); got != nil {
		t.Errorf("keep 60 in room 9: %v, want none", got)
	}
	// Clearing room 10 leaves 61 in room 9.
	if got := keys(10); got != nil {
		t.Errorf("clear room 10: %v, want none", got)
	}
	// The check leaves the spawn index alone.
	if len(spawnRoomsOf(60)) != 1 {
		t.Errorf("spawn index changed: %v", spawnRoomsOf(60))
	}
}
