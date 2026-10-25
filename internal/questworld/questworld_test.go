package questworld

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// liveRefs stands in for Validators(): its step and flag checks read the
// quests as they were loaded, the way the live registry does.
func liveRefs(loaded []*quests.Quest) *quests.QuestValidators {
	yesI := func(int) bool { return true }
	yesS := func(string) bool { return true }
	steps := map[string]bool{}
	flags := map[string][]string{}
	for _, q := range loaded {
		for _, s := range q.Steps {
			steps[fmt.Sprintf("%d-%s", q.QuestId, s.Id)] = true
		}
		for _, f := range q.Flags {
			flags[fmt.Sprintf("%d-%s", q.QuestId, f.Key)] = f.Values
		}
	}
	return &quests.QuestValidators{
		StepExists: func(tok string) bool { return steps[tok] },
		FlagDeclared: func(key, value string) bool {
			vals, ok := flags[key]
			if !ok {
				return false
			}
			if value == "" {
				return true
			}
			for _, v := range vals {
				if v == value {
					return true
				}
			}
			return false
		},
		MobExists: yesI, ItemExists: yesI, RoomExists: yesI, ConditionExists: yesI,
		SpellExists: yesS, SkillExists: yesS, StatExists: yesS, RecipeExists: yesS,
		FactionExists: yesS, DialogueGrants: yesS, MobHasDialogue: yesI,
	}
}

func q53() *quests.Quest {
	return &quests.Quest{QuestId: 53, Name: "Cull", Steps: []quests.QuestStep{
		{Id: "start", Hint: "h"}, {Id: "firstkill", Hint: "h"}, {Id: "end"}},
		Flags: []quests.QuestFlagDef{{Key: "mood", Values: []string{"calm", "wild"}}}}
}

// q54's trigger needs 53-firstkill and 53's mood flag.
func q54() *quests.Quest {
	return &quests.Quest{QuestId: 54, Name: "Next", Steps: []quests.QuestStep{
		{Id: "start", Hint: "h"}, {Id: "end"}},
		Triggers: []quests.TriggerDef{{Event: "mob_death", Mob: 9110,
			Conditions: quests.Conditions{Has: []string{"54-start", "53-firstkill"},
				HasFlag: map[string]string{"53-mood": "calm"}},
			Actions: []quests.ActionDef{{Grant: "54-end"}}}}}
}

func world() questloop.World {
	qs := []*quests.Quest{q53(), q54()}
	return questloop.World{Quests: qs, Refs: liveRefs(qs)}
}

func refsFindings(fs []questloop.Finding, questId int) []string {
	var out []string
	for _, f := range fs {
		if f.Rule == "refs" && f.QuestId == questId {
			out = append(out, f.Key())
		}
	}
	return out
}

func TestWithQuest_ReplacesByIdKeepingOrder(t *testing.T) {
	w := world()
	e := *q53()
	e.Name = "Edited"
	out := WithQuest(w, e)
	require.Len(t, out.Quests, 2)
	require.Equal(t, "Edited", out.Quests[0].Name)
	require.Equal(t, 54, out.Quests[1].QuestId)
	require.Equal(t, "Cull", w.Quests[0].Name, "the input world is not changed")
}

func TestWithQuest_AddsANewId(t *testing.T) {
	out := WithQuest(world(), quests.Quest{QuestId: 77, Steps: []quests.QuestStep{{Id: "start"}}})
	require.Len(t, out.Quests, 3)
	require.Equal(t, 77, out.Quests[2].QuestId)
	require.True(t, out.Refs.StepExists("77-start"), "a new quest's steps resolve")
}

// The editor's proposed world keeps the external grants, so a save is
// judged with the same ways in as boot and CI.
func TestWithQuest_KeepsTheExternalGrants(t *testing.T) {
	w := world()
	w.External = []questloop.ExternalGrant{{Where: "mob 9100 tree grant_quest 53-firstkill", Token: "53-firstkill", MobId: 9100}}
	require.Equal(t, w.External, WithQuest(w, *q53()).External)
}

func TestWithQuest_NilRefsStaysNil(t *testing.T) {
	w := world()
	w.Refs = nil
	require.Nil(t, WithQuest(w, *q53()).Refs)
}

// Renaming a step another quest needs must show up as that quest's refs
// finding in the proposed world (the live registry still has the old step).
func TestWithQuest_RefsSeeTheEditedSteps(t *testing.T) {
	w := world()
	require.Empty(t, refsFindings(questloop.Validate(w), 54), "the loaded world is clean")

	e := *q53()
	e.Steps = []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "scout", Hint: "h"}, {Id: "end"}}
	got := refsFindings(questloop.Validate(WithQuest(w, e)), 54)
	require.Len(t, got, 1, "%v", got)
	require.True(t, strings.Contains(got[0], "53-firstkill"), got[0])
	require.True(t, w.Refs.StepExists("53-firstkill"), "the shared validators are not changed")
}

// Dropping a flag value another quest reads is the same kind of break.
func TestWithQuest_RefsSeeTheEditedFlags(t *testing.T) {
	w := world()
	e := *q53()
	e.Flags = []quests.QuestFlagDef{{Key: "mood", Values: []string{"wild"}}}
	got := refsFindings(questloop.Validate(WithQuest(w, e)), 54)
	require.Len(t, got, 1, "%v", got)
	require.True(t, strings.Contains(got[0], "53-mood"), got[0])
}
