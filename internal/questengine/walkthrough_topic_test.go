package questengine

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/stretchr/testify/require"
)

// topicWorld is a talk step on a topic_heard trigger (#375 phase 3b): Garve
// (9112) offers quest 53, hearing Nell's (9200) "rumours" topic moves it to
// asked, a kill that needs the route rumours sets moves it to went, and
// Garve takes the hand-in.
func topicWorld() questloop.World {
	q := &quests.Quest{
		QuestId: 53, Name: "Cull the Wash",
		Steps: []quests.QuestStep{{Id: "start", Hint: "h"}, {Id: "asked", Hint: "h"}, {Id: "went", Hint: "h"}, {Id: "end"}},
		Triggers: []quests.TriggerDef{
			{Event: "topic_heard", Mob: 9200, Topic: "rumours",
				Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-asked"}},
				Actions:    []quests.ActionDef{{Grant: "53-asked"}}},
			{Event: "mob_death", Mob: 9110,
				Conditions: quests.Conditions{Has: []string{"53-asked"}, Missing: []string{"53-went"},
					HasFlag: map[string]string{"53-route": "north"}},
				Actions: []quests.ActionDef{{Grant: "53-went"}}},
		},
	}
	garve := &dialogue.DialogueFile{MobId: 9112, Topics: []dialogue.Topic{
		{Key: "cull_offer", Kind: dialogue.KindOffer, Quest: 53, Label: "Cull", Text: "t",
			Effects: dialogue.Effects{GrantsQuest: "53-start"}},
		{Key: "cull_in", Kind: dialogue.KindHandIn, Quest: 53, Label: "Cull", Text: "t",
			Gates: dialogue.Gates{QuestRequired: []string{"53-went"}}, Effects: dialogue.Effects{GrantsQuest: "53-end"}},
	}}
	nell := &dialogue.DialogueFile{MobId: 9200, Topics: []dialogue.Topic{
		{Key: "rumours", Kind: dialogue.KindInfo, Label: "Rumours", Text: "t",
			Gates:   dialogue.Gates{QuestRequired: []string{"53-start"}},
			Effects: dialogue.Effects{SetsQuestFlag: &dialogue.QuestFlagSet{Key: "53-route", Value: "north"}}},
	}}
	return questloop.World{Quests: []*quests.Quest{q}, Dialogue: []*dialogue.DialogueFile{garve, nell}}
}

// The walk hears the trigger's topic first: its gates met, its effects
// applied (the route flag the kill needs), then the event sent.
func TestWalk_TopicHeardTriggerHearsItsTopic(t *testing.T) {
	w := topicWorld()
	require.Empty(t, Walk(w, questloop.Sources{}))

	// Firing the trigger Way itself lands the topic's effects: the route is
	// set by hearing rumours, not by the flag search.
	env := newWalkEnv(NewEngine(), questloop.Sources{}, w.Quests, w.Dialogue)
	env.eng.RegisterQuest(w.Quests[0])
	p := newWalkPlayer(stepQuests(map[int][]string{53: {"start", "asked", "went", "end"}}))
	p.grant("53-start")
	var asked questloop.Way
	for _, wy := range questloop.WaysInto(w)["53-asked"] {
		asked = wy
	}
	require.NotNil(t, asked.Trigger)
	trail := fireWay(env, p, asked, 53)
	require.Equal(t, "asked", p.progress[53])
	require.True(t, trail["53-asked"], "%v", trail)
	require.Equal(t, "north", p.flags["53-route"], "the topic's setsQuestFlag landed")
	require.True(t, p.heard[dialogue.HeardKey(9200, "rumours")], "the info topic is marked heard")
}

// A topic that can never be heard (gated on a token nothing grants) leaves
// its topic_heard step dead.
func TestWalk_TopicHeardOnATopicThatNeverOpensIsADeadEnd(t *testing.T) {
	w := topicWorld()
	w.Dialogue[1].Topics[0].QuestRequired = []string{"53-secret"}
	got := keys(Walk(w, questloop.Sources{}))
	require.Contains(t, got, "walk-dead-end 53 step asked")
	require.Contains(t, got, "walk-dead-end 53 quest 53 trigger topic_heard mob=9200 topic=rumours grants=53-asked")

	// A topic the NPC does not have can never be heard either.
	w = topicWorld()
	w.Quests[0].Triggers[0].Topic = "gossip"
	require.Contains(t, keys(Walk(w, questloop.Sources{})), "walk-dead-end 53 step asked")
}

// The game raises topic_heard for every played topic, so a trigger may
// listen to a topic that is itself a Way in: playing Garve's "boast" (which
// grants firstkill) also fires the trigger that sets the flag the hand-in
// needs.
func TestWalk_PlayedTopicRaisesTopicHeard(t *testing.T) {
	w := walkWorld()
	w.Quests[0].Triggers = []quests.TriggerDef{{
		Event: "topic_heard", Mob: 9112, Topic: "boast",
		Conditions: quests.Conditions{Has: []string{"53-start"}},
		Actions:    []quests.ActionDef{{SetFlag: &quests.QuestFlagAction{Key: "53-boasted", Value: "yes"}}},
	}}
	w.Dialogue[0].Topics = append(w.Dialogue[0].Topics, dialogue.Topic{Key: "boast", Kind: dialogue.KindInfo,
		Label: "Boast", Text: "t", Gates: dialogue.Gates{QuestRequired: []string{"53-start"}},
		Effects: dialogue.Effects{GrantsQuest: "53-firstkill"}})
	w.Dialogue[0].Topics[1].QuestFlagRequired = map[string]string{"53-boasted": "yes"}
	require.Empty(t, Walk(w, questloop.Sources{}))

	// The topic Way's own firing raised it: the flag is set and the
	// queued grant landed after it.
	eng := NewEngine()
	eng.RegisterQuest(w.Quests[0])
	p := newWalkPlayer(stepQuests(map[int][]string{53: {"start", "firstkill", "end"}}))
	p.grant("53-start")
	boast := questloop.WaysInto(w)["53-firstkill"][0]
	require.NotNil(t, boast.Topic)
	trail := fireWay(&walkEnv{eng: eng}, p, boast, 53)
	require.Equal(t, "firstkill", p.progress[53])
	require.True(t, trail["53-firstkill"], "%v", trail)
	require.Equal(t, "yes", p.flags["53-boasted"], "fireTopic raised topic_heard")
}

// conversation.applyWithHandover raises topic_heard before the topic's own grant lands
// (the game queues a dialogue grant), so a trigger on the offer that needs
// the offer's own token never fires: its step is a dead end. The offer hides
// once taken (questExcluded on its own start, as shipped offers do), so it
// cannot be replayed from start either.
func TestWalk_TopicHeardSeesTheStateBeforeTheTopicsGrant(t *testing.T) {
	w := walkWorld()
	w.Dialogue[0].Topics[0].QuestExcluded = []string{"53-start"}
	w.Quests[0].Triggers = []quests.TriggerDef{{
		Event: "topic_heard", Mob: 9112, Topic: "cull_offer",
		Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-firstkill"}},
		Actions:    []quests.ActionDef{{Grant: "53-firstkill"}},
	}}
	require.Contains(t, keys(Walk(w, questloop.Sources{})), "walk-dead-end 53 step firstkill")
}

// The topic that sets the route also moves the quest (its topic_heard
// trigger grants asked), so it is no grantless flag setter: a kill gated only
// on start and the route cannot use it to jump over asked, since hearing the
// topic passes through asked. The flag search must see that.
func TestWalk_TopicHeardTopicIsNoGrantlessFlagSetter(t *testing.T) {
	w := topicWorld()
	w.Quests[0].Triggers[1].Conditions.Has = []string{"53-start"}
	require.Empty(t, Walk(w, questloop.Sources{}))

	// Proof the probe can fail: a second, grantless topic that sets the
	// same route does let the kill jump over asked.
	w.Dialogue[1].Topics = append(w.Dialogue[1].Topics, dialogue.Topic{Key: "maps", Kind: dialogue.KindInfo,
		Label: "Maps", Text: "t", Gates: dialogue.Gates{QuestRequired: []string{"53-start"}},
		Effects: dialogue.Effects{SetsQuestFlag: &dialogue.QuestFlagSet{Key: "53-route", Value: "north"}}})
	require.Contains(t, keys(Walk(w, questloop.Sources{})), "walk-dead-end 53 quest 53 trigger mob_death mob=9110 grants=53-went")
}

// handoffWorld is topicWorld with went reached by hearing Nell's "handoff",
// which takes item 900, and rumours handing 900 over. handoff is gated only
// on start, as the shipped hand-off topics are gated on the step before.
func handoffWorld() questloop.World {
	w := topicWorld()
	q := w.Quests[0]
	q.Triggers[1] = quests.TriggerDef{Event: "topic_heard", Mob: 9200, Topic: "handoff",
		Conditions: quests.Conditions{Has: []string{"53-start"}, Missing: []string{"53-went"}},
		Actions:    []quests.ActionDef{{Grant: "53-went"}}}
	nell := w.Dialogue[1]
	nell.Topics[0].GivesItem = 900
	nell.Topics = append(nell.Topics, dialogue.Topic{Key: "handoff", Kind: dialogue.KindInfo, Label: "Handoff", Text: "t",
		Gates: dialogue.Gates{QuestRequired: []string{"53-start"}, RequiresItem: 900}})
	return w
}

// A topic_heard trigger's topic needs its item: the skip probe fires it early
// only when the item can be held early, as it does a topic Way.
func TestWalk_TopicHeardTopicItemGatesTheSkipProbe(t *testing.T) {
	require.Empty(t, Walk(handoffWorld(), outside()), "900 comes only from rumours: no skip")

	got := Walk(handoffWorld(), outside(900))
	require.Contains(t, keys(got), "walk-dead-end 53 quest 53 trigger topic_heard mob=9200 topic=handoff grants=53-went")
}

// forkedWalkWorld is quest 11's shape (#958) with branch: on its four middle
// steps. Kael (9300) offers quest 54; hearing Sylara's (9301) or Rhett's
// (9302) stand topic sets the branch flag and, through a topic_heard trigger,
// grants that side's fetch step; picking up the side's item grants its return
// step; each NPC takes the hand-in of their own side's return step.
func forkedWalkWorld() questloop.World {
	q := &quests.Quest{
		QuestId: 54, Name: "Forked",
		Flags: []quests.QuestFlagDef{{Key: "branch", Values: []string{"sylara", "rhett"}}},
		Steps: []quests.QuestStep{
			{Id: "start", Hint: "h"},
			{Id: "sylara", Branch: "sylara", Hint: "h"}, {Id: "totem", Branch: "sylara", Hint: "h"},
			{Id: "rhett", Branch: "rhett", Hint: "h"}, {Id: "samples", Branch: "rhett", Hint: "h"},
			{Id: "end"},
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
	side := func(mob int, value, need string) *dialogue.DialogueFile {
		return &dialogue.DialogueFile{MobId: mob, Topics: []dialogue.Topic{
			{Key: "stand", Kind: dialogue.KindInfo, Label: "Stand", Text: "t",
				Gates:   dialogue.Gates{QuestRequired: []string{"54-start"}, QuestExcluded: []string{"54-end"}},
				Effects: dialogue.Effects{SetsQuestFlag: &dialogue.QuestFlagSet{Key: "54-branch", Value: value}}},
			{Key: "hand_in", Kind: dialogue.KindHandIn, Quest: 54, Label: "Forked", Text: "t",
				Gates: dialogue.Gates{QuestRequired: []string{need}}, Effects: dialogue.Effects{GrantsQuest: "54-end"}},
		}}
	}
	return questloop.World{Quests: []*quests.Quest{q}, Dialogue: []*dialogue.DialogueFile{
		kael, side(9301, "sylara", "54-totem"), side(9302, "rhett", "54-samples")}}
}

// findingOf is the finding with key, or a zero Finding.
func findingOf(fs []questloop.Finding, key string) questloop.Finding {
	for _, f := range fs {
		if f.Key() == key {
			return f
		}
	}
	return questloop.Finding{}
}

// Each path is walked on its own: Rhett's hand-in is not a dead end on
// Sylara's path, nor Sylara's steps on Rhett's.
func TestWalk_ForkedQuestFinishesOnEachPath(t *testing.T) {
	require.Empty(t, Walk(forkedWalkWorld(), questloop.Sources{}))
}

func TestWalk_DeadBranchIsReportedOnItsPath(t *testing.T) {
	w := forkedWalkWorld()
	w.Quests[0].Triggers = slices.Delete(w.Quests[0].Triggers, 3, 4) // no samples pickup
	fs := Walk(w, questloop.Sources{})
	require.Equal(t, []string{"walk-dead-end 54 step samples"}, keys(fs))
	require.Contains(t, fs[0].Msg, "rhett path", "the path is named in the message")
}

// A side whose flag is set only after its grant, or set to the other side,
// is reported dead on its path, as it is dead in play. The trigger engine's
// off-path gate (triggerOpen) is what refuses these grants first; this test
// proves the walk surfaces that refusal as a finding on the right path, and
// that the walk's search for a flag setter (meetFlag) does not rescue it,
// since the only setter is the Way that grants the step. The walk player's
// own path rule is TestWalkPlayer_FollowsThePath's.
func TestWalk_FlagSetAfterTheGrantIsReported(t *testing.T) {
	// The trigger grants, then sets the flag.
	w := forkedWalkWorld()
	w.Dialogue[1].Topics[0].SetsQuestFlag = nil
	w.Quests[0].Triggers[0].Actions = []quests.ActionDef{{Grant: "54-sylara"},
		{SetFlag: &quests.QuestFlagAction{Key: "54-branch", Value: "sylara"}}}
	fs := Walk(w, questloop.Sources{})
	require.Contains(t, keys(fs), "walk-dead-end 54 step sylara")
	require.Contains(t, findingOf(fs, "walk-dead-end 54 step sylara").Msg, "sylara path")

	// The topic sets the wrong side.
	w = forkedWalkWorld()
	w.Dialogue[2].Topics[0].SetsQuestFlag.Value = "sylara"
	require.Contains(t, keys(Walk(w, questloop.Sources{})), "walk-dead-end 54 step rhett")
}

// A side whose flag a separate topic sets (Rhett's "choose") is playable: the
// player chooses, then hears the stand topic whose trigger grants the side's
// step. The walk takes the flag from that setter, as it does for any flag a
// gate needs (meetFlag).
func TestWalk_SideFlagFromASeparateSetter(t *testing.T) {
	w := forkedWalkWorld()
	rhett := w.Dialogue[2]
	rhett.Topics[0].SetsQuestFlag = nil
	rhett.Topics = append(rhett.Topics, dialogue.Topic{Key: "choose", Kind: dialogue.KindInfo, Label: "Choose", Text: "t",
		Gates:   dialogue.Gates{QuestRequired: []string{"54-start"}, QuestExcluded: []string{"54-end"}},
		Effects: dialogue.Effects{SetsQuestFlag: &dialogue.QuestFlagSet{Key: "54-branch", Value: "rhett"}}})
	require.Empty(t, Walk(w, questloop.Sources{}))
}

// One defect found on both paths is one finding, under its one key, whose
// message names both paths.
func TestWalk_SameFindingOnTwoPathsNamesBoth(t *testing.T) {
	w := forkedWalkWorld()
	for _, df := range w.Dialogue[1:] {
		df.Topics[1].QuestExcluded = append(df.Topics[1].QuestExcluded, df.Topics[1].QuestRequired...)
	}
	fs := Walk(w, questloop.Sources{})
	f := findingOf(fs, "walk-dead-end 54 step end")
	require.Equal(t, "no way in reaches 54-end (on the sylara and rhett paths)", f.Msg)
}

// Two findings under one key whose texts differ are not merged: a skip from
// start passes a different step on each path, and the kept message names
// only the path its text is true of.
func TestWalk_DifferentTextsOnTwoPathsAreNotMerged(t *testing.T) {
	w := forkedWalkWorld()
	w.Quests[0].Triggers = append(w.Quests[0].Triggers, quests.TriggerDef{Event: "item_gain", Item: 903,
		Conditions: quests.Conditions{Has: []string{"54-start"}},
		Actions:    []quests.ActionDef{{Grant: "54-end"}}})
	f := findingOf(Walk(w, questloop.Sources{}), "walk-dead-end 54 quest 54 trigger item_gain item=903 grants=54-end")
	require.Equal(t, "skips to 54-end from 54-start, passing 54-sylara (on the sylara path)", f.Msg)
}

// A path's walk ends with the branch flag on its own side.
func TestWalk_PathMustEndOnItsSide(t *testing.T) {
	w := forkedWalkWorld()
	w.Quests[0].Triggers[1].Actions = append(w.Quests[0].Triggers[1].Actions,
		quests.ActionDef{SetFlag: &quests.QuestFlagAction{Key: "54-branch", Value: "rhett"}})
	fs := Walk(w, questloop.Sources{})
	require.Equal(t, []string{"walk-branch-flag 54 step end"}, keys(fs))
	require.Contains(t, fs[0].Msg, "sylara path")
}

// A topic_heard trigger is source-checked for its topic's NPC and items.
func TestWalk_TopicHeardTriggerIsSourceChecked(t *testing.T) {
	got := keys(Walk(handoffWorld(), srcWith(nil, map[int]bool{9112: true})))
	require.Contains(t, got, "walk-unspawned 53 quest 53 trigger topic_heard mob=9200 topic=handoff grants=53-went")
	require.Contains(t, got, "walk-no-source 53 quest 53 trigger topic_heard mob=9200 topic=handoff grants=53-went")
	require.Contains(t, got, "walk-unspawned 53 quest 53 trigger topic_heard mob=9200 topic=rumours grants=53-asked")
}
