package questloop

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// chainWorld adds quest 54, offered after 53 by a second NPC.
func chainWorld(offerMob int) World {
	w := goodWorld()
	q54 := &quests.Quest{QuestId: 54, Name: "Next", Steps: []quests.QuestStep{
		{Id: "start", Description: "d", Hint: "h"}, {Id: "end", Description: "d"}}}
	w.Quests = append(w.Quests, q54)
	offer := dialogue.Topic{Key: "next_offer", Kind: dialogue.KindOffer, Quest: 54, Label: "Next", Text: "t",
		Gates: dialogue.Gates{QuestRequired: []string{"53-end"}}, Effects: dialogue.Effects{GrantsQuest: "54-start"}}
	handin := dialogue.Topic{Key: "next_in", Kind: dialogue.KindHandIn, Quest: 54, Label: "Next", Text: "t",
		Gates: dialogue.Gates{QuestRequired: []string{"54-start"}}, Effects: dialogue.Effects{GrantsQuest: "54-end"}}
	if offerMob == 9112 {
		w.Dialogue[0].Topics = append(w.Dialogue[0].Topics, offer, handin)
	} else {
		w.Dialogue = append(w.Dialogue, &dialogue.DialogueFile{MobId: offerMob, Topics: []dialogue.Topic{offer, handin}})
	}
	return w
}

func TestChain_NextOfferOnTheHandInNPCPasses(t *testing.T) {
	require.Empty(t, Validate(chainWorld(9112)))
}

func TestChain_NextOfferElsewhereIsRefused(t *testing.T) {
	require.Contains(t, rules(Validate(chainWorld(9200))), "chain mob 9200 topic next_offer")
}

// A second door (#940): quest 54 is offered on the hand-in NPC 9112 and again
// on NPC 9200, both gated on 53-end.
func secondDoorWorld(handInMobOffers bool) World {
	w := chainWorld(9200)
	if handInMobOffers {
		offer := dialogue.Topic{Key: "next_offer", Kind: dialogue.KindOffer, Quest: 54, Label: "Next", Text: "t",
			Gates: dialogue.Gates{QuestRequired: []string{"53-end"}}, Effects: dialogue.Effects{GrantsQuest: "54-start"}}
		w.Dialogue[0].Topics = append(w.Dialogue[0].Topics, offer)
	}
	return w
}

func TestChain_SecondDoorBesideTheHandInOfferPasses(t *testing.T) {
	require.Empty(t, Validate(secondDoorWorld(true)))
}

func TestChain_SecondDoorWithNoHandInOfferIsRefused(t *testing.T) {
	require.Contains(t, rules(Validate(secondDoorWorld(false))), "chain mob 9200 topic next_offer")
}

// A sibling offer on a mob that takes only another quest's hand-in is no door
// the hand-in pitches from: the chain finding stays.
func TestChain_SiblingOnAnotherNonHandInMobIsRefused(t *testing.T) {
	w := secondDoorWorld(false)
	offer := dialogue.Topic{Key: "next_offer", Kind: dialogue.KindOffer, Quest: 54, Label: "Next", Text: "t",
		Gates: dialogue.Gates{QuestRequired: []string{"53-end"}}, Effects: dialogue.Effects{GrantsQuest: "54-start"}}
	other := dialogue.Topic{Key: "other_in", Kind: dialogue.KindHandIn, Quest: 54, Label: "Next", Text: "t",
		Gates: dialogue.Gates{QuestRequired: []string{"54-start"}}, Effects: dialogue.Effects{GrantsQuest: "54-end"}}
	// Mob 9300 offers quest 54 and takes quest 54's hand-in, not quest 53's.
	w.Dialogue = append(w.Dialogue, &dialogue.DialogueFile{MobId: 9300, Topics: []dialogue.Topic{offer, other}})
	found := rules(Validate(w))
	require.Contains(t, found, "chain mob 9200 topic next_offer")
	require.Contains(t, found, "chain mob 9300 topic next_offer")
}

func TestRepeat_Rules(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Repeatable = true
	require.Contains(t, rules(Validate(w)), "repeat quest 53:cooldown", "a repeatable needs a cooldown")
	w.Quests[0].CooldownRounds = 300
	require.Empty(t, Validate(w))
	w.Quests[0].RepeatLimit = -1
	require.Contains(t, rules(Validate(w)), "repeat quest 53:negative")
	w = goodWorld()
	w.Quests[0].RepeatLimit = 3
	require.Contains(t, rules(Validate(w)), "repeat quest 53:notrepeatable", "a limit on a quest that does not repeat")
}

func TestReward_OverrideMustNameWhatIsPaid(t *testing.T) {
	w := goodWorld()
	w.Names = quests.RewardNames{Item: func(int) string { return "Oak Bow" }}
	w.Quests[0].Rewards = quests.QuestReward{ItemId: 9002, Summary: "Garve will make it worth your while."}
	require.Contains(t, rules(Validate(w)), "reward quest 53 rewards summary")
	w.Quests[0].Rewards.Summary = "An oak bow, and Garve's thanks."
	require.Empty(t, Validate(w), "names are matched without case")
	w.Quests[0].Rewards.Summary = "Garve will make it worth your while."
	w.Quests[0].Rewards.Surprise = true
	require.Empty(t, Validate(w))
}

func TestReward_SurpriseNeedsSummary(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Rewards.Surprise = true
	require.Contains(t, rules(Validate(w)), "reward quest 53 rewards summary")
}

func TestRefs_RefusalsBecomeFindings(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Triggers[0].Mob = 424242
	v := quests.QuestValidators{MobExists: func(id int) bool { return id != 424242 }}
	fillValidators(&v)
	w.Refs = &v
	require.Contains(t, rules(Validate(w)), "refs quest 53 refs: trigger mob_death mob=424242 grants=53-firstkill mob filter: mob 424242 does not exist")
}

// fillValidators sets every unset validator func to an always-true one so
// ValidateQuestRefs can run.
func fillValidators(v *quests.QuestValidators) {
	yesI := func(int) bool { return true }
	yesS := func(string) bool { return true }
	if v.StepExists == nil {
		v.StepExists = yesS
	}
	if v.MobExists == nil {
		v.MobExists = yesI
	}
	if v.ItemExists == nil {
		v.ItemExists = yesI
	}
	if v.RoomExists == nil {
		v.RoomExists = yesI
	}
	if v.ConditionExists == nil {
		v.ConditionExists = yesI
	}
	if v.SpellExists == nil {
		v.SpellExists = yesS
	}
	if v.EffectExists == nil {
		v.EffectExists = yesS
	}
	if v.SkillExists == nil {
		v.SkillExists = yesS
	}
	if v.StatExists == nil {
		v.StatExists = yesS
	}
	if v.RecipeExists == nil {
		v.RecipeExists = yesS
	}
	if v.FactionExists == nil {
		v.FactionExists = yesS
	}
	if v.FlagDeclared == nil {
		v.FlagDeclared = func(string, string) bool { return true }
	}
	if v.DialogueGrants == nil {
		v.DialogueGrants = yesS
	}
	if v.MobHasDialogue == nil {
		v.MobHasDialogue = yesI
	}
	if v.TopicExists == nil {
		v.TopicExists = func(int, string) bool { return true }
	}
	if v.TopicGrant == nil {
		v.TopicGrant = func(int, string) string { return "" }
	}
}
