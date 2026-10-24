package questloop

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// goodWorld is one closed loop: Garve (mob 9112) offers quest 53, a kill
// trigger moves it to firstkill, and Garve takes the hand-in. It must
// produce no findings; each rule test breaks one thing.
func goodWorld() World {
	q := &quests.Quest{
		QuestId: 53, Name: "Cull the Wash",
		Steps: []quests.QuestStep{
			{Id: "start", Description: "d", Hint: "Type attack scout."},
			{Id: "firstkill", Description: "d", Hint: "Type talk garve."},
			{Id: "end", Description: "d"},
		},
		Triggers: []quests.TriggerDef{{
			Event: "mob_death", Mob: 9110,
			Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-firstkill"}},
			Actions:    []quests.ActionDef{{Grant: "53-firstkill"}},
		}},
	}
	df := &dialogue.DialogueFile{MobId: 9112, Zone: "pothole_coulee", Topics: []dialogue.Topic{
		{Key: "cull_offer", Kind: dialogue.KindOffer, Quest: 53, Label: "Cull the Wash", Text: "t",
			Effects: dialogue.Effects{GrantsQuest: "53-start"}},
		{Key: "cull_in", Kind: dialogue.KindHandIn, Quest: 53, Label: "Cull the Wash", Text: "t",
			Gates:   dialogue.Gates{QuestRequired: []string{"53-firstkill"}},
			Effects: dialogue.Effects{GrantsQuest: "53-end"}},
	}}
	return World{Quests: []*quests.Quest{q}, Dialogue: []*dialogue.DialogueFile{df}}
}

func rules(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Rule+" "+f.Where)
	}
	return out
}

func TestValidate_GoodWorldIsClean(t *testing.T) {
	require.Empty(t, Validate(goodWorld()))
}

func TestStart_TriggerGrantIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "room_enter", Room: 5, Actions: []quests.ActionDef{{Grant: "53-start"}}})
	require.Contains(t, rules(Validate(w)), "start quest 53 trigger 1")
}

func TestStart_RewardGrantIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Rewards.QuestId = "53-start"
	require.Contains(t, rules(Validate(w)), "start quest 53 rewards questid")
}

func TestStart_NoOfferIsRefused(t *testing.T) {
	w := goodWorld()
	w.Dialogue[0].Topics = w.Dialogue[0].Topics[1:]
	require.Contains(t, rules(Validate(w)), "start step 0 (start)")
}

func TestEnd_TriggerFinishIsRefused(t *testing.T) {
	w := goodWorld()
	w.Dialogue[0].Topics = w.Dialogue[0].Topics[:1]
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "mob_death", Mob: 9111,
		Conditions: quests.Conditions{Has: []string{"53-firstkill"}},
		Actions:    []quests.ActionDef{{Grant: "53-end"}}})
	got := rules(Validate(w))
	require.Contains(t, got, "end quest 53 trigger 1")
	require.Contains(t, got, "end step 2 (end)")
}

func TestEnd_OneStepQuestFailsByDesign(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Steps = []quests.QuestStep{{Id: "end"}}
	w.Quests[0].Triggers = nil
	w.Dialogue[0].Topics = w.Dialogue[0].Topics[:1]
	w.Dialogue[0].Topics[0].GrantsQuest = "53-end"
	got := rules(Validate(w))
	require.Contains(t, got, "end mob 9112 topic cull_offer", "an offer is not a hand-in")
}

func TestHint_MissingHintIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Steps[1].Hint = "  "
	require.Contains(t, rules(Validate(w)), "hint step 1 (firstkill)")
}

func TestHint_FinalStepNeedsNone(t *testing.T) {
	w := goodWorld()
	require.Empty(t, w.Quests[0].Steps[2].Hint)
	require.Empty(t, Validate(w))
}

func TestFinding_KeyIsRuleQuestWhere(t *testing.T) {
	f := Finding{Rule: "hint", QuestId: 53, Where: "step 1 (firstkill)", Msg: "no hint"}
	require.Equal(t, "hint 53 step 1 (firstkill)", f.Key())
	require.False(t, strings.Contains(f.Key(), "no hint"), "the message is not part of the key")
}
