package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// fakeQuestWorld is an in-memory stand-in for the quests package + engine
// re-index, mirroring fakeMobWorld / fakeDialogueWorld.
type fakeQuestWorld struct {
	specs     map[int]*quests.Quest
	saved     []quests.Quest
	deleted   []int
	reindexed int
	refs      []questRefEntry
	nextId    int
}

func newFakeQuestWorld() *fakeQuestWorld {
	return &fakeQuestWorld{specs: map[int]*quests.Quest{}, nextId: 90000}
}

func fakeQuest(id int, name string) *quests.Quest {
	return &quests.Quest{QuestId: id, Name: name, Steps: []quests.QuestStep{
		{Id: "start"}, {Id: "end"},
	}, Triggers: []quests.TriggerDef{
		{Event: "room_enter", Room: 1, Actions: []quests.ActionDef{{Grant: "%d-start"}}},
	}}
}

func (w *fakeQuestWorld) deps() questDeps {
	v := quests.QuestValidators{
		StepExists:      func(string) bool { return true },
		MobExists:       func(int) bool { return true },
		ItemExists:      func(id int) bool { return id != 888 },
		RoomExists:      func(int) bool { return true },
		ConditionExists: func(int) bool { return true },
		SpellExists:     func(string) bool { return true },
		EffectExists:    func(string) bool { return true },
		SkillExists:     func(string) bool { return true },
		StatExists:      func(string) bool { return true },
		RecipeExists:    func(string) bool { return true },
		FactionExists:   func(string) bool { return true },
		FlagDeclared:    func(string, string) bool { return true },
		DialogueGrants:  func(string) bool { return false },
		MobHasDialogue:  func(int) bool { return true },
		TopicExists:     func(int, string) bool { return true },
		TopicGrant:      func(int, string) string { return "" },
	}
	return questDeps{
		load: func(id int) *quests.Quest { return w.specs[id] },
		all: func() []quests.Quest {
			out := []quests.Quest{}
			for _, q := range w.specs {
				out = append(out, *q)
			}
			return out
		},
		save: func(q quests.Quest) error {
			w.saved = append(w.saved, q)
			cp := q
			w.specs[q.QuestId] = &cp
			return nil
		},
		create: func(name string) (int, error) {
			w.nextId++
			w.specs[w.nextId] = fakeQuest(w.nextId, name)
			return w.nextId, nil
		},
		del:        func(id int) error { w.deleted = append(w.deleted, id); delete(w.specs, id); return nil },
		references: func(id int) []questRefEntry { return w.refs },
		reindex:    func() { w.reindexed++ },
		validators: v,
	}
}

func validUpdateReq(id int) quests.Quest {
	q := *fakeQuest(id, "Handler Test")
	q.Triggers[0].Actions[0].Grant = "" // no grant → orphan-step warnings expected
	return q
}

func TestBuildQuestUpdate_RefusesInvalidSavesNothing(t *testing.T) {
	w := newFakeQuestWorld()
	w.specs[90001] = fakeQuest(90001, "Handler Test")

	q := validUpdateReq(90001)
	q.Rewards.ItemId = 888 // fake says it doesn't exist
	res := buildQuestUpdate(w.deps(), q)
	if res.Ok {
		t.Fatal("bad registry ref must be refused")
	}
	if len(w.saved) != 0 || w.reindexed != 0 {
		t.Fatalf("refused update must not save (%d) or reindex (%d)", len(w.saved), w.reindexed)
	}
}

// TestBuildQuestUpdate_RefusesSubjectlessRoomText guards the editor path to a
// boot failure. An observer line that names no one used to pass every save-time
// check, get written to disk, and panic the reindex; the listener recovered,
// the admin got no reply, and the next cold boot failed. It must be refused at
// save, with nothing written.
func TestBuildQuestUpdate_RefusesSubjectlessRoomText(t *testing.T) {
	w := newFakeQuestWorld()
	w.specs[90005] = fakeQuest(90005, "Handler Test")

	q := validUpdateReq(90005)
	q.Triggers[0].Actions = append(q.Triggers[0].Actions, quests.ActionDef{RoomText: "unlocks the strongbox."})
	res := buildQuestUpdate(w.deps(), q)
	if res.Ok {
		t.Fatal("an observer line without {actor} must be refused at save")
	}
	if len(w.saved) != 0 || w.reindexed != 0 {
		t.Fatalf("refused update must not save (%d) or reindex (%d)", len(w.saved), w.reindexed)
	}
}

func TestBuildQuestUpdate_WarnOnlySavesWithWarnings(t *testing.T) {
	w := newFakeQuestWorld()
	w.specs[90002] = fakeQuest(90002, "Handler Test")

	res := buildQuestUpdate(w.deps(), validUpdateReq(90002))
	if !res.Ok {
		t.Fatalf("warning-only update should save, got %+v", res)
	}
	if len(w.saved) != 1 || w.reindexed != 1 {
		t.Fatalf("expected 1 save + 1 reindex, got %d/%d", len(w.saved), w.reindexed)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("orphan steps should surface as warnings")
	}
}

func TestBuildQuestListAndGet(t *testing.T) {
	w := newFakeQuestWorld()
	w.specs[90003] = fakeQuest(90003, "Visible Quest")
	w.specs[1000000] = fakeQuest(1000000, "Generic Template")

	rows := buildQuestList(w.deps())
	if len(rows) != 1 || rows[0].Id != 90003 || rows[0].StepCount != 2 || rows[0].TriggerCount != 1 {
		t.Fatalf("list shape wrong (template id must be hidden): %+v", rows)
	}

	detail, ok := buildQuestGet(w.deps(), 90003)
	if !ok || detail.Quest.QuestId != 90003 {
		t.Fatalf("get failed: %+v", detail)
	}
	// 11 events (room_gate added 2026-10-17, dialogue retired with ask in
	// #375, spell_landed added in #375 phase 3b), 9 condition kinds, 24
	// action types (discover added in #375 phase 3a) — one vocab entry per
	// TriggerDef/Conditions/ActionDef field (count them, don't estimate).
	offersSpellLanded := false
	for _, e := range detail.Enums.Events {
		offersSpellLanded = offersSpellLanded || e.Key == "spell_landed"
	}
	if !offersSpellLanded {
		t.Fatalf("the builder's event picker must offer spell_landed: %+v", detail.Enums.Events)
	}
	if len(detail.Enums.Events) != 11 || len(detail.Enums.Actions) != 24 || len(detail.Enums.Conditions) != 9 {
		t.Fatalf("vocabulary sizes wrong: events=%d conditions=%d actions=%d",
			len(detail.Enums.Events), len(detail.Enums.Conditions), len(detail.Enums.Actions))
	}
}

func TestBuildQuestCreate_ReindexesAndReturnsId(t *testing.T) {
	w := newFakeQuestWorld()
	res := buildQuestCreate(w.deps(), "Fresh Quest")
	if !res.Ok || res.QuestId == 0 {
		t.Fatalf("create should return an id, got %+v", res)
	}
	if w.reindexed != 1 {
		t.Fatalf("create must reindex once, got %d", w.reindexed)
	}
}

func TestBuildQuestDelete_GuardAndClean(t *testing.T) {
	w := newFakeQuestWorld()
	w.specs[90004] = fakeQuest(90004, "Guarded Quest")
	w.refs = []questRefEntry{{Kind: "dialogue", Where: "mob 9 (townzone)", Detail: `grantsQuest "90004-start"`}}

	if res := buildQuestDelete(w.deps(), 90004); res.Ok {
		t.Fatal("referenced quest must not delete")
	}
	if len(w.deleted) != 0 {
		t.Fatal("guarded delete must not reach deps.del")
	}

	w.refs = nil
	res := buildQuestDelete(w.deps(), 90004)
	if !res.Ok || len(w.deleted) != 1 || w.reindexed != 1 {
		t.Fatalf("clean delete should remove + reindex: %+v deleted=%v reindexed=%d", res, w.deleted, w.reindexed)
	}
}

// A dialogue file with no opener (Opener is a POINTER, nil when absent) must
// walk without panicking. Caught by the 5c E2E on the old tree pointer: the
// fake deps in the tests above bypass the real walker.
func TestWalkDialogueGates_NilOpener(t *testing.T) {
	df := &dialogue.DialogueFile{MobId: 9, Topics: []dialogue.Topic{{Key: "job", Effects: dialogue.Effects{GrantsQuest: "10-start"}}}}
	seen := 0
	walkDialogueGates(df, func(where string, g dialogueGate) { seen++ })
	if seen != 1 {
		t.Fatalf("expected 1 gate visit for the topic, got %d", seen)
	}
}

// Every topic and every opener variant is walked, and each reference to the
// quest is reported: the topic's quest id, its grant, its gates and the flag
// it sets, and a variant's gates. Another quest's tokens are not.
func TestWalkDialogueGates_TopicsAndVariantsReportRefs(t *testing.T) {
	df := &dialogue.DialogueFile{
		MobId: 9,
		Opener: &dialogue.Opener{Text: "Hello.", Variants: []dialogue.OpenerVariant{
			{Gates: dialogue.Gates{QuestRequired: []string{"10-start"}, QuestFlagExcluded: map[string]string{"10-door": "open"}}, Text: "Back again."},
		}},
		Topics: []dialogue.Topic{
			{Key: "job", Kind: dialogue.KindOffer, Quest: 10,
				Gates:   dialogue.Gates{QuestExcluded: []string{"10-start", "10-end"}, QuestRequired: []string{"7-end"}},
				Effects: dialogue.Effects{GrantsQuest: "10-start", SetsQuestFlag: &dialogue.QuestFlagSet{Key: "10-door", Value: "shut"}}},
			{Key: "weather", Kind: dialogue.KindInfo},
		},
	}
	got := map[string][]string{}
	walkDialogueGates(df, func(where string, g dialogueGate) { got[where] = append(got[where], g.refs(10)...) })

	want := map[string][]string{
		"topic job":        {`quest 10`, `grantsQuest "10-start"`, `questExcluded "10-start"`, `questExcluded "10-end"`, `setsQuestFlag "10-door"`},
		"topic weather":    nil,
		"opener variant 1": {`questRequired "10-start"`, `questFlagExcluded "10-door"`},
	}
	if len(got) != len(want) {
		t.Fatalf("visited %v, want the keys of %v", got, want)
	}
	for where, refs := range want {
		g, ok := got[where]
		if !ok {
			t.Fatalf("%s was not visited: %v", where, got)
		}
		if len(g) != len(refs) {
			t.Fatalf("%s refs = %v, want %v", where, g, refs)
		}
		for i := range refs {
			if g[i] != refs[i] {
				t.Fatalf("%s refs = %v, want %v", where, g, refs)
			}
		}
	}
}
