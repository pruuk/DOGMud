package gmcp

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/questworld"
)

// The editor door with the REAL loop rules over a fixture world: Garve (mob
// 9112) offers and takes both 90010 and 90011, and 90011's kill trigger
// also needs 90010-firstkill. The fixture is clean unless a test breaks it.

func loopFixtureQuests() (q10, q11 *quests.Quest) {
	q10 = &quests.Quest{QuestId: 90010, Name: "Cull", Steps: []quests.QuestStep{
		{Id: "start", Description: "d", Hint: "h"}, {Id: "firstkill", Description: "d", Hint: "h"}, {Id: "end", Description: "d"}},
		Triggers: []quests.TriggerDef{{Event: "mob_death", Mob: 9110,
			Conditions: quests.Conditions{Has: []string{"90010-start"}},
			Actions:    []quests.ActionDef{{Grant: "90010-firstkill"}}}}}
	q11 = &quests.Quest{QuestId: 90011, Name: "Wolves", Steps: []quests.QuestStep{
		{Id: "start", Description: "d", Hint: "h"}, {Id: "wolf", Description: "d", Hint: "h"}, {Id: "end", Description: "d"}},
		Triggers: []quests.TriggerDef{{Event: "mob_death", Mob: 9111,
			Conditions: quests.Conditions{Has: []string{"90011-start", "90010-firstkill"}},
			Actions:    []quests.ActionDef{{Grant: "90011-wolf"}}}}}
	return q10, q11
}

func loopFixtureDialogue() []*dialogue.DialogueFile {
	topic := func(key, kind string, quest int, req, grant string) dialogue.Topic {
		t := dialogue.Topic{Key: key, Kind: kind, Quest: quest, Label: key, Text: "t",
			Effects: dialogue.Effects{GrantsQuest: grant}}
		if req != "" {
			t.QuestRequired = []string{req}
		}
		return t
	}
	return []*dialogue.DialogueFile{{MobId: 9112, Topics: []dialogue.Topic{
		topic("cull_offer", dialogue.KindOffer, 90010, "", "90010-start"),
		topic("cull_in", dialogue.KindHandIn, 90010, "90010-firstkill", "90010-end"),
		topic("wolf_offer", dialogue.KindOffer, 90011, "90010-end", "90011-start"),
		topic("wolf_in", dialogue.KindHandIn, 90011, "90011-wolf", "90011-end"),
	}}}
}

// loopFixture is the fake quest store and its loop world; edit q10 or q11
// before calling it to start from a broken world.
func loopFixture(t *testing.T, q10, q11 *quests.Quest, backlog questloop.Backlog) (*fakeQuestWorld, questDeps) {
	t.Helper()
	w := newFakeQuestWorld()
	w.specs[q10.QuestId] = q10
	w.specs[q11.QuestId] = q11
	d := w.deps()
	useLoopWorld(&d, func(v quests.QuestValidators) questloop.World {
		qs := []*quests.Quest{}
		for _, id := range []int{90010, 90011} {
			cp := *w.specs[id]
			qs = append(qs, &cp)
		}
		return questloop.World{Quests: qs, Dialogue: loopFixtureDialogue(), Refs: &v}
	})
	d.backlog = func() (questloop.Backlog, error) { return backlog, nil }
	return w, d
}

func TestLoopFixture_IsClean(t *testing.T) {
	q10, q11 := loopFixtureQuests()
	v := newFakeQuestWorld().deps().validators
	w := questloop.World{Quests: []*quests.Quest{q10, q11}, Dialogue: loopFixtureDialogue(), Refs: &v}
	if fs := questloop.Validate(w); len(fs) != 0 {
		t.Fatalf("the fixture must be clean, got %v", fs)
	}
}

func editedQ10(mod func(q *quests.Quest)) quests.Quest {
	q10, _ := loopFixtureQuests()
	mod(q10)
	return *q10
}

func TestBuildQuestUpdate_RealRules_RefusesWhatTheEditBreaks(t *testing.T) {
	q10, q11 := loopFixtureQuests()
	w, d := loopFixture(t, q10, q11, questloop.Backlog{})
	res := buildQuestUpdate(d, editedQ10(func(q *quests.Quest) { q.Steps[1].Hint = "" }))
	if res.Ok {
		t.Fatal("removing a hint must refuse the save")
	}
	if !strings.Contains(res.Error, "hint 90010 step firstkill") {
		t.Fatalf("error must name the finding, got %q", res.Error)
	}
	if len(w.saved) != 0 || w.reindexed != 0 {
		t.Fatalf("refused update must not save (%d) or reindex (%d)", len(w.saved), w.reindexed)
	}
}

func TestBuildQuestUpdate_RealRules_BacklogFindingSavesWithAWarning(t *testing.T) {
	q10, q11 := loopFixtureQuests()
	_, d := loopFixture(t, q10, q11, questloop.Backlog{"hint 90010 step firstkill": true})
	res := buildQuestUpdate(d, editedQ10(func(q *quests.Quest) { q.Steps[1].Hint = "" }))
	if !res.Ok {
		t.Fatalf("a backlog finding must not refuse the save, got %+v", res)
	}
	if !anyContains(res.Warnings, "hint 90010 step firstkill") {
		t.Fatalf("the backlog finding must be listed in Warnings, got %v", res.Warnings)
	}
}

// A finding the world already had on ANOTHER quest neither refuses this
// save nor shows up in its warnings.
func TestBuildQuestUpdate_RealRules_OtherQuestsFindingsAreNotThisSaves(t *testing.T) {
	q10, q11 := loopFixtureQuests()
	q11.Steps[1].Hint = ""
	_, d := loopFixture(t, q10, q11, questloop.Backlog{})
	res := buildQuestUpdate(d, editedQ10(func(q *quests.Quest) { q.Name = "Cull the Wash" }))
	if !res.Ok {
		t.Fatalf("a finding on another quest must not refuse this save, got %+v", res)
	}
	if anyContains(res.Warnings, "90011") {
		t.Fatalf("another quest's finding must not be warned about here, got %v", res.Warnings)
	}
}

// A finding this quest already had is warned about, not refused.
func TestBuildQuestUpdate_RealRules_OwnStandingFindingWarns(t *testing.T) {
	q10, q11 := loopFixtureQuests()
	q10.Steps[0].Hint = ""
	_, d := loopFixture(t, q10, q11, questloop.Backlog{})
	res := buildQuestUpdate(d, editedQ10(func(q *quests.Quest) { q.Steps[0].Hint = ""; q.Name = "Cull the Wash" }))
	if !res.Ok {
		t.Fatalf("a finding the edit did not add must not refuse, got %+v", res)
	}
	if !anyContains(res.Warnings, "hint 90010 step start") {
		t.Fatalf("the quest's own standing finding must be warned about, got %v", res.Warnings)
	}
}

// Renaming a step another quest needs refuses, naming the other quest's
// refs finding (the registry still knows the old step).
func TestBuildQuestUpdate_RealRules_StepRenameThatBreaksAnotherQuest(t *testing.T) {
	q10, q11 := loopFixtureQuests()
	_, d := loopFixture(t, q10, q11, questloop.Backlog{})
	res := buildQuestUpdate(d, editedQ10(func(q *quests.Quest) {
		q.Steps[1].Id = "scout"
		q.Triggers[0].Actions[0].Grant = "90010-scout"
	}))
	if res.Ok {
		t.Fatal("a rename that breaks quest 90011 must refuse")
	}
	if !strings.Contains(res.Error, `refs 90011 quest 90011 refs: trigger mob_death mob=9111 grants=90011-wolf has: token "90010-firstkill"`) {
		t.Fatalf("error must name quest 90011's refs finding, got %q", res.Error)
	}
}

// The door hands loopWorld the deps' own validators, so they are built once.
func TestBuildQuestUpdate_LoopWorldGetsTheDepsValidators(t *testing.T) {
	q10, q11 := loopFixtureQuests()
	_, d := loopFixture(t, q10, q11, questloop.Backlog{})
	inner := d.loopWorld
	calls := 0
	d.loopWorld = func(v quests.QuestValidators) questloop.World {
		calls++
		if v.ItemExists(888) {
			t.Fatal("loopWorld must receive d.validators (the fake refuses item 888)")
		}
		return inner(v)
	}
	if res := buildQuestUpdate(d, *q10); !res.Ok {
		t.Fatalf("clean edit must save, got %+v", res)
	}
	if calls != 1 {
		t.Fatalf("the world must be built once per save, built %d times", calls)
	}
}

func useLoopWorld(d *questDeps, lw func(v quests.QuestValidators) questloop.World) {
	d.loopWorld = lw
}

func anyContains(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

// realQuestDeps wires the shipped world builder.
var _ func(quests.QuestValidators) questloop.World = questworld.RegistryWith
