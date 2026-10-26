package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"gopkg.in/yaml.v2"
)

// checkSearchTriggerNouns returns one message per quest trigger that listens
// for `search` on a noun its room does not have. A search trigger with a noun
// the room neither shows (a room noun, matched the way the engine matches it:
// rooms.Room.NounNamedBy, hyphen and plural folding included) nor hides (a
// hidden noun key) can never fire, and nothing else reports it. A trigger
// with no noun listens to the whole room and is not checked.
func checkSearchTriggerNouns(qs []quests.Quest, roomLookup func(int) *rooms.Room) []string {
	var bad []string
	for _, q := range qs {
		for i, tr := range q.Triggers {
			if tr.Event != "room_interact" || quests.InteractVerb(tr.Verb) != quests.VerbSearch || tr.Noun == "" {
				continue
			}
			where := fmt.Sprintf("quest %d (%s) trigger %d: search %q in room %d", q.QuestId, q.Name, i, tr.Noun, tr.Room)
			r := roomLookup(tr.Room)
			if r == nil {
				bad = append(bad, where+": the room does not exist")
				continue
			}
			if r.NounNamedBy(tr.Noun) != "" {
				continue
			}
			if _, _, ok := r.FindHiddenNoun(tr.Noun); ok {
				continue
			}
			bad = append(bad, where+": the room has no such noun or hidden noun")
		}
	}
	return bad
}

func countSearchTriggers(qs []quests.Quest) int {
	n := 0
	for _, q := range qs {
		for _, tr := range q.Triggers {
			if tr.Event == "room_interact" && quests.InteractVerb(tr.Verb) == quests.VerbSearch {
				n++
			}
		}
	}
	return n
}

// TestSearchTriggerNounsNameARoomNoun: every shipped `verb: search` trigger
// with a noun names a room noun or a hidden noun of its room.
func TestSearchTriggerNounsNameARoomNoun(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	rooms.LoadBiomeDataFiles()
	rooms.LoadDataFiles()

	paths, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "quests", "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("found no quest files (err %v): the guard cannot run", err)
	}
	var qs []quests.Quest
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var q quests.Quest
		if err := yaml.Unmarshal(data, &q); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		qs = append(qs, q)
	}

	// Floor: the guard must have looked at something. Task 9 moves quests 49
	// and 58 to search triggers, so a count of zero means the scan, not the
	// content, is broken.
	if n := countSearchTriggers(qs); n == 0 {
		t.Fatalf("found no shipped verb: search trigger in %d quest files: the guard checked nothing (Task 9 adds the first)", len(qs))
	}

	for _, msg := range checkSearchTriggerNouns(qs, rooms.LoadRoom) {
		t.Error(msg)
	}
}

// TestSearchTriggerNounGuard_CanFail proves the check reports a planted
// trigger whose noun the room lacks, and accepts a room noun, an alias and a
// hidden noun.
func TestSearchTriggerNounGuard_CanFail(t *testing.T) {
	room := &rooms.Room{
		Nouns:       map[string]string{"loose flagstones": "Uneven.", "stone": ":loose flagstones"},
		HiddenNouns: map[string]rooms.HiddenNoun{"cache": {}},
	}
	lookup := func(id int) *rooms.Room {
		if id == 1 {
			return room
		}
		return nil
	}
	mk := func(room int, noun string) quests.Quest {
		return quests.Quest{QuestId: 9, Name: "planted", Triggers: []quests.TriggerDef{
			{Event: "room_interact", Verb: "search", Room: room, Noun: noun}}}
	}

	if got := checkSearchTriggerNouns([]quests.Quest{mk(1, "chandelier")}, lookup); len(got) != 1 {
		t.Fatalf("planted bad noun not reported: %v", got)
	}
	if got := checkSearchTriggerNouns([]quests.Quest{mk(2, "cache")}, lookup); len(got) != 1 {
		t.Fatalf("planted missing room not reported: %v", got)
	}
	for _, ok := range []string{"loose flagstones", "stone", "cache"} {
		if got := checkSearchTriggerNouns([]quests.Quest{mk(1, ok)}, lookup); len(got) != 0 {
			t.Errorf("noun %q wrongly reported: %v", ok, got)
		}
	}
	// A look trigger is not this guard's business.
	look := quests.Quest{Triggers: []quests.TriggerDef{{Event: "room_interact", Room: 1, Noun: "chandelier"}}}
	if got := checkSearchTriggerNouns([]quests.Quest{look}, lookup); len(got) != 0 {
		t.Errorf("look trigger reported: %v", got)
	}
}
