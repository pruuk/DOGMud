package questloop

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
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

// forkedWorld is quest 11's shape (#958) as one clean loop: Kael (9300)
// offers quest 54; hearing Sylara's (9301) or Rhett's (9302) stand topic
// grants that side's fetch step, picking up the side's item its return step,
// and each NPC takes the hand-in of their own side's return step.
func forkedWorld() World {
	q := &quests.Quest{
		QuestId: 54, Name: "Forked Loop",
		Flags: []quests.QuestFlagDef{{Key: "branch", Values: []string{"sylara", "rhett"}}},
		Steps: []quests.QuestStep{
			{Id: "start", Description: "d", Hint: "Type talk sylara."},
			{Id: "sylara", Branch: "sylara", Description: "d", Hint: "Type get totem."},
			{Id: "totem", Branch: "sylara", Description: "d", Hint: "Type talk sylara."},
			{Id: "rhett", Branch: "rhett", Description: "d", Hint: "Type get sample."},
			{Id: "samples", Branch: "rhett", Description: "d", Hint: "Type talk rhett."},
			{Id: "end", Description: "d"},
		},
		Triggers: []quests.TriggerDef{
			{Event: "topic_heard", Mob: 9301, Topic: "stand",
				Conditions: quests.Conditions{Has: []string{"54-start"}, Missing: []string{"54-sylara"}},
				Actions:    []quests.ActionDef{{Grant: "54-sylara"}}},
			{Event: "item_gain", Item: 901,
				Conditions: quests.Conditions{Has: []string{"54-sylara"}, Missing: []string{"54-totem"}},
				Actions:    []quests.ActionDef{{Grant: "54-totem"}}},
			{Event: "topic_heard", Mob: 9302, Topic: "stand",
				Conditions: quests.Conditions{Has: []string{"54-start"}, Missing: []string{"54-rhett"}},
				Actions:    []quests.ActionDef{{Grant: "54-rhett"}}},
			{Event: "item_gain", Item: 902,
				Conditions: quests.Conditions{Has: []string{"54-rhett"}, Missing: []string{"54-samples"}},
				Actions:    []quests.ActionDef{{Grant: "54-samples"}}},
		},
	}
	kael := &dialogue.DialogueFile{MobId: 9300, Topics: []dialogue.Topic{
		{Key: "offer", Kind: dialogue.KindOffer, Quest: 54, Label: "Forked", Text: "t",
			Effects: dialogue.Effects{GrantsQuest: "54-start"}},
	}}
	handIn := func(mob int, need string) *dialogue.DialogueFile {
		return &dialogue.DialogueFile{MobId: mob, Topics: []dialogue.Topic{
			{Key: "stand", Kind: dialogue.KindInfo, Label: "Stand", Text: "t",
				Gates: dialogue.Gates{QuestRequired: []string{"54-start"}}},
			{Key: "hand_in", Kind: dialogue.KindHandIn, Quest: 54, Label: "Forked", Text: "t",
				Gates: dialogue.Gates{QuestRequired: []string{need}}, Effects: dialogue.Effects{GrantsQuest: "54-end"}},
		}}
	}
	return World{Quests: []*quests.Quest{q},
		Dialogue: []*dialogue.DialogueFile{kael, handIn(9301, "54-totem"), handIn(9302, "54-samples")}}
}

func oneAction(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		if f.Rule == "one-action" {
			out = append(out, f.Rule+" "+f.Where)
		}
	}
	return out
}

// A branched quest is checked one path at a time: each side's first step is
// reached from start, and the end from each side's return step.
func TestOneAction_EachPathIsItsOwnLine(t *testing.T) {
	require.Empty(t, oneAction(Validate(forkedWorld())))
}

func TestOneAction_SideWithNoWayOn(t *testing.T) {
	w := forkedWorld()
	w.Quests[0].Triggers = slices.Delete(w.Quests[0].Triggers, 2, 3) // Rhett's stand
	require.Equal(t, []string{"one-action step rhett:none"}, oneAction(Validate(w)))

	// A way into Rhett's first step from Sylara's step is no way from start.
	w = forkedWorld()
	w.Quests[0].Triggers[2].Conditions.Has = []string{"54-sylara"}
	require.Equal(t, []string{"one-action quest 54 trigger topic_heard mob=9302 topic=stand grants=54-rhett:prev"},
		oneAction(Validate(w)))
}

// The end joins both sides: a hand-in from only one side's return step
// leaves the other side with no way on.
func TestOneAction_JoinNeedsAWayFromEachSide(t *testing.T) {
	w := forkedWorld()
	w.Dialogue[2].Topics[1].QuestRequired = []string{"54-totem"} // Rhett takes Sylara's totem
	require.Equal(t, []string{"one-action step end from samples:none"}, oneAction(Validate(w)))

	// A hand-in requiring both sides' steps lies on no path.
	w = forkedWorld()
	w.Dialogue[2].Topics[1].QuestRequired = []string{"54-samples", "54-totem"}
	require.Equal(t, []string{"one-action mob 9302 topic hand_in:prev", "one-action step end from samples:none"},
		oneAction(Validate(w)))
}

func TestOneAction_RewardGrantOfMiddleStepIsRefused(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Rewards.QuestId = "53-firstkill"
	require.Contains(t, rules(Validate(w)), "one-action quest 53 rewards questid:reward")
}
