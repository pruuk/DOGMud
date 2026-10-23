package questengine_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// TestNoQuestTriggersDifferOnlyByNounNumber fails when a quest has two
// triggers that share an event and every other filter (room, mob, item,
// skill, command, topic, verb) and whose nouns are singular/plural variants
// of each other ("pillar" and "pillars" in one room).
//
// The engine matches a typed noun against a trigger's noun exactly or as a
// number variant, so one spelling is enough: `look pillar` and `look pillars`
// both reach a `noun: pillar` trigger. Authoring both spellings is the old
// workaround for exact matching and is now redundant; merge them into one.
//
// Probe: the guard is proven to fire by TestNounPairsGuardFindsAPair below.
func TestNoQuestTriggersDifferOnlyByNounNumber(t *testing.T) {
	// Anchored on this file, not the working directory: test binaries do not
	// start in the package dir, and a path that resolves to nothing would
	// make this guard skip and pass for the wrong reason.
	_, thisFile, _, _ := runtime.Caller(0)
	dataPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "_datafiles", "world", "dogmud", "quests")
	if _, err := os.Stat(dataPath); err != nil {
		t.Fatalf("quest data path unreadable: %v", err)
	}
	quests, err := fileloader.LoadAllFlatFiles[int, *questengine.QuestDef](dataPath)
	if err != nil {
		t.Fatalf("failed to load quest definitions: %v", err)
	}
	if len(quests) < 50 {
		t.Fatalf("loaded only %d quests from %s; the guard is not seeing the data", len(quests), dataPath)
	}

	var problems []string
	for _, q := range quests {
		for _, p := range nounNumberPairs(q.Triggers) {
			problems = append(problems, fmt.Sprintf("quest %d (%s): triggers %d and %d (%s) use nouns %q and %q",
				q.QuestId, q.Name, p[0], p[1], q.Triggers[p[0]].Event, q.Triggers[p[0]].Noun, q.Triggers[p[1]].Noun))
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Errorf("%d trigger pairs differ only by noun number; merge each into one trigger:\n  %s",
			len(problems), strings.Join(problems, "\n  "))
	}
}

func TestNounPairsGuardFindsAPair(t *testing.T) {
	trigs := []questengine.TriggerDef{
		{Event: "room_interact", Room: 7, Noun: "pillars"},
		{Event: "room_interact", Room: 7, Noun: "pillar"},
		{Event: "room_interact", Room: 8, Noun: "pillar"},                // other room: fine
		{Event: "command_issued", Room: 7, Noun: "pillar"},               // other event: fine
		{Event: "room_interact", Room: 7, Noun: "column"},                // other noun: fine
		{Event: "room_interact", Room: 7, Verb: "touch", Noun: "pillar"}, // other verb: fine
	}
	got := nounNumberPairs(trigs)
	if len(got) != 1 || got[0] != [2]int{0, 1} {
		t.Fatalf("nounNumberPairs = %v, want [[0 1]]", got)
	}
}

// nounNumberPairs returns index pairs (i<j) of triggers that share every
// filter except Noun, with nouns that differ only by number.
func nounNumberPairs(trigs []questengine.TriggerDef) [][2]int {
	var out [][2]int
	for i := range trigs {
		for j := i + 1; j < len(trigs); j++ {
			a, b := &trigs[i], &trigs[j]
			if a.Event != b.Event || a.Room != b.Room || a.Mob != b.Mob || a.Item != b.Item ||
				a.Skill != b.Skill || a.Command != b.Command ||
				a.QuestToken != b.QuestToken || a.Verb != b.Verb {
				continue
			}
			if util.NounsDifferOnlyByNumber(a.Noun, b.Noun) {
				out = append(out, [2]int{i, j})
			}
		}
	}
	return out
}
