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
	require.Contains(t, rules(Validate(w)), "one-action step firstkill:none")
}

// A step only a behaviour tree or an item questtoken grants (Hadwen's tree
// grants 30-rite) has a way in. The grant has no trigger conditions, so it
// is never a :prev or :cond finding: the topic that fires the tree, or
// getting the item, is its gate.
func TestOneAction_ExternalGrantIsAWayIn(t *testing.T) {
	for _, g := range []ExternalGrant{
		{Where: "mob 9112 tree grant_quest 53-firstkill", Token: "53-firstkill", MobId: 9112, Topics: []string{"hunt"}},
		{Where: "item 900 questtoken", Token: "53-firstkill", ItemId: 900},
	} {
		w := goodWorld()
		w.Quests[0].Triggers = nil
		w.External = []ExternalGrant{g}
		require.Empty(t, Validate(w), g.Where)
	}
}

func TestOneAction_MustRequirePreviousStep(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Conditions.Has = nil
	require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger mob_death mob=9110 grants=53-firstkill:prev")
}

func TestOneAction_TopicMustRequirePreviousStep(t *testing.T) {
	w := goodWorld()
	w.Dialogue[0].Topics[1].QuestRequired = nil
	require.Contains(t, rules(Validate(w)), "one-action mob 9112 topic cull_in:prev")
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
		require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger mob_death mob=9110 grants=53-firstkill:cond", name)
	}
}

func TestOneAction_DoubleGrantIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Actions = append(w.Quests[0].Triggers[0].Actions, quests.ActionDef{Grant: "53-end"})
	require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger mob_death mob=9110 grants=53-end,53-firstkill:double")
}

func TestOneAction_SequenceGrantIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Actions = []quests.ActionDef{{Sequence: &quests.SequenceDef{
		OnComplete: []quests.ActionDef{{Grant: "53-firstkill"}}}}}
	require.Contains(t, rules(Validate(w)), "one-action quest 53 trigger mob_death mob=9110 grants=53-firstkill:seq")
}

func TestOneAction_RewardGrantOfMiddleStepIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Rewards.QuestId = "53-firstkill"
	require.Contains(t, rules(Validate(w)), "one-action quest 53 rewards questid:reward")
}
