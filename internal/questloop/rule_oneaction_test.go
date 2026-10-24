package questloop

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

func TestOneAction_AlternativesPass(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{
		Event: "skill_use", Skill: "brawling",
		Conditions: quests.Conditions{Has: []string{"53-start"}},
		Actions:    []quests.ActionDef{{Grant: "53-firstkill"}}})
	require.Empty(t, Validate(w))
}

func TestOneAction_NoWayIn(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers = nil
	require.Contains(t, rules(Validate(w)), "one-action step 1 (firstkill)")
}

func TestOneAction_MustRequirePreviousStep(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Conditions.Has = nil
	require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger 0")
}

func TestOneAction_TopicMustRequirePreviousStep(t *testing.T) {
	w := goodWorld()
	w.Dialogue[0].Topics[1].QuestRequired = nil
	require.Contains(t, rules(Validate(w)), "one-action mob 9112 topic cull_in")
}

func TestOneAction_ExtraConditionsAreRefused(t *testing.T) {
	for name, c := range map[string]quests.Conditions{
		"has_item":     {HasItem: 7},
		"missing_item": {MissingItem: 7},
		"has_flag":     {HasFlag: map[string]string{"53-x": "y"}},
		"missing_flag": {MissingFlag: map[string]string{"53-x": "y"}},
		"has_gold":     {HasGold: 5},
	} {
		w := goodWorld()
		c.Has = []string{"53-start"}
		w.Quests[0].Triggers[0].Conditions = c
		require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger 0", name)
	}
}

func TestOneAction_DoubleGrantIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Actions = append(w.Quests[0].Triggers[0].Actions, quests.ActionDef{Grant: "53-end"})
	require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger 0")
}

func TestOneAction_SequenceGrantIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Actions = []quests.ActionDef{{Sequence: &quests.SequenceDef{
		OnComplete: []quests.ActionDef{{Grant: "53-firstkill"}}}}}
	require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger 0")
}

func TestOneAction_RewardGrantOfMiddleStepIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Rewards.QuestId = "53-firstkill"
	require.Contains(t, rules(Validate(w)), "one-action quest 53 rewards questid")
}
