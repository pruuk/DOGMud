package questengine

import (
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
	p := newWalkPlayer(map[int][]string{53: {"start", "asked", "went", "end"}})
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
	p := newWalkPlayer(map[int][]string{53: {"start", "firstkill", "end"}})
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

// A topic_heard trigger is source-checked for its topic's NPC and items.
func TestWalk_TopicHeardTriggerIsSourceChecked(t *testing.T) {
	got := keys(Walk(handoffWorld(), srcWith(nil, map[int]bool{9112: true})))
	require.Contains(t, got, "walk-unspawned 53 quest 53 trigger topic_heard mob=9200 topic=handoff grants=53-went")
	require.Contains(t, got, "walk-no-source 53 quest 53 trigger topic_heard mob=9200 topic=handoff grants=53-went")
	require.Contains(t, got, "walk-unspawned 53 quest 53 trigger topic_heard mob=9200 topic=rumours grants=53-asked")
}
